package cmd

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/FABLOUSFALCON/localmesh/internal/auth"
	"github.com/FABLOUSFALCON/localmesh/internal/config"
	"github.com/FABLOUSFALCON/localmesh/internal/core"
	"github.com/FABLOUSFALCON/localmesh/internal/mesh"
	"github.com/FABLOUSFALCON/localmesh/internal/network"
	"github.com/FABLOUSFALCON/localmesh/internal/registry"
	"github.com/FABLOUSFALCON/localmesh/internal/services"
	"github.com/FABLOUSFALCON/localmesh/internal/storage"
	"github.com/FABLOUSFALCON/localmesh/internal/tui"
	"github.com/FABLOUSFALCON/localmesh/plugins/attendance"
	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

var (
	// Version info
	versionStr = "dev"
	commitStr  = "none"
	dateStr    = "unknown"

	// Config file path
	cfgFile string
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "localmesh",
	Short: "LocalMesh - Secure campus mesh network framework",
	Long: `LocalMesh is a secure, offline-first framework for building 
location-aware services on campus mesh networks.

No internet. No GPS. Just WiFi-based identity and blazing fast local services.

Get started:
  localmesh init      Initialize a new LocalMesh node
  localmesh start     Start the framework
  localmesh dashboard Interactive TUI dashboard
  localmesh status    Check running services`,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

// SetVersionInfo sets version information from build flags
func SetVersionInfo(version, commit, date string) {
	versionStr = version
	commitStr = commit
	dateStr = date
}

func init() {
	cobra.OnInitialize(initConfig)

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./localmesh.yaml)")
	rootCmd.PersistentFlags().Bool("debug", false, "enable debug logging")

	// Add sub-commands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(pluginCmd)
	rootCmd.AddCommand(serviceCmd)
	rootCmd.AddCommand(networkCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(dashboardCmd)
	rootCmd.AddCommand(authCmd)

	// Top-level convenience commands for service registration
	rootCmd.AddCommand(registerCmd)
	rootCmd.AddCommand(unregisterCmd)
	rootCmd.AddCommand(servicesCmd)
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from flag
		// TODO: Load config from file
	}
}

// dashboardCmd launches the interactive TUI
var dashboardCmd = &cobra.Command{
	Use:     "dashboard",
	Aliases: []string{"ui", "tui"},
	Short:   "Launch interactive TUI dashboard",
	Long: `Launch an interactive terminal dashboard for LocalMesh.

The dashboard provides a btop-like interface to:
  - Monitor running services and their health
  - View mesh nodes and network zones
  - Watch real-time activity logs
  - Manage plugins and configuration

Keyboard shortcuts:
  d - Dashboard view
  s - Services view
  n - Network view
  l - Logs view
  p - Plugins view
  c - Configuration
  q - Quit`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run()
	},
}

// versionCmd shows version info
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("LocalMesh %s\n", versionStr)
		fmt.Printf("  Commit: %s\n", commitStr)
		fmt.Printf("  Built:  %s\n", dateStr)
	},
}

// initCmd initializes a new LocalMesh node
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new LocalMesh node",
	Long:  `Initialize a new LocalMesh node in the current directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🚀 Initializing LocalMesh node...")

		// Create directories
		dirs := []string{"data", "data/badger", "data/backups", "data/keys", "plugins", "configs"}
		for _, dir := range dirs {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create %s: %w", dir, err)
			}
		}

		// Create default config if not exists
		configPath := "localmesh.yaml"
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			defaultConfig := `# LocalMesh Configuration
node:
  name: my-node
  role: node
  zone: campus-main
  campus: "My Campus"
  building: "Main Building"

network:
  port: 8420
  discovery_interval: 30s

gateway:
  host: 0.0.0.0
  port: 8080
  hostname: campus  # Access at http://campus.local:8080

storage:
  data_dir: ./data
  sqlite_path: ./data/localmesh.db
  badger_path: ./data/badger

security:
  require_zone_auth: true
  token_ttl: 15m

