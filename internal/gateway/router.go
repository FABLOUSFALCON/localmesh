// Package gateway provides the HTTP API gateway for LocalMesh.
// Routes requests to services with zone-based access control.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/FABLOUSFALCON/localmesh/internal/auth"
	"github.com/FABLOUSFALCON/localmesh/internal/registry"
	"github.com/FABLOUSFALCON/localmesh/internal/services"
	"github.com/rs/zerolog"
)

// Gateway is the HTTP API gateway
type Gateway struct {
	server   *http.Server
	registry *registry.Registry
	auth     *auth.Service
	mux      *http.ServeMux

	// External services
	serviceRegistry  *services.Registry
	serviceProxy     *services.Proxy
	serviceDiscovery *services.Discovery
	serviceHandlers  *services.Handlers

	// Middleware chain
	middlewares []Middleware

	// Configuration
	host         string
	port         int
	readTimeout  time.Duration
	writeTimeout time.Duration

	// Control
	mu     sync.RWMutex
	logger *slog.Logger
}

// Middleware is a function that wraps an http.Handler
type Middleware func(http.Handler) http.Handler

// GatewayConfig configures the gateway
type GatewayConfig struct {
	Host         string
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	Registry     *registry.Registry
	Auth         *auth.Service
	Logger       *slog.Logger
}

