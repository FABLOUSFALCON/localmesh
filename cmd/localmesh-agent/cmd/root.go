package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/hashicorp/mdns"
	"github.com/spf13/cobra"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func SetVersionInfo(version, commit, date string) {
	Version = version
	Commit = commit
	Date = date
}

var serverAddr string

var rootCmd = &cobra.Command{
	Use:   "localmesh-agent",
	Short: "LocalMesh agent for service registration",
	Long: `LocalMesh Agent is a lightweight client that registers local services
with a LocalMesh server via mDNS advertising.`,
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&serverAddr, "server", "s", "", "LocalMesh server address (auto-discovered if not set)")

	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(registerCmd)
	rootCmd.AddCommand(unregisterCmd)
	rootCmd.AddCommand(statusCmd)
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("localmesh-agent %s\n", Version)
		fmt.Printf("  Commit: %s\n", Commit)
		fmt.Printf("  Built:  %s\n", Date)
	},
}

var registerCmd = &cobra.Command{
	Use:   "register [service-name]",
	Short: "Register a service with LocalMesh",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]
		port, _ := cmd.Flags().GetInt("port")
		ip, _ := cmd.Flags().GetString("ip")
		description, _ := cmd.Flags().GetString("description")
		keepAlive, _ := cmd.Flags().GetBool("keep-alive")

		if port <= 0 {
			return fmt.Errorf("--port is required")
		}

		// Auto-detect IP if not provided
		if ip == "" {
			detectedIP, err := getOutboundIP()
			if err != nil {
				return fmt.Errorf("failed to detect local IP: %w (use --ip to specify)", err)
			}
			ip = detectedIP
		}

		// Get server address
		server, err := getServer()
		if err != nil {
			return err
		}

		// Build request
		reqBody := map[string]interface{}{
			"name":        serviceName,
			"port":        port,
			"ip":          ip,
			"description": description,
		}

		jsonBody, _ := json.Marshal(reqBody)

		// Register via HTTP
		url := fmt.Sprintf("http://%s/api/v1/services/register", server)
		resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
		if err != nil {
			return fmt.Errorf("failed to register: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		if resp.StatusCode != http.StatusOK {
			if errMsg, ok := result["error"].(string); ok {
				return fmt.Errorf("registration failed: %s", errMsg)
			}
			return fmt.Errorf("registration failed: status %d", resp.StatusCode)
		}

		hostname := result["hostname"].(string)
		svcURL := result["url"].(string)

		fmt.Printf("✅ Service registered successfully!\n")
		fmt.Printf("   Name:     %s\n", serviceName)
		fmt.Printf("   Hostname: %s\n", hostname)
		fmt.Printf("   URL:      %s\n", svcURL)
		fmt.Printf("   IP:       %s\n", ip)
		fmt.Printf("   Port:     %d\n", port)

		if keepAlive {
			fmt.Println("\n🔄 Keeping registration alive (Ctrl+C to stop)...")
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh
			fmt.Println("\n⏹️  Stopping...")

			// Unregister on exit
			unregisterService(server, serviceName)
		}

		return nil
	},
}

var unregisterCmd = &cobra.Command{
	Use:   "unregister [service-name]",
	Short: "Unregister a service from LocalMesh",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := args[0]

		server, err := getServer()
		if err != nil {
			return err
		}

		if err := unregisterService(server, serviceName); err != nil {
			return err
		}

		fmt.Printf("✅ Service %s unregistered\n", serviceName)
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of registered services",
	RunE: func(cmd *cobra.Command, args []string) error {
		server, err := getServer()
		if err != nil {
			return err
		}

		url := fmt.Sprintf("http://%s/api/v1/services", server)
		resp, err := http.Get(url)
		if err != nil {
			return fmt.Errorf("failed to get services: %w", err)
		}
		defer resp.Body.Close()

		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)

		services, ok := result["services"].([]interface{})
		if !ok || len(services) == 0 {
			fmt.Println("No services registered")
			return nil
		}

		fmt.Printf("Registered services (%d):\n", len(services))
		for _, svc := range services {
			s := svc.(map[string]interface{})
			fmt.Printf("  • %s\n", s["name"])
			fmt.Printf("    URL:  %s\n", s["url"])
			fmt.Printf("    IP:   %s:%v\n", s["ip"], s["port"])
		}

		return nil
	},
}

func init() {
	registerCmd.Flags().IntP("port", "p", 0, "Port the service runs on (required)")
	registerCmd.Flags().String("ip", "", "IP address (auto-detected if not set)")
	registerCmd.Flags().StringP("description", "d", "", "Service description")
	registerCmd.Flags().Bool("keep-alive", false, "Keep running and unregister on exit")
	registerCmd.MarkFlagRequired("port")
}

func unregisterService(server, name string) error {
	reqBody := map[string]string{"name": name}
	jsonBody, _ := json.Marshal(reqBody)

	url := fmt.Sprintf("http://%s/api/v1/services/unregister", server)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to unregister: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var result map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&result)
		if errMsg, ok := result["error"].(string); ok {
			return fmt.Errorf("unregister failed: %s", errMsg)
		}
		return fmt.Errorf("unregister failed: status %d", resp.StatusCode)
	}

	return nil
}

func getServer() (string, error) {
	if serverAddr != "" {
		return serverAddr, nil
	}

	// Try to discover LocalMesh server via mDNS
	server, err := discoverLocalMesh()
	if err != nil {
		return "", fmt.Errorf("could not find LocalMesh server: %w\nUse --server to specify address", err)
	}

	return server, nil
}

func discoverLocalMesh() (string, error) {
	entriesCh := make(chan *mdns.ServiceEntry, 10)
	var server string
	var mu sync.Mutex
	done := make(chan struct{})

	go func() {
		for entry := range entriesCh {
			mu.Lock()
			if server == "" && entry.Port > 0 && len(entry.AddrV4) > 0 {
				server = fmt.Sprintf("%s:%d", entry.AddrV4, entry.Port)
			}
			mu.Unlock()
		}
		close(done)
	}()

	params := &mdns.QueryParam{
		Service: "_localmesh._tcp",
		Domain:  "local",
		Timeout: 2 * time.Second,
		Entries: entriesCh,
	}

	_ = mdns.Query(params)
	close(entriesCh)
	<-done

	mu.Lock()
	defer mu.Unlock()
	if server == "" {
		return "", fmt.Errorf("no LocalMesh server found")
	}

	return server, nil
}

func getOutboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}
