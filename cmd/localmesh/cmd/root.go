package cmd

import (
	"fmt"
	"os"

	"github.com/FABLOUSFALCON/localmesh/internal/config"
	"github.com/FABLOUSFALCON/localmesh/internal/core"
	"github.com/spf13/cobra"
)

var (
	versionStr = "dev"
	commitStr  = "none"
	dateStr    = "unknown"
	cfgFile    string
)

var rootCmd = &cobra.Command{
	Use:   "localmesh",
	Short: "LocalMesh - Local service mesh with mDNS advertising",
	Long: `LocalMesh is a lightweight service mesh for local networks.
It enables service discovery and registration via mDNS.

Get started:
  localmesh init      Initialize a new LocalMesh node
  localmesh start     Start the LocalMesh server
  localmesh status    Check running services

Use localmesh-agent to register services from any device on the network.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func SetVersionInfo(version, commit, date string) {
	versionStr = version
	commitStr = commit
	dateStr = date
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ./localmesh.yaml)")
	rootCmd.PersistentFlags().Bool("debug", false, "enable debug logging")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(stopCmd)
	rootCmd.AddCommand(statusCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("LocalMesh %s\n", versionStr)
		fmt.Printf("  Commit: %s\n", commitStr)
		fmt.Printf("  Built:  %s\n", dateStr)
	},
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new LocalMesh node",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🚀 Initializing LocalMesh node...")

		dirs := []string{"data", "configs"}
		for _, dir := range dirs {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create %s: %w", dir, err)
			}
		}

		// Create default config if not exists
		if _, err := os.Stat("localmesh.yaml"); os.IsNotExist(err) {
			defaultConfig := `# LocalMesh Configuration
node:
  name: "localmesh-node"
  zone: "default"

gateway:
  host: "0.0.0.0"
  port: 8080
  hostname: "campus"

grpc:
  enabled: true
  port: 9000

log:
  level: "info"
  format: "text"
`
			if err := os.WriteFile("localmesh.yaml", []byte(defaultConfig), 0644); err != nil {
				return fmt.Errorf("failed to create config: %w", err)
			}
			fmt.Println("✅ Created localmesh.yaml")
		}

		fmt.Println("✅ LocalMesh initialized!")
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Edit localmesh.yaml if needed")
		fmt.Println("  2. Run 'localmesh start' to start the server")
		return nil
	},
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the LocalMesh server",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🚀 Starting LocalMesh...")

		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		framework, err := core.New(cfg)
		if err != nil {
			return fmt.Errorf("creating framework: %w", err)
		}

		if err := framework.Start(); err != nil {
			return fmt.Errorf("starting framework: %w", err)
		}

		fmt.Println("✅ LocalMesh is running!")
		fmt.Printf("   API:   http://%s\n", cfg.GatewayAddr())
		proxyPort := cfg.Gateway.ProxyPort
		if proxyPort == 0 {
			proxyPort = 8081
		}
		fmt.Printf("   Proxy: http://%s:%d (for *.local routing)\n", cfg.Gateway.Host, proxyPort)
		fmt.Println("\nPress Ctrl+C to stop...")

		framework.Wait()
		return framework.Stop()
	},
}

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the LocalMesh server",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Stopping LocalMesh...")
		// TODO: Implement proper stop via PID file or socket
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show LocalMesh status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		fmt.Printf("LocalMesh Status\n")
		fmt.Printf("  HTTP: http://%s\n", cfg.GatewayAddr())
		if cfg.GRPC.Enabled {
			fmt.Printf("  gRPC: %s\n", cfg.GRPCAddr())
		}
		return nil
	},
}
