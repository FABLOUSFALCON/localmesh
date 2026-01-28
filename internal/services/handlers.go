package services

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Handlers provides HTTP handlers for service management API.
type Handlers struct {
	registry  *Registry
	discovery *Discovery
	proxy     *Proxy
	logger    zerolog.Logger
}

// NewHandlers creates service API handlers.
func NewHandlers(registry *Registry, discovery *Discovery, proxy *Proxy, logger zerolog.Logger) *Handlers {
	return &Handlers{
		registry:  registry,
		discovery: discovery,
		proxy:     proxy,
		logger:    logger.With().Str("component", "service-handlers").Logger(),
	}
}

// ServiceRequest is the request body for creating/updating a service.
type ServiceRequest struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	HealthPath  string   `json:"health_path"`
	Zones       []string `json:"zones"`
	Roles       []string `json:"roles"`
	RequireAuth bool     `json:"require_auth"`
	Public      bool     `json:"public"`
	Tags        []string `json:"tags"`
}

// ServiceResponse is the response for service operations.
type ServiceResponse struct {
	Service *Service `json:"service,omitempty"`
	Message string   `json:"message,omitempty"`
	Error   string   `json:"error,omitempty"`
}

// ServicesListResponse is the response for listing services.
type ServicesListResponse struct {
	Services []*Service    `json:"services"`
	Stats    RegistryStats `json:"stats"`
}

// RegisterRoutes registers all service API routes on a mux.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	// External service management (distinct from internal service registry)
	mux.HandleFunc("/api/v1/external/services", h.handleServices)
	mux.HandleFunc("/api/v1/external/services/", h.handleService)

	// Discovery
	mux.HandleFunc("/api/v1/external/discovery/nodes", h.handleDiscoveredNodes)
	mux.HandleFunc("/api/v1/external/discovery/scan", h.handleDiscoveryScan)

	// Stats
	mux.HandleFunc("/api/v1/external/stats", h.handleStats)
}

