package services

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"github.com/rs/zerolog"
)

// Proxy handles reverse proxying requests to external services.
// It injects authentication headers, validates zone access, and
// provides metrics and circuit breaking.
type Proxy struct {
	registry  *Registry
	logger    zerolog.Logger
	transport *http.Transport

	// Circuit breaker state per service
	circuitBreakers map[string]*circuitBreaker
}

// circuitBreaker tracks failure state for a service.
type circuitBreaker struct {
	failures     int64
	lastFailure  time.Time
	state        string // "closed", "open", "half-open"
	threshold    int64
	resetTimeout time.Duration
}

// ProxyRequest contains context for a proxy request.
type ProxyRequest struct {
	UserID    string
	Username  string
	Zone      string
	Roles     []string
	SessionID string
	NetworkID string // WiFi SSID
	Verified  bool   // Network identity verified
}

// NewProxy creates a new service proxy.
func NewProxy(registry *Registry, logger zerolog.Logger) *Proxy {
	return &Proxy{
		registry: registry,
		logger:   logger.With().Str("component", "service-proxy").Logger(),
		transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   20,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			// Optimized for LAN - larger buffers
			WriteBufferSize: 64 * 1024, // 64KB
			ReadBufferSize:  64 * 1024, // 64KB
		},
		circuitBreakers: make(map[string]*circuitBreaker),
	}
}

// Handler returns an HTTP handler that proxies to a specific service.
func (p *Proxy) Handler(serviceName string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.ServeHTTP(w, r, serviceName, nil)
	})
}

// HandlerWithAuth returns a handler that requires authentication context.
func (p *Proxy) HandlerWithAuth(serviceName string, getContext func(*http.Request) *ProxyRequest) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := getContext(r)
		p.ServeHTTP(w, r, serviceName, ctx)
	})
}

// ServeHTTP handles the actual proxying.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request, serviceName string, pctx *ProxyRequest) {
	start := time.Now()

	// Get service
	svc, exists := p.registry.Get(serviceName)
	if !exists {
		p.logger.Warn().
			Str("service", serviceName).
			Msg("Service not found")
		http.Error(w, "Service not found", http.StatusNotFound)
		return
	}

	// Check circuit breaker
	if p.isCircuitOpen(serviceName) {
		p.logger.Warn().
			Str("service", serviceName).
			Msg("Circuit breaker open")
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}

	// Check zone access
	if pctx != nil && !svc.Access.Public {
		if svc.Access.RequireAuth && pctx.UserID == "" {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}

		if !svc.CanAccess(pctx.Zone, pctx.Roles) {
			p.logger.Warn().
				Str("service", serviceName).
				Str("user", pctx.UserID).
				Str("zone", pctx.Zone).
				Strs("required_zones", svc.Access.Zones).
				Msg("Access denied - zone mismatch")
			http.Error(w, "Access denied - zone mismatch", http.StatusForbidden)
			return
		}
	}

	// Parse target URL
	targetURL, err := svc.GetProxyURL()
	if err != nil {
		p.logger.Error().Err(err).Str("service", serviceName).Msg("Invalid service URL")
		http.Error(w, "Service configuration error", http.StatusInternalServerError)
		return
	}

	// Create reverse proxy
	proxy := p.createReverseProxy(targetURL, svc, pctx)

	// Serve request
	proxy.ServeHTTP(w, r)

	// Record metrics
	latency := time.Since(start)
	svc.RecordRequest(true, 0, latency)

	p.logger.Debug().
		Str("service", serviceName).
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Dur("latency", latency).
		Msg("Request proxied")
}

