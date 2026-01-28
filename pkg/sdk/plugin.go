// Package sdk provides the public API for LocalMesh plugin development.
//
// Plugins implement the Plugin interface to integrate with the LocalMesh framework.
// The framework handles discovery, routing, authentication, and storage - plugins
// focus on business logic.
//
// Example:
//
//	type MyPlugin struct {
//	    sdk.BasePlugin
//	}
//
//	func (p *MyPlugin) Info() sdk.PluginInfo {
//	    return sdk.PluginInfo{
//	        Name:        "my-plugin",
//	        Version:     "1.0.0",
//	        Description: "My awesome plugin",
//	    }
//	}
//
//	func (p *MyPlugin) Routes() []sdk.Route {
//	    return []sdk.Route{
//	        {Method: "GET", Path: "/hello", Handler: p.handleHello},
//	    }
//	}
package sdk

import (
	"context"
	"net/http"

	"github.com/FABLOUSFALCON/localmesh/pkg/types"
)

// Plugin defines the interface that all LocalMesh plugins must implement.
type Plugin interface {
	// Info returns plugin metadata.
	Info() PluginInfo

	// Init initializes the plugin with the given configuration.
	// Called once when the plugin is loaded.
	Init(ctx context.Context, cfg PluginConfig) error

	// Start starts the plugin.
	// Called after Init and when the framework starts.
	Start(ctx context.Context) error

	// Stop gracefully stops the plugin.
	// Called during framework shutdown.
	Stop(ctx context.Context) error

	// Routes returns the HTTP routes this plugin handles.
	// Routes are mounted at /plugins/{plugin-name}/
	Routes() []Route

	// RequiredZones returns the network zones that can access this plugin.
	// Empty slice means accessible from all zones.
	RequiredZones() []string

	// Health returns the current health status of the plugin.
	Health() HealthStatus
}

// PluginInfo contains metadata about a plugin.
type PluginInfo struct {
	// Name is the unique identifier for this plugin (lowercase, hyphenated)
	Name string `json:"name"`

	// Version follows semver (e.g., "1.0.0")
	Version string `json:"version"`

	// Description is a short description of the plugin
	Description string `json:"description"`

	// Author is the plugin author
	Author string `json:"author"`

	// MinFrameworkVersion is the minimum LocalMesh version required
	MinFrameworkVersion string `json:"min_framework_version"`

	// Homepage is the plugin's homepage URL (optional)
	Homepage string `json:"homepage,omitempty"`

	// License is the plugin's license (e.g., "MIT")
	License string `json:"license,omitempty"`
}

// PluginConfig contains configuration passed to plugins during Init.
type PluginConfig struct {
	// DataDir is the directory where the plugin can store data
	DataDir string

	// Logger is a pre-configured logger for the plugin
	Logger Logger

	// Storage provides access to the plugin's database
	Storage Storage

	// Custom is plugin-specific configuration loaded from config file
	Custom map[string]any
}

// Route defines an HTTP route handled by a plugin.
type Route struct {
	// Method is the HTTP method (GET, POST, PUT, DELETE, etc.)
	Method string

	// Path is the route path (relative to plugin mount point)
	Path string

	// Handler is the HTTP handler function
	Handler http.HandlerFunc

	// RequireAuth specifies if authentication is required
	RequireAuth bool

	// AllowedZones overrides plugin-level zones for this specific route
	// Empty slice means use plugin-level zones
	AllowedZones []string

	// Description is used for documentation/introspection
	Description string
}

// HealthStatus represents the health of a plugin.
type HealthStatus struct {
	// Status is "healthy", "degraded", or "unhealthy"
	Status string `json:"status"`

	// Message provides additional context
	Message string `json:"message,omitempty"`

	// Details contains component-specific health info
	Details map[string]any `json:"details,omitempty"`
}

const (
	HealthStatusHealthy   = "healthy"
	HealthStatusDegraded  = "degraded"
	HealthStatusUnhealthy = "unhealthy"
)

// BasePlugin provides default implementations for Plugin interface.
// Embed this in your plugin to only override what you need.
type BasePlugin struct {
	config PluginConfig
}