// handleServices handles GET/POST /api/v1/services
func (h *Handlers) handleServices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listServices(w, r)
	case http.MethodPost:
		h.createService(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listServices returns all registered services.
func (h *Handlers) listServices(w http.ResponseWriter, r *http.Request) {
	// Check for zone filter
	zone := r.URL.Query().Get("zone")
	tag := r.URL.Query().Get("tag")
	healthyOnly := r.URL.Query().Get("healthy") == "true"

	var services []*Service

	if zone != "" {
		// Get roles from header (set by auth middleware)
		rolesHeader := r.Header.Get("X-LocalMesh-Roles")
		var roles []string
		if rolesHeader != "" {
			roles = strings.Split(rolesHeader, ",")
		}
		services = h.registry.ListByZone(zone, roles)
	} else if tag != "" {
		services = h.registry.ListByTag(tag)
	} else if healthyOnly {
		services = h.registry.ListHealthy()
	} else {
		services = h.registry.List()
	}

	resp := ServicesListResponse{
		Services: services,
		Stats:    h.registry.Stats(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// createService registers a new service.
func (h *Handlers) createService(w http.ResponseWriter, r *http.Request) {
	var req ServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Create service
	svc := NewService(req.Name, req.DisplayName, req.URL)
	svc.Info.Description = req.Description
	svc.Info.Tags = req.Tags

	if req.HealthPath != "" {
		svc.Endpoint.HealthPath = req.HealthPath
	}

	svc.Access.Zones = req.Zones
	svc.Access.Roles = req.Roles
	svc.Access.RequireAuth = req.RequireAuth
	svc.Access.Public = req.Public

	// Register
	if err := h.registry.Register(svc); err != nil {
		h.writeError(w, err.Error(), http.StatusConflict)
		return
	}

	// Advertise via mDNS if discovery is running
	if h.discovery != nil {
		h.discovery.AdvertiseService(svc.Info.Name)
	}

	h.logger.Info().
		Str("service", svc.Info.Name).
		Str("url", svc.Endpoint.URL).
		Msg("Service created via API")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ServiceResponse{
		Service: svc,
		Message: "Service registered successfully",
	})
}

// handleService handles GET/PUT/DELETE /api/v1/external/services/{name}
func (h *Handlers) handleService(w http.ResponseWriter, r *http.Request) {
	// Extract service name from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/external/services/")
	parts := strings.Split(path, "/")
	name := parts[0]

	if name == "" {
		http.Error(w, "Service name required", http.StatusBadRequest)
		return
	}

	// Check for sub-resources
	if len(parts) > 1 {
		switch parts[1] {
		case "health":
			h.handleServiceHealth(w, r, name)
			return
		case "metrics":
			h.handleServiceMetrics(w, r, name)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		h.getService(w, r, name)
	case http.MethodPut:
		h.updateService(w, r, name)
	case http.MethodDelete:
		h.deleteService(w, r, name)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getService returns a single service.
func (h *Handlers) getService(w http.ResponseWriter, r *http.Request, name string) {
	svc, exists := h.registry.Get(name)
	if !exists {
		h.writeError(w, "Service not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ServiceResponse{Service: svc})
}

// updateService updates an existing service.
func (h *Handlers) updateService(w http.ResponseWriter, r *http.Request, name string) {
	var req ServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.registry.Update(name, func(svc *Service) error {
		if req.DisplayName != "" {
			svc.Info.DisplayName = req.DisplayName
		}
		if req.Description != "" {
			svc.Info.Description = req.Description
		}
		if req.URL != "" {
			svc.Endpoint.URL = req.URL
		}
		if req.HealthPath != "" {
			svc.Endpoint.HealthPath = req.HealthPath
		}
		if req.Zones != nil {
			svc.Access.Zones = req.Zones
		}
		if req.Roles != nil {
			svc.Access.Roles = req.Roles
		}
		svc.Access.RequireAuth = req.RequireAuth
		svc.Access.Public = req.Public
		if req.Tags != nil {
			svc.Info.Tags = req.Tags
		}
		return nil
	})

	if err != nil {
		h.writeError(w, err.Error(), http.StatusNotFound)
		return
	}

	svc, _ := h.registry.Get(name)

	h.logger.Info().
		Str("service", name).
		Msg("Service updated via API")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ServiceResponse{
		Service: svc,
		Message: "Service updated successfully",
	})
}

// deleteService removes a service.
func (h *Handlers) deleteService(w http.ResponseWriter, r *http.Request, name string) {
	if err := h.registry.Unregister(name); err != nil {
		h.writeError(w, err.Error(), http.StatusNotFound)
		return
	}

	h.logger.Info().
		Str("service", name).
		Msg("Service deleted via API")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ServiceResponse{
		Message: "Service unregistered successfully",
	})
}

// handleServiceHealth handles health check for a service.
func (h *Handlers) handleServiceHealth(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method == http.MethodPost {
		// Trigger a health check
		if err := h.registry.CheckHealth(r.Context(), name); err != nil {
			h.writeError(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
	}

	svc, exists := h.registry.Get(name)
	if !exists {
		h.writeError(w, "Service not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(svc.Health)
}

// handleServiceMetrics handles metrics for a service.
func (h *Handlers) handleServiceMetrics(w http.ResponseWriter, r *http.Request, name string) {
	svc, exists := h.registry.Get(name)
	if !exists {
		h.writeError(w, "Service not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(svc.Metrics)
}

// handleStats returns registry statistics.
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := h.registry.Stats()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleDiscoveredNodes returns discovered LocalMesh nodes.
func (h *Handlers) handleDiscoveredNodes(w http.ResponseWriter, r *http.Request) {
	if h.discovery == nil {
		h.writeError(w, "Discovery not enabled", http.StatusServiceUnavailable)
		return
	}

	nodes := h.discovery.ListDiscovered()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"nodes":      nodes,
		"count":      len(nodes),
		"discovered": time.Now(),
	})
}

// handleDiscoveryScan triggers an immediate mDNS scan.
func (h *Handlers) handleDiscoveryScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.discovery == nil {
		h.writeError(w, "Discovery not enabled", http.StatusServiceUnavailable)
		return
	}

	// Trigger scan
	go h.discovery.scan()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Scan initiated",
		"time":    time.Now(),
	})
}

// writeError writes an error response.
func (h *Handlers) writeError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ServiceResponse{Error: message})
}

// BrowsableServicesHandler returns a handler that shows available services
// based on the user's zone. This is the "discovery" endpoint for clients.
func (h *Handlers) BrowsableServicesHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get zone from auth context (set by middleware)
		zone := r.Header.Get("X-LocalMesh-Zone")
		rolesHeader := r.Header.Get("X-LocalMesh-Roles")

		var roles []string
		if rolesHeader != "" {
			roles = strings.Split(rolesHeader, ",")
		}

		// Get services accessible to this user
		services := h.registry.ListByZone(zone, roles)

		// Filter to only healthy services
		healthy := make([]*ServiceBrowseInfo, 0)
		for _, svc := range services {
			if svc.IsHealthy() {
				healthy = append(healthy, &ServiceBrowseInfo{
					Name:        svc.Info.Name,
					DisplayName: svc.Info.DisplayName,
					Description: svc.Info.Description,
					Tags:        svc.Info.Tags,
					Version:     svc.Info.Version,
					URL:         "/svc/" + svc.Info.Name,
					Healthy:     true,
					Latency:     svc.Health.Latency.Milliseconds(),
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"services": healthy,
			"zone":     zone,
			"count":    len(healthy),
		})
	}
}

// ServiceBrowseInfo is the simplified service info for browsing.
type ServiceBrowseInfo struct {
	Name        string   `json:"name"`
	DisplayName string   `json:"display_name"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Version     string   `json:"version"`
	URL         string   `json:"url"` // Relative URL to access the service
	Healthy     bool     `json:"healthy"`
	Latency     int64    `json:"latency_ms"`
}
