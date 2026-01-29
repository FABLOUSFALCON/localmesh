// Package network provides network interface detection and selection.
package network

import (
	"fmt"
	"net"
	"strings"
)

// Interface represents a network interface with its details
type Interface struct {
	Name         string   // Interface name (e.g., "wlan0", "eth0")
	HardwareAddr string   // MAC address
	IPs          []string // IPv4 addresses
	IsUp         bool     // Interface is up
	IsLoopback   bool     // Is loopback interface
	IsWireless   bool     // Likely a WiFi interface
	Type         string   // "wifi", "ethernet", "loopback", "virtual", "unknown"
}

// ListInterfaces returns all available network interfaces with details
func ListInterfaces() ([]Interface, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to list interfaces: %w", err)
	}

	var result []Interface
	for _, iface := range ifaces {
		intf := Interface{
			Name:         iface.Name,
			HardwareAddr: iface.HardwareAddr.String(),
			IsUp:         iface.Flags&net.FlagUp != 0,
			IsLoopback:   iface.Flags&net.FlagLoopback != 0,
			IsWireless:   isWirelessInterface(iface.Name),
			Type:         classifyInterface(iface),
		}

		// Get IP addresses
		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				// Only include IPv4
				if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
					intf.IPs = append(intf.IPs, ip.String())
				}
			}
		}

		result = append(result, intf)
	}

	return result, nil
}

// ListUsableInterfaces returns only interfaces that can be used for LocalMesh
// (non-loopback, up, with at least one IPv4 address)
func ListUsableInterfaces() ([]Interface, error) {
	all, err := ListInterfaces()
	if err != nil {
		return nil, err
	}

	var usable []Interface
	for _, iface := range all {
		// Skip loopback
		if iface.IsLoopback {
			continue
		}
		// Must be up
		if !iface.IsUp {
			continue
		}
		// Must have at least one IP
		if len(iface.IPs) == 0 {
			continue
		}
		// Skip virtual/docker interfaces by default
		if iface.Type == "virtual" {
			continue
		}
		usable = append(usable, iface)
	}

	return usable, nil
}

// GetInterfaceByName returns interface details by name
func GetInterfaceByName(name string) (*Interface, error) {
	all, err := ListInterfaces()
	if err != nil {
		return nil, err
	}

	for _, iface := range all {
		if iface.Name == name {
			return &iface, nil
		}
	}

	return nil, fmt.Errorf("interface %q not found", name)
}

// GetPrimaryInterface returns the best interface to use
// Prefers WiFi over ethernet, skips loopback and virtual
func GetPrimaryInterface() (*Interface, error) {
	usable, err := ListUsableInterfaces()
	if err != nil {
		return nil, err
	}

	if len(usable) == 0 {
		return nil, fmt.Errorf("no usable network interfaces found")
	}

	// Prefer WiFi
	for _, iface := range usable {
		if iface.IsWireless || iface.Type == "wifi" {
			return &iface, nil
		}
	}

	// Fall back to first usable
	return &usable[0], nil
}

// isWirelessInterface checks if interface is likely WiFi
func isWirelessInterface(name string) bool {
	name = strings.ToLower(name)
	// Common wireless interface prefixes
	wirelessPrefixes := []string{"wlan", "wlp", "wifi", "ath", "wl", "en0", "en1"} // en0/en1 common on macOS
	for _, prefix := range wirelessPrefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// classifyInterface determines the interface type
func classifyInterface(iface net.Interface) string {
	name := strings.ToLower(iface.Name)

	// Loopback
	if iface.Flags&net.FlagLoopback != 0 {
		return "loopback"
	}

	// Virtual/container interfaces
	virtualPrefixes := []string{
		"docker", "br-", "veth", "virbr", "vbox", "vmnet",
		"tun", "tap", "lxc", "lxd", "cni", "flannel", "calico",
	}
	for _, prefix := range virtualPrefixes {
		if strings.HasPrefix(name, prefix) {
			return "virtual"
		}
	}

	// WiFi
	if isWirelessInterface(name) {
		return "wifi"
	}

	// Ethernet
	ethernetPrefixes := []string{"eth", "enp", "eno", "ens", "em"}
	for _, prefix := range ethernetPrefixes {
		if strings.HasPrefix(name, prefix) {
			return "ethernet"
		}
	}

	return "unknown"
}

// ValidateInterfaces checks if the given interface names are valid and usable
func ValidateInterfaces(names []string) error {
	for _, name := range names {
		iface, err := GetInterfaceByName(name)
		if err != nil {
			return fmt.Errorf("interface %q: %w", name, err)
		}
		if !iface.IsUp {
			return fmt.Errorf("interface %q is down", name)
		}
		if len(iface.IPs) == 0 {
			return fmt.Errorf("interface %q has no IP addresses", name)
		}
	}
	return nil
}

// FormatInterfaceInfo returns a formatted string for an interface
func (i *Interface) FormatInterfaceInfo() string {
	typeIcon := map[string]string{
		"wifi":     "📶",
		"ethernet": "🔌",
		"loopback": "🔄",
		"virtual":  "🐳",
		"unknown":  "❓",
	}

	icon := typeIcon[i.Type]
	if icon == "" {
		icon = "❓"
	}

	status := "⬇️ down"
	if i.IsUp {
		status = "✅ up"
	}

	ips := "no IP"
	if len(i.IPs) > 0 {
		ips = strings.Join(i.IPs, ", ")
	}

	return fmt.Sprintf("%s %-10s %s [%s]", icon, i.Name, ips, status)
}
