// Package cmd provides global administration CLI commands for LocalMesh.
package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var adminCmd = &cobra.Command{
	Use:   "admin",
	Short: "Global administration commands",
	Long: `Global administration commands for managing multiple LocalMesh realms.

The admin commands allow a super-admin realm to:
  - View all connected realms and their status
  - Monitor services across all realms
  - Manage alerts and policies
  - View aggregated statistics

Examples:
  localmesh admin dashboard          # View admin dashboard
  localmesh admin realms             # List all managed realms
  localmesh admin services           # List services across all realms
  localmesh admin alerts             # View active alerts
  localmesh admin policies           # List configured policies`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var adminDashboardCmd = &cobra.Command{
	Use:   "dashboard",
	Short: "Show admin dashboard overview",
	Run: func(cmd *cobra.Command, args []string) {
		printAdminDashboard()
	},
}

var adminRealmsCmd = &cobra.Command{
	Use:   "realms",
	Short: "List all managed realms",
	Run: func(cmd *cobra.Command, args []string) {
		printManagedRealms()
	},
}

var adminRealmCmd = &cobra.Command{
	Use:   "realm [id]",
	Short: "Show details for a specific realm",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		printRealmDetails(args[0])
	},
}

var adminServicesRealmFilter string

var adminServicesCmd = &cobra.Command{
	Use:   "services",
	Short: "List services across all realms",
	Run: func(cmd *cobra.Command, args []string) {
		printAdminServices(adminServicesRealmFilter)
	},
}

var adminAlertsActiveOnly bool
var adminAlertsRealmFilter string

var adminAlertsCmd = &cobra.Command{
	Use:   "alerts",
	Short: "View alerts across all realms",
	Run: func(cmd *cobra.Command, args []string) {
		printAdminAlerts(adminAlertsRealmFilter, adminAlertsActiveOnly)
	},
}

var adminPoliciesCmd = &cobra.Command{
	Use:   "policies",
	Short: "List configured policies",
	Run: func(cmd *cobra.Command, args []string) {
		printAdminPolicies()
	},
}

var adminStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show aggregated statistics",
	Run: func(cmd *cobra.Command, args []string) {
		printAdminStats()
	},
}

func init() {
	rootCmd.AddCommand(adminCmd)

	// Subcommands
	adminCmd.AddCommand(adminDashboardCmd)
	adminCmd.AddCommand(adminRealmsCmd)
	adminCmd.AddCommand(adminRealmCmd)
	adminCmd.AddCommand(adminServicesCmd)
	adminCmd.AddCommand(adminAlertsCmd)
	adminCmd.AddCommand(adminPoliciesCmd)
	adminCmd.AddCommand(adminStatsCmd)

	// Flags
	adminServicesCmd.Flags().StringVar(&adminServicesRealmFilter, "realm", "", "Filter by realm ID")
	adminAlertsCmd.Flags().BoolVar(&adminAlertsActiveOnly, "active", false, "Show only active (unacknowledged) alerts")
	adminAlertsCmd.Flags().StringVar(&adminAlertsRealmFilter, "realm", "", "Filter by realm ID")
}

func printAdminDashboard() {
	fmt.Println("🌐 Global Admin Dashboard")
	fmt.Println("═══════════════════════════════════════════════════════════════")
	fmt.Println()

	// Stats summary
	fmt.Println("📊 Overview")
	fmt.Println("───────────────────────────────────────")
	fmt.Printf("  Total Realms:     %d\n", 0)
	fmt.Printf("  Online Realms:    %d\n", 0)
	fmt.Printf("  Total Services:   %d\n", 0)
	fmt.Printf("  Healthy Services: %d\n", 0)
	fmt.Printf("  Active Alerts:    %d\n", 0)
	fmt.Println()

	// No realms connected yet
	fmt.Println("📡 Connected Realms")
	fmt.Println("───────────────────────────────────────")
	fmt.Println("  No realms connected.")
	fmt.Println()
	fmt.Println("  To connect a realm, use federation:")
	fmt.Println("    localmesh federation join --peer <realm:port>")
	fmt.Println()

	// Recent alerts
	fmt.Println("🚨 Recent Alerts")
	fmt.Println("───────────────────────────────────────")
	fmt.Println("  No alerts.")
	fmt.Println()

	fmt.Printf("Last updated: %s\n", time.Now().Format(time.RFC3339))
}

