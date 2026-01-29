// Package registry provides mDNS hostname registration for services.
// Services are advertised via avahi-publish-address so they can be accessed
// via friendly URLs like http://myapp.campus.local
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// MDNSService represents a service registered with mDNS hostname
type MDNSService struct {
	Name         string    `json:"name"`        // Service name (becomes hostname prefix)
	Port         int       `json:"port"`        // Port the service runs on
	IP           string    `json:"ip"`          // IP address
	Description  string    `json:"description"` // Optional description
	Hostname     string    `json:"hostname"`    // Full hostname (e.g., myapp.campus.local)
	URL          string    `json:"url"`         // Full URL
	RegisteredAt time.Time `json:"registered_at"`
	LastHealthy  time.Time `json:"last_healthy"`
	Healthy      bool      `json:"healthy"`
	PID          int       `json:"pid"` // avahi-publish-address PID
}

// MDNSRegistry manages services registered with mDNS hostnames
type MDNSRegistry struct {
	services  map[string]*MDNSService
	domain    string
	dataDir   string
	logger    *slog.Logger
	mu        sync.RWMutex
	processes map[string]*exec.Cmd
}

// MDNSRegistryConfig configures the mDNS registry
type MDNSRegistryConfig struct {
	Domain  string // Base domain (e.g., "campus" → *.campus.local)
	DataDir string // Directory to persist services
	Logger  *slog.Logger
}

// MDNSRegisterOptions configures service registration
type MDNSRegisterOptions struct {
	IP          string // IP address (auto-detected if empty)
	Interface   string // Network interface for IP detection
	Description string // Optional description
	HealthPath  string // Health check endpoint
}

// NewMDNSRegistry creates a new mDNS service registry
func NewMDNSRegistry(cfg MDNSRegistryConfig) *MDNSRegistry {
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	return &MDNSRegistry{
		services:  make(map[string]*MDNSService),
		domain:    cfg.Domain,
		dataDir:   cfg.DataDir,
		logger:    cfg.Logger,
		processes: make(map[string]*exec.Cmd),
	}
}

// Register registers a service and advertises it via mDNS
func (r *MDNSRegistry) Register(name string, port int, opts MDNSRegisterOptions) (*MDNSService, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate name
	if err := validateMDNSName(name); err != nil {
		return nil, fmt.Errorf("invalid service name: %w", err)
	}

	// Check if already registered
	if _, exists := r.services[name]; exists {
		return nil, fmt.Errorf("service %q is already registered", name)
	}

	// Get IP address
	ip := opts.IP
	if ip == "" {
		detectedIP, err := detectIP(opts.Interface)
		if err != nil {
			return nil, fmt.Errorf("failed to detect IP: %w", err)
		}
		ip = detectedIP
	}

	// Build hostname and URL
	hostname := fmt.Sprintf("%s.%s.local", name, r.domain)
	url := fmt.Sprintf("http://%s", hostname)
	if port != 80 {
		url = fmt.Sprintf("%s:%d", url, port)
	}

	// Check avahi-publish-address availability
	avahiPath, err := exec.LookPath("avahi-publish-address")
	if err != nil {
		return nil, fmt.Errorf("avahi-publish-address not found. Install: sudo apt install avahi-utils")
	}

	// Start avahi-publish-address
	cmd := exec.Command(avahiPath, "-R", hostname, ip)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start mDNS advertisement: %w", err)
	}

	r.processes[name] = cmd

	// Create service record
	svc := &MDNSService{
		Name:         name,
		Port:         port,
		IP:           ip,
		Description:  opts.Description,
		Hostname:     hostname,
		URL:          url,
		RegisteredAt: time.Now(),
		Healthy:      true,
		LastHealthy:  time.Now(),
		PID:          cmd.Process.Pid,
	}

	r.services[name] = svc

	// Persist to disk
	if err := r.save(); err != nil {
		r.logger.Warn("failed to persist service", "name", name, "error", err)
	}

	r.logger.Info("service registered via mDNS",
		"name", name,
		"url", url,
		"ip", ip,
		"port", port,
	)

	return svc, nil
}

// Unregister removes a service and stops its mDNS advertisement
func (r *MDNSRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	svc, exists := r.services[name]
	if !exists {
		return fmt.Errorf("service %q is not registered", name)
	}

	// Stop avahi process
	if cmd, ok := r.processes[name]; ok && cmd.Process != nil {
		if err := cmd.Process.Kill(); err != nil {
			r.logger.Warn("failed to kill avahi process", "name", name, "error", err)
		}
		cmd.Wait()
		delete(r.processes, name)
	}

	delete(r.services, name)

	// Persist
	if err := r.save(); err != nil {
		r.logger.Warn("failed to persist after unregister", "error", err)
	}

	r.logger.Info("service unregistered", "name", name, "was_url", svc.URL)
	return nil
}

