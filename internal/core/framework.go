// Package core ties all LocalMesh components together.
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

	"github.com/FABLOUSFALCON/localmesh/internal/config"
	"github.com/FABLOUSFALCON/localmesh/internal/gateway"
)

// Framework is the main LocalMesh framework instance
type Framework struct {
	config  *config.Config
	gateway *gateway.Gateway
	logger  *slog.Logger

	mu      sync.RWMutex
	running bool
	nodeID  string

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new LocalMesh framework instance
func New(cfg *config.Config) (*Framework, error) {
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

	nodeID := cfg.Node.ID
	if nodeID == "" {
		nodeID = uuid.New().String()
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Framework{
		config: cfg,
		logger: logger,
		nodeID: nodeID,
		ctx:    ctx,
		cancel: cancel,
	}, nil
}

// Start initializes and starts all framework components
func (f *Framework) Start() error {
	f.mu.Lock()
	if f.running {
		f.mu.Unlock()
		return fmt.Errorf("framework already running")
	}
	f.mu.Unlock()

	f.logger.Info("starting LocalMesh", "node_id", f.nodeID)

	// Initialize HTTP gateway
	cfg := gateway.DefaultGatewayConfig()
	cfg.Host = f.config.Gateway.Host
	cfg.Port = f.config.Gateway.Port
	if f.config.Gateway.ProxyPort > 0 {
		cfg.ProxyPort = f.config.Gateway.ProxyPort
	}
	cfg.ReadTimeout = f.config.Gateway.ReadTimeout
	cfg.WriteTimeout = f.config.Gateway.WriteTimeout
	cfg.Logger = f.logger

	f.gateway = gateway.NewGateway(cfg)

	if err := f.gateway.Start(); err != nil {
		return fmt.Errorf("starting gateway: %w", err)
	}

	f.mu.Lock()
	f.running = true
	f.mu.Unlock()

	f.logger.Info("LocalMesh started", "gateway", f.config.GatewayAddr())
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
	f.cancel()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if f.gateway != nil {
		if err := f.gateway.Stop(ctx); err != nil {
			f.logger.Warn("error stopping gateway", "error", err)
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
