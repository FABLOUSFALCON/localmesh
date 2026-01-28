// Package plugins provides the plugin loader and lifecycle management.
package plugins

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/FABLOUSFALCON/localmesh/pkg/sdk"
)

// Loader manages plugin lifecycle.
type Loader struct {
	mu      sync.RWMutex
	plugins map[string]*pluginEntry
	config  LoaderConfig
	logger  *slog.Logger

	// State
	started bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// pluginEntry tracks a loaded plugin
type pluginEntry struct {
	plugin    sdk.Plugin
	info      sdk.PluginInfo
	state     PluginState
	handler   http.Handler
	loadedAt  time.Time
	startedAt *time.Time
	lastError error
}

// PluginState represents the current state of a plugin
type PluginState string

const (
	StateLoaded   PluginState = "loaded"
	StateStarting PluginState = "starting"
	StateRunning  PluginState = "running"
	StateStopping PluginState = "stopping"
	StateStopped  PluginState = "stopped"
	StateError    PluginState = "error"
)

// LoaderConfig configures the plugin loader
type LoaderConfig struct {
	// PluginDir is the base directory for plugin data
	PluginDir string

	// EnabledPlugins is a list of plugin names to enable (empty = all)
	EnabledPlugins []string

	// DisabledPlugins is a list of plugin names to disable
	DisabledPlugins []string

	// Logger for the loader
	Logger *slog.Logger

	// Storage factory for creating plugin storage
	StorageFactory StorageFactory

	// HealthCheckInterval for periodic health checks
	HealthCheckInterval time.Duration
}

// StorageFactory creates isolated storage for each plugin
type StorageFactory func(pluginName string) (sdk.Storage, error)

// DefaultLoaderConfig returns sensible defaults
func DefaultLoaderConfig() LoaderConfig {
	return LoaderConfig{
		PluginDir:           "./plugins",
		HealthCheckInterval: 30 * time.Second,
	}
}

// NewLoader creates a new plugin loader
func NewLoader(cfg LoaderConfig) *Loader {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Loader{
		plugins: make(map[string]*pluginEntry),
		config:  cfg,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Register adds a plugin to the loader.
// Plugins must be registered before Start is called.
func (l *Loader) Register(plugin sdk.Plugin) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.started {
		return fmt.Errorf("cannot register plugins after loader has started")
	}

	info := plugin.Info()
	if info.Name == "" {
		return fmt.Errorf("plugin name is required")
	}

	// Validate plugin name (lowercase, alphanumeric, hyphens)
	if !isValidPluginName(info.Name) {
		return fmt.Errorf("invalid plugin name: %s (must be lowercase alphanumeric with hyphens)", info.Name)
	}

	if _, exists := l.plugins[info.Name]; exists {
		return fmt.Errorf("plugin %s is already registered", info.Name)
	}

	// Check if plugin is disabled
	if l.isDisabled(info.Name) {
		l.logger.Debug("plugin is disabled, skipping", "plugin", info.Name)
		return nil
	}

	// Check if we have an enabled list and this plugin isn't in it
	if len(l.config.EnabledPlugins) > 0 && !l.isEnabled(info.Name) {
		l.logger.Debug("plugin is not in enabled list, skipping", "plugin", info.Name)
		return nil
	}

	l.plugins[info.Name] = &pluginEntry{
		plugin:   plugin,
		info:     info,
		state:    StateLoaded,
		loadedAt: time.Now(),
	}

	l.logger.Info("plugin registered",
		"plugin", info.Name,
		"version", info.Version,
	)

	return nil
}

// Start initializes and starts all registered plugins
func (l *Loader) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.started {
		return fmt.Errorf("loader already started")
	}

	l.logger.Info("starting plugins", "count", len(l.plugins))

	// Sort plugins by name for deterministic startup order
	names := make([]string, 0, len(l.plugins))
	for name := range l.plugins {
		names = append(names, name)
	}
	sort.Strings(names)

	// Initialize plugins
	for _, name := range names {
		entry := l.plugins[name]
		if err := l.initPlugin(ctx, entry); err != nil {
			l.logger.Error("failed to init plugin",
				"plugin", name,
				"error", err,
			)
			entry.state = StateError
			entry.lastError = err
			continue
		}
	}

	// Start plugins
	for _, name := range names {
		entry := l.plugins[name]
		if entry.state == StateError {
			continue // Skip plugins that failed init
		}

		if err := l.startPlugin(ctx, entry); err != nil {
			l.logger.Error("failed to start plugin",
				"plugin", name,
				"error", err,
			)
			entry.state = StateError
			entry.lastError = err
			continue
		}
	}

	l.started = true

	// Start health check loop
	go l.healthCheckLoop()

	l.logger.Info("plugins started", "running", l.runningCount())

	return nil
}

// initPlugin initializes a single plugin
func (l *Loader) initPlugin(ctx context.Context, entry *pluginEntry) error {
	// Create plugin data directory
	dataDir := filepath.Join(l.config.PluginDir, entry.info.Name, "data")

	// Create storage if factory is available
	var storage sdk.Storage
	if l.config.StorageFactory != nil {
		var err error
		storage, err = l.config.StorageFactory(entry.info.Name)
		if err != nil {
			return fmt.Errorf("creating plugin storage: %w", err)
		}
	}

	cfg := sdk.PluginConfig{
		DataDir: dataDir,
		Logger:  sdk.NewLogger(l.logger.With("plugin", entry.info.Name)),
		Storage: storage,
	}

	return entry.plugin.Init(ctx, cfg)
}

// startPlugin starts a single plugin
func (l *Loader) startPlugin(ctx context.Context, entry *pluginEntry) error {
	entry.state = StateStarting

	if err := entry.plugin.Start(ctx); err != nil {
		return err
	}

	now := time.Now()
	entry.startedAt = &now
	entry.state = StateRunning

	// Build the HTTP handler for this plugin
	entry.handler = l.buildPluginHandler(entry)

	l.logger.Info("plugin started",
		"plugin", entry.info.Name,
		"routes", len(entry.plugin.Routes()),
	)

	return nil
}

// buildPluginHandler creates an HTTP handler for a plugin's routes
func (l *Loader) buildPluginHandler(entry *pluginEntry) http.Handler {
	mux := http.NewServeMux()

	for _, route := range entry.plugin.Routes() {
		pattern := route.Method + " " + route.Path
		mux.HandleFunc(pattern, route.Handler)
	}

	return mux
}

// Stop gracefully stops all plugins
func (l *Loader) Stop(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.started {
		return nil
	}

	l.cancel() // Stop health checks

	l.logger.Info("stopping plugins")

	// Stop in reverse order (reverse of sorted names)
	names := make([]string, 0, len(l.plugins))
	for name := range l.plugins {
		names = append(names, name)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))

	var lastErr error
	for _, name := range names {
		entry := l.plugins[name]
		if entry.state != StateRunning {
			continue
		}

		entry.state = StateStopping
		if err := entry.plugin.Stop(ctx); err != nil {
			l.logger.Error("failed to stop plugin",
				"plugin", name,
				"error", err,
			)
			entry.lastError = err
			lastErr = err
		}
		entry.state = StateStopped
	}

	l.started = false
	l.logger.Info("plugins stopped")

	return lastErr
}

