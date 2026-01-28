// Package core ties all LocalMesh components together.
// This is the main entry point for starting the framework.
package core

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"

	"github.com/FABLOUSFALCON/localmesh/internal/auth"
	"github.com/FABLOUSFALCON/localmesh/internal/config"
	"github.com/FABLOUSFALCON/localmesh/internal/gateway"
	"github.com/FABLOUSFALCON/localmesh/internal/mesh"
	"github.com/FABLOUSFALCON/localmesh/internal/network"
	"github.com/FABLOUSFALCON/localmesh/internal/plugins"
	"github.com/FABLOUSFALCON/localmesh/internal/registry"
	"github.com/FABLOUSFALCON/localmesh/internal/services"
	"github.com/FABLOUSFALCON/localmesh/internal/storage"
	"github.com/FABLOUSFALCON/localmesh/pkg/sdk"
)

// Framework is the main LocalMesh framework instance
type Framework struct {
	config    *config.Config
	storage   *storage.Storage
	discovery *mesh.Discovery
	registry  *registry.Registry
	gateway   *gateway.Gateway
	hostname  *gateway.HostnameAdvertiser
	auth      *auth.Service
	network   *network.Service
	plugins   *plugins.Loader
	logger    *slog.Logger

	// State
	mu      sync.RWMutex
	running bool
	nodeID  string

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new LocalMesh framework instance
func New(cfg *config.Config) (*Framework, error) {
	// Setup logging
	logLevel := slog.LevelInfo
	switch cfg.Log.Level {
	case "debug":
		logLevel = slog.LevelDebug
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	}

	var handler slog.Handler
	if cfg.Log.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}
	logger := slog.New(handler)

	// Generate or load node ID
	nodeID := cfg.Node.ID
	if nodeID == "" {
		nodeID = uuid.New().String()
	}

	ctx, cancel := context.WithCancel(context.Background())

	f := &Framework{
		config: cfg,
		logger: logger,
		nodeID: nodeID,
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize plugin loader early so plugins can be registered before Start
	pluginCfg := plugins.DefaultLoaderConfig()
	pluginCfg.PluginDir = cfg.Storage.DataDir + "/plugins"
	pluginCfg.Logger = logger
	f.plugins = plugins.NewLoader(pluginCfg)

	return f, nil
}

// Start initializes and starts all framework components
func (f *Framework) Start() error {
	f.mu.Lock()
	if f.running {
		f.mu.Unlock()
		return fmt.Errorf("framework already running")
	}
	f.mu.Unlock()

	f.logger.Info("starting LocalMesh",
		"node_id", f.nodeID,
		"zone", f.config.Node.Zone,
	)

	// 1. Initialize storage
	if err := f.initStorage(); err != nil {
		return fmt.Errorf("initializing storage: %w", err)
	}

	// 2. Initialize network identity detection
	if err := f.initNetwork(); err != nil {
		return fmt.Errorf("initializing network: %w", err)
	}

	// 3. Initialize auth service
	if err := f.initAuth(); err != nil {
		return fmt.Errorf("initializing auth: %w", err)
	}

	// 4. Initialize service registry
	if err := f.initRegistry(); err != nil {
		return fmt.Errorf("initializing registry: %w", err)
	}

	// 5. Initialize mDNS discovery
	if err := f.initDiscovery(); err != nil {
		return fmt.Errorf("initializing discovery: %w", err)
	}

	// 6. Initialize HTTP gateway
	if err := f.initGateway(); err != nil {
		return fmt.Errorf("initializing gateway: %w", err)
	}

	// 7. Start plugins (after gateway is ready)
	if err := f.plugins.Start(f.ctx); err != nil {
		return fmt.Errorf("starting plugins: %w", err)
	}

	f.mu.Lock()
	f.running = true
	f.mu.Unlock()

	f.logger.Info("LocalMesh started successfully",
		"gateway", f.config.GatewayAddr(),
		"services", f.registry.Count(),
		"plugins", len(f.plugins.List()),
	)

	return nil
}

// initStorage sets up SQLite and Badger
func (f *Framework) initStorage() error {
	store, err := storage.New(storage.Options{
		SQLitePath: f.config.Storage.SQLitePath,
		BadgerPath: f.config.Storage.BadgerPath,
		Logger:     f.logger,
	})
	if err != nil {
		return err
	}

	f.storage = store
	f.logger.Debug("storage initialized",
		"sqlite", f.config.Storage.SQLitePath,
		"badger", f.config.Storage.BadgerPath,
	)

	return nil
}

// initNetwork sets up network identity detection
func (f *Framework) initNetwork() error {
	cfg := network.DefaultServiceConfig()
	cfg.Logger = f.logger

	// Add default zone mapping from config
	cfg.ZoneMappings = append(cfg.ZoneMappings, network.ZoneMapping{
		ID:       f.config.Node.Zone,
		Zone:     f.config.Node.Zone,
		Priority: 1,
	})

	// Load zone mappings from config if available
	for _, zone := range f.config.Zones {
		cfg.ZoneMappings = append(cfg.ZoneMappings, network.ZoneMapping{
			ID:          zone.ID,
			Zone:        zone.ID,
			SSIDs:       zone.SSIDs,
			Subnets:     zone.Subnets,
			Description: zone.Description,
			Priority:    zone.Priority,
		})
	}

	f.network = network.NewService(cfg)

	if err := f.network.Start(); err != nil {
		return err
	}

	f.logger.Debug("network identity detection initialized",
		"zone", f.config.Node.Zone,
		"mappings", len(cfg.ZoneMappings),
	)

	return nil
}

// initAuth sets up the authentication service
func (f *Framework) initAuth() error {
	cfg := auth.ServiceConfig{
		KeyPath:         f.config.Storage.DataDir + "/keys",
		AccessTokenTTL:  f.config.Security.TokenTTL,
		RefreshTokenTTL: f.config.Security.RefreshTokenTTL,
		SessionTTL:      f.config.Security.RefreshTokenTTL,
		MaxSessions:     f.config.Security.MaxSessions,
		SQLite:          f.storage.SQLite,
		Badger:          f.storage.Badger,
		Logger:          f.logger,
	}

	authService, err := auth.NewService(cfg)
	if err != nil {
		return err
	}

	f.auth = authService

	// Register default zone based on config
	if err := f.auth.RegisterZone(&auth.Zone{
		ID:          f.config.Node.Zone,
		Name:        f.config.Node.Zone,
		Description: "Local node zone",
		AccessLevel: 0,
	}); err != nil {
		return err
	}

	f.logger.Debug("auth service initialized",
		"key_path", cfg.KeyPath,
		"access_ttl", cfg.AccessTokenTTL,
	)

	return nil
}

// initRegistry sets up the service registry
func (f *Framework) initRegistry() error {
	cfg := registry.DefaultRegistryConfig()
	cfg.NodeID = f.nodeID
	cfg.Zone = f.config.Node.Zone
	cfg.SQLite = f.storage.SQLite
	cfg.Badger = f.storage.Badger
	cfg.Logger = f.logger

	f.registry = registry.NewRegistry(cfg)
	return f.registry.Start()
}

// initDiscovery sets up mDNS discovery
func (f *Framework) initDiscovery() error {
	cfg := mesh.DefaultDiscoveryConfig()
	cfg.NodeID = f.nodeID
	cfg.NodeName = f.config.Node.Name
	cfg.Zone = f.config.Node.Zone
	cfg.Port = f.config.Network.Port
	cfg.ServiceName = f.config.Network.ServiceName
	cfg.Logger = f.logger

	f.discovery = mesh.NewDiscovery(cfg)

	// Set up callbacks
	f.discovery.OnNodeFound(func(node *mesh.Node) {
		f.logger.Info("node discovered",
			"node_id", node.ID,
			"host", node.Host,
			"zone", node.Zone,
		)
	})

	f.discovery.OnNodeLost(func(node *mesh.Node) {
		f.logger.Info("node lost",
			"node_id", node.ID,
		)
	})

	return f.discovery.Start()
}

// initGateway sets up the HTTP gateway
func (f *Framework) initGateway() error {
	cfg := gateway.DefaultGatewayConfig()
	cfg.Host = f.config.Gateway.Host
	cfg.Port = f.config.Gateway.Port
	cfg.ReadTimeout = f.config.Gateway.ReadTimeout
	cfg.WriteTimeout = f.config.Gateway.WriteTimeout
	cfg.Registry = f.registry
	cfg.Auth = f.auth
	cfg.Logger = f.logger

	f.gateway = gateway.NewGateway(cfg)

	// Register network identity routes
	if f.network != nil {
		networkHandler := network.NewHandler(f.network)
		networkHandler.RegisterRoutes(f.gateway.Mux())
	}

	// Register plugin routes
	if f.plugins != nil {
		pluginHandler := plugins.NewHandler(f.plugins)
		pluginHandler.RegisterRoutes(f.gateway.Mux())
	}

	// Register external services from config
	if err := f.initExternalServices(); err != nil {
		return fmt.Errorf("initializing external services: %w", err)
	}

	// Start the gateway
	if err := f.gateway.Start(); err != nil {
		return err
	}

	// Start hostname advertiser for .local domain
	f.initHostnameAdvertiser()

	return nil
}

// initHostnameAdvertiser sets up mDNS hostname for gateway
func (f *Framework) initHostnameAdvertiser() {
	hostname := f.config.Gateway.Hostname
	if hostname == "" {
		hostname = "mesh" // Default
	}

	cfg := gateway.HostnameConfig{
		Hostname: hostname,
		Port:     f.config.Gateway.Port,
		Logger:   f.logger,
	}

	f.hostname = gateway.NewHostnameAdvertiser(cfg)

	if err := f.hostname.Start(); err != nil {
		// Non-fatal - gateway still works without .local hostname
		f.logger.Warn("failed to start hostname advertiser",
			"error", err,
			"note", "gateway still accessible via IP",
		)
		return
	}

	f.logger.Info("gateway accessible at",
		"url", f.hostname.URL(),
		"hostname", f.hostname.Hostname()+".local",
	)
}

// initExternalServices registers external services from config
func (f *Framework) initExternalServices() error {
	if len(f.config.Services) == 0 {
		f.logger.Debug("no external services configured")
		return nil
	}

	for _, svcCfg := range f.config.Services {
		svc := services.NewService(svcCfg.Name, svcCfg.Name, svcCfg.URL)
		svc.Info.Description = svcCfg.Description
		svc.Info.Tags = svcCfg.Tags

		if svcCfg.HealthPath != "" {
			svc.Endpoint.HealthPath = svcCfg.HealthPath
		}

		svc.Access.Zones = svcCfg.Zones
		svc.Access.Roles = svcCfg.Roles
		svc.Access.Public = svcCfg.Public
		svc.Access.RequireAuth = !svcCfg.Public

		if err := f.gateway.RegisterService(svc); err != nil {
			f.logger.Warn("failed to register service",
				"service", svcCfg.Name,
				"error", err,
			)
			continue
		}

		f.logger.Info("registered external service",
			"name", svcCfg.Name,
			"url", svcCfg.URL,
			"zones", svcCfg.Zones,
		)
	}

	// Start service discovery and health checks
	instanceName := fmt.Sprintf("localmesh-%s", f.nodeID[:8])
	if err := f.gateway.StartServiceDiscovery(instanceName, f.config.Gateway.Port, true); err != nil {
		f.logger.Warn("failed to start service discovery", "error", err)
	}

	f.gateway.StartHealthChecks(30 * time.Second)

	f.logger.Info("external services initialized",
		"count", len(f.config.Services),
	)

	return nil
}

// Stop gracefully shuts down all components
func (f *Framework) Stop() error {
	f.mu.Lock()
	if !f.running {
		f.mu.Unlock()
		return nil
	}
	f.running = false
	f.mu.Unlock()

	f.logger.Info("stopping LocalMesh")

	// Cancel context
	f.cancel()

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop plugins first
	if f.plugins != nil {
		if err := f.plugins.Stop(ctx); err != nil {
			f.logger.Warn("error stopping plugins", "error", err)
		}
	}

	// Stop components in reverse order
	if f.hostname != nil {
		f.hostname.Stop()
	}

	if f.gateway != nil {
		if err := f.gateway.Stop(ctx); err != nil {
			f.logger.Warn("error stopping gateway", "error", err)
		}
	}

	if f.discovery != nil {
		if err := f.discovery.Stop(); err != nil {
			f.logger.Warn("error stopping discovery", "error", err)
		}
	}

	if f.registry != nil {
		if err := f.registry.Stop(); err != nil {
			f.logger.Warn("error stopping registry", "error", err)
		}
	}

	if f.storage != nil {
		if err := f.storage.Close(); err != nil {
			f.logger.Warn("error closing storage", "error", err)
		}
	}

	f.logger.Info("LocalMesh stopped")
	return nil
}

// Wait blocks until a shutdown signal is received
func (f *Framework) Wait() {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	f.logger.Info("received signal", "signal", sig)
}

// --- Accessors ---

// Registry returns the service registry
func (f *Framework) Registry() *registry.Registry {
	return f.registry
}

// Discovery returns the mesh discovery service
func (f *Framework) Discovery() *mesh.Discovery {
	return f.discovery
}

// Storage returns the storage layer
func (f *Framework) Storage() *storage.Storage {
	return f.storage
}

// Config returns the current configuration
func (f *Framework) Config() *config.Config {
	return f.config
}

// NodeID returns this node's unique identifier
func (f *Framework) NodeID() string {
	return f.nodeID
}

// IsRunning returns true if the framework is running
func (f *Framework) IsRunning() bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.running
}

// Auth returns the authentication service
func (f *Framework) Auth() *auth.Service {
	return f.auth
}

// Network returns the network identity service
func (f *Framework) Network() *network.Service {
	return f.network
}

// Plugins returns the plugin loader
func (f *Framework) Plugins() *plugins.Loader {
	return f.plugins
}

// RegisterPlugin registers a plugin with the framework.
// Must be called before Start().
func (f *Framework) RegisterPlugin(plugin sdk.Plugin) error {
	if f.plugins == nil {
		return fmt.Errorf("plugin loader not initialized")
	}
	return f.plugins.Register(plugin)
}
