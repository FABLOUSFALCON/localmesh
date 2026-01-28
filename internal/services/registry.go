package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Registry manages external services registered with LocalMesh.
// It provides service registration, lookup, health checking, and discovery.
type Registry struct {
	services map[string]*Service
	mu       sync.RWMutex
	logger   zerolog.Logger

	// Health checking
	healthCtx    context.Context
	healthCancel context.CancelFunc
	healthClient *http.Client

	// Callbacks for service events
	onServiceAdded   func(*Service)
	onServiceRemoved func(*Service)
	onHealthChange   func(*Service, ServiceState, ServiceState)
}

// NewRegistry creates a new service registry.
func NewRegistry(logger zerolog.Logger) *Registry {
	ctx, cancel := context.WithCancel(context.Background())

	return &Registry{
		services:     make(map[string]*Service),
		logger:       logger.With().Str("component", "service-registry").Logger(),
		healthCtx:    ctx,
		healthCancel: cancel,
		healthClient: &http.Client{
			Timeout: 5 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}
}

// Register adds a new service to the registry.
func (r *Registry) Register(svc *Service) error {
	if err := svc.Validate(); err != nil {
		return fmt.Errorf("invalid service: %w", err)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.services[svc.Info.Name]; exists {
		return fmt.Errorf("service %q already registered", svc.Info.Name)
	}

	svc.CreatedAt = time.Now()
	svc.UpdatedAt = time.Now()
	svc.Health.State = StateUnknown

	r.services[svc.Info.Name] = svc

	r.logger.Info().
		Str("service", svc.Info.Name).
		Str("url", svc.Endpoint.URL).
		Strs("zones", svc.Access.Zones).
		Msg("Service registered")

	if r.onServiceAdded != nil {
		go r.onServiceAdded(svc)
	}

	return nil
}

// Unregister removes a service from the registry.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	svc, exists := r.services[name]
	if !exists {
		return fmt.Errorf("service %q not found", name)
	}

	delete(r.services, name)

	r.logger.Info().
		Str("service", name).
		Msg("Service unregistered")

	if r.onServiceRemoved != nil {
		go r.onServiceRemoved(svc)
	}

	return nil
}

// Get returns a service by name.
func (r *Registry) Get(name string) (*Service, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	svc, exists := r.services[name]
	return svc, exists
}

// List returns all registered services.
func (r *Registry) List() []*Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Service, 0, len(r.services))
	for _, svc := range r.services {
		result = append(result, svc)
	}
	return result
}

// ListHealthy returns only healthy services.
func (r *Registry) ListHealthy() []*Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Service, 0)
	for _, svc := range r.services {
		if svc.IsHealthy() {
			result = append(result, svc)
		}
	}
	return result
}

// ListByZone returns services accessible from a given zone.
func (r *Registry) ListByZone(zone string, roles []string) []*Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Service, 0)
	for _, svc := range r.services {
		if svc.CanAccess(zone, roles) {
			result = append(result, svc)
		}
	}
	return result
}

// ListByTag returns services with a specific tag.
func (r *Registry) ListByTag(tag string) []*Service {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*Service, 0)
	for _, svc := range r.services {
		for _, t := range svc.Info.Tags {
			if t == tag {
				result = append(result, svc)
				break
			}
		}
	}
	return result
}

// Update updates an existing service.
func (r *Registry) Update(name string, updateFn func(*Service) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	svc, exists := r.services[name]
	if !exists {
		return fmt.Errorf("service %q not found", name)
	}

	if err := updateFn(svc); err != nil {
		return err
	}

	if err := svc.Validate(); err != nil {
		return fmt.Errorf("invalid service after update: %w", err)
	}

	svc.UpdatedAt = time.Now()

	r.logger.Info().
		Str("service", name).
		Msg("Service updated")

	return nil
}

// CheckHealth performs a health check on a service.
func (r *Registry) CheckHealth(ctx context.Context, name string) error {
	r.mu.RLock()
	svc, exists := r.services[name]
	r.mu.RUnlock()

	if !exists {
		return fmt.Errorf("service %q not found", name)
	}

	return r.performHealthCheck(ctx, svc)
}