func printManagedRealms() {
	fmt.Println("📡 Managed Realms")
	fmt.Println("─────────────────────────────────────────────────────────────")
	fmt.Println()

	// Show example output format
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "REALM\tSTATUS\tSERVICES\tLAST SEEN\tENDPOINT\n")
	fmt.Fprintf(w, "─────\t──────\t────────\t─────────\t────────\n")
	w.Flush()

	fmt.Println()
	fmt.Println("  No realms registered.")
	fmt.Println()
	fmt.Println("  Register realms via federation or the API:")
	fmt.Println("    POST /api/admin/realms")
	fmt.Println()
}

func printRealmDetails(realmID string) {
	fmt.Printf("📡 Realm: %s\n", realmID)
	fmt.Println("─────────────────────────────────────────────────────────────")
	fmt.Println()
	fmt.Printf("  Status: Not found\n")
	fmt.Println()
	fmt.Println("  This realm is not registered with the global admin.")
	fmt.Println()
}

func printAdminServices(realmFilter string) {
	fmt.Println("🔧 Services Across Realms")
	fmt.Println("─────────────────────────────────────────────────────────────")

	if realmFilter != "" {
		fmt.Printf("  Filtered by realm: %s\n", realmFilter)
	}
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "SERVICE\tREALM\tHOSTNAME\tHEALTHY\tPUBLIC\n")
	fmt.Fprintf(w, "───────\t─────\t────────\t───────\t──────\n")
	w.Flush()

	fmt.Println()
	fmt.Println("  No services registered.")
	fmt.Println()
}

func printAdminAlerts(realmFilter string, activeOnly bool) {
	fmt.Println("🚨 Alerts")
	fmt.Println("─────────────────────────────────────────────────────────────")

	if realmFilter != "" {
		fmt.Printf("  Filtered by realm: %s\n", realmFilter)
	}
	if activeOnly {
		fmt.Println("  Showing only active (unacknowledged) alerts")
	}
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "TIME\tLEVEL\tREALM\tMESSAGE\tSTATUS\n")
	fmt.Fprintf(w, "────\t─────\t─────\t───────\t──────\n")
	w.Flush()

	fmt.Println()
	fmt.Println("  No alerts.")
	fmt.Println()

	fmt.Println("  Alert levels:")
	fmt.Println("    • info     - Informational message")
	fmt.Println("    • warning  - Potential issue, monitor closely")
	fmt.Println("    • error    - Error condition, action may be needed")
	fmt.Println("    • critical - Critical issue, immediate action required")
	fmt.Println()
}

func printAdminPolicies() {
	fmt.Println("📜 Policies")
	fmt.Println("─────────────────────────────────────────────────────────────")
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ID\tNAME\tTYPE\tREALMS\tENABLED\tVERSION\n")
	fmt.Fprintf(w, "──\t────\t────\t──────\t───────\t───────\n")
	w.Flush()

	fmt.Println()
	fmt.Println("  No policies configured.")
	fmt.Println()

	fmt.Println("  Policy types:")
	fmt.Println("    • rbac    - Role-based access control policies")
	fmt.Println("    • network - Network configuration policies")
	fmt.Println("    • service - Service registration policies")
	fmt.Println()

	fmt.Println("  Create policies via the API:")
	fmt.Println("    POST /api/admin/policies")
	fmt.Println()
}

func printAdminStats() {
	fmt.Println("📊 Aggregated Statistics")
	fmt.Println("─────────────────────────────────────────────────────────────")
	fmt.Println()

	fmt.Println("  System Overview")
	fmt.Println("  ───────────────")
	fmt.Printf("    Total Realms:      %d\n", 0)
	fmt.Printf("    Online Realms:     %d\n", 0)
	fmt.Printf("    Total Services:    %d\n", 0)
	fmt.Printf("    Healthy Services:  %d\n", 0)
	fmt.Printf("    Total Alerts:      %d\n", 0)
	fmt.Printf("    Active Alerts:     %d\n", 0)
	fmt.Println()

	fmt.Println("  Per-Realm Statistics")
	fmt.Println("  ────────────────────")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "  REALM\tSTATUS\tSERVICES\tHEALTHY\tALERTS\tLAST SEEN\n")
	fmt.Fprintf(w, "  ─────\t──────\t────────\t───────\t──────\t─────────\n")
	w.Flush()

	fmt.Println()
	fmt.Println("    No realms to display.")
	fmt.Println()

	fmt.Printf("  Updated: %s\n", time.Now().Format(time.RFC3339))
}
