// Package gateway provides the HTTP API gateway for LocalMesh.
package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// MDNSService represents a service advertised via mDNS
type MDNSService struct {
	Name         string            `json:"name"`
	Port         int               `json:"port"`
	IP           string            `json:"ip"`
	Hostname     string            `json:"hostname"`
	URL          string            `json:"url"`
	Description  string            `json:"description"`
	Tags         []string          `json:"tags"`
	Metadata     map[string]string `json:"metadata"`
	Healthy      bool              `json:"healthy"`
	RegisteredAt time.Time         `json:"registered_at"`
}

// Gateway is the HTTP API gateway
type Gateway struct {
	server      *http.Server
	proxyServer *http.Server // Reverse proxy on port 80
	mux         *http.ServeMux

	// mDNS services
	services   map[string]*MDNSService
	processes  map[string]*exec.Cmd
	serverMDNS *exec.Cmd // mDNS advertisement for the server itself
	mu         sync.RWMutex

	// Configuration
	host         string
	port         int
	proxyPort    int // Port for reverse proxy (default 80)
	domain       string
	readTimeout  time.Duration
	writeTimeout time.Duration

	logger *slog.Logger
}

// GatewayConfig configures the gateway
type GatewayConfig struct {
	Host         string
	Port         int
	ProxyPort    int // Port for reverse proxy (default 80)
	Domain       string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	Logger       *slog.Logger
}

// DefaultGatewayConfig returns sensible defaults
func DefaultGatewayConfig() GatewayConfig {
	return GatewayConfig{
		Host:         "0.0.0.0",
		Port:         8080,
		ProxyPort:    8081, // Higher port so no sudo needed
		Domain:       "campus",
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}
}

// NewGateway creates a new HTTP gateway
func NewGateway(cfg GatewayConfig) *Gateway {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	proxyPort := cfg.ProxyPort
	if proxyPort == 0 {
		proxyPort = 80
	}

	g := &Gateway{
		mux:          http.NewServeMux(),
		services:     make(map[string]*MDNSService),
		processes:    make(map[string]*exec.Cmd),
		host:         cfg.Host,
		port:         cfg.Port,
		proxyPort:    proxyPort,
		domain:       cfg.Domain,
		readTimeout:  cfg.ReadTimeout,
		writeTimeout: cfg.WriteTimeout,
		logger:       logger,
	}

	g.setupRoutes()
	return g
}

func (g *Gateway) setupRoutes() {
	// Health check
	g.mux.HandleFunc("GET /health", g.handleHealth)

	// Service registration API
	g.mux.HandleFunc("POST /api/v1/services/register", g.handleRegister)
	g.mux.HandleFunc("POST /api/v1/services/unregister", g.handleUnregister)
	g.mux.HandleFunc("GET /api/v1/services", g.handleListServices)
	g.mux.HandleFunc("GET /api/v1/services/{name}", g.handleGetService)

	// Fallback
	g.mux.HandleFunc("/", g.handleNotFound)
}