log:
  level: info
  format: text
`
			if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
				return fmt.Errorf("failed to write config: %w", err)
			}
			fmt.Println("✅ Created localmesh.yaml")
		}

		fmt.Println("✅ LocalMesh initialized successfully!")
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Edit localmesh.yaml to configure your node")
		fmt.Println("  2. Run 'localmesh start' to start the framework")
		fmt.Println("  3. Run 'localmesh dashboard' for the interactive TUI")
		fmt.Println("\n💡 Tip: Change 'gateway.hostname' to set your .local URL")
		fmt.Println("         e.g., hostname: myuni → http://myuni.local:8080")
		return nil
	},
}

// startCmd starts the LocalMesh framework
var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the LocalMesh framework",
	Long:  `Start the LocalMesh gateway and all enabled plugins.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dev, _ := cmd.Flags().GetBool("dev")
		withDemoPlugins, _ := cmd.Flags().GetBool("demo-plugins")
		if dev {
			fmt.Println("🔧 Starting in development mode...")
		} else {
			fmt.Println("🚀 Starting LocalMesh...")
		}

		// Load configuration
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		if dev {
			cfg.Log.Level = "debug"
			withDemoPlugins = true // Auto-enable demo plugins in dev mode
		}

		// Create framework
		framework, err := core.New(cfg)
		if err != nil {
			return fmt.Errorf("creating framework: %w", err)
		}

		// Register demo plugins
		if withDemoPlugins {
			if err := registerDemoPlugins(framework); err != nil {
				return fmt.Errorf("registering demo plugins: %w", err)
			}
		}

		// Start framework
		if err := framework.Start(); err != nil {
			return fmt.Errorf("starting framework: %w", err)
		}

		fmt.Println("✅ LocalMesh is running!")
		fmt.Printf("   Gateway: http://%s\n", cfg.GatewayAddr())

		// Show .local hostname if configured
		hostname := cfg.Gateway.Hostname
		if hostname == "" {
			hostname = "campus"
		}
		fmt.Printf("   URL:     http://%s.local", hostname)
		if cfg.Gateway.Port != 80 {
			fmt.Printf(":%d", cfg.Gateway.Port)
		}
		fmt.Println()

		fmt.Printf("   mDNS:    %s on port %d\n", cfg.Network.ServiceName, cfg.Network.Port)
		if withDemoPlugins {
			fmt.Println("   Plugins: attendance, echo (demo)")
		}
		fmt.Println("\nPress Ctrl+C to stop...")

		// Wait for shutdown signal
		framework.Wait()

		// Graceful shutdown
		return framework.Stop()
	},
}

func init() {
	startCmd.Flags().Bool("dev", false, "start in development mode")
	startCmd.Flags().Bool("demo-plugins", false, "load demo plugins (attendance, echo)")
}

// registerDemoPlugins registers built-in demo plugins
func registerDemoPlugins(f *core.Framework) error {
	// Register attendance plugin
	if err := f.RegisterPlugin(attendance.New()); err != nil {
		return fmt.Errorf("registering attendance plugin: %w", err)
	}

	return nil
}

// stopCmd stops the LocalMesh framework
var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the LocalMesh framework",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("⏹️  Stopping LocalMesh...")
		// TODO: Implement graceful shutdown
		fmt.Println("✅ LocalMesh stopped")
		return nil
	},
}

// statusCmd shows current status
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show LocalMesh status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("📊 LocalMesh Status")
		fmt.Println("─────────────────────────────────")
		fmt.Println("  Status:    Not Running")
		fmt.Println("  Nodes:     0 discovered")
		fmt.Println("  Plugins:   0 loaded")
		fmt.Println("  Sync:      Not configured")
		// TODO: Implement real status check
		return nil
	},
}

// pluginCmd manages plugins
var pluginCmd = &cobra.Command{
	Use:   "plugin",
	Short: "Manage LocalMesh plugins",
	Long:  `Install, remove, enable, and disable LocalMesh plugins.`,
}