// Handler returns an HTTP handler that routes to the correct plugin
func (l *Loader) Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract plugin name from path: /plugins/{name}/...
		path := strings.TrimPrefix(r.URL.Path, "/plugins/")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 0 || parts[0] == "" {
			http.Error(w, `{"error":"plugin name required"}`, http.StatusBadRequest)
			return
		}

		pluginName := parts[0]
		remainingPath := "/"
		if len(parts) > 1 {
			remainingPath = "/" + parts[1]
		}

		l.mu.RLock()
		entry, exists := l.plugins[pluginName]
		l.mu.RUnlock()

		if !exists {
			http.Error(w, `{"error":"plugin not found"}`, http.StatusNotFound)
			return
		}

		if entry.state != StateRunning {
			http.Error(w, `{"error":"plugin not running"}`, http.StatusServiceUnavailable)
			return
		}

		// Check zone restrictions
		requiredZones := entry.plugin.RequiredZones()
		if len(requiredZones) > 0 {
			// Get client's zone from request context (set by auth middleware)
			clientZone := r.Header.Get("X-Network-Zone")
			if !containsZone(requiredZones, clientZone) {
				http.Error(w, `{"error":"zone access denied"}`, http.StatusForbidden)
				return
			}
		}

		// Rewrite the path for the plugin handler
		r.URL.Path = remainingPath
		r.URL.RawPath = ""

		entry.handler.ServeHTTP(w, r)
	})
}

