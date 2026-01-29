// Package admin provides global administration capabilities for LocalMesh.
// The GlobalAdmin manages multiple realms, provides cross-realm visibility,
// and enables centralized policy distribution.
package admin

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// RealmStatus represents the current state of a realm.
type RealmStatus string

const (
	RealmStatusOnline      RealmStatus = "online"
	RealmStatusOffline     RealmStatus = "offline"
	RealmStatusDegraded    RealmStatus = "degraded"
	RealmStatusUnreachable RealmStatus = "unreachable"
)

// RealmInfo contains information about a managed realm.
type RealmInfo struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Endpoint     string            `json:"endpoint"`
	Status       RealmStatus       `json:"status"`
	ServiceCount int               `json:"service_count"`
	PeerCount    int               `json:"peer_count"`
	LastSeen     time.Time         `json:"last_seen"`
	JoinedAt     time.Time         `json:"joined_at"`
	Version      string            `json:"version,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
}

// ServiceInfo represents a service from any realm.
type ServiceInfo struct {
	Name      string    `json:"name"`
	RealmID   string    `json:"realm_id"`
	RealmName string    `json:"realm_name"`
	Hostname  string    `json:"hostname"`
	Port      int       `json:"port"`
	Healthy   bool      `json:"healthy"`
	Public    bool      `json:"public"`
	Tags      []string  `json:"tags"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AlertLevel defines the severity of an alert.
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelError    AlertLevel = "error"
	AlertLevelCritical AlertLevel = "critical"
)

// Alert represents a system alert from any realm.
type Alert struct {
	ID        string            `json:"id"`
	RealmID   string            `json:"realm_id"`
	RealmName string            `json:"realm_name"`
	Level     AlertLevel        `json:"level"`
	Message   string            `json:"message"`
	Source    string            `json:"source"`
	CreatedAt time.Time         `json:"created_at"`
	AckedAt   *time.Time        `json:"acked_at,omitempty"`
	AckedBy   string            `json:"acked_by,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Policy represents a configuration policy to distribute.
type Policy struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Type        string         `json:"type"`   // "rbac", "network", "service", etc.
	Realms      []string       `json:"realms"` // Target realms, empty = all
	Content     map[string]any `json:"content"`
	Enabled     bool           `json:"enabled"`
	Version     int            `json:"version"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// Stats holds aggregated statistics across all realms.
type Stats struct {
	TotalRealms     int          `json:"total_realms"`
	OnlineRealms    int          `json:"online_realms"`
	TotalServices   int          `json:"total_services"`
	HealthyServices int          `json:"healthy_services"`
	TotalAlerts     int          `json:"total_alerts"`
	ActiveAlerts    int          `json:"active_alerts"`
	RealmStats      []RealmStats `json:"realm_stats"`
	UpdatedAt       time.Time    `json:"updated_at"`
}

// RealmStats holds per-realm statistics.
type RealmStats struct {
	RealmID      string      `json:"realm_id"`
	RealmName    string      `json:"realm_name"`
	Status       RealmStatus `json:"status"`
	ServiceCount int         `json:"service_count"`
	HealthyCount int         `json:"healthy_count"`
	AlertCount   int         `json:"alert_count"`
	LastSeen     time.Time   `json:"last_seen"`
}

// GlobalAdmin manages the global administration of all realms.
type GlobalAdmin struct {
	realmID   string
	realmName string

	realms   map[string]*RealmInfo
	services map[string]*ServiceInfo // "realm/service" -> info
	alerts   map[string]*Alert
	policies map[string]*Policy

	healthCheckInterval time.Duration
	mu                  sync.RWMutex

	// Callbacks
	onRealmJoined func(*RealmInfo)
	onRealmLeft   func(string)
	onAlertFired  func(*Alert)
}

// NewGlobalAdmin creates a new global admin instance.
func NewGlobalAdmin(realmID, realmName string) *GlobalAdmin {
	return &GlobalAdmin{
		realmID:             realmID,
		realmName:           realmName,
		realms:              make(map[string]*RealmInfo),
		services:            make(map[string]*ServiceInfo),
		alerts:              make(map[string]*Alert),
		policies:            make(map[string]*Policy),
		healthCheckInterval: 30 * time.Second,
	}
}

// RegisterRealm adds a realm to be managed by this global admin.
func (ga *GlobalAdmin) RegisterRealm(realm *RealmInfo) error {
	if realm.ID == "" {
		return fmt.Errorf("realm ID is required")
	}

	ga.mu.Lock()
	defer ga.mu.Unlock()

	realm.JoinedAt = time.Now()
	realm.LastSeen = time.Now()
	if realm.Status == "" {
		realm.Status = RealmStatusOnline
	}

	ga.realms[realm.ID] = realm

	if ga.onRealmJoined != nil {
		go ga.onRealmJoined(realm)
	}

	return nil
}

// UnregisterRealm removes a realm from management.
func (ga *GlobalAdmin) UnregisterRealm(realmID string) bool {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	if _, exists := ga.realms[realmID]; !exists {
		return false
	}

	delete(ga.realms, realmID)

	// Remove realm's services
	for key := range ga.services {
		if svc := ga.services[key]; svc.RealmID == realmID {
			delete(ga.services, key)
		}
	}

	if ga.onRealmLeft != nil {
		go ga.onRealmLeft(realmID)
	}

	return true
}

// GetRealm returns information about a specific realm.
func (ga *GlobalAdmin) GetRealm(realmID string) (*RealmInfo, bool) {
	ga.mu.RLock()
	defer ga.mu.RUnlock()
	realm, ok := ga.realms[realmID]
	return realm, ok
}

// ListRealms returns all managed realms.
func (ga *GlobalAdmin) ListRealms() []*RealmInfo {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	realms := make([]*RealmInfo, 0, len(ga.realms))
	for _, realm := range ga.realms {
		realms = append(realms, realm)
	}

	// Sort by name
	sort.Slice(realms, func(i, j int) bool {
		return realms[i].Name < realms[j].Name
	})

	return realms
}

// UpdateRealmStatus updates a realm's status (from health check).
func (ga *GlobalAdmin) UpdateRealmStatus(realmID string, status RealmStatus, serviceCount, peerCount int) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	if realm, ok := ga.realms[realmID]; ok {
		realm.Status = status
		realm.ServiceCount = serviceCount
		realm.PeerCount = peerCount
		realm.LastSeen = time.Now()
	}
}

