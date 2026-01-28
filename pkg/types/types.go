// Package types contains shared types used across LocalMesh.
package types

import (
	"net"
	"time"
)

// Service represents a registered service in the mesh.
type Service struct {
	// ID is the unique service identifier
	ID string `json:"id"`

	// Name is the human-readable service name
	Name string `json:"name"`

	// Version is the service version
	Version string `json:"version"`

	// Host is the hostname or IP where the service runs
	Host string `json:"host"`

	// Port is the port the service listens on
	Port int `json:"port"`

	// Zones are the network zones this service is accessible from
	Zones []string `json:"zones"`

	// HealthEndpoint is the path to the health check endpoint
	HealthEndpoint string `json:"health_endpoint"`

	// Metadata contains additional service information
	Metadata map[string]string `json:"metadata,omitempty"`

	// Status is the current service status
	Status ServiceStatus `json:"status"`

	// RegisteredAt is when the service was registered
	RegisteredAt time.Time `json:"registered_at"`

	// LastSeen is when the service was last seen healthy
	LastSeen time.Time `json:"last_seen"`
}

// ServiceStatus represents the status of a service.
type ServiceStatus string

const (
	ServiceStatusHealthy   ServiceStatus = "healthy"
	ServiceStatusDegraded  ServiceStatus = "degraded"
	ServiceStatusUnhealthy ServiceStatus = "unhealthy"
	ServiceStatusUnknown   ServiceStatus = "unknown"
)

// Address returns the full address (host:port) of the service.
func (s *Service) Address() string {
	return net.JoinHostPort(s.Host, string(rune(s.Port)))
}

// ServiceResponse is the response from a service call.
type ServiceResponse struct {
	// StatusCode is the HTTP status code
	StatusCode int `json:"status_code"`

	// Body is the response body
	Body []byte `json:"body"`

	// Headers are the response headers
	Headers map[string][]string `json:"headers"`
}

// Node represents a LocalMesh node in the network.
type Node struct {
	// ID is the unique node identifier
	ID string `json:"id"`

	// Name is the human-readable node name
	Name string `json:"name"`

	// Host is the node's hostname or IP
	Host string `json:"host"`

	// Port is the node's gateway port
	Port int `json:"port"`

	// Role is the node's role (gateway, worker, etc.)
	Role NodeRole `json:"role"`

	// Zone is the network zone this node belongs to
	Zone string `json:"zone"`

	// Services are the services running on this node
	Services []string `json:"services"`

	// Status is the current node status
	Status NodeStatus `json:"status"`

	// Version is the LocalMesh version running on this node
	Version string `json:"version"`

	// DiscoveredAt is when this node was discovered
	DiscoveredAt time.Time `json:"discovered_at"`

	// LastSeen is when this node was last seen
	LastSeen time.Time `json:"last_seen"`
}

// NodeRole represents the role of a node.
type NodeRole string

const (
	NodeRoleGateway NodeRole = "gateway"
	NodeRoleWorker  NodeRole = "worker"
)

// NodeStatus represents the status of a node.
type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"
	NodeStatusOffline NodeStatus = "offline"
)

// NetworkZone represents a network zone configuration.
type NetworkZone struct {
	// ID is the unique zone identifier (e.g., "cs-department")
	ID string `json:"id"`

	// Name is the human-readable zone name
	Name string `json:"name"`

	// SSIDs are the WiFi SSIDs that belong to this zone
	SSIDs []string `json:"ssids"`

	// IPRanges are the IP ranges that belong to this zone
	IPRanges []string `json:"ip_ranges"`

	// Parent is the parent zone (for hierarchical zones)
	Parent string `json:"parent,omitempty"`

	// Description describes the zone
	Description string `json:"description,omitempty"`
}

// User represents a user in the system.
type User struct {
	// ID is the unique user identifier
	ID string `json:"id"`

	// Username is the user's login name
	Username string `json:"username"`

	// Email is the user's email (optional)
	Email string `json:"email,omitempty"`

	// Roles are the user's roles
	Roles []string `json:"roles"`

	// Metadata contains additional user information
	Metadata map[string]string `json:"metadata,omitempty"`

	// CreatedAt is when the user was created
	CreatedAt time.Time `json:"created_at"`

	// LastLogin is when the user last logged in
	LastLogin time.Time `json:"last_login"`
}