// createReverseProxy creates a configured reverse proxy.
func (p *Proxy) createReverseProxy(target *url.URL, svc *Service, pctx *ProxyRequest) *httputil.ReverseProxy {
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = p.transport

	// Custom director to modify the request
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		// Strip the /svc/{name} prefix if configured
		if svc.Config.StripPrefix {
			// Path comes in as /svc/{name}/actual/path
			// We need to extract just /actual/path
			parts := strings.SplitN(req.URL.Path, "/", 4)
			if len(parts) >= 4 {
				req.URL.Path = "/" + parts[3]
			} else {
				req.URL.Path = "/"
			}
		}

		// Inject LocalMesh headers
		req.Header.Set("X-LocalMesh-Service", svc.Info.Name)
		req.Header.Set("X-Forwarded-Proto", "http")
		req.Header.Set("X-Real-IP", req.RemoteAddr)

		// Inject auth context headers if available
		if pctx != nil {
			if pctx.UserID != "" {
				req.Header.Set("X-LocalMesh-User-ID", pctx.UserID)
			}
			if pctx.Username != "" {
				req.Header.Set("X-LocalMesh-Username", pctx.Username)
			}
			if pctx.Zone != "" {
				req.Header.Set("X-LocalMesh-Zone", pctx.Zone)
			}
			if len(pctx.Roles) > 0 {
				req.Header.Set("X-LocalMesh-Roles", strings.Join(pctx.Roles, ","))
			}
			if pctx.SessionID != "" {
				req.Header.Set("X-LocalMesh-Session-ID", pctx.SessionID)
			}
			if pctx.NetworkID != "" {
				req.Header.Set("X-LocalMesh-Network-ID", pctx.NetworkID)
			}
			if pctx.Verified {
				req.Header.Set("X-LocalMesh-Verified", "true")
			}
		}

		// Remove hop-by-hop headers
		req.Header.Del("Connection")
		req.Header.Del("Proxy-Connection")
		req.Header.Del("Keep-Alive")
		req.Header.Del("Transfer-Encoding")
		req.Header.Del("Upgrade")
	}

	// Custom error handler
	proxy.ErrorHandler = func(w http.ResponseWriter, req *http.Request, err error) {
		p.logger.Error().
			Err(err).
			Str("service", svc.Info.Name).
			Str("path", req.URL.Path).
			Msg("Proxy error")

		p.recordFailure(svc.Info.Name)
		svc.RecordRequest(false, 0, 0)

		http.Error(w, "Service unavailable", http.StatusBadGateway)
	}

	// Modify response headers
	proxy.ModifyResponse = func(resp *http.Response) error {
		// Add service identification header
		resp.Header.Set("X-Served-By", "LocalMesh")
		resp.Header.Set("X-LocalMesh-Service", svc.Info.Name)

		// Record success
		p.resetCircuit(svc.Info.Name)

		return nil
	}

	return proxy
}

// Circuit breaker methods

func (p *Proxy) getCircuitBreaker(name string) *circuitBreaker {
	if cb, exists := p.circuitBreakers[name]; exists {
		return cb
	}

	cb := &circuitBreaker{
		state:        "closed",
		threshold:    5,
		resetTimeout: 30 * time.Second,
	}
	p.circuitBreakers[name] = cb
	return cb
}

func (p *Proxy) isCircuitOpen(name string) bool {
	cb := p.getCircuitBreaker(name)

	if cb.state == "open" {
		// Check if we should try half-open
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.state = "half-open"
			return false
		}
		return true
	}
	return false
}

func (p *Proxy) recordFailure(name string) {
	cb := p.getCircuitBreaker(name)
	atomic.AddInt64(&cb.failures, 1)
	cb.lastFailure = time.Now()

	if cb.failures >= cb.threshold {
		cb.state = "open"
		p.logger.Warn().
			Str("service", name).
			Int64("failures", cb.failures).
			Msg("Circuit breaker opened")
	}
}

func (p *Proxy) resetCircuit(name string) {
	cb := p.getCircuitBreaker(name)
	if cb.state == "half-open" {
		cb.state = "closed"
		atomic.StoreInt64(&cb.failures, 0)
		p.logger.Info().
			Str("service", name).
			Msg("Circuit breaker reset")
	}
}

// StreamProxy provides optimized streaming for large data transfers.
// This is useful for live lecture streaming scenarios.
type StreamProxy struct {
	*Proxy
	bufferSize int
}

// NewStreamProxy creates a proxy optimized for streaming.
func NewStreamProxy(registry *Registry, logger zerolog.Logger) *StreamProxy {
	proxy := NewProxy(registry, logger)
	return &StreamProxy{
		Proxy:      proxy,
		bufferSize: 256 * 1024, // 256KB buffer for streaming
	}
}