// AddService registers a service from a realm.
func (ga *GlobalAdmin) AddService(svc *ServiceInfo) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	key := fmt.Sprintf("%s/%s", svc.RealmID, svc.Name)
	svc.UpdatedAt = time.Now()
	ga.services[key] = svc
}

// RemoveService unregisters a service.
func (ga *GlobalAdmin) RemoveService(realmID, serviceName string) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	key := fmt.Sprintf("%s/%s", realmID, serviceName)
	delete(ga.services, key)
}

// ListServices returns all services, optionally filtered by realm.
func (ga *GlobalAdmin) ListServices(realmID string) []*ServiceInfo {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	services := make([]*ServiceInfo, 0)
	for _, svc := range ga.services {
		if realmID == "" || svc.RealmID == realmID {
			services = append(services, svc)
		}
	}

	// Sort by realm then name
	sort.Slice(services, func(i, j int) bool {
		if services[i].RealmID != services[j].RealmID {
			return services[i].RealmID < services[j].RealmID
		}
		return services[i].Name < services[j].Name
	})

	return services
}

// FindService looks up a service by name across all realms.
func (ga *GlobalAdmin) FindService(name string) []*ServiceInfo {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	var matches []*ServiceInfo
	for _, svc := range ga.services {
		if svc.Name == name {
			matches = append(matches, svc)
		}
	}
	return matches
}

// FireAlert creates a new alert.
func (ga *GlobalAdmin) FireAlert(alert *Alert) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	alert.CreatedAt = time.Now()
	if alert.ID == "" {
		alert.ID = fmt.Sprintf("%s-%d", alert.RealmID, time.Now().UnixNano())
	}

	ga.alerts[alert.ID] = alert

	if ga.onAlertFired != nil {
		go ga.onAlertFired(alert)
	}
}

// AckAlert acknowledges an alert.
func (ga *GlobalAdmin) AckAlert(alertID, ackBy string) bool {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	alert, ok := ga.alerts[alertID]
	if !ok {
		return false
	}

	now := time.Now()
	alert.AckedAt = &now
	alert.AckedBy = ackBy
	return true
}

