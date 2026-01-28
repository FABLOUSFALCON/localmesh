// Package network provides network identity detection for LocalMesh.
// Detects WiFi SSID, network interfaces, and maps them to zones.
package network

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Identity represents the network identity of a client or node
type Identity struct {
	// Network interface info
	InterfaceName string   `json:"interface_name"`
	MacAddress    string   `json:"mac_address"`
	IPAddresses   []string `json:"ip_addresses"`

	// WiFi info (if applicable)
	SSID      string `json:"ssid,omitempty"`
	BSSID     string `json:"bssid,omitempty"` // Access point MAC
	Signal    int    `json:"signal,omitempty"`
	Frequency int    `json:"frequency,omitempty"`
	Channel   int    `json:"channel,omitempty"`
	Security  string `json:"security,omitempty"`

	// Derived zone
	Zone       string `json:"zone"`
	ZoneSource string `json:"zone_source"` // How zone was determined

	// Verification status
	Verified   bool      `json:"verified"`
	VerifiedAt time.Time `json:"verified_at,omitempty"`

	// Metadata
	DetectedAt time.Time `json:"detected_at"`
	Hostname   string    `json:"hostname"`
}

// ZoneMapping maps network identifiers to zones
type ZoneMapping struct {
	ID          string   `json:"id"`
	Zone        string   `json:"zone"`
	SSIDs       []string `json:"ssids,omitempty"`   // WiFi SSIDs
	Subnets     []string `json:"subnets,omitempty"` // CIDR ranges
	BSSIDs      []string `json:"bssids,omitempty"`  // Specific APs
	Description string   `json:"description,omitempty"`
	Priority    int      `json:"priority"` // Higher = checked first
}

// Detector handles network identity detection
type Detector struct {
	mappings []ZoneMapping
	cache    map[string]*Identity // IP -> Identity cache
	cacheTTL time.Duration
	mu       sync.RWMutex
}

// NewDetector creates a new network identity detector
func NewDetector() *Detector {
	return &Detector{
		mappings: make([]ZoneMapping, 0),
		cache:    make(map[string]*Identity),
		cacheTTL: 5 * time.Minute,
	}
}

// AddMapping adds a zone mapping
func (d *Detector) AddMapping(m ZoneMapping) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.mappings = append(d.mappings, m)
	// Sort by priority (higher first)
	for i := len(d.mappings) - 1; i > 0; i-- {
		if d.mappings[i].Priority > d.mappings[i-1].Priority {
			d.mappings[i], d.mappings[i-1] = d.mappings[i-1], d.mappings[i]
		}
	}
}

// DetectLocal detects the local node's network identity
func (d *Detector) DetectLocal(ctx context.Context) (*Identity, error) {
	hostname, _ := os.Hostname()

	identity := &Identity{
		DetectedAt: time.Now(),
		Hostname:   hostname,
	}

	// Get network interfaces
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("listing interfaces: %w", err)
	}

	// Find primary interface (first non-loopback with IP)
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil || len(addrs) == 0 {
			continue
		}

		identity.InterfaceName = iface.Name
		identity.MacAddress = iface.HardwareAddr.String()

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					identity.IPAddresses = append(identity.IPAddresses, ipnet.IP.String())
				}
			}
		}

		// Check if this is a WiFi interface
		if isWiFiInterface(iface.Name) {
			wifi, err := detectWiFi(ctx, iface.Name)
			if err == nil && wifi != nil {
				identity.SSID = wifi.SSID
				identity.BSSID = wifi.BSSID
				identity.Signal = wifi.Signal
				identity.Frequency = wifi.Frequency
				identity.Channel = wifi.Channel
				identity.Security = wifi.Security
			}
		}

		break // Use first valid interface
	}

	// Determine zone from identity
	identity.Zone, identity.ZoneSource = d.determineZone(identity)
	identity.Verified = true
	identity.VerifiedAt = time.Now()

	return identity, nil
}

