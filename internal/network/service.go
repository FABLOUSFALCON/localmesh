// Package network provides network identity service for LocalMesh.
package network

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Service provides network identity detection and verification
type Service struct {
	detector *Detector
	verifier *Verifier
	logger   *slog.Logger

	// Local identity cache
	localIdentity *Identity
	localMu       sync.RWMutex

	// Background refresh
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// ServiceConfig configures the network service
type ServiceConfig struct {
	ZoneMappings    []ZoneMapping
	TrustedAPs      map[string]string // BSSID -> Zone
	RefreshInterval time.Duration
	Logger          *slog.Logger
}

// DefaultServiceConfig returns sensible defaults
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		ZoneMappings:    []ZoneMapping{},
		TrustedAPs:      make(map[string]string),
		RefreshInterval: 5 * time.Minute,
	}
}

// NewService creates a new network identity service
func NewService(cfg ServiceConfig) *Service {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	detector := NewDetector()
	for _, m := range cfg.ZoneMappings {
		detector.AddMapping(m)
	}

	verifier := NewVerifier(detector)
	for bssid, zone := range cfg.TrustedAPs {
		verifier.AddTrustedAP(bssid, zone)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Service{
		detector: detector,
		verifier: verifier,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
	}
}

// Start begins background identity refresh
func (s *Service) Start() error {
	// Initial detection
	identity, err := s.detector.DetectLocal(s.ctx)
	if err != nil {
		s.logger.Warn("initial network detection failed", "error", err)
	} else {
		s.localMu.Lock()
		s.localIdentity = identity
		s.localMu.Unlock()

		s.logger.Info("network identity detected",
			"interface", identity.InterfaceName,
			"ssid", identity.SSID,
			"zone", identity.Zone,
			"ips", identity.IPAddresses,
		)
	}

	// Start background refresh
	s.wg.Add(1)
	go s.refreshLoop()

	return nil
}

// Stop stops the service
func (s *Service) Stop() error {
	s.cancel()
	s.wg.Wait()
	return nil
}

// refreshLoop periodically refreshes local identity
func (s *Service) refreshLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			identity, err := s.detector.DetectLocal(s.ctx)
			if err != nil {
				s.logger.Warn("network identity refresh failed", "error", err)
				continue
			}

			s.localMu.Lock()
			oldZone := ""
			if s.localIdentity != nil {
				oldZone = s.localIdentity.Zone
			}
			s.localIdentity = identity
			s.localMu.Unlock()

			// Log zone changes
			if oldZone != "" && oldZone != identity.Zone {
				s.logger.Info("network zone changed",
					"old_zone", oldZone,
					"new_zone", identity.Zone,
				)
			}
		}
	}
}

// GetLocalIdentity returns the local node's network identity
func (s *Service) GetLocalIdentity() *Identity {
	s.localMu.RLock()
	defer s.localMu.RUnlock()
	return s.localIdentity
}

// DetectIdentity detects network identity for an IP
func (s *Service) DetectIdentity(ctx context.Context, ip string) (*Identity, error) {
	return s.detector.DetectFromIP(ctx, ip)
}

// VerifyIdentity verifies claimed network identity
func (s *Service) VerifyIdentity(ctx context.Context, req VerifyRequest) (*VerifyResult, error) {
	return s.verifier.Verify(ctx, req)
}

// AddZoneMapping adds a new zone mapping
func (s *Service) AddZoneMapping(m ZoneMapping) {
	s.detector.AddMapping(m)
	s.logger.Debug("added zone mapping",
		"zone", m.Zone,
		"ssids", m.SSIDs,
		"subnets", m.Subnets,
	)
}

// AddTrustedAP adds a trusted access point
func (s *Service) AddTrustedAP(bssid, zone string) {
	s.verifier.AddTrustedAP(bssid, zone)
	s.logger.Debug("added trusted AP",
		"bssid", bssid,
		"zone", zone,
	)
}

// GetZoneMappings returns all zone mappings
func (s *Service) GetZoneMappings() []ZoneMapping {
	return s.detector.GetMappings()
}

// Detector returns the underlying detector
func (s *Service) Detector() *Detector {
	return s.detector
}

// Verifier returns the underlying verifier
func (s *Service) Verifier() *Verifier {
	return s.verifier
}

// RefreshLocalIdentity forces a refresh of local identity
func (s *Service) RefreshLocalIdentity(ctx context.Context) (*Identity, error) {
	identity, err := s.detector.DetectLocal(ctx)
	if err != nil {
		return nil, err
	}

	s.localMu.Lock()
	s.localIdentity = identity
	s.localMu.Unlock()

	return identity, nil
}

// GetZoneForIP returns the zone for a given IP
func (s *Service) GetZoneForIP(ip string) string {
	identity, err := s.detector.DetectFromIP(context.Background(), ip)
	if err != nil {
		return "default"
	}
	return identity.Zone
}

// String returns a summary of the service state
func (s *Service) String() string {
	s.localMu.RLock()
	defer s.localMu.RUnlock()

	if s.localIdentity == nil {
		return "network: not detected"
	}

	return fmt.Sprintf("network: %s (%s) zone=%s verified=%v",
		s.localIdentity.InterfaceName,
		s.localIdentity.SSID,
		s.localIdentity.Zone,
		s.localIdentity.Verified,
	)
}