// StreamHandler handles streaming requests with larger buffers.
func (sp *StreamProxy) StreamHandler(serviceName string, getContext func(*http.Request) *ProxyRequest) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Get service
		svc, exists := sp.registry.Get(serviceName)
		if !exists {
			http.Error(w, "Service not found", http.StatusNotFound)
			return
		}

		// Check access
		pctx := getContext(r)
		if pctx != nil && !svc.Access.Public && !svc.CanAccess(pctx.Zone, pctx.Roles) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		// Create streaming request
		targetURL, _ := svc.GetProxyURL()
		proxyURL := targetURL.String() + r.URL.Path
		if r.URL.RawQuery != "" {
			proxyURL += "?" + r.URL.RawQuery
		}

		req, err := http.NewRequestWithContext(r.Context(), r.Method, proxyURL, r.Body)
		if err != nil {
			http.Error(w, "Failed to create request", http.StatusInternalServerError)
			return
		}

		// Copy headers
		for key, values := range r.Header {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}

		// Add LocalMesh headers
		if pctx != nil {
			req.Header.Set("X-LocalMesh-User-ID", pctx.UserID)
			req.Header.Set("X-LocalMesh-Zone", pctx.Zone)
		}

		// Execute request
		resp, err := sp.transport.RoundTrip(req)
		if err != nil {
			http.Error(w, "Upstream error", http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Copy response headers
		for key, values := range resp.Header {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
		w.WriteHeader(resp.StatusCode)

		// Stream with flushing for real-time data
		flusher, canFlush := w.(http.Flusher)
		buffer := make([]byte, sp.bufferSize)
		var totalBytes int64

		for {
			n, err := resp.Body.Read(buffer)
			if n > 0 {
				written, writeErr := w.Write(buffer[:n])
				totalBytes += int64(written)
				if writeErr != nil {
					break
				}
				if canFlush {
					flusher.Flush() // Flush immediately for real-time streaming
				}
			}
			if err != nil {
				if err != io.EOF {
					sp.logger.Error().Err(err).Msg("Stream read error")
				}
				break
			}
		}

		sp.logger.Debug().
			Str("service", serviceName).
			Int64("bytes", totalBytes).
			Dur("duration", time.Since(start)).
			Msg("Stream completed")

		svc.RecordRequest(true, totalBytes, time.Since(start))
	})
}

// WebSocketProxy handles WebSocket connections (for real-time features).
type WebSocketProxy struct {
	registry *Registry
	logger   zerolog.Logger
}

// NewWebSocketProxy creates a WebSocket proxy.
func NewWebSocketProxy(registry *Registry, logger zerolog.Logger) *WebSocketProxy {
	return &WebSocketProxy{
		registry: registry,
		logger:   logger.With().Str("component", "ws-proxy").Logger(),
	}
}

// Handler returns a WebSocket proxy handler.
func (wsp *WebSocketProxy) Handler(serviceName string, getContext func(*http.Request) *ProxyRequest) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if it's a WebSocket upgrade
		if r.Header.Get("Upgrade") != "websocket" {
			http.Error(w, "Expected WebSocket", http.StatusBadRequest)
			return
		}

		svc, exists := wsp.registry.Get(serviceName)
		if !exists {
			http.Error(w, "Service not found", http.StatusNotFound)
			return
		}

		// Check access
		pctx := getContext(r)
		if pctx != nil && !svc.Access.Public && !svc.CanAccess(pctx.Zone, pctx.Roles) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		// Get target URL and convert to ws://
		targetURL, _ := svc.GetProxyURL()
		wsURL := "ws" + strings.TrimPrefix(targetURL.String(), "http") + r.URL.Path

		wsp.logger.Info().
			Str("service", serviceName).
			Str("target", wsURL).
			Msg("WebSocket connection")

		// Hijack connection for bidirectional communication
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "WebSocket not supported", http.StatusInternalServerError)
			return
		}

		clientConn, _, err := hijacker.Hijack()
		if err != nil {
			http.Error(w, "Failed to hijack connection", http.StatusInternalServerError)
			return
		}
		defer clientConn.Close()

		// Connect to backend
		backendConn, err := net.Dial("tcp", targetURL.Host)
		if err != nil {
			wsp.logger.Error().Err(err).Msg("Failed to connect to backend")
			return
		}
		defer backendConn.Close()

		// Forward the original request
		r.Write(backendConn)

		// Bidirectional copy
		ctx, cancel := context.WithCancel(r.Context())
		defer cancel()

		go func() {
			io.Copy(backendConn, clientConn)
			cancel()
		}()

		go func() {
			io.Copy(clientConn, backendConn)
			cancel()
		}()

		<-ctx.Done()

		wsp.logger.Debug().
			Str("service", serviceName).
			Msg("WebSocket connection closed")
	})
}