// List returns info about all registered plugins
func (l *Loader) List() []PluginStatus {
	l.mu.RLock()
	defer l.mu.RUnlock()

	result := make([]PluginStatus, 0, len(l.plugins))
	for _, entry := range l.plugins {
		status := PluginStatus{
			Info:     entry.info,
			State:    entry.state,
			Health:   entry.plugin.Health(),
			LoadedAt: entry.loadedAt,
		}
		if entry.startedAt != nil {
			status.StartedAt = *entry.startedAt
		}
		if entry.lastError != nil {
			status.LastError = entry.lastError.Error()
		}
		result = append(result, status)
	}

	// Sort by name
	sort.Slice(result, func(i, j int) bool {
		return result[i].Info.Name < result[j].Info.Name
	})

	return result
}

// PluginStatus contains runtime status of a plugin
type PluginStatus struct {
	Info      sdk.PluginInfo   `json:"info"`
	State     PluginState      `json:"state"`
	Health    sdk.HealthStatus `json:"health"`
	LoadedAt  time.Time        `json:"loaded_at"`
	StartedAt time.Time        `json:"started_at,omitempty"`
	LastError string           `json:"last_error,omitempty"`
}

// Get returns a specific plugin by name
func (l *Loader) Get(name string) (sdk.Plugin, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entry, exists := l.plugins[name]
	if !exists {
		return nil, false
	}
	return entry.plugin, true
}

// healthCheckLoop periodically checks plugin health
func (l *Loader) healthCheckLoop() {
	if l.config.HealthCheckInterval <= 0 {
		return
	}

	ticker := time.NewTicker(l.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-l.ctx.Done():
			return
		case <-ticker.C:
			l.checkHealth()
		}
	}
}

// checkHealth checks the health of all running plugins
func (l *Loader) checkHealth() {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for name, entry := range l.plugins {
		if entry.state != StateRunning {
			continue
		}

		health := entry.plugin.Health()
		if health.Status != sdk.HealthStatusHealthy {
			l.logger.Warn("plugin health degraded",
				"plugin", name,
				"status", health.Status,
				"message", health.Message,
			)
		}
	}
}

// Helper functions

func (l *Loader) isEnabled(name string) bool {
	for _, n := range l.config.EnabledPlugins {
		if n == name {
			return true
		}
	}
	return false
}

func (l *Loader) isDisabled(name string) bool {
	for _, n := range l.config.DisabledPlugins {
		if n == name {
			return true
		}
	}
	return false
}

func (l *Loader) runningCount() int {
	count := 0
	for _, entry := range l.plugins {
		if entry.state == StateRunning {
			count++
		}
	}
	return count
}

func isValidPluginName(name string) bool {
	if len(name) == 0 || len(name) > 64 {
		return false
	}
	for i, c := range name {
		if c >= 'a' && c <= 'z' {
			continue
		}
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '-' && i > 0 && i < len(name)-1 {
			continue
		}
		return false
	}
	return true
}

func containsZone(zones []string, zone string) bool {
	for _, z := range zones {
		if z == zone || z == "*" {
			return true
		}
	}
	return false
}