// ListAlerts returns alerts, optionally filtered by realm and level.
func (ga *GlobalAdmin) ListAlerts(realmID string, level AlertLevel, activeOnly bool) []*Alert {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	alerts := make([]*Alert, 0)
	for _, alert := range ga.alerts {
		if realmID != "" && alert.RealmID != realmID {
			continue
		}
		if level != "" && alert.Level != level {
			continue
		}
		if activeOnly && alert.AckedAt != nil {
			continue
		}
		alerts = append(alerts, alert)
	}

	// Sort by time, newest first
	sort.Slice(alerts, func(i, j int) bool {
		return alerts[i].CreatedAt.After(alerts[j].CreatedAt)
	})

	return alerts
}

// AddPolicy creates or updates a policy.
func (ga *GlobalAdmin) AddPolicy(policy *Policy) error {
	if policy.ID == "" {
		return fmt.Errorf("policy ID is required")
	}

	ga.mu.Lock()
	defer ga.mu.Unlock()

	now := time.Now()
	if existing, ok := ga.policies[policy.ID]; ok {
		policy.Version = existing.Version + 1
		policy.CreatedAt = existing.CreatedAt
	} else {
		policy.Version = 1
		policy.CreatedAt = now
	}
	policy.UpdatedAt = now

	ga.policies[policy.ID] = policy
	return nil
}

// GetPolicy retrieves a policy by ID.
func (ga *GlobalAdmin) GetPolicy(policyID string) (*Policy, bool) {
	ga.mu.RLock()
	defer ga.mu.RUnlock()
	policy, ok := ga.policies[policyID]
	return policy, ok
}

// ListPolicies returns all policies.
func (ga *GlobalAdmin) ListPolicies() []*Policy {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	policies := make([]*Policy, 0, len(ga.policies))
	for _, policy := range ga.policies {
		policies = append(policies, policy)
	}

	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Name < policies[j].Name
	})

	return policies
}

// DeletePolicy removes a policy.
func (ga *GlobalAdmin) DeletePolicy(policyID string) bool {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	if _, ok := ga.policies[policyID]; !ok {
		return false
	}
	delete(ga.policies, policyID)
	return true
}

// GetPoliciesForRealm returns policies applicable to a specific realm.
func (ga *GlobalAdmin) GetPoliciesForRealm(realmID string) []*Policy {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	var applicable []*Policy
	for _, policy := range ga.policies {
		if !policy.Enabled {
			continue
		}
		// Empty realms list means applies to all
		if len(policy.Realms) == 0 {
			applicable = append(applicable, policy)
			continue
		}
		// Check if realm is in the list
		for _, r := range policy.Realms {
			if r == realmID || r == "*" {
				applicable = append(applicable, policy)
				break
			}
		}
	}
	return applicable
}

// GetStats returns aggregated statistics.
func (ga *GlobalAdmin) GetStats(ctx context.Context) *Stats {
	ga.mu.RLock()
	defer ga.mu.RUnlock()

	stats := &Stats{
		TotalRealms:   len(ga.realms),
		TotalServices: len(ga.services),
		RealmStats:    make([]RealmStats, 0, len(ga.realms)),
		UpdatedAt:     time.Now(),
	}

	for _, realm := range ga.realms {
		if realm.Status == RealmStatusOnline {
			stats.OnlineRealms++
		}

		// Count per-realm stats
		realmStats := RealmStats{
			RealmID:   realm.ID,
			RealmName: realm.Name,
			Status:    realm.Status,
			LastSeen:  realm.LastSeen,
		}

		for _, svc := range ga.services {
			if svc.RealmID == realm.ID {
				realmStats.ServiceCount++
				if svc.Healthy {
					realmStats.HealthyCount++
					stats.HealthyServices++
				}
			}
		}

		for _, alert := range ga.alerts {
			if alert.RealmID == realm.ID && alert.AckedAt == nil {
				realmStats.AlertCount++
				stats.ActiveAlerts++
			}
		}

		stats.RealmStats = append(stats.RealmStats, realmStats)
	}

	stats.TotalAlerts = len(ga.alerts)

	return stats
}

// SetCallbacks sets callback functions for events.
func (ga *GlobalAdmin) SetCallbacks(onJoined func(*RealmInfo), onLeft func(string), onAlert func(*Alert)) {
	ga.mu.Lock()
	defer ga.mu.Unlock()

	ga.onRealmJoined = onJoined
	ga.onRealmLeft = onLeft
	ga.onAlertFired = onAlert
}

// RealmID returns the global admin's realm ID.
func (ga *GlobalAdmin) RealmID() string {
	return ga.realmID
}

// RealmName returns the global admin's realm name.
func (ga *GlobalAdmin) RealmName() string {
	return ga.realmName
}