// performHealthCheck does the actual health check.
func (r *Registry) performHealthCheck(ctx context.Context, svc *Service) error {
	healthURL := svc.Endpoint.URL + svc.Endpoint.HealthPath
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, healthURL, nil)
	if err != nil {
		r.updateHealth(svc, StateUnhealthy, fmt.Sprintf("Failed to create request: %v", err), 0)
		return err
	}

	req.Header.Set("User-Agent", "LocalMesh/1.0 HealthCheck")

	resp, err := r.healthClient.Do(req)
	latency := time.Since(start)

	if err != nil {
		r.updateHealth(svc, StateUnhealthy, fmt.Sprintf("Health check failed: %v", err), latency)
		return err
	}
	defer resp.Body.Close()

	// Read body to complete connection
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		r.updateHealth(svc, StateHealthy, "OK", latency)
		return nil
	} else if resp.StatusCode >= 500 {
		r.updateHealth(svc, StateUnhealthy, fmt.Sprintf("Health check returned %d", resp.StatusCode), latency)
		return fmt.Errorf("unhealthy status: %d", resp.StatusCode)
	} else {
		r.updateHealth(svc, StateDegraded, fmt.Sprintf("Health check returned %d", resp.StatusCode), latency)
		return fmt.Errorf("degraded status: %d", resp.StatusCode)
	}
}

// updateHealth updates the health status of a service.
func (r *Registry) updateHealth(svc *Service, state ServiceState, message string, latency time.Duration) {
	r.mu.Lock()
	oldState := svc.Health.State
	svc.Health.State = state
	svc.Health.LastCheck = time.Now()
	svc.Health.Message = message
	svc.Health.Latency = latency
	svc.Health.CheckCount++

	if state == StateHealthy {
		svc.Health.LastHealthy = time.Now()
	} else {
		svc.Health.FailCount++
	}
	r.mu.Unlock()

	if oldState != state {
		r.logger.Info().
			Str("service", svc.Info.Name).
			Str("old_state", string(oldState)).
			Str("new_state", string(state)).
			Str("message", message).
			Msg("Service health changed")

		if r.onHealthChange != nil {
			go r.onHealthChange(svc, oldState, state)
		}
	}
}

// StartHealthChecks starts background health checking for all services.
func (r *Registry) StartHealthChecks(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Initial check
		r.checkAllHealth()

		for {
			select {
			case <-r.healthCtx.Done():
				return
			case <-ticker.C:
				r.checkAllHealth()
			}
		}
	}()

	r.logger.Info().
		Dur("interval", interval).
		Msg("Started health checks")
}

// checkAllHealth checks health of all services.
func (r *Registry) checkAllHealth() {
	services := r.List()

	for _, svc := range services {
		ctx, cancel := context.WithTimeout(r.healthCtx, 5*time.Second)
		r.performHealthCheck(ctx, svc)
		cancel()
	}
}

// OnServiceAdded sets callback for when a service is added.
func (r *Registry) OnServiceAdded(fn func(*Service)) {
	r.onServiceAdded = fn
}

// OnServiceRemoved sets callback for when a service is removed.
func (r *Registry) OnServiceRemoved(fn func(*Service)) {
	r.onServiceRemoved = fn
}

// OnHealthChange sets callback for health state changes.
func (r *Registry) OnHealthChange(fn func(*Service, ServiceState, ServiceState)) {
	r.onHealthChange = fn
}

// Close stops health checking and cleans up.
func (r *Registry) Close() error {
	r.healthCancel()
	r.healthClient.CloseIdleConnections()

	r.logger.Info().Msg("Service registry closed")
	return nil
}

// Stats returns registry statistics.
func (r *Registry) Stats() RegistryStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := RegistryStats{
		Total: len(r.services),
	}

	for _, svc := range r.services {
		switch svc.Health.State {
		case StateHealthy:
			stats.Healthy++
		case StateUnhealthy:
			stats.Unhealthy++
		case StateDegraded:
			stats.Degraded++
		default:
			stats.Unknown++
		}

		if svc.Discovered {
			stats.Discovered++
		}
	}

	return stats
}

// RegistryStats contains registry statistics.
type RegistryStats struct {
	Total      int `json:"total"`
	Healthy    int `json:"healthy"`
	Unhealthy  int `json:"unhealthy"`
	Degraded   int `json:"degraded"`
	Unknown    int `json:"unknown"`
	Discovered int `json:"discovered"`
}
