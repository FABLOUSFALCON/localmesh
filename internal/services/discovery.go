package services

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/rs/zerolog"
)

const (
	// LocalMesh service type for mDNS
	ServiceTypeMesh = "_localmesh._tcp"
	// Domain for local discovery
	LocalDomain = "local."
)

// Discovery handles automatic service discovery using mDNS.
// Services can advertise themselves on the network and be
// automatically discovered and registered.
type Discovery struct {
	registry *Registry
	logger   zerolog.Logger

	// mDNS server for advertising our services
	server *mdns.Server

	// Discovery state
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.RWMutex
	discovered map[string]*discoveredService

	// Configuration
	instanceName string
	port         int
	advertise    bool
}

// discoveredService tracks a service found via mDNS.
type discoveredService struct {
	Name      string
	Host      string
	Port      int
	Info      map[string]string
	LastSeen  time.Time
	Addresses []net.IP
}

// DiscoveryConfig configures the discovery system.
type DiscoveryConfig struct {
	InstanceName string        // Name to advertise (e.g., "localmesh-node-1")
	Port         int           // Port LocalMesh is running on
	Advertise    bool          // Whether to advertise services
	ScanInterval time.Duration // How often to scan for services
}

// NewDiscovery creates a new service discovery instance.
func NewDiscovery(registry *Registry, logger zerolog.Logger, config DiscoveryConfig) *Discovery {
	ctx, cancel := context.WithCancel(context.Background())

	return &Discovery{
		registry:     registry,
		logger:       logger.With().Str("component", "service-discovery").Logger(),
		ctx:          ctx,
		cancel:       cancel,
		discovered:   make(map[string]*discoveredService),
		instanceName: config.InstanceName,
		port:         config.Port,
		advertise:    config.Advertise,
	}
}

// Start begins service discovery and optional advertising.
func (d *Discovery) Start() error {
	d.logger.Info().
		Str("instance", d.instanceName).
		Int("port", d.port).
		Bool("advertise", d.advertise).
		Msg("Starting service discovery")

	// Start advertising if enabled
	if d.advertise {
		if err := d.startAdvertising(); err != nil {
			d.logger.Error().Err(err).Msg("Failed to start advertising")
			// Don't fail completely, just log the error
		}
	}

	// Start background discovery
	go d.discoveryLoop()

	return nil
}

// startAdvertising sets up mDNS advertising for LocalMesh.
func (d *Discovery) startAdvertising() error {
	// Get local IPs
	ips, err := getLocalIPs()
	if err != nil {
		return fmt.Errorf("failed to get local IPs: %w", err)
	}

	// Build service info
	info := []string{
		"version=1.0.0",
		"type=localmesh",
		fmt.Sprintf("instance=%s", d.instanceName),
	}

	// Add registered services to TXT record
	for _, svc := range d.registry.List() {
		info = append(info, fmt.Sprintf("svc=%s", svc.Info.Name))
	}

	// Create mDNS service
	service, err := mdns.NewMDNSService(
		d.instanceName,
		ServiceTypeMesh,
		LocalDomain,
		"",
		d.port,
		ips,
		info,
	)
	if err != nil {
		return fmt.Errorf("failed to create mDNS service: %w", err)
	}

	// Create mDNS server
	server, err := mdns.NewServer(&mdns.Config{
		Zone: service,
	})
	if err != nil {
		return fmt.Errorf("failed to start mDNS server: %w", err)
	}

	d.server = server

	d.logger.Info().
		Strs("ips", ipStrings(ips)).
		Int("port", d.port).
		Msg("Advertising LocalMesh via mDNS")

	return nil
}

// discoveryLoop periodically scans for LocalMesh services.
func (d *Discovery) discoveryLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial scan
	d.scan()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.scan()
			d.cleanupStale()
		}
	}
}

// scan performs an mDNS query for LocalMesh services.
func (d *Discovery) scan() {
	d.logger.Debug().Msg("Scanning for services")

	entriesCh := make(chan *mdns.ServiceEntry, 10)

	go func() {
		for entry := range entriesCh {
			d.handleEntry(entry)
		}
	}()

	// Query for LocalMesh services
	err := mdns.Query(&mdns.QueryParam{
		Service:             ServiceTypeMesh,
		Domain:              LocalDomain,
		Timeout:             5 * time.Second,
		Entries:             entriesCh,
		WantUnicastResponse: false,
	})

	close(entriesCh)

	if err != nil {
		d.logger.Error().Err(err).Msg("mDNS query failed")
	}
}