// Start begins listening for HTTP requests
func (g *Gateway) Start() error {
	addr := fmt.Sprintf("%s:%d", g.host, g.port)

	g.server = &http.Server{
		Addr:         addr,
		Handler:      g.mux,
		ReadTimeout:  g.readTimeout,
		WriteTimeout: g.writeTimeout,
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	g.logger.Info("gateway started", "addr", addr)

	// Advertise LocalMesh server via mDNS so agents can discover it
	if err := g.advertiseServer(); err != nil {
		g.logger.Warn("failed to advertise server via mDNS", "error", err)
	}

	go func() {
		if err := g.server.Serve(listener); err != http.ErrServerClosed {
			g.logger.Error("gateway error", "error", err)
		}
	}()

	// Start reverse proxy on port 80 (or configured proxy port)
	if err := g.startReverseProxy(); err != nil {
		g.logger.Warn("failed to start reverse proxy", "error", err, "port", g.proxyPort)
	}

	return nil
}

// startReverseProxy starts an HTTP reverse proxy on port 80
// This allows users to access services like http://myapp.local without specifying a port
func (g *Gateway) startReverseProxy() error {
	proxyAddr := fmt.Sprintf("%s:%d", g.host, g.proxyPort)

	proxyHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract service name from Host header (e.g., "myapp.local" -> "myapp")
		host := r.Host
		if colonIdx := strings.Index(host, ":"); colonIdx != -1 {
			host = host[:colonIdx]
		}

		// Remove .local suffix to get service name
		serviceName := strings.TrimSuffix(host, ".local")

		// Look up the service
		g.mu.RLock()
		svc, exists := g.services[serviceName]
		g.mu.RUnlock()

		if !exists {
			http.Error(w, fmt.Sprintf("Service %q not found", serviceName), http.StatusNotFound)
			return
		}

		// Create reverse proxy to the actual service
		target, err := url.Parse(fmt.Sprintf("http://%s:%d", svc.IP, svc.Port))
		if err != nil {
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.ServeHTTP(w, r)
	})

	g.proxyServer = &http.Server{
		Addr:         proxyAddr,
		Handler:      proxyHandler,
		ReadTimeout:  g.readTimeout,
		WriteTimeout: g.writeTimeout,
	}

	listener, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on proxy port %d: %w (try running with sudo or use a port > 1024)", g.proxyPort, err)
	}

	g.logger.Info("reverse proxy started", "addr", proxyAddr)

	go func() {
		if err := g.proxyServer.Serve(listener); err != http.ErrServerClosed {
			g.logger.Error("proxy error", "error", err)
		}
	}()

	return nil
}

// advertiseServer advertises the LocalMesh server via mDNS using avahi-publish-service
func (g *Gateway) advertiseServer() error {
	ip, err := detectIP()
	if err != nil {
		return fmt.Errorf("failed to detect IP: %w", err)
	}

	// Use avahi-publish-service to advertise _localmesh._tcp service
	// This allows agents to discover the server automatically
	cmd := exec.Command("avahi-publish-service",
		"localmesh",               // service name
		"_localmesh._tcp",         // service type
		fmt.Sprintf("%d", g.port), // port
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start avahi-publish-service: %w", err)
	}

	g.serverMDNS = cmd
	g.logger.Info("server mDNS advertised", "service", "_localmesh._tcp", "port", g.port, "ip", ip)
	return nil
}

// Stop gracefully shuts down the gateway
func (g *Gateway) Stop(ctx context.Context) error {
	// Stop server mDNS advertisement
	if g.serverMDNS != nil && g.serverMDNS.Process != nil {
		g.serverMDNS.Process.Kill()
		g.serverMDNS = nil
	}

	// Stop all service mDNS advertisements
	g.mu.Lock()
	for name, cmd := range g.processes {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		delete(g.processes, name)
	}
	g.mu.Unlock()

	// Stop reverse proxy
	if g.proxyServer != nil {
		g.proxyServer.Shutdown(ctx)
	}

	if g.server == nil {
		return nil
	}
	return g.server.Shutdown(ctx)
}

// Mux returns the underlying http.ServeMux
func (g *Gateway) Mux() *http.ServeMux {
	return g.mux
}

// ServiceDiscovery returns the gateway itself for mDNS advertising
func (g *Gateway) ServiceDiscovery() *Gateway {
	return g
}

// RegisterService registers a service (for config-based services)
func (g *Gateway) RegisterService(svc interface{}) error {
	// Placeholder for external service registration
	return nil
}

// StartServiceDiscovery is a no-op for compatibility
func (g *Gateway) StartServiceDiscovery(instanceName string, port int, advertise bool) error {
	return nil
}

// StartHealthChecks is a no-op for compatibility
func (g *Gateway) StartHealthChecks(interval time.Duration) {
	// No-op
}