var pluginListCmd = &cobra.Command{
	Use:   "list",
	Short: "List installed plugins",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("📦 Installed Plugins")
		fmt.Println("─────────────────────────────────")
		fmt.Println("  (none installed)")
		// TODO: List plugins from registry
		return nil
	},
}

var pluginInstallCmd = &cobra.Command{
	Use:   "install <path>",
	Short: "Install a plugin",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("📥 Installing plugin from %s...\n", args[0])
		// TODO: Implement plugin installation
		return nil
	},
}

var pluginScaffoldCmd = &cobra.Command{
	Use:   "scaffold <name>",
	Short: "Generate a new plugin skeleton",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		fmt.Printf("🏗️  Scaffolding new plugin: %s\n", name)
		// TODO: Generate plugin boilerplate
		fmt.Printf("✅ Plugin scaffold created at plugins/%s/\n", name)
		return nil
	},
}

func init() {
	pluginCmd.AddCommand(pluginListCmd)
	pluginCmd.AddCommand(pluginInstallCmd)
	pluginCmd.AddCommand(pluginScaffoldCmd)
}

// networkCmd manages network operations
var networkCmd = &cobra.Command{
	Use:   "network",
	Short: "Network discovery and management",
}

var networkScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan for LocalMesh nodes on the network",
	RunE: func(cmd *cobra.Command, args []string) error {
		timeout, _ := cmd.Flags().GetInt("timeout")
		fmt.Printf("🔍 Scanning for LocalMesh nodes (timeout: %ds)...\n", timeout)

		// Create a temporary discovery instance for scanning
		cfg := mesh.DefaultDiscoveryConfig()
		discovery := mesh.NewDiscovery(cfg)

		discovery.OnNodeFound(func(node *mesh.Node) {
			fmt.Printf("  ✓ Found: %s (%s) - Zone: %s\n", node.Name, node.Host, node.Zone)
		})

		if err := discovery.Start(); err != nil {
			return fmt.Errorf("starting discovery: %w", err)
		}

		// Wait for scan duration
		time.Sleep(time.Duration(timeout) * time.Second)

		nodes := discovery.GetNodes()
		discovery.Stop()

		fmt.Printf("\n📊 Found %d node(s)\n", len(nodes))
		return nil
	},
}

var networkZonesCmd = &cobra.Command{
	Use:   "zones",
	Short: "List configured network zones",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🌐 Network Zones")
		fmt.Println("─────────────────────────────────")
		fmt.Println("  (none configured)")
		// TODO: List zones from config
		return nil
	},
}

// networkInterfacesCmd lists available network interfaces
var networkInterfacesCmd = &cobra.Command{
	Use:     "interfaces",
	Aliases: []string{"ifaces", "if"},
	Short:   "List available network interfaces",
	Long: `List all network interfaces that LocalMesh can use.

Shows interface name, type (wifi/ethernet/virtual), IP addresses, and status.
Use --all to include virtual interfaces (docker, veth, etc.)`,
	RunE: func(cmd *cobra.Command, args []string) error {
		showAll, _ := cmd.Flags().GetBool("all")

		fmt.Println("🔌 Network Interfaces")
		fmt.Println("─────────────────────────────────")

		var ifaces []network.Interface
		var err error

		if showAll {
			ifaces, err = network.ListInterfaces()
		} else {
			ifaces, err = network.ListUsableInterfaces()
		}

		if err != nil {
			return fmt.Errorf("failed to list interfaces: %w", err)
		}

		if len(ifaces) == 0 {
			fmt.Println("  No usable interfaces found")
			fmt.Println("  Use --all to show all interfaces")
			return nil
		}

		for _, iface := range ifaces {
			fmt.Printf("  %s\n", iface.FormatInterfaceInfo())
		}

		// Show primary interface suggestion
		primary, err := network.GetPrimaryInterface()
		if err == nil {
			fmt.Printf("\n💡 Suggested: %s (%s)\n", primary.Name, primary.Type)
		}

		// Show config hint
		fmt.Println("\n📝 Configure in localmesh.yaml:")
		fmt.Println("   network:")
		fmt.Println("     interfaces: [wlan0, eth0]")

		return nil
	},
}

