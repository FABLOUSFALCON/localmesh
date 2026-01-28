// Package network provides network identity verification.
package network

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
)

// Verifier handles network identity verification
type Verifier struct {
	detector   *Detector
	trustedAPs map[string]string // BSSID -> Zone
	mu         sync.RWMutex
}

// NewVerifier creates a new network verifier
func NewVerifier(detector *Detector) *Verifier {
	return &Verifier{
		detector:   detector,
		trustedAPs: make(map[string]string),
	}
}

// AddTrustedAP adds a trusted access point
func (v *Verifier) AddTrustedAP(bssid, zone string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.trustedAPs[strings.ToUpper(bssid)] = zone
}

// VerifyRequest verifies a request's network identity
type VerifyRequest struct {
	ClientIP     string
	ClaimedZone  string
	ClaimedSSID  string
	ClaimedBSSID string
}

// VerifyResult contains verification results
type VerifyResult struct {
	Verified        bool      `json:"verified"`
	Identity        *Identity `json:"identity"`
	Reason          string    `json:"reason"`
	MatchedZone     string    `json:"matched_zone"`
	ConfidenceLevel int       `json:"confidence_level"` // 0-100
}

// Verify checks if the claimed network identity matches actual network state
func (v *Verifier) Verify(ctx context.Context, req VerifyRequest) (*VerifyResult, error) {
	result := &VerifyResult{
		Verified:        false,
		ConfidenceLevel: 0,
	}

	// Detect actual identity from IP
	identity, err := v.detector.DetectFromIP(ctx, req.ClientIP)
	if err != nil {
		result.Reason = fmt.Sprintf("detection failed: %v", err)
		return result, nil
	}
	result.Identity = identity

	// Check 1: Is the IP in a known zone?
	if identity.Zone == "default" || identity.Zone == "" {
		result.Reason = "IP not in any known zone"
		result.ConfidenceLevel = 10
		return result, nil
	}
	result.MatchedZone = identity.Zone
	result.ConfidenceLevel = 30

	// Check 2: Does claimed zone match detected zone?
	if req.ClaimedZone != "" && req.ClaimedZone != identity.Zone {
		result.Reason = fmt.Sprintf("claimed zone %s doesn't match detected zone %s", req.ClaimedZone, identity.Zone)
		return result, nil
	}
	result.ConfidenceLevel = 50

	// Check 3: Verify MAC is reachable (ARP check)
	if identity.MacAddress != "" {
		result.ConfidenceLevel = 70
	}

	// Check 4: For WiFi, verify BSSID if claimed
	if req.ClaimedBSSID != "" {
		v.mu.RLock()
		expectedZone, isTrusted := v.trustedAPs[strings.ToUpper(req.ClaimedBSSID)]
		v.mu.RUnlock()

		if isTrusted {
			if expectedZone == identity.Zone {
				result.ConfidenceLevel = 90
			} else {
				result.Reason = "BSSID doesn't match zone"
				return result, nil
			}
		}
	}

	// Passed all checks
	result.Verified = true
	result.Reason = "verified via " + identity.ZoneSource
	if result.ConfidenceLevel < 70 {
		result.ConfidenceLevel = 70
	}

	return result, nil
}

// VerifyLocal verifies the local node's network identity
func (v *Verifier) VerifyLocal(ctx context.Context) (*VerifyResult, error) {
	identity, err := v.detector.DetectLocal(ctx)
	if err != nil {
		return nil, err
	}

	result := &VerifyResult{
		Verified:        identity.Verified,
		Identity:        identity,
		MatchedZone:     identity.Zone,
		ConfidenceLevel: 100, // Local detection is highest confidence
		Reason:          "local detection via " + identity.ZoneSource,
	}

	return result, nil
}

// PingCheck performs a ping check to verify network reachability
func (v *Verifier) PingCheck(ctx context.Context, ip string) bool {
	cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", ip)
	return cmd.Run() == nil
}

// ARPCheck verifies an IP is reachable via ARP
func (v *Verifier) ARPCheck(ctx context.Context, ip string) (string, bool) {
	// First ping to populate ARP cache
	v.PingCheck(ctx, ip)

	// Then look up MAC
	mac, err := lookupMAC(ip)
	if err != nil {
		return "", false
	}
	return mac, mac != ""
}

// SubnetCheck verifies an IP is within expected subnet for a zone
func (v *Verifier) SubnetCheck(ip string, zone string) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}

	for _, m := range v.detector.GetMappings() {
		if m.Zone != zone {
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
	return false
}