// DetectFromIP detects network identity for a remote IP
func (d *Detector) DetectFromIP(ctx context.Context, ip string) (*Identity, error) {
	// Check cache first
	d.mu.RLock()
	if cached, ok := d.cache[ip]; ok {
		if time.Since(cached.DetectedAt) < d.cacheTTL {
			d.mu.RUnlock()
			return cached, nil
		}
	}
	d.mu.RUnlock()

	identity := &Identity{
		IPAddresses: []string{ip},
		DetectedAt:  time.Now(),
	}

	// Try to resolve hostname
	names, err := net.LookupAddr(ip)
	if err == nil && len(names) > 0 {
		identity.Hostname = strings.TrimSuffix(names[0], ".")
	}

	// Try to get MAC from ARP cache
	mac, err := lookupMAC(ip)
	if err == nil {
		identity.MacAddress = mac
	}

	// Determine zone from IP subnet
	identity.Zone, identity.ZoneSource = d.determineZone(identity)

	// Verify the identity (check if IP is in expected subnet)
	identity.Verified = d.verifyIdentity(identity)
	if identity.Verified {
		identity.VerifiedAt = time.Now()
	}

	// Cache the result
	d.mu.Lock()
	d.cache[ip] = identity
	d.mu.Unlock()

	return identity, nil
}

// determineZone finds the zone for an identity
func (d *Detector) determineZone(identity *Identity) (zone, source string) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Check SSID first (highest priority for WiFi)
	if identity.SSID != "" {
		for _, m := range d.mappings {
			for _, ssid := range m.SSIDs {
				if matchPattern(identity.SSID, ssid) {
					return m.Zone, "ssid:" + ssid
				}
			}
		}
	}

	// Check BSSID (specific access point)
	if identity.BSSID != "" {
		for _, m := range d.mappings {
			for _, bssid := range m.BSSIDs {
				if strings.EqualFold(identity.BSSID, bssid) {
					return m.Zone, "bssid:" + bssid
				}
			}
		}
	}

	// Check IP subnets
	for _, ip := range identity.IPAddresses {
		parsedIP := net.ParseIP(ip)
		if parsedIP == nil {
			continue
		}

		for _, m := range d.mappings {
			for _, subnet := range m.Subnets {
				_, ipnet, err := net.ParseCIDR(subnet)
				if err != nil {
					continue
				}
				if ipnet.Contains(parsedIP) {
					return m.Zone, "subnet:" + subnet
				}
			}
		}
	}

	// Default zone
	return "default", "none"
}

// verifyIdentity checks if the identity is legitimate
func (d *Detector) verifyIdentity(identity *Identity) bool {
	if identity.Zone == "default" || identity.Zone == "" {
		return false
	}

	// Find the mapping for this zone
	d.mu.RLock()
	defer d.mu.RUnlock()

	for _, m := range d.mappings {
		if m.Zone != identity.Zone {
			continue
		}

		// Verify IP is in expected subnet
		for _, ip := range identity.IPAddresses {
			parsedIP := net.ParseIP(ip)
			if parsedIP == nil {
				continue
			}

			for _, subnet := range m.Subnets {
				_, ipnet, err := net.ParseCIDR(subnet)
				if err != nil {
					continue
				}
				if ipnet.Contains(parsedIP) {
					return true
				}
			}
		}

		// If SSID matches, trust it
		for _, ssid := range m.SSIDs {
			if matchPattern(identity.SSID, ssid) {
				return true
			}
		}
	}

	return false
}

// matchPattern matches a string against a pattern (supports * wildcard)
func matchPattern(s, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		// Convert to regex
		regexPattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), `\*`, ".*") + "$"
		matched, _ := regexp.MatchString(regexPattern, s)
		return matched
	}
	return strings.EqualFold(s, pattern)
}

// WiFiInfo contains WiFi-specific information
type WiFiInfo struct {
	SSID      string
	BSSID     string
	Signal    int
	Frequency int
	Channel   int
	Security  string
}

// isWiFiInterface checks if an interface is a WiFi interface
func isWiFiInterface(name string) bool {
	// Common WiFi interface prefixes
	prefixes := []string{"wlan", "wlp", "wifi", "wl", "ath", "ra"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}

	// Check /sys/class/net/<iface>/wireless on Linux
	if _, err := os.Stat("/sys/class/net/" + name + "/wireless"); err == nil {
		return true
	}

	return false
}

// detectWiFi gets WiFi information for an interface
func detectWiFi(ctx context.Context, iface string) (*WiFiInfo, error) {
	// Try iwconfig first
	wifi, err := detectWiFiIwconfig(ctx, iface)
	if err == nil {
		return wifi, nil
	}

	// Try iw
	wifi, err = detectWiFiIw(ctx, iface)
	if err == nil {
		return wifi, nil
	}

	// Try nmcli (NetworkManager)
	wifi, err = detectWiFiNmcli(ctx)
	if err == nil {
		return wifi, nil
	}

	return nil, fmt.Errorf("no wifi detection method succeeded")
}