func init() {
	networkScanCmd.Flags().Int("timeout", 5, "scan timeout in seconds")
	networkInterfacesCmd.Flags().Bool("all", false, "show all interfaces including virtual")
	networkCmd.AddCommand(networkScanCmd)
	networkCmd.AddCommand(networkZonesCmd)
	networkCmd.AddCommand(networkIdentityCmd)
	networkCmd.AddCommand(networkInterfacesCmd)
}

// networkIdentityCmd shows current network identity
var networkIdentityCmd = &cobra.Command{
	Use:   "identity",
	Short: "Show current network identity",
	Long:  `Detect and display the current network identity based on WiFi SSID and subnet.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		verbose, _ := cmd.Flags().GetBool("verbose")

		fmt.Println("🔐 Network Identity Detection")
		fmt.Println("─────────────────────────────────")

		// Create detector
		detector := network.NewDetector()

		// Detect identity
		identity, err := detector.DetectLocal(context.Background())
		if err != nil {
			return fmt.Errorf("detection failed: %w", err)
		}

		if identity == nil {
			fmt.Println("  ⚠️  No network identity detected")
			return nil
		}

		fmt.Printf("  Zone:       %s\n", identity.Zone)
		fmt.Printf("  ZoneSource: %s\n", identity.ZoneSource)
		fmt.Printf("  WiFi SSID:  %s\n", valueOrNA(identity.SSID))
		fmt.Printf("  BSSID:      %s\n", valueOrNA(identity.BSSID))
		fmt.Printf("  Interface:  %s\n", valueOrNA(identity.InterfaceName))
		fmt.Printf("  MAC:        %s\n", valueOrNA(identity.MacAddress))
		fmt.Printf("  Detected:   %s\n", identity.DetectedAt.Format(time.RFC1123))

		if verbose {
			fmt.Println("\n📡 IP Addresses")
			fmt.Println("─────────────────────────────────")
			for _, addr := range identity.IPAddresses {
				fmt.Printf("    - %s\n", addr)
			}
		}

		return nil
	},
}

func valueOrNA(s string) string {
	if s == "" {
		return "N/A"
	}
	return s
}

func init() {
	networkIdentityCmd.Flags().BoolP("verbose", "v", false, "show detailed interface info")
}

// syncCmd manages cloud sync
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Cloud sync operations",
}

var syncStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show sync status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("☁️  Sync Status")
		fmt.Println("─────────────────────────────────")
		fmt.Println("  Provider:  Not configured")
		fmt.Println("  Last sync: Never")
		return nil
	},
}

var syncNowCmd = &cobra.Command{
	Use:   "now",
	Short: "Trigger immediate sync",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("☁️  Syncing to cloud...")
		// TODO: Implement sync
		return nil
	},
}

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore from cloud backup",
	RunE: func(cmd *cobra.Command, args []string) error {
		from, _ := cmd.Flags().GetString("from")
		if from == "" {
			return fmt.Errorf("please specify --from <uri>")
		}
		fmt.Printf("📥 Restoring from %s...\n", from)
		// TODO: Implement restore
		return nil
	},
}

func init() {
	restoreCmd.Flags().String("from", "", "backup URI to restore from")
	syncCmd.AddCommand(syncStatusCmd)
	syncCmd.AddCommand(syncNowCmd)
	syncCmd.AddCommand(restoreCmd)
}

// authCmd manages authentication
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Authentication and user management",
}

var authCreateUserCmd = &cobra.Command{
	Use:   "create-user",
	Short: "Create a new user",
	RunE: func(cmd *cobra.Command, args []string) error {
		username, _ := cmd.Flags().GetString("username")
		password, _ := cmd.Flags().GetString("password")
		role, _ := cmd.Flags().GetString("role")
		zone, _ := cmd.Flags().GetString("zone")
		displayName, _ := cmd.Flags().GetString("name")

		if username == "" || password == "" {
			return fmt.Errorf("username and password are required")
		}
		if displayName == "" {
			displayName = username
		}
		if role == "" {
			role = "user"
		}
		if zone == "" {
			zone = "campus-main"
		}

		// Load config
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Initialize storage
		store, err := storage.New(storage.Options{
			SQLitePath: cfg.Storage.SQLitePath,
			BadgerPath: cfg.Storage.BadgerPath,
		})
		if err != nil {
			return fmt.Errorf("opening storage: %w", err)
		}
		defer store.Close()

		// Create user
		userStore := auth.NewUserStore(store.SQLite)
		user := &auth.User{
			Username:    username,
			DisplayName: displayName,
			Role:        role,
			Zone:        zone,
		}

		if err := userStore.Create(context.Background(), user, password); err != nil {
			return fmt.Errorf("creating user: %w", err)
		}

		fmt.Printf("✅ Created user: %s (ID: %s)\n", username, user.ID)
		fmt.Printf("   Role: %s, Zone: %s\n", role, zone)
		return nil
	},
}

var authListUsersCmd = &cobra.Command{
	Use:   "list-users",
	Short: "List all users",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		store, err := storage.New(storage.Options{
			SQLitePath: cfg.Storage.SQLitePath,
			BadgerPath: cfg.Storage.BadgerPath,
		})
		if err != nil {
			return fmt.Errorf("opening storage: %w", err)
		}
		defer store.Close()

		userStore := auth.NewUserStore(store.SQLite)
		users, err := userStore.List(context.Background(), 100, 0)
		if err != nil {
			return fmt.Errorf("listing users: %w", err)
		}

		fmt.Println("👥 Users")
		fmt.Println("─────────────────────────────────────────────────────")
		if len(users) == 0 {
			fmt.Println("  (no users)")
		}
		for _, u := range users {
			fmt.Printf("  %-15s %-10s %-15s %s\n", u.Username, u.Role, u.Zone, u.ID)
		}
		return nil
	},
}

func init() {
	authCreateUserCmd.Flags().StringP("username", "u", "", "username (required)")
	authCreateUserCmd.Flags().StringP("password", "p", "", "password (required)")
	authCreateUserCmd.Flags().StringP("role", "r", "user", "user role (admin, user)")
	authCreateUserCmd.Flags().StringP("zone", "z", "campus-main", "user's home zone")
	authCreateUserCmd.Flags().StringP("name", "n", "", "display name")

	authCmd.AddCommand(authCreateUserCmd)
	authCmd.AddCommand(authListUsersCmd)
}

// serviceCmd manages external services
var serviceCmd = &cobra.Command{
	Use:   "service",
	Short: "Manage external services",
	Long: `Register, remove, and manage external services that LocalMesh proxies to.

External services can be written in any language (Node.js, Python, Rust, etc.)
and LocalMesh will handle authentication, zone-based access, and routing.

Example:
  localmesh service add attendance http://localhost:3001 --zones=cs-department
  localmesh service list
  localmesh service remove attendance`,
}

var serviceAddCmd = &cobra.Command{
	Use:   "add <name> <url>",
	Short: "Register an external service",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		url := args[1]
		zones, _ := cmd.Flags().GetStringSlice("zones")
		description, _ := cmd.Flags().GetString("description")
		healthPath, _ := cmd.Flags().GetString("health")
		public, _ := cmd.Flags().GetBool("public")
		tags, _ := cmd.Flags().GetStringSlice("tags")

		fmt.Printf("📦 Registering service: %s\n", name)
		fmt.Printf("   URL: %s\n", url)

		// Create service
		svc := services.NewService(name, name, url)
		svc.Info.Description = description
		svc.Info.Tags = tags
		if healthPath != "" {
			svc.Endpoint.HealthPath = healthPath
		}
		svc.Access.Zones = zones
		svc.Access.Public = public
		svc.Access.RequireAuth = !public

		// Validate
		if err := svc.Validate(); err != nil {
			return fmt.Errorf("invalid service: %w", err)
		}

		// Save to config file
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Add to config
		cfg.Services = append(cfg.Services, config.ServiceConfig{
			Name:        name,
			URL:         url,
			HealthPath:  svc.Endpoint.HealthPath,
			Zones:       zones,
			Public:      public,
			Description: description,
			Tags:        tags,
		})

		// Save config
		if err := config.Save(cfgFile, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Println("✅ Service registered!")
		fmt.Printf("   Access: /svc/%s/*\n", name)
		if len(zones) > 0 {
			fmt.Printf("   Zones:  %v\n", zones)
		} else {
			fmt.Println("   Zones:  all (no restriction)")
		}
		return nil
	},
}

var serviceListCmd = &cobra.Command{
	Use:   "list",
	Short: "List registered services",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		fmt.Println("🌐 Registered Services (External)")
		fmt.Println("─────────────────────────────────────────────────────")

		if len(cfg.Services) == 0 {
			fmt.Println("  (no services registered)")
			fmt.Println("\n  Add a service:")
			fmt.Println("    localmesh service add <name> <url> --zones=zone1,zone2")
			return nil
		}

		for _, svc := range cfg.Services {
			status := "🔒"
			if svc.Public {
				status = "🌐"
			}
			zones := "all"
			if len(svc.Zones) > 0 {
				zones = fmt.Sprintf("%v", svc.Zones)
			}
			fmt.Printf("  %s %-15s → %s\n", status, svc.Name, svc.URL)
			fmt.Printf("      Zones: %s | Path: /svc/%s/*\n", zones, svc.Name)
		}

		fmt.Printf("\n  Total: %d service(s)\n", len(cfg.Services))
		return nil
	},
}

var serviceRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a registered service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Find and remove
		found := false
		newServices := make([]config.ServiceConfig, 0)
		for _, svc := range cfg.Services {
			if svc.Name == name {
				found = true
				continue
			}
			newServices = append(newServices, svc)
		}

		if !found {
			return fmt.Errorf("service %q not found", name)
		}

		cfg.Services = newServices

		if err := config.Save(cfgFile, cfg); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}

		fmt.Printf("✅ Removed service: %s\n", name)
		return nil
	},
}

var serviceHealthCmd = &cobra.Command{
	Use:   "health [name]",
	Short: "Check service health",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
		registry := services.NewRegistry(logger)

		// Register services from config
		for _, svcCfg := range cfg.Services {
			svc := services.NewService(svcCfg.Name, svcCfg.Name, svcCfg.URL)
			svc.Endpoint.HealthPath = svcCfg.HealthPath
			if svc.Endpoint.HealthPath == "" {
				svc.Endpoint.HealthPath = "/health"
			}
			registry.Register(svc)
		}

		fmt.Println("🏥 Service Health Check")
		fmt.Println("─────────────────────────────────────────────────────")

		services := registry.List()
		if len(services) == 0 {
			fmt.Println("  (no services)")
			return nil
		}

		for _, svc := range services {
			// Only check specified service if name provided
			if len(args) > 0 && svc.Info.Name != args[0] {
				continue
			}

			err := registry.CheckHealth(context.Background(), svc.Info.Name)
			status := "✅ healthy"
			if err != nil {
				status = "❌ unhealthy"
			}

			fmt.Printf("  %-15s %s (%s)\n", svc.Info.Name, status, svc.Health.Latency.Round(time.Millisecond))
		}

		registry.Close()
		return nil
	},
}

var serviceDiscoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover services on the local network",
	RunE: func(cmd *cobra.Command, args []string) error {
		timeout, _ := cmd.Flags().GetInt("timeout")

		fmt.Printf("🔍 Discovering services via mDNS (timeout: %ds)...\n", timeout)

		logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
		registry := services.NewRegistry(logger)
		discovery := services.NewDiscovery(registry, logger, services.DiscoveryConfig{
			InstanceName: "localmesh-cli",
			Advertise:    false, // Don't advertise, just discover
		})

		if err := discovery.Start(); err != nil {
			return fmt.Errorf("starting discovery: %w", err)
		}

		time.Sleep(time.Duration(timeout) * time.Second)

		nodes := discovery.ListDiscovered()

		fmt.Println("\n📡 Discovered LocalMesh Nodes")
		fmt.Println("─────────────────────────────────────────────────────")

		if len(nodes) == 0 {
			fmt.Println("  (no nodes discovered)")
		} else {
			for _, node := range nodes {
				fmt.Printf("  %s (%s:%d)\n", node.Name, node.Host, node.Port)
				if len(node.Services) > 0 {
					fmt.Printf("    Services: %v\n", node.Services)
				}
			}
		}

		discovery.Stop()
		registry.Close()

		return nil
	},
}

func init() {
	// Service add flags
	serviceAddCmd.Flags().StringSlice("zones", nil, "required zones (comma-separated)")
	serviceAddCmd.Flags().String("description", "", "service description")
	serviceAddCmd.Flags().String("health", "/health", "health check path")
	serviceAddCmd.Flags().Bool("public", false, "make service public (no auth)")
	serviceAddCmd.Flags().StringSlice("tags", nil, "service tags (comma-separated)")

	// Service discover flags
	serviceDiscoverCmd.Flags().Int("timeout", 5, "discovery timeout in seconds")

	serviceCmd.AddCommand(serviceAddCmd)
	serviceCmd.AddCommand(serviceListCmd)
	serviceCmd.AddCommand(serviceRemoveCmd)
	serviceCmd.AddCommand(serviceHealthCmd)
	serviceCmd.AddCommand(serviceDiscoverCmd)
}

// =============================================================================
// TOP-LEVEL CONVENIENCE COMMANDS FOR SERVICE REGISTRATION
// =============================================================================
// These are the user-facing commands for the "developer registers their service" workflow:
//   localmesh register myapp --port 3000
//   localmesh unregister myapp
//   localmesh services

var mdnsRegistry *registry.MDNSRegistry

// registerCmd registers a service with mDNS hostname
var registerCmd = &cobra.Command{
	Use:   "register <name> --port <port>",
	Short: "Register a service with a .local hostname",
	Long: `Register a local service and advertise it via mDNS.

This creates a friendly URL for accessing your service:
  localmesh register myapp --port 3000
  → Access at http://myapp.campus.local:3000

The hostname uses the format: <name>.<gateway-hostname>.local

Examples:
  localmesh register lecture --port 3000        # http://lecture.campus.local:3000
  localmesh register api --port 8080            # http://api.campus.local:8080
  localmesh register frontend --port 5173 -i wlan0  # Use specific interface

Requirements:
  - avahi-utils must be installed (sudo apt install avahi-utils)
  - The service must already be running on the specified port`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		port, _ := cmd.Flags().GetInt("port")
		iface, _ := cmd.Flags().GetString("interface")
		description, _ := cmd.Flags().GetString("description")

		if port == 0 {
			return fmt.Errorf("--port is required")
		}

		// Load config to get hostname
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		domain := cfg.Gateway.Hostname
		if domain == "" {
			domain = "campus"
		}

		// Create registry
		reg := registry.NewMDNSRegistry(registry.MDNSRegistryConfig{
			Domain:  domain,
			DataDir: cfg.Storage.DataDir,
			Logger:  slog.Default(),
		})

		// Register the service
		svc, err := reg.Register(name, port, registry.MDNSRegisterOptions{
			Interface:   iface,
			Description: description,
		})
		if err != nil {
			return fmt.Errorf("registration failed: %w", err)
		}

		fmt.Printf("✅ Service registered!\n")
		fmt.Printf("   Name:     %s\n", svc.Name)
		fmt.Printf("   URL:      %s\n", svc.URL)
		fmt.Printf("   IP:       %s\n", svc.IP)
		fmt.Printf("   Port:     %d\n", svc.Port)
		fmt.Printf("   Hostname: %s\n", svc.Hostname)
		fmt.Println()
		fmt.Printf("🌐 Access your service at: %s\n", svc.URL)
		fmt.Println()
		fmt.Println("⚠️  Keep this process running to maintain the mDNS advertisement.")
		fmt.Println("   Press Ctrl+C to unregister and exit.")

		// Wait for interrupt
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh

		fmt.Println("\n🛑 Unregistering service...")
		reg.Stop()
		fmt.Println("✅ Service unregistered")

		return nil
	},
}

// unregisterCmd removes a service registration
var unregisterCmd = &cobra.Command{
	Use:   "unregister <name>",
	Short: "Unregister a service (stop mDNS advertisement)",
	Long: `Stop the mDNS advertisement for a registered service.

This removes the .local hostname from the network.

Example:
  localmesh unregister myapp`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		// Load config
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		domain := cfg.Gateway.Hostname
		if domain == "" {
			domain = "campus"
		}

		// Create registry and load existing services
		reg := registry.NewMDNSRegistry(registry.MDNSRegistryConfig{
			Domain:  domain,
			DataDir: cfg.Storage.DataDir,
			Logger:  slog.Default(),
		})

		if err := reg.Load(); err != nil {
			return fmt.Errorf("loading services: %w", err)
		}

		// Check if service exists
		svc, exists := reg.Get(name)
		if !exists {
			return fmt.Errorf("service %q is not registered", name)
		}

		// Kill the avahi process if it's still running
		if svc.PID > 0 {
			proc, err := os.FindProcess(svc.PID)
			if err == nil && proc != nil {
				proc.Kill()
			}
		}

		if err := reg.Unregister(name); err != nil {
			return fmt.Errorf("unregistration failed: %w", err)
		}

		fmt.Printf("✅ Service %q unregistered\n", name)
		return nil
	},
}

// servicesCmd lists all registered services
var servicesCmd = &cobra.Command{
	Use:     "services",
	Aliases: []string{"svc", "ls"},
	Short:   "List registered services with mDNS hostnames",
	Long: `List all services that have been registered with LocalMesh.

These services are accessible via their .local hostnames.

Example:
  localmesh services`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load config
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		domain := cfg.Gateway.Hostname
		if domain == "" {
			domain = "campus"
		}

		// Create registry and load existing services
		reg := registry.NewMDNSRegistry(registry.MDNSRegistryConfig{
			Domain:  domain,
			DataDir: cfg.Storage.DataDir,
			Logger:  slog.Default(),
		})

		if err := reg.Load(); err != nil {
			return fmt.Errorf("loading services: %w", err)
		}

		services := reg.List()

		fmt.Printf("🌐 Registered Services (domain: %s.local)\n", domain)
		fmt.Println("─────────────────────────────────────────────────────")

		if len(services) == 0 {
			fmt.Println("  (no services registered)")
			fmt.Println()
			fmt.Println("  Register a service:")
			fmt.Println("    localmesh register myapp --port 3000")
			return nil
		}

		for _, svc := range services {
			status := "❓"
			if svc.Healthy {
				status = "✅"
			} else {
				status = "❌"
			}
			fmt.Printf("  %s %-15s %s\n", status, svc.Name, svc.URL)
			fmt.Printf("      → %s:%d (PID: %d)\n", svc.IP, svc.Port, svc.PID)
		}

		fmt.Printf("\n  Total: %d service(s)\n", len(services))
		return nil
	},
}

func init() {
	// Register command flags
	registerCmd.Flags().IntP("port", "p", 0, "port the service is running on (required)")
	registerCmd.Flags().StringP("interface", "i", "", "network interface to use for IP detection")
	registerCmd.Flags().StringP("description", "d", "", "service description")
	registerCmd.MarkFlagRequired("port")
}
