package auth

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ContextKey string

const (
	ClaimsKey ContextKey = "claims"
	UserKey   ContextKey = "user"
)

type Middleware struct {
	auth         *Service
	excludePaths map[string]bool
}

func NewMiddleware(auth *Service) *Middleware {
	return &Middleware{
		auth: auth,
		excludePaths: map[string]bool{
			"/health":                  true,
			"/ready":                   true,
			"/api/v1/auth/login":       true,
			"/api/v1/auth/refresh":     true,
			"/api/v1/network/identity": true, // Local identity is public
			"/api/v1/network/mappings": true, // Zone mappings are public
			"/api/v1/plugins":          true, // Plugin list is public
			"PREFIX:/plugins/":         true, // Plugin routes handle their own auth
		},
	}
}

func (m *Middleware) ExcludePath(path string) {
	m.excludePaths[path] = true
}

// ExcludePrefix adds a path prefix to the auth exclusion list
func (m *Middleware) ExcludePrefix(prefix string) {
	m.excludePaths["PREFIX:"+prefix] = true
}

func (m *Middleware) Authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check exact path exclusion
		if m.excludePaths[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}

		// Check prefix exclusions
		for path := range m.excludePaths {
			if strings.HasPrefix(path, "PREFIX:") {
				prefix := strings.TrimPrefix(path, "PREFIX:")
				if strings.HasPrefix(r.URL.Path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		token := extractToken(r)
		if token == "" {
			http.Error(w, `{"error":"missing authorization token"}`, http.StatusUnauthorized)
			return
		}

		claims, err := m.auth.ValidateToken(token)
		if err != nil {
			status := http.StatusUnauthorized
			msg := "invalid token"
			if err == ErrTokenExpired {
				msg = "token expired"
			}
			http.Error(w, `{"error":"`+msg+`"}`, status)
			return
		}

		ctx := context.WithValue(r.Context(), ClaimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (m *Middleware) RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			for _, role := range roles {
				if claims.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}

			http.Error(w, `{"error":"insufficient permissions"}`, http.StatusForbidden)
		})
	}
}

func (m *Middleware) RequireZone(zone string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := GetClaims(r.Context())
			if claims == nil {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}

			clientIP := getClientIP(r)
			if err := m.auth.CheckZoneAccess(r.Context(), claims, zone, clientIP); err != nil {
				http.Error(w, `{"error":"zone access denied"}`, http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func GetClaims(ctx context.Context) *Claims {
	claims, _ := ctx.Value(ClaimsKey).(*Claims)
	return claims
}

func extractToken(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if auth == "" {
		return r.URL.Query().Get("token")
	}

	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return strings.Trim(ip, "[]")
}

type RateLimiter struct {
	requests map[string]*rateLimitEntry
	mu       sync.RWMutex
	limit    int
	window   time.Duration
	burst    int
}

type rateLimitEntry struct {
	count       int
	windowStart time.Time
}

func NewRateLimiter(limit int, window time.Duration, burst int) *RateLimiter {
	rl := &RateLimiter{
		requests: make(map[string]*rateLimitEntry),
		limit:    limit,
		window:   window,
		burst:    burst,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, exists := rl.requests[key]

	if !exists || now.Sub(entry.windowStart) > rl.window {
		rl.requests[key] = &rateLimitEntry{count: 1, windowStart: now}
		return true
	}

	if entry.count >= rl.limit+rl.burst {
		return false
	}

	entry.count++
	return true
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := getClientIP(r)
		if claims := GetClaims(r.Context()); claims != nil {
			key = "user:" + claims.Subject
		}

		if !rl.Allow(key) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(rl.window)
	defer ticker.Stop()

	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for key, entry := range rl.requests {
			if now.Sub(entry.windowStart) > rl.window*2 {
				delete(rl.requests, key)
			}
		}
		rl.mu.Unlock()
	}
}