// List returns all registered services
func (r *MDNSRegistry) List() []*MDNSService {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*MDNSService, 0, len(r.services))
	for _, svc := range r.services {
		result = append(result, svc)
	}
	return result
}

// Get returns a specific service
func (r *MDNSRegistry) Get(name string) (*MDNSService, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	svc, ok := r.services[name]
	return svc, ok
}

// CheckHealth performs TCP health check
func (r *MDNSRegistry) CheckHealth(ctx context.Context, name string) (bool, error) {
	r.mu.RLock()
	svc, exists := r.services[name]
	r.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("service %q not found", name)
	}

	target := fmt.Sprintf("%s:%d", svc.IP, svc.Port)
	conn, err := net.DialTimeout("tcp", target, 5*time.Second)
	if err != nil {
		r.mu.Lock()
		svc.Healthy = false
		r.mu.Unlock()
		return false, nil
	}
	conn.Close()

	r.mu.Lock()
	svc.Healthy = true
	svc.LastHealthy = time.Now()
	r.mu.Unlock()

	return true, nil
}

// CheckHealthHTTP performs HTTP health check
func (r *MDNSRegistry) CheckHealthHTTP(ctx context.Context, name string, path string) (bool, error) {
	r.mu.RLock()
	svc, exists := r.services[name]
	r.mu.RUnlock()

	if !exists {
		return false, fmt.Errorf("service %q not found", name)
	}

	url := fmt.Sprintf("http://%s:%d%s", svc.IP, svc.Port, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		r.mu.Lock()
		svc.Healthy = false
		r.mu.Unlock()
		return false, nil
	}
	defer resp.Body.Close()

	healthy := resp.StatusCode >= 200 && resp.StatusCode < 400

	r.mu.Lock()
	svc.Healthy = healthy
	if healthy {
		svc.LastHealthy = time.Now()
	}
	r.mu.Unlock()

	return healthy, nil
}

// Stop stops all mDNS advertisements
func (r *MDNSRegistry) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for name, cmd := range r.processes {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
		r.logger.Info("stopped mDNS advertisement", "name", name)
	}
	r.processes = make(map[string]*exec.Cmd)
}

// Load loads persisted services from disk
func (r *MDNSRegistry) Load() error {
	path := filepath.Join(r.dataDir, "mdns_services.json")

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("reading services file: %w", err)
	}

	var services []*MDNSService
	if err := json.Unmarshal(data, &services); err != nil {
		return fmt.Errorf("parsing services file: %w", err)
	}

	r.mu.Lock()
	for _, svc := range services {
		r.services[svc.Name] = svc
	}
	r.mu.Unlock()

	r.logger.Info("loaded persisted mDNS services", "count", len(services))
	return nil
}

// save persists services to disk
func (r *MDNSRegistry) save() error {
	path := filepath.Join(r.dataDir, "mdns_services.json")

	if err := os.MkdirAll(r.dataDir, 0755); err != nil {
		return fmt.Errorf("creating data dir: %w", err)
	}

	services := make([]*MDNSService, 0, len(r.services))
	for _, svc := range r.services {
		services = append(services, svc)
	}

	data, err := json.MarshalIndent(services, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling services: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing services file: %w", err)
	}

	return nil
}

// validateMDNSName checks if name is valid for mDNS hostname
func validateMDNSName(name string) error {
	if name == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if len(name) > 63 {
		return fmt.Errorf("name too long (max 63 characters)")
	}

	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-') {
			return fmt.Errorf("invalid character %q (only alphanumeric and hyphen allowed)", c)
		}
	}

	if name[0] == '-' || name[len(name)-1] == '-' {
		return fmt.Errorf("name cannot start or end with hyphen")
	}

	return nil
}

// detectIP gets the local IP, optionally from a specific interface
func detectIP(ifaceName string) (string, error) {
	var ifaces []net.Interface
	var err error

	if ifaceName != "" {
		iface, err := net.InterfaceByName(ifaceName)
		if err != nil {
			return "", fmt.Errorf("interface %q not found: %w", ifaceName, err)
		}
		ifaces = []net.Interface{*iface}
	} else {
		ifaces, err = net.Interfaces()
		if err != nil {
			return "", err
		}
	}

	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip != nil && ip.To4() != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
				return ip.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no suitable IP address found")
}
