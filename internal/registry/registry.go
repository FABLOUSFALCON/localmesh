// Package registry provides service registration and discovery.
// Acts as a local service directory with health checking.
package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/FABLOUSFALCON/localmesh/internal/storage"
	"github.com/FABLOUSFALCON/localmesh/pkg/types"
)

// ServiceStatus represents the health status of a service
type ServiceStatus string

const (
	StatusHealthy   ServiceStatus = "healthy"
	StatusUnhealthy ServiceStatus = "unhealthy"
	StatusUnknown   ServiceStatus = "unknown"
	StatusStarting  ServiceStatus = "starting"
	StatusStopping  ServiceStatus = "stopping"
)

// ServiceEntry is a registered service in the registry
type ServiceEntry struct {
	types.Service
	Status          ServiceStatus     `json:"status"`
	HealthScore     float64           `json:"health_score"`
	RegisteredAt    time.Time         `json:"registered_at"`
	LastHealthCheck time.Time         `json:"last_health_check"`
	FailureCount    int               `json:"failure_count"`
	Metadata        map[string]string `json:"metadata"`
}

// Registry manages local service registration
type Registry struct {
	nodeID   string
	zone     string
	services map[string]*ServiceEntry
	mu       sync.RWMutex

	// Storage backends
	sqlite *storage.SQLiteStore
	badger *storage.BadgerStore

	// Health check settings
	healthCheckInterval time.Duration
	healthCheckTimeout  time.Duration
	maxFailures         int

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	logger *slog.Logger
}

// RegistryConfig configures the service registry
type RegistryConfig struct {
	NodeID              string
	Zone                string
	SQLite              *storage.SQLiteStore
	Badger              *storage.BadgerStore
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration
	MaxFailures         int
	Logger              *slog.Logger
}

// DefaultRegistryConfig returns sensible defaults
func DefaultRegistryConfig() RegistryConfig {
	return RegistryConfig{
		HealthCheckInterval: 10 * time.Second,
		HealthCheckTimeout:  5 * time.Second,
		MaxFailures:         3,
	}
}

// NewRegistry creates a new service registry
func NewRegistry(cfg RegistryConfig) *Registry {
	ctx, cancel := context.WithCancel(context.Background())

	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	return &Registry{
		nodeID:              cfg.NodeID,
		zone:                cfg.Zone,
		services:            make(map[string]*ServiceEntry),
		sqlite:              cfg.SQLite,
		badger:              cfg.Badger,
		healthCheckInterval: cfg.HealthCheckInterval,
		healthCheckTimeout:  cfg.HealthCheckTimeout,
		maxFailures:         cfg.MaxFailures,
		ctx:                 ctx,
		cancel:              cancel,
		logger:              logger,
	}
}

// Start begins the registry and health check loop
func (r *Registry) Start() error {
	// Load persisted services
	if err := r.loadServices(); err != nil {
		r.logger.Warn("failed to load persisted services", "error", err)
	}

	// Start health check loop
	r.wg.Add(1)
	go r.healthCheckLoop()

	r.logger.Info("service registry started",
		"node_id", r.nodeID,
		"zone", r.zone,
	)

	return nil
}

// Register adds a new service to the registry
func (r *Registry) Register(svc types.Service) (string, error) {
	if svc.ID == "" {
		svc.ID = uuid.New().String()
	}
	if svc.Zone == "" {
		svc.Zone = r.zone
	}
	if svc.NodeID == "" {
		svc.NodeID = r.nodeID
	}

	entry := &ServiceEntry{
		Service:         svc,
		Status:          StatusStarting,
		HealthScore:     1.0,
		RegisteredAt:    time.Now(),
		LastHealthCheck: time.Now(),
		FailureCount:    0,
		Metadata:        make(map[string]string),
	}

	r.mu.Lock()
	r.services[svc.ID] = entry
	r.mu.Unlock()

	// Persist to storage
	if err := r.persistService(entry); err != nil {
		r.logger.Warn("failed to persist service", "service", svc.Name, "error", err)
	}

	// Cache in Badger for fast lookups
	if r.badger != nil {
		key := fmt.Sprintf("service:%s", svc.ID)
		r.badger.SetJSON(key, entry, 0)
	}

	r.logger.Info("registered service",
		"id", svc.ID,
		"name", svc.Name,
		"endpoint", svc.Endpoint,
	)

	return svc.ID, nil
}

// Deregister removes a service from the registry
func (r *Registry) Deregister(serviceID string) error {
	r.mu.Lock()
	entry, exists := r.services[serviceID]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("service not found: %s", serviceID)
	}
	delete(r.services, serviceID)
	r.mu.Unlock()

	// Remove from storage
	if r.sqlite != nil {
		r.sqlite.Exec(r.ctx, "DELETE FROM services WHERE id = ?", serviceID)
	}
	if r.badger != nil {
		r.badger.Delete(fmt.Sprintf("service:%s", serviceID))
	}

	r.logger.Info("deregistered service",
		"id", serviceID,
		"name", entry.Name,
	)

	return nil
}

