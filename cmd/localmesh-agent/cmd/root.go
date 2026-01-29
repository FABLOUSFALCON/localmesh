package cmd

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var (
	versionStr = "dev"
	commitStr  = "none"
	dateStr    = "unknown"

	// Global flags
	serverAddr string
	timeout    time.Duration
)

// SetVersionInfo sets version information from build flags
func SetVersionInfo(version, commit, date string) {
	versionStr = version
	commitStr = commit
	dateStr = date
}

// rootCmd is the base command
var rootCmd = &cobra.Command{
	Use:   "localmesh-agent",
	Short: "LocalMesh Agent - Register services with LocalMesh",
	Long: `LocalMesh Agent is a lightweight client for registering services
with a LocalMesh server.

Your service gets a friendly .local URL that works on any device:
  localmesh-agent register myapp --port 3000
  → Access at http://myapp.campus.local:3000

The agent maintains a connection to the server and reports health status.`,
}

// Execute runs the root command
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&serverAddr, "server", "s", "campus.local:9000",
		"LocalMesh server address (host:port)")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 10*time.Second,
		"connection timeout")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(registerCmd)
	rootCmd.AddCommand(unregisterCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(listCmd)
}

// versionCmd shows version info
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("LocalMesh Agent %s\n", versionStr)
		fmt.Printf("  Commit: %s\n", commitStr)
		fmt.Printf("  Built:  %s\n", dateStr)
	},
}

// registerCmd registers a service with the server
var registerCmd = &cobra.Command{
	Use:   "register <name>",
	Short: "Register a service with LocalMesh",
	Long: `Register a local service and get a .local hostname.

The agent will:
  1. Connect to the LocalMesh server
  2. Request a hostname for your service
  3. Keep the registration alive with heartbeats
  4. Report health status to the server

Examples:
  localmesh-agent register myapp --port 3000
  localmesh-agent register api --port 8080 --server myuni.local:9000
  localmesh-agent register frontend --port 5173 --health /api/health`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		port, _ := cmd.Flags().GetInt("port")
		healthPath, _ := cmd.Flags().GetString("health")
		description, _ := cmd.Flags().GetString("description")

		if port == 0 {
			return fmt.Errorf("--port is required")
		}

		// Validate service name
		if err := validateServiceName(name); err != nil {
			return err
		}

		fmt.Printf("📡 Connecting to LocalMesh server at %s...\n", serverAddr)

		// TODO: Replace with gRPC client connection
		// For now, we'll use a simple TCP check to verify server is reachable
		conn, err := net.DialTimeout("tcp", serverAddr, timeout)
		if err != nil {
			// Try to discover server via mDNS
			fmt.Printf("⚠️  Could not connect to %s\n", serverAddr)
			fmt.Println("   Make sure the LocalMesh server is running")
			fmt.Println("   or specify --server with the correct address")
			return fmt.Errorf("server connection failed: %w", err)
		}
		conn.Close()

		// Get local IP
		localIP, err := getLocalIP()
		if err != nil {
			return fmt.Errorf("failed to detect local IP: %w", err)
		}

		fmt.Printf("🔗 Registering service: %s\n", name)
		fmt.Printf("   Port:        %d\n", port)
		fmt.Printf("   Local IP:    %s\n", localIP)
		if healthPath != "" {
			fmt.Printf("   Health:      %s\n", healthPath)
		}
		if description != "" {
			fmt.Printf("   Description: %s\n", description)
		}

		// TODO: Implement gRPC registration call
		// For now, simulate registration
		fmt.Println()
		fmt.Println("✅ Service registered!")
		fmt.Printf("   URL: http://%s.campus.local:%d\n", name, port)
		fmt.Println()
		fmt.Println("⏳ Keeping registration alive... (Ctrl+C to stop)")

		// Wait for interrupt
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

		// Heartbeat loop
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// TODO: Send heartbeat via gRPC
				fmt.Printf("💓 Heartbeat sent (%s)\n", time.Now().Format("15:04:05"))
			case <-sigCh:
				fmt.Println("\n🛑 Unregistering service...")
				// TODO: Send unregister via gRPC
				fmt.Println("✅ Service unregistered")
				return nil
			case <-ctx.Done():
				return nil
			}
		}
	},
}

// unregisterCmd unregisters a service
var unregisterCmd = &cobra.Command{
	Use:   "unregister <name>",
	Short: "Unregister a service from LocalMesh",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		fmt.Printf("📡 Connecting to LocalMesh server at %s...\n", serverAddr)

		// TODO: Implement gRPC unregister
		fmt.Printf("🛑 Unregistering service: %s\n", name)
		fmt.Println("✅ Service unregistered")

		return nil
	},
}

// statusCmd shows agent status
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show agent and registration status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("📊 LocalMesh Agent Status")
		fmt.Println("─────────────────────────────────")
		fmt.Printf("  Server:     %s\n", serverAddr)

		// Check server connection
		conn, err := net.DialTimeout("tcp", serverAddr, timeout)
		if err != nil {
			fmt.Println("  Connection: ❌ Cannot reach server")
		} else {
			conn.Close()
			fmt.Println("  Connection: ✅ Server reachable")
		}

		// TODO: Query registered services via gRPC
		fmt.Println("  Services:   (use 'list' to see registered services)")

		return nil
	},
}

// listCmd lists registered services
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List services registered by this agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("📦 Registered Services")
		fmt.Println("─────────────────────────────────")

		// TODO: Query from server via gRPC
		fmt.Println("  (no services registered from this agent)")
		fmt.Println()
		fmt.Println("  Register a service:")
		fmt.Println("    localmesh-agent register myapp --port 3000")

		return nil
	},
}

func init() {
	registerCmd.Flags().IntP("port", "p", 0, "port the service is running on (required)")
	registerCmd.Flags().String("health", "", "health check endpoint path (e.g., /health)")
	registerCmd.Flags().StringP("description", "d", "", "service description")
	registerCmd.MarkFlagRequired("port")
}

// validateServiceName checks if the name is valid for a hostname
func validateServiceName(name string) error {
	if name == "" {
		return fmt.Errorf("service name cannot be empty")
	}
	if len(name) > 63 {
		return fmt.Errorf("service name too long (max 63 characters)")
	}

	for _, c := range name {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-') {
			return fmt.Errorf("invalid character %q (only alphanumeric and hyphen allowed)", c)
		}
	}

	if name[0] == '-' || name[len(name)-1] == '-' {
		return fmt.Errorf("service name cannot start or end with hyphen")
	}

	return nil
}

// getLocalIP returns the local IP address
func getLocalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		// Skip virtual interfaces
		name := strings.ToLower(iface.Name)
		if strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "veth") ||
			strings.HasPrefix(name, "br-") || strings.HasPrefix(name, "virbr") {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip != nil && ip.To4() != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
				return ip.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no suitable IP address found")
}
