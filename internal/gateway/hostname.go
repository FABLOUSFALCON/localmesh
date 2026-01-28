// Package gateway provides hostname advertising for the LocalMesh gateway.
// Advertises a .local hostname via mDNS so devices can access gateway
// using a friendly URL like http://campus.local instead of IP:port
package gateway

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/grandcat/zeroconf"
)

// HostnameAdvertiser advertises the gateway with a .local hostname
type HostnameAdvertiser struct {
	hostname string
	port     int
	server   *zeroconf.Server
	logger   *slog.Logger
}

// HostnameConfig configures the hostname advertiser
type HostnameConfig struct {
	// Hostname is the .local name (without .local suffix)
	// e.g., "campus" becomes "campus.local"
	Hostname string

	// Port is the gateway port
	Port int

	// Logger for logging
	Logger *slog.Logger
}

// DefaultHostnameConfig returns sensible defaults
func DefaultHostnameConfig() HostnameConfig {
	hostname, _ := os.Hostname()
	// Clean hostname - remove any existing domain parts
	if idx := strings.Index(hostname, "."); idx > 0 {
		hostname = hostname[:idx]
	}
	// Default to "mesh" if hostname is empty or weird
	if hostname == "" || hostname == "localhost" {
		hostname = "mesh"
	}

	return HostnameConfig{
		Hostname: hostname,
		Port:     8080,
		Logger:   slog.Default(),
	}
}

// NewHostnameAdvertiser creates a new hostname advertiser
func NewHostnameAdvertiser(cfg HostnameConfig) *HostnameAdvertiser {
	return &HostnameAdvertiser{
		hostname: cfg.Hostname,
		port:     cfg.Port,
		logger:   cfg.Logger,
	}
}

// Start begins advertising the hostname via mDNS
func (h *HostnameAdvertiser) Start() error {
	// Get local IP addresses
	ips, err := getLocalIPs()
	if err != nil {
		return fmt.Errorf("failed to get local IPs: %w", err)
	}

	if len(ips) == 0 {
		return fmt.Errorf("no local IP addresses found")
	}

	// Register HTTP service with our hostname
	// Service type: _http._tcp allows browsers to discover us
	server, err := zeroconf.Register(
		h.hostname,   // Instance name (becomes hostname.local)
		"_http._tcp", // Service type
		"local.",     // Domain
		h.port,       // Port
		[]string{ // TXT records
			"path=/",
			"localmesh=gateway",
			"version=1.0.0",
		},
		nil, // All network interfaces
	)
	if err != nil {
		return fmt.Errorf("failed to register hostname: %w", err)
	}

	h.server = server

	h.logger.Info("hostname advertised",
		"hostname", fmt.Sprintf("%s.local", h.hostname),
		"port", h.port,
		"url", fmt.Sprintf("http://%s.local:%d", h.hostname, h.port),
		"ips", ipsToStrings(ips),
	)

	// Also log a nicer URL if port is 80
	if h.port == 80 {
		h.logger.Info("access gateway at",
			"url", fmt.Sprintf("http://%s.local", h.hostname),
		)
	}

	return nil
}

// Stop stops advertising the hostname
func (h *HostnameAdvertiser) Stop() {
	if h.server != nil {
		h.server.Shutdown()
		h.server = nil
		h.logger.Info("hostname advertisement stopped")
	}
}

// Hostname returns the advertised hostname
func (h *HostnameAdvertiser) Hostname() string {
	return h.hostname
}

// URL returns the full URL for accessing the gateway
func (h *HostnameAdvertiser) URL() string {
	if h.port == 80 {
		return fmt.Sprintf("http://%s.local", h.hostname)
	}
	return fmt.Sprintf("http://%s.local:%d", h.hostname, h.port)
}

// getLocalIPs returns all local non-loopback IP addresses
func getLocalIPs() ([]net.IP, error) {
	var ips []net.IP

	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range interfaces {
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

			// Skip loopback and IPv6 link-local
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}

			// Prefer IPv4
			if ip4 := ip.To4(); ip4 != nil {
				ips = append(ips, ip4)
			}
		}
	}

	return ips, nil
}

func ipsToStrings(ips []net.IP) []string {
	var strs []string
	for _, ip := range ips {
		strs = append(strs, ip.String())
	}
	return strs
}
