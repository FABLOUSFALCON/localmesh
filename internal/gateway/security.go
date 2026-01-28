// Package gateway provides security headers middleware.
package gateway

import (
	"net/http"
	"strings"
)

// SecurityConfig configures security headers
type SecurityConfig struct {
	// Content Security Policy
	CSPEnabled    bool
	CSPDirectives map[string][]string

	// HTTP Strict Transport Security
	HSTSEnabled           bool
	HSTSMaxAge            int // seconds
	HSTSIncludeSubdomains bool
	HSTSPreload           bool

	// Frame options
	FrameOptions string // DENY, SAMEORIGIN, or empty

	// Content type options
	NoSniff bool

	// XSS Protection (legacy, but still useful)
	XSSProtection bool

	// Referrer Policy
	ReferrerPolicy string

	// Permissions Policy (formerly Feature Policy)
	PermissionsPolicy map[string][]string

	// Cross-Origin policies
	COEP string // Cross-Origin-Embedder-Policy
	COOP string // Cross-Origin-Opener-Policy
	CORP string // Cross-Origin-Resource-Policy

	// Cache control for sensitive responses
	NoCacheOnAuth bool
}

// DefaultSecurityConfig returns secure defaults
func DefaultSecurityConfig() SecurityConfig {
	return SecurityConfig{
		CSPEnabled: true,
		CSPDirectives: map[string][]string{
			"default-src": {"'self'"},
			"script-src":  {"'self'"},
			"style-src":   {"'self'", "'unsafe-inline'"}, // Allow inline styles for TUI
			"img-src":     {"'self'", "data:"},
			"font-src":    {"'self'"},
			"connect-src": {"'self'"},
			"frame-src":   {"'none'"},
			"object-src":  {"'none'"},
			"base-uri":    {"'self'"},
			"form-action": {"'self'"},
		},
		HSTSEnabled:           false, // Enable when using HTTPS
		HSTSMaxAge:            31536000,
		HSTSIncludeSubdomains: true,
		HSTSPreload:           false,
		FrameOptions:          "DENY",
		NoSniff:               true,
		XSSProtection:         true,
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		PermissionsPolicy: map[string][]string{
			"geolocation":          {},
			"microphone":           {},
			"camera":               {},
			"payment":              {},
			"usb":                  {},
			"magnetometer":         {},
			"gyroscope":            {},
			"accelerometer":        {},
			"ambient-light-sensor": {},
		},
		COEP:          "",
		COOP:          "same-origin",
		CORP:          "same-origin",
		NoCacheOnAuth: true,
	}
}

// SecurityMiddleware adds security headers to responses
func SecurityMiddleware(cfg SecurityConfig) Middleware {
	// Pre-build header values
	cspHeader := buildCSP(cfg.CSPDirectives)
	hstsHeader := buildHSTS(cfg)
	permissionsHeader := buildPermissionsPolicy(cfg.PermissionsPolicy)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h := w.Header()

			// Content Security Policy
			if cfg.CSPEnabled && cspHeader != "" {
				h.Set("Content-Security-Policy", cspHeader)
			}

			// HTTP Strict Transport Security
			if cfg.HSTSEnabled && hstsHeader != "" {
				h.Set("Strict-Transport-Security", hstsHeader)
			}

			// X-Frame-Options
			if cfg.FrameOptions != "" {
				h.Set("X-Frame-Options", cfg.FrameOptions)
			}

			// X-Content-Type-Options
			if cfg.NoSniff {
				h.Set("X-Content-Type-Options", "nosniff")
			}

			// X-XSS-Protection (legacy but harmless)
			if cfg.XSSProtection {
				h.Set("X-XSS-Protection", "1; mode=block")
			}

			// Referrer-Policy
			if cfg.ReferrerPolicy != "" {
				h.Set("Referrer-Policy", cfg.ReferrerPolicy)
			}

			// Permissions-Policy
			if permissionsHeader != "" {
				h.Set("Permissions-Policy", permissionsHeader)
			}

			// Cross-Origin policies
			if cfg.COEP != "" {
				h.Set("Cross-Origin-Embedder-Policy", cfg.COEP)
			}
			if cfg.COOP != "" {
				h.Set("Cross-Origin-Opener-Policy", cfg.COOP)
			}
			if cfg.CORP != "" {
				h.Set("Cross-Origin-Resource-Policy", cfg.CORP)
			}

			// Cache control for authenticated requests
			if cfg.NoCacheOnAuth {
				if auth := r.Header.Get("Authorization"); auth != "" {
					h.Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
					h.Set("Pragma", "no-cache")
					h.Set("Expires", "0")
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// buildCSP constructs the Content-Security-Policy header value
func buildCSP(directives map[string][]string) string {
	if len(directives) == 0 {
		return ""
	}

	var parts []string
	for directive, values := range directives {
		if len(values) == 0 {
			parts = append(parts, directive)
		} else {
			parts = append(parts, directive+" "+strings.Join(values, " "))
		}
	}
	return strings.Join(parts, "; ")
}

// buildHSTS constructs the Strict-Transport-Security header value
func buildHSTS(cfg SecurityConfig) string {
	if !cfg.HSTSEnabled {
		return ""
	}

	var parts []string
	parts = append(parts, "max-age="+itoa(cfg.HSTSMaxAge))

	if cfg.HSTSIncludeSubdomains {
		parts = append(parts, "includeSubDomains")
	}

	if cfg.HSTSPreload {
		parts = append(parts, "preload")
	}

	return strings.Join(parts, "; ")
}

// buildPermissionsPolicy constructs the Permissions-Policy header value
func buildPermissionsPolicy(policies map[string][]string) string {
	if len(policies) == 0 {
		return ""
	}

	var parts []string
	for feature, allowList := range policies {
		if len(allowList) == 0 {
			parts = append(parts, feature+"=()")
		} else {
			parts = append(parts, feature+"=("+strings.Join(allowList, " ")+")")
		}
	}
	return strings.Join(parts, ", ")
}

// Simple int to string without importing strconv
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	neg := n < 0
	if neg {
		n = -n
	}

	var buf [20]byte
	i := len(buf)

	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	if neg {
		i--
		buf[i] = '-'
	}

	return string(buf[i:])
}

// APISecurityConfig returns security config optimized for APIs
func APISecurityConfig() SecurityConfig {
	cfg := DefaultSecurityConfig()
	// APIs don't need CSP as they return JSON
	cfg.CSPEnabled = false
	// Always no-cache for API responses
	cfg.NoCacheOnAuth = true
	return cfg
}