// Token represents an authentication token.
type Token struct {
	// Raw is the raw token string
	Raw string `json:"token"`

	// Claims are the decoded token claims
	Claims TokenClaims `json:"claims"`

	// ExpiresAt is when the token expires
	ExpiresAt time.Time `json:"expires_at"`
}

// TokenClaims are the claims embedded in a token.
type TokenClaims struct {
	// Subject is the user ID
	Subject string `json:"sub"`

	// NetworkZone is the verified network zone
	NetworkZone string `json:"network_zone"`

	// NetworkID is the specific network identifier (SSID)
	NetworkID string `json:"network_id"`

	// AllowedServices are the services this token can access
	AllowedServices []string `json:"allowed_services"`

	// LocationVerified indicates if location was verified
	LocationVerified bool `json:"location_verified"`

	// Roles are the user's roles
	Roles []string `json:"roles"`

	// IssuedAt is when the token was issued (Unix timestamp)
	IssuedAt int64 `json:"iat"`

	// ExpiresAt is when the token expires (Unix timestamp)
	ExpiresAt int64 `json:"exp"`

	// Issuer is the token issuer
	Issuer string `json:"iss"`

	// Audience is the intended audience
	Audience []string `json:"aud"`
}

// Valid checks if the token claims are valid.
func (c *TokenClaims) Valid() bool {
	now := time.Now().Unix()
	return c.ExpiresAt > now && c.IssuedAt <= now
}

// Config represents the LocalMesh configuration.
type Config struct {
	// Node configuration
	Node NodeConfig `yaml:"node"`

	// Gateway configuration
	Gateway GatewayConfig `yaml:"gateway"`

	// Auth configuration
	Auth AuthConfig `yaml:"auth"`

	// Zones configuration
	Zones []NetworkZone `yaml:"zones"`

	// Storage configuration
	Storage StorageConfig `yaml:"storage"`

	// Sync configuration
	Sync SyncConfig `yaml:"sync"`

	// Plugins configuration
	Plugins map[string]PluginInstanceConfig `yaml:"plugins"`
}

// NodeConfig contains node-specific configuration.
type NodeConfig struct {
	ID   string   `yaml:"id"`
	Name string   `yaml:"name"`
	Role NodeRole `yaml:"role"`
	Zone string   `yaml:"zone"`
}

// GatewayConfig contains gateway configuration.
type GatewayConfig struct {
	Host         string `yaml:"host"`
	Port         int    `yaml:"port"`
	Domain       string `yaml:"domain"`
	TLSEnabled   bool   `yaml:"tls_enabled"`
	TLSCertFile  string `yaml:"tls_cert_file"`
	TLSKeyFile   string `yaml:"tls_key_file"`
	ReadTimeout  int    `yaml:"read_timeout"`
	WriteTimeout int    `yaml:"write_timeout"`
}

// AuthConfig contains authentication configuration.
type AuthConfig struct {
	TokenTTL         int      `yaml:"token_ttl"`         // Token TTL in seconds
	RefreshTokenTTL  int      `yaml:"refresh_token_ttl"` // Refresh token TTL in seconds
	AllowedAudiences []string `yaml:"allowed_audiences"`
	RequireLocation  bool     `yaml:"require_location"`
	SecretKeyFile    string   `yaml:"secret_key_file"`
}

// StorageConfig contains storage configuration.
type StorageConfig struct {
	DataDir      string `yaml:"data_dir"`
	Engine       string `yaml:"engine"` // "sqlite" or "badger"
	MaxOpenConns int    `yaml:"max_open_conns"`
}

// SyncConfig contains cloud sync configuration.
type SyncConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider"` // "s3", "gcs", "local"
	Endpoint string `yaml:"endpoint"`
	Bucket   string `yaml:"bucket"`
	Interval int    `yaml:"interval"` // Sync interval in minutes
	Encrypt  bool   `yaml:"encrypt"`
	KeyFile  string `yaml:"key_file"`
}

// PluginInstanceConfig contains per-plugin configuration.
type PluginInstanceConfig struct {
	Enabled bool           `yaml:"enabled"`
	Config  map[string]any `yaml:"config"`
}