// Get retrieves a service by ID
func (r *Registry) Get(serviceID string) (*ServiceEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entry, exists := r.services[serviceID]
	return entry, exists
}

// GetByName finds services by name
func (r *Registry) GetByName(name string) []*ServiceEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var entries []*ServiceEntry
	for _, entry := range r.services {
		if entry.Name == name && entry.Status == StatusHealthy {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetByZone finds services in a zone
func (r *Registry) GetByZone(zone string) []*ServiceEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var entries []*ServiceEntry
	for _, entry := range r.services {
		if entry.Zone == zone {
			entries = append(entries, entry)
		}
	}
	return entries
}

// GetHealthy returns all healthy services
func (r *Registry) GetHealthy() []*ServiceEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var entries []*ServiceEntry
	for _, entry := range r.services {
		if entry.Status == StatusHealthy {
			entries = append(entries, entry)
		}
	}
	return entries
}

// All returns all registered services
func (r *Registry) All() []*ServiceEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	entries := make([]*ServiceEntry, 0, len(r.services))
	for _, entry := range r.services {
		entries = append(entries, entry)
	}
	return entries
}

// Count returns the number of registered services
func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.services)
}

// UpdateHealth updates a service's health status
func (r *Registry) UpdateHealth(serviceID string, healthy bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry, exists := r.services[serviceID]
	if !exists {
		return
	}

	entry.LastHealthCheck = time.Now()

	if healthy {
		entry.Status = StatusHealthy
		entry.FailureCount = 0
		entry.HealthScore = 1.0
	} else {
		entry.FailureCount++
		entry.HealthScore = 1.0 - (float64(entry.FailureCount) / float64(r.maxFailures+1))

		if entry.FailureCount >= r.maxFailures {
			entry.Status = StatusUnhealthy
		}
	}
}

// healthCheckLoop periodically checks service health
func (r *Registry) healthCheckLoop() {
	defer r.wg.Done()

	ticker := time.NewTicker(r.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.ctx.Done():
			return
		case <-ticker.C:
			r.checkAllHealth()
		}
	}
}

// checkAllHealth performs health checks on all services
func (r *Registry) checkAllHealth() {
	r.mu.RLock()
	services := make([]*ServiceEntry, 0, len(r.services))
	for _, s := range r.services {
		services = append(services, s)
	}
	r.mu.RUnlock()

	for _, svc := range services {
		// TODO: Implement actual HTTP health check
		// For now, just mark as healthy
		r.UpdateHealth(svc.ID, true)
	}
}

// persistService saves a service to SQLite
func (r *Registry) persistService(entry *ServiceEntry) error {
	if r.sqlite == nil {
		return nil
	}

	metadata, _ := json.Marshal(entry.Metadata)

	_, err := r.sqlite.Exec(r.ctx, `
		INSERT INTO services (id, name, version, node_id, endpoint, zone, status, health_score, registered_at, metadata)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			status = excluded.status,
			health_score = excluded.health_score,
			metadata = excluded.metadata
	`, entry.ID, entry.Name, entry.Version, entry.NodeID, entry.Endpoint, entry.Zone,
		string(entry.Status), entry.HealthScore, entry.RegisteredAt, string(metadata))

	return err
}

// loadServices loads persisted services from SQLite
func (r *Registry) loadServices() error {
	if r.sqlite == nil {
		return nil
	}

	rows, err := r.sqlite.Query(r.ctx, `
		SELECT id, name, version, node_id, endpoint, zone, status, health_score, registered_at, metadata
		FROM services
		WHERE node_id = ?
	`, r.nodeID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var entry ServiceEntry
		var status string
		var metadata string

		err := rows.Scan(
			&entry.ID, &entry.Name, &entry.Version, &entry.NodeID,
			&entry.Endpoint, &entry.Zone, &status, &entry.HealthScore,
			&entry.RegisteredAt, &metadata,
		)
		if err != nil {
			continue
		}

		entry.Status = ServiceStatus(status)
		entry.Metadata = make(map[string]string)
		json.Unmarshal([]byte(metadata), &entry.Metadata)

		r.services[entry.ID] = &entry
	}

	r.logger.Info("loaded persisted services", "count", len(r.services))
	return nil
}

// Stop gracefully stops the registry
func (r *Registry) Stop() error {
	r.cancel()
	r.wg.Wait()

	// Persist all services before shutdown
	r.mu.RLock()
	for _, entry := range r.services {
		r.persistService(entry)
	}
	r.mu.RUnlock()

	r.logger.Info("service registry stopped")
	return nil
}