// detectWiFiIwconfig uses iwconfig to get WiFi info
func detectWiFiIwconfig(ctx context.Context, iface string) (*WiFiInfo, error) {
	cmd := exec.CommandContext(ctx, "iwconfig", iface)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	wifi := &WiFiInfo{}
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// ESSID:"NetworkName"
		if strings.Contains(line, "ESSID:") {
			re := regexp.MustCompile(`ESSID:"([^"]*)"`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				wifi.SSID = matches[1]
			}
		}

		// Access Point: XX:XX:XX:XX:XX:XX
		if strings.Contains(line, "Access Point:") {
			re := regexp.MustCompile(`Access Point:\s*([0-9A-Fa-f:]+)`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				wifi.BSSID = strings.ToUpper(matches[1])
			}
		}

		// Frequency:2.437 GHz
		if strings.Contains(line, "Frequency:") {
			re := regexp.MustCompile(`Frequency:(\d+\.?\d*)\s*GHz`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				var freq float64
				fmt.Sscanf(matches[1], "%f", &freq)
				wifi.Frequency = int(freq * 1000)
			}
		}

		// Signal level=-42 dBm
		if strings.Contains(line, "Signal level") {
			re := regexp.MustCompile(`Signal level[=:]?\s*(-?\d+)`)
			if matches := re.FindStringSubmatch(line); len(matches) > 1 {
				fmt.Sscanf(matches[1], "%d", &wifi.Signal)
			}
		}
	}

	if wifi.SSID == "" {
		return nil, fmt.Errorf("no SSID found")
	}

	return wifi, nil
}

// detectWiFiIw uses iw to get WiFi info
func detectWiFiIw(ctx context.Context, iface string) (*WiFiInfo, error) {
	cmd := exec.CommandContext(ctx, "iw", "dev", iface, "link")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	wifi := &WiFiInfo{}
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Connected to XX:XX:XX:XX:XX:XX
		if strings.HasPrefix(line, "Connected to") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				wifi.BSSID = strings.ToUpper(parts[2])
			}
		}

		// SSID: NetworkName
		if strings.HasPrefix(line, "SSID:") {
			wifi.SSID = strings.TrimSpace(strings.TrimPrefix(line, "SSID:"))
		}

		// freq: 2437
		if strings.HasPrefix(line, "freq:") {
			fmt.Sscanf(line, "freq: %d", &wifi.Frequency)
		}

		// signal: -42 dBm
		if strings.HasPrefix(line, "signal:") {
			fmt.Sscanf(line, "signal: %d", &wifi.Signal)
		}
	}

	if wifi.SSID == "" {
		return nil, fmt.Errorf("no SSID found")
	}

	return wifi, nil
}

// detectWiFiNmcli uses NetworkManager's nmcli
func detectWiFiNmcli(ctx context.Context) (*WiFiInfo, error) {
	cmd := exec.CommandContext(ctx, "nmcli", "-t", "-f", "ACTIVE,SSID,BSSID,SIGNAL,FREQ,CHAN,SECURITY", "dev", "wifi")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "yes:") {
			// Active connection
			parts := strings.Split(line, ":")
			if len(parts) >= 7 {
				wifi := &WiFiInfo{
					SSID:     parts[1],
					BSSID:    strings.ToUpper(strings.ReplaceAll(parts[2], "\\:", ":")),
					Security: parts[6],
				}
				fmt.Sscanf(parts[3], "%d", &wifi.Signal)
				fmt.Sscanf(parts[4], "%d", &wifi.Frequency)
				fmt.Sscanf(parts[5], "%d", &wifi.Channel)
				return wifi, nil
			}
		}
	}

	return nil, fmt.Errorf("no active wifi connection")
}

// lookupMAC finds MAC address from ARP cache for an IP
func lookupMAC(ip string) (string, error) {
	// Read /proc/net/arp on Linux
	file, err := os.Open("/proc/net/arp")
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Scan() // Skip header line

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) >= 4 && fields[0] == ip {
			mac := fields[3]
			if mac != "00:00:00:00:00:00" {
				return strings.ToUpper(mac), nil
			}
		}
	}

	return "", fmt.Errorf("MAC not found for %s", ip)
}

// GetMappings returns all zone mappings
func (d *Detector) GetMappings() []ZoneMapping {
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]ZoneMapping, len(d.mappings))
	copy(result, d.mappings)
	return result
}

// ClearCache clears the identity cache
func (d *Detector) ClearCache() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cache = make(map[string]*Identity)
}
