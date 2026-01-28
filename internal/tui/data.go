package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/FABLOUSFALCON/localmesh/internal/config"
	"github.com/FABLOUSFALCON/localmesh/internal/services"
)

// DataProvider fetches real data from LocalMesh components.
type DataProvider struct {
	registry *services.Registry
	config   *config.Config

	// Cached data
	services []ServiceItem
	logs     []LogEntry
	stats    SystemStats

	// Update callbacks
	onServiceUpdate func([]ServiceItem)
	onLogUpdate     func([]LogEntry)
	onStatsUpdate   func(SystemStats)

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc
}

// NewDataProvider creates a new data provider.
func NewDataProvider(registry *services.Registry, cfg *config.Config) *DataProvider {
	ctx, cancel := context.WithCancel(context.Background())

	return &DataProvider{
		registry: registry,
		config:   cfg,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// SetCallbacks sets the update callbacks.
func (p *DataProvider) SetCallbacks(
	onService func([]ServiceItem),
	onLog func([]LogEntry),
	onStats func(SystemStats),
) {
	p.onServiceUpdate = onService
	p.onLogUpdate = onLog
	p.onStatsUpdate = onStats
}

// Start begins polling for data updates.
func (p *DataProvider) Start() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return dataTickMsg(t)
	})
}

// Stop stops the data provider.
func (p *DataProvider) Stop() {
	p.cancel()
}

// Message types
type dataTickMsg time.Time
type serviceDataMsg []ServiceItem
type logDataMsg []LogEntry
type statsDataMsg SystemStats

// FetchServices gets current service data.
func (p *DataProvider) FetchServices() []ServiceItem {
	if p.registry == nil {
		return nil
	}

	svcs := p.registry.List()
	items := make([]ServiceItem, 0, len(svcs))

	for _, svc := range svcs {
		item := ServiceItem{
			ID:          svc.Info.Name,
			Name:        svc.Info.Name,
			Desc:        svc.Info.Description,
			URL:         svc.Endpoint.URL,
			Zone:        "",
			StatusStr:   serviceStateToString(svc.Health.State),
			Latency:     svc.Metrics.AvgLatency,
			RequestCnt:  svc.Metrics.TotalRequests,
			LastChecked: svc.Health.LastCheck,
		}

		if len(svc.Access.Zones) > 0 {
			item.Zone = svc.Access.Zones[0]
		}

		items = append(items, item)
	}

	p.services = items
	return items
}

// FetchStats gets current system stats.
func (p *DataProvider) FetchStats() SystemStats {
	if p.registry == nil {
		return SystemStats{}
	}

	stats := p.registry.Stats()

	p.stats = SystemStats{
		TotalRequests:  0, // Not tracked in RegistryStats
		ActiveServices: stats.Total - stats.Unknown,
		HealthyNodes:   stats.Healthy,
		TotalNodes:     stats.Total,
	}

	if p.config != nil {
		// Add network info from config if available
		p.stats.NetworkID = "localmesh-node"
		p.stats.Zone = "local"
	}

	return p.stats
}

// Update handles tick messages and fetches new data.
func (p *DataProvider) Update(msg tea.Msg) tea.Cmd {
	switch msg.(type) {
	case dataTickMsg:
		// Fetch updated data
		p.FetchServices()
		p.FetchStats()

		// Trigger callbacks
		if p.onServiceUpdate != nil {
			p.onServiceUpdate(p.services)
		}
		if p.onStatsUpdate != nil {
			p.onStatsUpdate(p.stats)
		}

		// Schedule next tick
		return p.Start()
	}
	return nil
}

// GetServices returns cached services.
func (p *DataProvider) GetServices() []ServiceItem {
	return p.services
}

// GetStats returns cached stats.
func (p *DataProvider) GetStats() SystemStats {
	return p.stats
}

// serviceStateToString converts service state to a string.
func serviceStateToString(state services.ServiceState) string {
	switch state {
	case services.StateHealthy:
		return "healthy"
	case services.StateDegraded:
		return "degraded"
	case services.StateUnhealthy:
		return "offline"
	default:
		return "unknown"
	}
}

// MockDataProvider provides sample data for testing.
type MockDataProvider struct {
	services []ServiceItem
	plugins  []PluginItem
	networks []NetworkItem
	logs     []LogEntry
	stats    SystemStats
}

