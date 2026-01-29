// Package config handles all configuration loading and management.
// Uses Viper for flexible config from files, env vars, and defaults.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Config holds all LocalMesh configuration
type Config struct {
	Node     NodeConfig      `mapstructure:"node"`
	Network  NetworkConfig   `mapstructure:"network"`
	Storage  StorageConfig   `mapstructure:"storage"`
	Security SecurityConfig  `mapstructure:"security"`
	Gateway  GatewayConfig   `mapstructure:"gateway"`
	Sync     SyncConfig      `mapstructure:"sync"`
	Log      LogConfig       `mapstructure:"log"`
	Zones    []ZoneConfig    `mapstructure:"zones"`
	Services []ServiceConfig `mapstructure:"services"`
}

// ServiceConfig defines an external service registration
type ServiceConfig struct {
	Name        string   `mapstructure:"name"`
	URL         string   `mapstructure:"url"`
	HealthPath  string   `mapstructure:"health_path"`
	Zones       []string `mapstructure:"zones"`
	Roles       []string `mapstructure:"roles"`
	Public      bool     `mapstructure:"public"`
	Description string   `mapstructure:"description"`
	Tags        []string `mapstructure:"tags"`
}

// ZoneConfig defines a network zone mapping
type ZoneConfig struct {
	ID          string   `mapstructure:"id"`
	SSIDs       []string `mapstructure:"ssids"`
	Subnets     []string `mapstructure:"subnets"`
	BSSIDs      []string `mapstructure:"bssids"`
	Description string   `mapstructure:"description"`
	Priority    int      `mapstructure:"priority"`
}

// NodeConfig identifies this node in the mesh
type NodeConfig struct {
	ID       string `mapstructure:"id"`
	Name     string `mapstructure:"name"`
	Role     string `mapstructure:"role"`
	Zone     string `mapstructure:"zone"`
	Campus   string `mapstructure:"campus"`
	Building string `mapstructure:"building"`
}

// NetworkConfig for mesh networking
type NetworkConfig struct {
	ServiceName       string        `mapstructure:"service_name"`
	Domain            string        `mapstructure:"domain"`
	Port              int           `mapstructure:"port"`
	TTL               time.Duration `mapstructure:"ttl"`
	DiscoveryInterval time.Duration `mapstructure:"discovery_interval"`
	HealthCheckPeriod time.Duration `mapstructure:"health_check_period"`
	Interfaces        []string      `mapstructure:"interfaces"`
	AllowedSubnets    []string      `mapstructure:"allowed_subnets"`
}

// StorageConfig for database paths
type StorageConfig struct {
	DataDir         string        `mapstructure:"data_dir"`
	SQLitePath      string        `mapstructure:"sqlite_path"`
	BadgerPath      string        `mapstructure:"badger_path"`
	BackupDir       string        `mapstructure:"backup_dir"`
	BackupInterval  time.Duration `mapstructure:"backup_interval"`
	MaxBackups      int           `mapstructure:"max_backups"`
	CompactInterval time.Duration `mapstructure:"compact_interval"`
}

// SecurityConfig for authentication and encryption
type SecurityConfig struct {
	TokenTTL        time.Duration `mapstructure:"token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
	KeyPath         string        `mapstructure:"key_path"`
	RequireZoneAuth bool          `mapstructure:"require_zone_auth"`
	RateLimit       int           `mapstructure:"rate_limit"`
	RateLimitBurst  int           `mapstructure:"rate_limit_burst"`
	RateLimitWindow time.Duration `mapstructure:"rate_limit_window"`
	MaxSessions     int           `mapstructure:"max_sessions"`
}

// GatewayConfig for HTTP gateway
type GatewayConfig struct {
	Host         string        `mapstructure:"host"`
	Port         int           `mapstructure:"port"`
	Hostname     string        `mapstructure:"hostname"` // .local hostname (e.g., "campus" → campus.local)
	TLSEnabled   bool          `mapstructure:"tls_enabled"`
	CertFile     string        `mapstructure:"cert_file"`
	KeyFile      string        `mapstructure:"key_file"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	IdleTimeout  time.Duration `mapstructure:"idle_timeout"`
	MaxBodySize  int64         `mapstructure:"max_body_size"`
	CORSOrigins  []string      `mapstructure:"cors_origins"`
}

// SyncConfig for cloud backup
type SyncConfig struct {
	Enabled      bool          `mapstructure:"enabled"`
	Provider     string        `mapstructure:"provider"`
	Bucket       string        `mapstructure:"bucket"`
	Prefix       string        `mapstructure:"prefix"`
	Endpoint     string        `mapstructure:"endpoint"`
	SyncInterval time.Duration `mapstructure:"sync_interval"`
	RetryCount   int           `mapstructure:"retry_count"`
	RetryDelay   time.Duration `mapstructure:"retry_delay"`
}

// LogConfig for logging
type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
	Output string `mapstructure:"output"`
	File   string `mapstructure:"file"`
}

