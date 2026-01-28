// Package mesh handles network discovery and mesh communication.
// Uses mDNS for zero-config service discovery on local networks.
package mesh

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/mdns"
)

// mdnsLogFilter filters out only noisy mDNS IPv6 warnings while keeping useful logs
type mdnsLogFilter struct {
	original io.Writer
}

func (f *mdnsLogFilter) Write(p []byte) (n int, err error) {
	msg := string(p)
	// Only filter out specific mDNS IPv6 warnings - keep all other logs
	if strings.Contains(msg, "Failed to listen to both unicast and multicast on IPv6") ||
		strings.Contains(msg, "mdns: Closing client") {
		return len(p), nil // Silently discard these specific messages
	}
	return f.original.Write(p)
}

func init() {
	// Install log filter for hashicorp/mdns library
	log.SetOutput(&mdnsLogFilter{original: os.Stderr})
}

// Service represents a discovered service on the mesh
type Service struct {
	ID        string
	Name      string
	Host      string
	Port      int
	Zone      string
	NodeID    string
	Version   string
	Addresses []net.IP
	Info      map[string]string
	LastSeen  time.Time
}

// Node represents a discovered node on the mesh
type Node struct {
	ID        string
	Name      string
	Host      string
	Port      int
	Zone      string
	Role      string
	Addresses []net.IP
	Services  []string
	LastSeen  time.Time
}

// Discovery handles mDNS-based service discovery
type Discovery struct {
	nodeID      string
	nodeName    string
	zone        string
	port        int
	serviceName string
	domain      string

	// mDNS server (for advertising)
	server *mdns.Server

	// Discovered services and nodes
	services map[string]*Service
	nodes    map[string]*Node
	mu       sync.RWMutex

	// Callbacks
	onServiceFound func(*Service)
	onServiceLost  func(*Service)
	onNodeFound    func(*Node)
	onNodeLost     func(*Node)

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *slog.Logger
}

// DiscoveryConfig configures the discovery service
type DiscoveryConfig struct {
	NodeID            string
	NodeName          string
	Zone              string
	Port              int
	ServiceName       string        // e.g., "_localmesh._tcp"
	Domain            string        // e.g., "local."
	DiscoveryInterval time.Duration // How often to scan
	TTL               time.Duration // mDNS record TTL
	Logger            *slog.Logger
}

// DefaultDiscoveryConfig returns sensible defaults
func DefaultDiscoveryConfig() DiscoveryConfig {
	hostname, _ := os.Hostname()
	return DiscoveryConfig{
		NodeID:            uuid.New().String(),
		NodeName:          hostname,
		Zone:              "default",
		Port:              8420,
		ServiceName:       "_localmesh._tcp",
		Domain:            "local.",
		DiscoveryInterval: 30 * time.Second,
		TTL:               60 * time.Second,
	}
}

// NewDiscovery creates a new mDNS discovery service
func NewDiscovery(cfg DiscoveryConfig) *Discovery {
	ctx, cancel := context.WithCancel(context.Background())

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Discovery{
		nodeID:      cfg.NodeID,
		nodeName:    cfg.NodeName,
		zone:        cfg.Zone,
		port:        cfg.Port,
		serviceName: cfg.ServiceName,
		domain:      cfg.Domain,
		services:    make(map[string]*Service),
		nodes:       make(map[string]*Node),
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
	}
}

// OnServiceFound sets callback for when a service is discovered
func (d *Discovery) OnServiceFound(fn func(*Service)) {
	d.onServiceFound = fn
}

// OnServiceLost sets callback for when a service is lost
func (d *Discovery) OnServiceLost(fn func(*Service)) {
	d.onServiceLost = fn
}

// OnNodeFound sets callback for when a node is discovered
func (d *Discovery) OnNodeFound(fn func(*Node)) {
	d.onNodeFound = fn
}

// OnNodeLost sets callback for when a node is lost
func (d *Discovery) OnNodeLost(fn func(*Node)) {
	d.onNodeLost = fn
}

// Start begins advertising and discovering services
func (d *Discovery) Start() error {
	// Start advertising this node
	if err := d.startAdvertising(); err != nil {
		return fmt.Errorf("starting mDNS advertising: %w", err)
	}

	// Start discovery loop
	d.wg.Add(1)
	go d.discoveryLoop()

	d.logger.Info("mDNS discovery started",
		"node_id", d.nodeID,
		"zone", d.zone,
		"port", d.port,
	)

	return nil
}

// startAdvertising starts the mDNS server to advertise this node
func (d *Discovery) startAdvertising() error {
	// Get local IPs
	ips, err := getLocalIPs()
	if err != nil {
		return fmt.Errorf("getting local IPs: %w", err)
	}

	// Build TXT records with node info
	txtRecords := []string{
		fmt.Sprintf("node_id=%s", d.nodeID),
		fmt.Sprintf("zone=%s", d.zone),
		fmt.Sprintf("version=1.0.0"),
		fmt.Sprintf("role=node"),
	}

	// Create mDNS service
	service, err := mdns.NewMDNSService(
		d.nodeName,    // Instance name
		d.serviceName, // Service type
		d.domain,      // Domain
		"",            // Host (empty = auto)
		d.port,        // Port
		ips,           // IPs to advertise
		txtRecords,    // TXT records
	)
	if err != nil {
		return fmt.Errorf("creating mDNS service: %w", err)
	}

	// Create and start server
	server, err := mdns.NewServer(&mdns.Config{
		Zone: service,
	})
	if err != nil {
		return fmt.Errorf("creating mDNS server: %w", err)
	}

	d.server = server
	return nil
}