// DefaultGatewayConfig returns sensible defaults
func DefaultGatewayConfig() GatewayConfig {
	return GatewayConfig{
		Host:         "0.0.0.0",
		Port:         8080,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
}

// NewGateway creates a new HTTP gateway
func NewGateway(cfg GatewayConfig) *Gateway {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	g := &Gateway{
		registry:     cfg.Registry,
		auth:         cfg.Auth,
		mux:          http.NewServeMux(),
		host:         cfg.Host,
		port:         cfg.Port,
		readTimeout:  cfg.ReadTimeout,
		writeTimeout: cfg.WriteTimeout,
		logger:       logger,
	}

	// Initialize external services registry
	zlogger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()
	g.serviceRegistry = services.NewRegistry(zlogger)
	g.serviceProxy = services.NewProxy(g.serviceRegistry, zlogger)

	// Setup routes
	g.setupRoutes()

	return g
}

// setupRoutes configures the HTTP routes
func (g *Gateway) setupRoutes() {
	// Health check (no auth required)
	g.mux.HandleFunc("GET /health", g.handleHealth)
	g.mux.HandleFunc("GET /ready", g.handleReady)

	// Auth endpoints (handled separately)
	if g.auth != nil {
		authHandler := auth.NewHandler(g.auth)
		authHandler.RegisterRoutes(g.mux)
	}

	// API v1
	g.mux.HandleFunc("GET /api/v1/services", g.handleListServices)
	g.mux.HandleFunc("GET /api/v1/services/{id}", g.handleGetService)
	g.mux.HandleFunc("POST /api/v1/services", g.handleRegisterService)
	g.mux.HandleFunc("DELETE /api/v1/services/{id}", g.handleDeregisterService)

	g.mux.HandleFunc("GET /api/v1/nodes", g.handleListNodes)

	// Only register zones handler if auth is not handling it
	if g.auth == nil {
		g.mux.HandleFunc("GET /api/v1/zones", g.handleListZones)
	}

	g.mux.HandleFunc("GET /api/v1/status", g.handleStatus)
	g.mux.HandleFunc("GET /api/v1/stats", g.handleStats)

	// Service proxy (routes requests to services)
	g.mux.HandleFunc("/svc/{service}/", g.handleServiceProxy)

	// Fallback
	g.mux.HandleFunc("/", g.handleNotFound)
}

// Use adds a middleware to the chain
func (g *Gateway) Use(mw Middleware) {
	g.middlewares = append(g.middlewares, mw)
}

// Start begins listening for HTTP requests
func (g *Gateway) Start() error {
	addr := fmt.Sprintf("%s:%d", g.host, g.port)

	// Build middleware chain
	var handler http.Handler = g.mux
	for i := len(g.middlewares) - 1; i >= 0; i-- {
		handler = g.middlewares[i](handler)
	}

	// Add default middlewares (order matters: last added = first executed)
	handler = g.recoveryMiddleware(handler)
	handler = g.loggingMiddleware(handler)
	handler = g.corsMiddleware(handler)

	// Add security headers middleware
	handler = SecurityMiddleware(APISecurityConfig())(handler)

	// Add auth middleware if available
	if g.auth != nil {
		authMiddleware := auth.NewMiddleware(g.auth)
		handler = authMiddleware.Authenticate(handler)
	}

	g.server = &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  g.readTimeout,
		WriteTimeout: g.writeTimeout,
		IdleTimeout:  120 * time.Second,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	g.logger.Info("gateway started",
		"addr", addr,
	)

	go func() {
		if err := g.server.Serve(listener); err != http.ErrServerClosed {
			g.logger.Error("gateway error", "error", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the gateway
func (g *Gateway) Stop(ctx context.Context) error {
	if g.server == nil {
		return nil
	}

	g.logger.Info("gateway stopping")
	return g.server.Shutdown(ctx)
}

// Addr returns the gateway's listen address
func (g *Gateway) Addr() string {
	return fmt.Sprintf("%s:%d", g.host, g.port)
}

// Mux returns the underlying http.ServeMux for registering additional routes
func (g *Gateway) Mux() *http.ServeMux {
	return g.mux
}

// ServiceRegistry returns the external services registry
func (g *Gateway) ServiceRegistry() *services.Registry {
	return g.serviceRegistry
}

// RegisterService registers an external service with the gateway
func (g *Gateway) RegisterService(svc *services.Service) error {
	return g.serviceRegistry.Register(svc)
}

// StartServiceDiscovery starts mDNS discovery for external services
func (g *Gateway) StartServiceDiscovery(instanceName string, port int, advertise bool) error {
	zlogger := zerolog.New(zerolog.NewConsoleWriter()).With().Timestamp().Logger()
	g.serviceDiscovery = services.NewDiscovery(g.serviceRegistry, zlogger, services.DiscoveryConfig{
		InstanceName: instanceName,
		Port:         port,
		Advertise:    advertise,
	})

	// Create and register handlers for external services API
	g.serviceHandlers = services.NewHandlers(g.serviceRegistry, g.serviceDiscovery, g.serviceProxy, zlogger)
	g.serviceHandlers.RegisterRoutes(g.mux)

	// Also register the browsable endpoint for clients to discover available services
	g.mux.HandleFunc("GET /api/v1/browse", g.serviceHandlers.BrowsableServicesHandler())

	return g.serviceDiscovery.Start()
}

// StartHealthChecks starts periodic health checks for external services
func (g *Gateway) StartHealthChecks(interval time.Duration) {
	g.serviceRegistry.StartHealthChecks(interval)
}

// --- Handlers ---

func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	g.jsonResponse(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (g *Gateway) handleReady(w http.ResponseWriter, r *http.Request) {
	// Check if essential services are ready
	ready := true
	checks := map[string]bool{
		"registry": g.registry != nil,
	}

	for _, ok := range checks {
		if !ok {
			ready = false
			break
		}
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}

	g.jsonResponse(w, status, map[string]any{
		"ready":  ready,
		"checks": checks,
	})
}

func (g *Gateway) handleListServices(w http.ResponseWriter, r *http.Request) {
	if g.registry == nil {
		g.jsonError(w, http.StatusServiceUnavailable, "registry not available")
		return
	}

	zone := r.URL.Query().Get("zone")
	name := r.URL.Query().Get("name")
	healthyOnly := r.URL.Query().Get("healthy") == "true"

	var services []*registry.ServiceEntry

	if healthyOnly {
		services = g.registry.GetHealthy()
	} else if zone != "" {
		services = g.registry.GetByZone(zone)
	} else if name != "" {
		services = g.registry.GetByName(name)
	} else {
		services = g.registry.All()
	}

	g.jsonResponse(w, http.StatusOK, map[string]any{
		"services": services,
		"count":    len(services),
	})
}

func (g *Gateway) handleGetService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		g.jsonError(w, http.StatusBadRequest, "service id required")
		return
	}

	svc, exists := g.registry.Get(id)
	if !exists {
		g.jsonError(w, http.StatusNotFound, "service not found")
		return
	}

	g.jsonResponse(w, http.StatusOK, svc)
}

func (g *Gateway) handleRegisterService(w http.ResponseWriter, r *http.Request) {
	// TODO: Implement service registration via API
	g.jsonError(w, http.StatusNotImplemented, "use plugin SDK to register services")
}

func (g *Gateway) handleDeregisterService(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		g.jsonError(w, http.StatusBadRequest, "service id required")
		return
	}

	if err := g.registry.Deregister(id); err != nil {
		g.jsonError(w, http.StatusNotFound, err.Error())
		return
	}

	g.jsonResponse(w, http.StatusOK, map[string]string{
		"status": "deregistered",
		"id":     id,
	})
}

func (g *Gateway) handleListNodes(w http.ResponseWriter, r *http.Request) {
	// TODO: Get nodes from discovery service
	g.jsonResponse(w, http.StatusOK, map[string]any{
		"nodes": []any{},
		"count": 0,
	})
}

func (g *Gateway) handleListZones(w http.ResponseWriter, r *http.Request) {
	// TODO: Get zones from config/storage
	g.jsonResponse(w, http.StatusOK, map[string]any{
		"zones": []any{},
		"count": 0,
	})
}

func (g *Gateway) handleStatus(w http.ResponseWriter, r *http.Request) {
	serviceCount := 0
	if g.registry != nil {
		serviceCount = g.registry.Count()
	}

	g.jsonResponse(w, http.StatusOK, map[string]any{
		"status":        "running",
		"version":       "0.1.0",
		"uptime":        "TODO",
		"service_count": serviceCount,
	})
}

func (g *Gateway) handleStats(w http.ResponseWriter, r *http.Request) {
	g.jsonResponse(w, http.StatusOK, map[string]any{
		"requests_total":  0, // TODO: Track metrics
		"requests_active": 0,
		"services":        g.registry.Count(),
	})
}

func (g *Gateway) handleServiceProxy(w http.ResponseWriter, r *http.Request) {
	serviceName := r.PathValue("service")
	if serviceName == "" {
		g.jsonError(w, http.StatusBadRequest, "service name required")
		return
	}

	// First check external services (priority)
	if _, exists := g.serviceRegistry.Get(serviceName); exists {
		// Build proxy context from auth headers
		pctx := &services.ProxyRequest{
			UserID:   r.Header.Get("X-User-ID"),
			Username: r.Header.Get("X-Username"),
			Zone:     r.Header.Get("X-Zone"),
		}
		if roles := r.Header.Get("X-Roles"); roles != "" {
			pctx.Roles = strings.Split(roles, ",")
		}
		if r.Header.Get("X-Network-Verified") == "true" {
			pctx.Verified = true
		}
		pctx.NetworkID = r.Header.Get("X-Network-ID")
		pctx.SessionID = r.Header.Get("X-Session-ID")

		// Proxy the request
		g.serviceProxy.ServeHTTP(w, r, serviceName, pctx)
		return
	}

	// Fall back to internal service registry
	svcs := g.registry.GetByName(serviceName)
	if len(svcs) == 0 {
		g.jsonError(w, http.StatusNotFound, fmt.Sprintf("service %q not found", serviceName))
		return
	}

	// Get first healthy instance
	var targetSvc *registry.ServiceEntry
	for _, s := range svcs {
		if s.Status == registry.StatusHealthy {
			targetSvc = s
			break
		}
	}

	if targetSvc == nil {
		g.jsonError(w, http.StatusServiceUnavailable, "no healthy instances available")
		return
	}

	// TODO: Implement internal service proxying
	g.jsonResponse(w, http.StatusOK, map[string]any{
		"message":   "internal service proxy - implementation pending",
		"service":   serviceName,
		"instances": len(svcs),
		"target":    targetSvc.Host,
	})
}

func (g *Gateway) handleNotFound(w http.ResponseWriter, r *http.Request) {
	g.jsonError(w, http.StatusNotFound, "endpoint not found")
}

// --- Middlewares ---

func (g *Gateway) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status
		wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		g.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", wrapped.status,
			"duration", time.Since(start).String(),
			"ip", getClientIP(r),
		)
	})
}

func (g *Gateway) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				g.logger.Error("panic recovered",
					"error", err,
					"path", r.URL.Path,
				)
				g.jsonError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (g *Gateway) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// --- Helpers ---

func (g *Gateway) jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (g *Gateway) jsonError(w http.ResponseWriter, status int, message string) {
	g.jsonResponse(w, status, map[string]string{
		"error": message,
	})
}

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	// Check X-Real-IP header
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