// AdvertiseExternalService advertises a service via mDNS using avahi-publish-address
func (g *Gateway) AdvertiseExternalService(name, serviceType string, port int, hostIP string, txtRecords map[string]string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check if already registered
	if _, exists := g.services[name]; exists {
		return fmt.Errorf("service %q already registered", name)
	}

	// Get IP if not provided
	ip := hostIP
	if ip == "" {
		var err error
		ip, err = detectIP()
		if err != nil {
			return fmt.Errorf("failed to detect IP: %w", err)
		}
	}

	// Build hostname
	hostname := fmt.Sprintf("%s.local", name)
	url := fmt.Sprintf("http://%s", hostname) // No port needed - reverse proxy handles it

	// Start avahi-publish-address
	cmd := exec.Command("avahi-publish-address", "-R", hostname, ip)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start avahi-publish-address: %w", err)
	}

	// Track service
	svc := &MDNSService{
		Name:         name,
		Port:         port,
		IP:           ip,
		Hostname:     hostname,
		URL:          url,
		Metadata:     txtRecords,
		Healthy:      true,
		RegisteredAt: time.Now(),
	}

	if desc, ok := txtRecords["description"]; ok {
		svc.Description = desc
	}

	g.services[name] = svc
	g.processes[name] = cmd

	g.logger.Info("mDNS advertised", "name", name, "hostname", hostname, "ip", ip, "port", port)
	return nil
}

// StopAdvertisingService stops mDNS advertisement for a service
func (g *Gateway) StopAdvertisingService(name string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	cmd, exists := g.processes[name]
	if !exists {
		return fmt.Errorf("service %q not found", name)
	}

	if cmd.Process != nil {
		cmd.Process.Kill()
	}

	delete(g.processes, name)
	delete(g.services, name)

	g.logger.Info("mDNS stopped", "name", name)
	return nil
}

// --- Handlers ---

func (g *Gateway) handleHealth(w http.ResponseWriter, r *http.Request) {
	g.jsonResponse(w, http.StatusOK, map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (g *Gateway) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string            `json:"name"`
		Port        int               `json:"port"`
		IP          string            `json:"ip"`
		Description string            `json:"description"`
		Tags        []string          `json:"tags"`
		Metadata    map[string]string `json:"metadata"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		g.jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.Port <= 0 || req.Port > 65535 {
		g.jsonError(w, http.StatusBadRequest, "port must be between 1 and 65535")
		return
	}

	// Build txt records
	txtRecords := req.Metadata
	if txtRecords == nil {
		txtRecords = make(map[string]string)
	}
	if req.Description != "" {
		txtRecords["description"] = req.Description
	}

	if err := g.AdvertiseExternalService(req.Name, "_http._tcp", req.Port, req.IP, txtRecords); err != nil {
		g.jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	g.mu.RLock()
	svc := g.services[req.Name]
	g.mu.RUnlock()

	g.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success":  true,
		"hostname": svc.Hostname,
		"url":      svc.URL,
		"message":  fmt.Sprintf("Service %s registered at %s", req.Name, svc.Hostname),
	})
}

func (g *Gateway) handleUnregister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		g.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		g.jsonError(w, http.StatusBadRequest, "name is required")
		return
	}

	if err := g.StopAdvertisingService(req.Name); err != nil {
		g.jsonError(w, http.StatusNotFound, err.Error())
		return
	}

	g.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Service %s unregistered", req.Name),
	})
}

func (g *Gateway) handleListServices(w http.ResponseWriter, r *http.Request) {
	g.mu.RLock()
	services := make([]*MDNSService, 0, len(g.services))
	for _, svc := range g.services {
		services = append(services, svc)
	}
	g.mu.RUnlock()

	g.jsonResponse(w, http.StatusOK, map[string]interface{}{
		"services": services,
		"count":    len(services),
	})
}

func (g *Gateway) handleGetService(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		g.jsonError(w, http.StatusBadRequest, "service name required")
		return
	}

	g.mu.RLock()
	svc, exists := g.services[name]
	g.mu.RUnlock()

	if !exists {
		g.jsonError(w, http.StatusNotFound, "service not found")
		return
	}

	g.jsonResponse(w, http.StatusOK, svc)
}

func (g *Gateway) handleNotFound(w http.ResponseWriter, r *http.Request) {
	g.jsonError(w, http.StatusNotFound, "not found")
}

func (g *Gateway) jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (g *Gateway) jsonError(w http.ResponseWriter, status int, message string) {
	g.jsonResponse(w, status, map[string]string{"error": message})
}

// detectIP returns the local IP address
func detectIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no suitable IP address found")
}