// Init stores the configuration.
func (p *BasePlugin) Init(ctx context.Context, cfg PluginConfig) error {
	p.config = cfg
	return nil
}

// Start is a no-op by default.
func (p *BasePlugin) Start(ctx context.Context) error {
	return nil
}

// Stop is a no-op by default.
func (p *BasePlugin) Stop(ctx context.Context) error {
	return nil
}

// RequiredZones returns empty slice (accessible from all zones) by default.
func (p *BasePlugin) RequiredZones() []string {
	return nil
}

// Health returns healthy status by default.
func (p *BasePlugin) Health() HealthStatus {
	return HealthStatus{Status: HealthStatusHealthy}
}

// Config returns the plugin configuration.
func (p *BasePlugin) Config() PluginConfig {
	return p.config
}

// RequestContext contains information about the current request.
// Plugins receive this via context.
type RequestContext struct {
	// Network identity
	NetworkZone string `json:"network_zone"`
	NetworkID   string `json:"network_id"`
	ClientIP    string `json:"client_ip"`
	IsVerified  bool   `json:"is_verified"`

	// User info (if authenticated)
	UserID   string   `json:"user_id,omitempty"`
	Username string   `json:"username,omitempty"`
	Roles    []string `json:"roles,omitempty"`

	// Request metadata
	TraceID   string `json:"trace_id"`
	RequestID string `json:"request_id"`
}

// contextKey is used for context values.
type contextKey string

const requestContextKey contextKey = "localmesh_request_context"

// WithRequestContext adds RequestContext to a context.
func WithRequestContext(ctx context.Context, rc *RequestContext) context.Context {
	return context.WithValue(ctx, requestContextKey, rc)
}

// GetRequestContext retrieves RequestContext from a context.
func GetRequestContext(ctx context.Context) (*RequestContext, bool) {
	rc, ok := ctx.Value(requestContextKey).(*RequestContext)
	return rc, ok
}

// GetRequestContextFromRequest is a convenience method for HTTP handlers.
func GetRequestContextFromRequest(r *http.Request) (*RequestContext, bool) {
	return GetRequestContext(r.Context())
}

// Event represents an event that plugins can emit or subscribe to.
type Event struct {
	// Type is the event type (e.g., "user.login", "attendance.marked")
	Type string `json:"type"`

	// Source is the plugin that emitted the event
	Source string `json:"source"`

	// Data is the event payload
	Data any `json:"data"`

	// Timestamp is when the event occurred (Unix timestamp)
	Timestamp int64 `json:"timestamp"`
}

// EventHandler is called when an event is received.
type EventHandler func(ctx context.Context, event Event) error

// EventEmitter allows plugins to emit events.
type EventEmitter interface {
	// Emit sends an event to all subscribers
	Emit(ctx context.Context, eventType string, data any) error
}

// EventSubscriber allows plugins to subscribe to events.
type EventSubscriber interface {
	// Subscribe registers a handler for the given event type
	// Use "*" to subscribe to all events
	Subscribe(eventType string, handler EventHandler) error

	// Unsubscribe removes a handler
	Unsubscribe(eventType string, handler EventHandler) error
}

// Storage provides database access for plugins.
type Storage interface {
	// Get retrieves a value by key
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value
	Set(ctx context.Context, key string, value []byte) error

	// Delete removes a value
	Delete(ctx context.Context, key string) error

	// List returns all keys with the given prefix
	List(ctx context.Context, prefix string) ([]string, error)

	// Transaction executes a function within a transaction
	Transaction(ctx context.Context, fn func(tx StorageTx) error) error
}

// StorageTx represents a storage transaction.
type StorageTx interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
	Delete(key string) error
}

// Logger provides structured logging for plugins.
type Logger interface {
	Debug(msg string, args ...any)
	Info(msg string, args ...any)
	Warn(msg string, args ...any)
	Error(msg string, args ...any)
	With(args ...any) Logger
}

// ServiceClient allows plugins to call other plugins/services.
type ServiceClient interface {
	// Call invokes a service endpoint
	Call(ctx context.Context, service, method, path string, body any) (*types.ServiceResponse, error)
}