// handleEntry processes a discovered mDNS entry.
func (d *Discovery) handleEntry(entry *mdns.ServiceEntry) {
	d.logger.Debug().
		Str("name", entry.Name).
		Str("host", entry.Host).
		Int("port", entry.Port).
		Strs("info", entry.InfoFields).
		Msg("Discovered service")

	// Parse service info from TXT records
	info := make(map[string]string)
	for _, field := range entry.InfoFields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) == 2 {
			info[parts[0]] = parts[1]
		}
	}

	// Skip if it's us
	if info["instance"] == d.instanceName {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	// Track the discovered service
	discovered := &discoveredService{
		Name:      entry.Name,
		Host:      entry.Host,
		Port:      entry.Port,
		Info:      info,
		LastSeen:  time.Now(),
		Addresses: []net.IP{entry.Addr, entry.AddrV6},
	}

	d.discovered[entry.Name] = discovered

	// Process any advertised services
	for key, value := range info {
		if key == "svc" {
			d.tryRegisterDiscovered(value, discovered)
		}
	}
}

// tryRegisterDiscovered attempts to register a discovered service.
func (d *Discovery) tryRegisterDiscovered(name string, ds *discoveredService) {
	// Check if already registered
	if _, exists := d.registry.Get(name); exists {
		return
	}

	// Determine the best address to use
	var addr net.IP
	for _, ip := range ds.Addresses {
		if ip != nil && !ip.IsUnspecified() {
			addr = ip
			break
		}
	}

	if addr == nil {
		d.logger.Warn().Str("service", name).Msg("No valid address for discovered service")
		return
	}

	// Create service URL
	url := fmt.Sprintf("http://%s:%d", addr.String(), ds.Port)

	// Create and register the service
	svc := NewService(name, name, url)
	svc.Discovered = true
	svc.Info.Description = fmt.Sprintf("Auto-discovered from %s", ds.Host)

	if err := d.registry.Register(svc); err != nil {
		d.logger.Warn().Err(err).Str("service", name).Msg("Failed to register discovered service")
		return
	}

	d.logger.Info().
		Str("service", name).
		Str("url", url).
		Str("from", ds.Host).
		Msg("Registered discovered service")
}

// cleanupStale removes services that haven't been seen recently.
func (d *Discovery) cleanupStale() {
	d.mu.Lock()
	defer d.mu.Unlock()

	staleThreshold := 5 * time.Minute
	now := time.Now()

	for name, ds := range d.discovered {
		if now.Sub(ds.LastSeen) > staleThreshold {
			delete(d.discovered, name)

			// Unregister any services from this node
			for key := range ds.Info {
				if key == "svc" {
					svcName := ds.Info[key]
					if svc, exists := d.registry.Get(svcName); exists && svc.Discovered {
						d.registry.Unregister(svcName)
						d.logger.Info().
							Str("service", svcName).
							Msg("Removed stale discovered service")
					}
				}
			}
		}
	}
}

// AdvertiseService adds a service to the mDNS advertisement.
func (d *Discovery) AdvertiseService(name string) error {
	if d.server == nil {
		return fmt.Errorf("mDNS server not running")
	}

	// Restart advertising with updated service list
	d.server.Shutdown()
	return d.startAdvertising()
}

// Stop shuts down discovery.
func (d *Discovery) Stop() error {
	d.cancel()

	if d.server != nil {
		d.server.Shutdown()
	}

	d.logger.Info().Msg("Service discovery stopped")
	return nil
}

// ListDiscovered returns all discovered nodes.
func (d *Discovery) ListDiscovered() []DiscoveredNode {
	d.mu.RLock()
	defer d.mu.RUnlock()

	nodes := make([]DiscoveredNode, 0, len(d.discovered))
	for _, ds := range d.discovered {
		services := make([]string, 0)
		for key, value := range ds.Info {
			if key == "svc" {
				services = append(services, value)
			}
		}

		nodes = append(nodes, DiscoveredNode{
			Name:      ds.Name,
			Host:      ds.Host,
			Port:      ds.Port,
			Services:  services,
			LastSeen:  ds.LastSeen,
			Addresses: ipStrings(ds.Addresses),
		})
	}

	return nodes
}

// DiscoveredNode represents a discovered LocalMesh node.
type DiscoveredNode struct {
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      int       `json:"port"`
	Services  []string  `json:"services"`
	LastSeen  time.Time `json:"last_seen"`
	Addresses []string  `json:"addresses"`
}

// Helper functions

func getLocalIPs() ([]net.IP, error) {
	var ips []net.IP

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.IP)
				}
			}
		}
	}

	return ips, nil
}

func ipStrings(ips []net.IP) []string {
	result := make([]string, 0, len(ips))
	for _, ip := range ips {
		if ip != nil && !ip.IsUnspecified() {
			result = append(result, ip.String())
		}
	}
	return result
}