// discoveryLoop periodically scans for other nodes
func (d *Discovery) discoveryLoop() {
	defer d.wg.Done()

	// Initial scan
	d.scan()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.scan()
			d.pruneStale()
		}
	}
}

// scan performs mDNS discovery
func (d *Discovery) scan() {
	// Channel to receive entries
	entriesCh := make(chan *mdns.ServiceEntry, 16)

	// Start lookup
	go func() {
		params := mdns.DefaultParams(d.serviceName)
		params.Domain = d.domain
		params.Entries = entriesCh
		params.Timeout = 5 * time.Second
		params.DisableIPv6 = true // Most campus networks are IPv4-only

		// Suppress mdns library logging by redirecting to our logger
		if err := mdns.Query(params); err != nil {
			// Only log actual errors, not IPv6 warnings
			if !strings.Contains(err.Error(), "IPv6") {
				d.logger.Warn("mDNS query failed", "error", err)
			}
		}
		close(entriesCh)
	}()

	// Process entries
	for entry := range entriesCh {
		d.handleEntry(entry)
	}
}

// handleEntry processes a discovered mDNS entry
func (d *Discovery) handleEntry(entry *mdns.ServiceEntry) {
	// Skip our own node
	if entry.Name == d.nodeName {
		return
	}

	// Parse TXT records
	info := parseTxtRecords(entry.InfoFields)

	nodeID := info["node_id"]
	if nodeID == "" {
		nodeID = entry.Name
	}

	zone := info["zone"]
	if zone == "" {
		zone = "default"
	}

	// Build addresses list
	var addrs []net.IP
	if entry.AddrV4 != nil {
		addrs = append(addrs, entry.AddrV4)
	}
	if entry.AddrV6 != nil {
		addrs = append(addrs, entry.AddrV6)
	}

	// Create/update node
	d.mu.Lock()
	node, exists := d.nodes[nodeID]
	if !exists {
		node = &Node{
			ID:        nodeID,
			Name:      entry.Name,
			Host:      entry.Host,
			Port:      entry.Port,
			Zone:      zone,
			Role:      info["role"],
			Addresses: addrs,
			Services:  []string{},
			LastSeen:  time.Now(),
		}
		d.nodes[nodeID] = node
		d.mu.Unlock()

		d.logger.Info("discovered node",
			"node_id", nodeID,
			"host", entry.Host,
			"zone", zone,
		)

		if d.onNodeFound != nil {
			d.onNodeFound(node)
		}
	} else {
		node.LastSeen = time.Now()
		node.Addresses = addrs
		d.mu.Unlock()
	}
}

// pruneStale removes nodes that haven't been seen recently
func (d *Discovery) pruneStale() {
	d.mu.Lock()
	defer d.mu.Unlock()

	threshold := time.Now().Add(-2 * time.Minute)

	for id, node := range d.nodes {
		if node.LastSeen.Before(threshold) {
			delete(d.nodes, id)

			d.logger.Info("node lost", "node_id", id)

			if d.onNodeLost != nil {
				d.onNodeLost(node)
			}
		}
	}

	for id, svc := range d.services {
		if svc.LastSeen.Before(threshold) {
			delete(d.services, id)

			if d.onServiceLost != nil {
				d.onServiceLost(svc)
			}
		}
	}
}

// RegisterService advertises a service on this node
func (d *Discovery) RegisterService(name, version string, port int, meta map[string]string) error {
	serviceID := fmt.Sprintf("%s-%s", d.nodeID, name)

	d.mu.Lock()
	d.services[serviceID] = &Service{
		ID:       serviceID,
		Name:     name,
		Host:     d.nodeName,
		Port:     port,
		Zone:     d.zone,
		NodeID:   d.nodeID,
		Version:  version,
		Info:     meta,
		LastSeen: time.Now(),
	}
	d.mu.Unlock()

	d.logger.Info("registered service",
		"service", name,
		"version", version,
		"port", port,
	)

	return nil
}

// GetNodes returns all discovered nodes
func (d *Discovery) GetNodes() []*Node {
	d.mu.RLock()
	defer d.mu.RUnlock()

	nodes := make([]*Node, 0, len(d.nodes))
	for _, n := range d.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// GetServices returns all discovered services
func (d *Discovery) GetServices() []*Service {
	d.mu.RLock()
	defer d.mu.RUnlock()

	services := make([]*Service, 0, len(d.services))
	for _, s := range d.services {
		services = append(services, s)
	}
	return services
}

// GetNodesByZone returns nodes in a specific zone
func (d *Discovery) GetNodesByZone(zone string) []*Node {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var nodes []*Node
	for _, n := range d.nodes {
		if n.Zone == zone {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// GetServiceByName finds a service by name (load balanced)
func (d *Discovery) GetServiceByName(name string) *Service {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Simple selection: return first matching service
	// TODO: Implement proper load balancing (round-robin, least-connections)
	for _, s := range d.services {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// NodeCount returns number of discovered nodes
func (d *Discovery) NodeCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.nodes)
}

// ServiceCount returns number of discovered services
func (d *Discovery) ServiceCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.services)
}

// Stop gracefully stops the discovery service
func (d *Discovery) Stop() error {
	d.cancel()
	d.wg.Wait()

	if d.server != nil {
		d.server.Shutdown()
	}

	d.logger.Info("mDNS discovery stopped")
	return nil
}

// Helper functions

func getLocalIPs() ([]net.IP, error) {
	var ips []net.IP

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range ifaces {
		// Skip loopback and down interfaces
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

			// Only IPv4 for now
			if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
				ips = append(ips, ip)
			}
		}
	}

	return ips, nil
}

func parseTxtRecords(fields []string) map[string]string {
	info := make(map[string]string)
	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) == 2 {
			info[parts[0]] = parts[1]
		}
	}
	return info
}