// NewMockDataProvider creates a mock data provider with sample data.
func NewMockDataProvider() *MockDataProvider {
	return &MockDataProvider{
		services: []ServiceItem{
			{ID: "1", Name: "Attendance API", Desc: "Student attendance system", URL: "http://localhost:8081", Zone: "campus", StatusStr: "healthy", Latency: 45 * time.Millisecond, RequestCnt: 1234},
			{ID: "2", Name: "Library Portal", Desc: "Book management", URL: "http://localhost:8082", Zone: "library", StatusStr: "healthy", Latency: 32 * time.Millisecond, RequestCnt: 567},
			{ID: "3", Name: "Cafeteria Menu", Desc: "Daily menu & orders", URL: "http://localhost:8083", Zone: "campus", StatusStr: "degraded", Latency: 250 * time.Millisecond, RequestCnt: 89},
			{ID: "4", Name: "Lab Booking", Desc: "Reserve lab slots", URL: "http://localhost:8084", Zone: "labs", StatusStr: "offline", Latency: 0, RequestCnt: 45},
		},
		plugins: []PluginItem{
			{Name: "auth-ldap", Desc: "LDAP authentication provider", Version: "1.2.0", StatusStr: "running", Loaded: true},
			{Name: "metrics", Desc: "Prometheus metrics exporter", Version: "2.0.1", StatusStr: "running", Loaded: true},
			{Name: "rate-limiter", Desc: "Request rate limiting", Version: "1.0.0", StatusStr: "stopped", Loaded: false},
		},
		networks: []NetworkItem{
			{SSID: "Campus-Main", Zone: "campus", IP: "192.168.1.0/24", StatusStr: "online", Users: 234},
			{SSID: "Library-WiFi", Zone: "library", IP: "192.168.2.0/24", StatusStr: "online", Users: 45},
			{SSID: "Lab-Network", Zone: "labs", IP: "192.168.3.0/24", StatusStr: "online", Users: 12},
			{SSID: "Guest-WiFi", Zone: "guest", IP: "10.0.0.0/24", StatusStr: "degraded", Users: 89},
		},
		logs: []LogEntry{
			{Time: time.Now().Add(-5 * time.Minute), Level: "INFO", Source: "gateway", Message: "Started HTTP server on :8080"},
			{Time: time.Now().Add(-4 * time.Minute), Level: "INFO", Source: "mdns", Message: "Discovered service: attendance-api"},
			{Time: time.Now().Add(-3 * time.Minute), Level: "WARN", Source: "proxy", Message: "High latency detected for cafeteria-menu (250ms)"},
			{Time: time.Now().Add(-2 * time.Minute), Level: "ERROR", Source: "health", Message: "Service lab-booking failed health check"},
			{Time: time.Now().Add(-1 * time.Minute), Level: "INFO", Source: "auth", Message: "User student123 authenticated from campus zone"},
			{Time: time.Now(), Level: "DEBUG", Source: "registry", Message: "Service metrics updated"},
		},
		stats: SystemStats{
			Uptime:         2*time.Hour + 34*time.Minute,
			TotalRequests:  15789,
			ActiveServices: 3,
			HealthyNodes:   3,
			TotalNodes:     4,
			NetworkID:      "campus-mesh-001",
			Zone:           "campus",
		},
	}
}

// GetServices returns mock services.
func (m *MockDataProvider) GetServices() []ServiceItem {
	return m.services
}

// GetPlugins returns mock plugins.
func (m *MockDataProvider) GetPlugins() []PluginItem {
	return m.plugins
}

// GetNetworks returns mock networks.
func (m *MockDataProvider) GetNetworks() []NetworkItem {
	return m.networks
}

// GetLogs returns mock logs.
func (m *MockDataProvider) GetLogs() []LogEntry {
	return m.logs
}

// GetStats returns mock stats.
func (m *MockDataProvider) GetStats() SystemStats {
	return m.stats
}

// AddLog adds a mock log entry.
func (m *MockDataProvider) AddLog(level, source, message string) {
	m.logs = append([]LogEntry{{
		Time:    time.Now(),
		Level:   level,
		Source:  source,
		Message: message,
	}}, m.logs...)

	// Keep only recent logs
	if len(m.logs) > 100 {
		m.logs = m.logs[:100]
	}
}

// UpdateServiceStatus updates a mock service status.
func (m *MockDataProvider) UpdateServiceStatus(name, status string) {
	for i := range m.services {
		if m.services[i].Name == name {
			m.services[i].StatusStr = status
			return
		}
	}
}