var (
	cfg   *Config
	cfgMu sync.RWMutex
)

// Load reads configuration from file and environment
func Load(configPath string) (*Config, error) {
	v := viper.New()
	setDefaults(v)

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("localmesh")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
		v.AddConfigPath("/etc/localmesh")
		if home, _ := os.UserHomeDir(); home != "" {
			v.AddConfigPath(filepath.Join(home, ".config", "localmesh"))
		}
	}

	v.SetEnvPrefix("LOCALMESH")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("reading config: %w", err)
		}
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	cfgMu.Lock()
	cfg = &config
	cfgMu.Unlock()

	return &config, nil
}

// Get returns the current configuration
func Get() *Config {
	cfgMu.RLock()
	defer cfgMu.RUnlock()
	return cfg
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("node.role", "node")
	v.SetDefault("node.zone", "default")

	v.SetDefault("network.service_name", "_localmesh._tcp")
	v.SetDefault("network.domain", "local.")
	v.SetDefault("network.port", 8420)
	v.SetDefault("network.ttl", "60s")
	v.SetDefault("network.discovery_interval", "30s")
	v.SetDefault("network.health_check_period", "10s")

	v.SetDefault("storage.data_dir", "./data")
	v.SetDefault("storage.sqlite_path", "./data/localmesh.db")
	v.SetDefault("storage.badger_path", "./data/badger")
	v.SetDefault("storage.backup_dir", "./data/backups")
	v.SetDefault("storage.backup_interval", "1h")
	v.SetDefault("storage.max_backups", 24)
	v.SetDefault("storage.compact_interval", "24h")

	v.SetDefault("security.token_ttl", "15m")
	v.SetDefault("security.refresh_token_ttl", "24h")
	v.SetDefault("security.key_path", "./data/keys")
	v.SetDefault("security.require_zone_auth", true)
	v.SetDefault("security.rate_limit", 100)
	v.SetDefault("security.rate_limit_burst", 20)
	v.SetDefault("security.rate_limit_window", "1m")
	v.SetDefault("security.max_sessions", 5)

	v.SetDefault("gateway.host", "0.0.0.0")
	v.SetDefault("gateway.port", 8080)
	v.SetDefault("gateway.hostname", "campus") // campus.local
	v.SetDefault("gateway.tls_enabled", false)
	v.SetDefault("gateway.read_timeout", "30s")
	v.SetDefault("gateway.write_timeout", "30s")
	v.SetDefault("gateway.idle_timeout", "120s")
	v.SetDefault("gateway.max_body_size", 10485760)
	v.SetDefault("gateway.cors_origins", []string{"*"})

	v.SetDefault("sync.enabled", false)
	v.SetDefault("sync.provider", "local")
	v.SetDefault("sync.sync_interval", "5m")
	v.SetDefault("sync.retry_count", 3)
	v.SetDefault("sync.retry_delay", "10s")

	v.SetDefault("log.level", "info")
	v.SetDefault("log.format", "text")
	v.SetDefault("log.output", "stdout")
}

func (c *Config) validate() error {
	c.Storage.DataDir = expandPath(c.Storage.DataDir)
	c.Storage.SQLitePath = expandPath(c.Storage.SQLitePath)
	c.Storage.BadgerPath = expandPath(c.Storage.BadgerPath)
	c.Storage.BackupDir = expandPath(c.Storage.BackupDir)
	c.Security.KeyPath = expandPath(c.Security.KeyPath)

	dirs := []string{
		c.Storage.DataDir,
		c.Storage.BadgerPath,
		c.Storage.BackupDir,
		c.Security.KeyPath,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	if c.Network.Port < 1 || c.Network.Port > 65535 {
		return fmt.Errorf("invalid network port: %d", c.Network.Port)
	}
	if c.Gateway.Port < 1 || c.Gateway.Port > 65535 {
		return fmt.Errorf("invalid gateway port: %d", c.Gateway.Port)
	}

	return nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

// GatewayAddr returns the gateway listen address
func (c *Config) GatewayAddr() string {
	return fmt.Sprintf("%s:%d", c.Gateway.Host, c.Gateway.Port)
}

// NetworkAddr returns the mDNS listen address
func (c *Config) NetworkAddr() string {
	return fmt.Sprintf(":%d", c.Network.Port)
}

// Save writes the configuration to a file
func Save(configPath string, c *Config) error {
	if configPath == "" {
		configPath = "localmesh.yaml"
	}

	v := viper.New()
	v.SetConfigFile(configPath)
	v.SetConfigType("yaml")

	// Set all config values
	v.Set("node", c.Node)
	v.Set("network", c.Network)
	v.Set("storage", c.Storage)
	v.Set("security", c.Security)
	v.Set("gateway", c.Gateway)
	v.Set("sync", c.Sync)
	v.Set("log", c.Log)
	v.Set("zones", c.Zones)
	v.Set("services", c.Services)

	return v.WriteConfig()
}
