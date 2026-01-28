// Package services provides external service management for LocalMesh.
// Services are external applications (any language/framework) that LocalMesh
// proxies to with zone-based authentication. Unlike internal plugins (Go),
// services can be written in any language and communicate via HTTP.
package services

import (
	"fmt"
	"net/url"
	"time"
)

// ServiceState represents the current state of a service.
type ServiceState string

const (
	StateUnknown   ServiceState = "unknown"
	StateHealthy   ServiceState = "healthy"
	StateUnhealthy ServiceState = "unhealthy"
	StateDegraded  ServiceState = "degraded"
)

// ServiceInfo contains metadata about a service.
type ServiceInfo struct {
	Name        string   `json:"name" yaml:"name"`                 // Unique identifier
	DisplayName string   `json:"display_name" yaml:"display_name"` // Human-readable name
	Description string   `json:"description" yaml:"description"`   // What this service does
	Version     string   `json:"version" yaml:"version"`           // Service version
	Provider    string   `json:"provider" yaml:"provider"`         // Who provides this service
	Tags        []string `json:"tags" yaml:"tags"`                 // Categorization tags
}

// ServiceEndpoint represents where to reach the service.
type ServiceEndpoint struct {
	URL        string `json:"url" yaml:"url"`                 // Base URL (e.g., http://localhost:3001)
	HealthPath string `json:"health_path" yaml:"health_path"` // Health check endpoint (e.g., /health)
}

// ServiceAccess defines who can access this service.
type ServiceAccess struct {
	Zones       []string `json:"zones" yaml:"zones"`               // Required zones (empty = all zones)
	Roles       []string `json:"roles" yaml:"roles"`               // Required roles (empty = all roles)
	RequireAuth bool     `json:"require_auth" yaml:"require_auth"` // Whether auth is required
	Public      bool     `json:"public" yaml:"public"`             // If true, no auth needed
}

// ServiceConfig holds service configuration.
type ServiceConfig struct {
	Timeout        time.Duration `json:"timeout" yaml:"timeout"`                 // Request timeout
	RetryCount     int           `json:"retry_count" yaml:"retry_count"`         // Retry attempts
	RetryDelay     time.Duration `json:"retry_delay" yaml:"retry_delay"`         // Delay between retries
	CircuitBreaker bool          `json:"circuit_breaker" yaml:"circuit_breaker"` // Enable circuit breaker
	RateLimit      int           `json:"rate_limit" yaml:"rate_limit"`           // Requests per second (0 = unlimited)
	StripPrefix    bool          `json:"strip_prefix" yaml:"strip_prefix"`       // Strip /svc/{name} from path
}

// ServiceHealth contains health check information.
type ServiceHealth struct {
	State       ServiceState  `json:"state"`
	LastCheck   time.Time     `json:"last_check"`
	LastHealthy time.Time     `json:"last_healthy,omitempty"`
	Message     string        `json:"message,omitempty"`
	Latency     time.Duration `json:"latency"`
	CheckCount  int64         `json:"check_count"`
	FailCount   int64         `json:"fail_count"`
}

// ServiceMetrics tracks service usage metrics.
type ServiceMetrics struct {
	TotalRequests   int64         `json:"total_requests"`
	SuccessRequests int64         `json:"success_requests"`
	FailedRequests  int64         `json:"failed_requests"`
	TotalBytes      int64         `json:"total_bytes"`
	AvgLatency      time.Duration `json:"avg_latency"`
	LastRequest     time.Time     `json:"last_request,omitempty"`
}

// Service represents an external service registered with LocalMesh.
type Service struct {
	Info       ServiceInfo     `json:"info" yaml:"info"`
	Endpoint   ServiceEndpoint `json:"endpoint" yaml:"endpoint"`
	Access     ServiceAccess   `json:"access" yaml:"access"`
	Config     ServiceConfig   `json:"config" yaml:"config"`
	Health     ServiceHealth   `json:"health" yaml:"-"`
	Metrics    ServiceMetrics  `json:"metrics" yaml:"-"`
	Discovered bool            `json:"discovered" yaml:"-"` // True if found via mDNS
	CreatedAt  time.Time       `json:"created_at" yaml:"-"`
	UpdatedAt  time.Time       `json:"updated_at" yaml:"-"`
}

// DefaultConfig returns sensible default configuration.
func DefaultConfig() ServiceConfig {
	return ServiceConfig{
		Timeout:        30 * time.Second,
		RetryCount:     3,
		RetryDelay:     100 * time.Millisecond,
		CircuitBreaker: true,
		RateLimit:      0, // Unlimited
		StripPrefix:    true,
	}
}

// NewService creates a new service with defaults.
func NewService(name, displayName, url string) *Service {
	now := time.Now()
	return &Service{
		Info: ServiceInfo{
			Name:        name,
			DisplayName: displayName,
			Version:     "1.0.0",
		},
		Endpoint: ServiceEndpoint{
			URL:        url,
			HealthPath: "/health",
		},
		Access: ServiceAccess{
			RequireAuth: true,
		},
		Config: DefaultConfig(),
		Health: ServiceHealth{
			State: StateUnknown,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// Validate checks if the service configuration is valid.
func (s *Service) Validate() error {
	if s.Info.Name == "" {
		return fmt.Errorf("service name is required")
	}

	// Validate name format (alphanumeric, hyphens, underscores)
	for _, c := range s.Info.Name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return fmt.Errorf("service name must be alphanumeric with hyphens/underscores only")
		}
	}

	if s.Endpoint.URL == "" {
		return fmt.Errorf("service URL is required")
	}

	// Validate URL format
	u, err := url.Parse(s.Endpoint.URL)
	if err != nil {
		return fmt.Errorf("invalid service URL: %w", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("service URL must use http or https scheme")
	}

	if u.Host == "" {
		return fmt.Errorf("service URL must have a host")
	}

	return nil
}

// IsHealthy returns true if the service is healthy.
func (s *Service) IsHealthy() bool {
	return s.Health.State == StateHealthy
}

// CanAccess checks if the given zone and roles can access this service.
func (s *Service) CanAccess(zone string, roles []string) bool {
	// Public services are accessible to everyone
	if s.Access.Public {
		return true
	}

	// Check zone access
	if len(s.Access.Zones) > 0 {
		zoneMatch := false
		for _, z := range s.Access.Zones {
			if z == zone || z == "*" {
				zoneMatch = true
				break
			}
		}
		if !zoneMatch {
			return false
		}
	}

	// Check role access
	if len(s.Access.Roles) > 0 {
		roleMatch := false
		for _, required := range s.Access.Roles {
			for _, has := range roles {
				if required == has || required == "*" {
					roleMatch = true
					break
				}
			}
			if roleMatch {
				break
			}
		}
		if !roleMatch {
			return false
		}
	}

	return true
}

// GetProxyURL returns the URL to proxy requests to.
func (s *Service) GetProxyURL() (*url.URL, error) {
	return url.Parse(s.Endpoint.URL)
}

// RecordRequest updates metrics for a request.
func (s *Service) RecordRequest(success bool, bytes int64, latency time.Duration) {
	s.Metrics.TotalRequests++
	s.Metrics.TotalBytes += bytes
	s.Metrics.LastRequest = time.Now()

	if success {
		s.Metrics.SuccessRequests++
	} else {
		s.Metrics.FailedRequests++
	}

	// Calculate running average latency
	if s.Metrics.TotalRequests == 1 {
		s.Metrics.AvgLatency = latency
	} else {
		s.Metrics.AvgLatency = (s.Metrics.AvgLatency + latency) / 2
	}
}
