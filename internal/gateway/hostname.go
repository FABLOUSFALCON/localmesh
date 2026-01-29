// Package gateway provides hostname advertising for the LocalMesh gateway.
// Advertises a .local hostname via Avahi/mDNS so devices can access gateway
// using a friendly URL like http://campus.local instead of IP:port
package gateway

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"strings"
)

// HostnameAdvertiser advertises the gateway with a .local hostname
// Uses avahi-publish-address which integrates with the system's mDNS
type HostnameAdvertiser struct {
	hostname string
	port     int
	logger   *slog.Logger
	ips      []net.IP
	cmd      *exec.Cmd
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
	// Default to "campus" (not "mesh" to avoid collision with mDNS service name)
	if hostname == "" || hostname == "localhost" {
		hostname = "campus"
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

// Start begins advertising the hostname via mDNS using avahi-publish-address
func (h *HostnameAdvertiser) Start() error {
	// Get local IP addresses
	ips, err := getLocalIPs()
	if err != nil {
		return fmt.Errorf("failed to get local IPs: %w", err)
	}

	if len(ips) == 0 {
		return fmt.Errorf("no local IP addresses found")
	}
	h.ips = ips

	// Use the first IP for the hostname
	ip := ips[0].String()
	fqdn := h.hostname + ".local"

	// Check if avahi-publish-address is available
	_, err = exec.LookPath("avahi-publish-address")
	if err != nil {
		h.logger.Warn("avahi-publish-address not found, mDNS hostname won't work",
			"install", "sudo pacman -S avahi OR sudo apt install avahi-utils")
		return nil // Don't fail, just warn
	}

	// Start avahi-publish-address in background
	// -R disables reverse lookup (PTR record) which can cause issues
	h.cmd = exec.Command("avahi-publish-address", "-R", fqdn, ip)
	h.cmd.Stdout = nil
	h.cmd.Stderr = nil

	if err := h.cmd.Start(); err != nil {
		h.logger.Warn("failed to start avahi-publish-address", "error", err)
		return nil // Don't fail, just warn
	}

	h.logger.Info("hostname advertised via mDNS (Avahi)",
		"hostname", fqdn,
		"port", h.port,
		"url", h.URL(),
		"ip", ip,
	)

	return nil
}

// Stop stops advertising the hostname
func (h *HostnameAdvertiser) Stop() {
	if h.cmd != nil && h.cmd.Process != nil {
		h.cmd.Process.Kill()
		h.cmd.Wait()
		h.cmd = nil
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

// IPs returns the advertised IP addresses
func (h *HostnameAdvertiser) IPs() []net.IP {
	return h.ips
}

// getLocalIPs returns all local non-loopback IPv4 addresses
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

		// Skip docker/virtual interfaces for cleaner mDNS
		name := strings.ToLower(iface.Name)
		if strings.HasPrefix(name, "docker") ||
			strings.HasPrefix(name, "veth") ||
			strings.HasPrefix(name, "br-") ||
			strings.HasPrefix(name, "virbr") {
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

			// Only use IPv4, skip loopback and link-local
			if ip == nil || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
				continue
			}

			// Only IPv4 for broader compatibility
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
