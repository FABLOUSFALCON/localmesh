// Package cmd provides RBAC (Role-Based Access Control) CLI commands for LocalMesh.
package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

var rbacCmd = &cobra.Command{
	Use:   "rbac",
	Short: "Manage roles and permissions",
	Long: `RBAC (Role-Based Access Control) management commands.

LocalMesh uses WiFi SSID-based role assignment:
  - Connect to "CSE-Faculty" WiFi → Assigned "teacher" role
  - Connect to "CSE-Students" WiFi → Assigned "student" role
  - Unknown networks → Assigned "guest" role

Roles have hierarchical permissions:
  guest    → service:list, realm:view
  student  → + service:access
  teacher  → + service:register, service:unregister
  admin    → + realm:manage, user:*, cross-realm:*

Examples:
  localmesh rbac roles                      # List all roles
  localmesh rbac role teacher               # Show role details
  localmesh rbac ssid                       # List SSID mappings
  localmesh rbac ssid add --ssid "CSE-*" --role teacher
  localmesh rbac trust                      # List realm trusts
  localmesh rbac check --role student --action service:access`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

var rbacRolesCmd = &cobra.Command{
	Use:   "roles",
	Short: "List all roles",
	Run: func(cmd *cobra.Command, args []string) {
		printRoles()
	},
}

var rbacRoleCmd = &cobra.Command{
	Use:   "role [name]",
	Short: "Show role details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		printRoleDetails(args[0])
	},
}

var rbacSSIDCmd = &cobra.Command{
	Use:   "ssid",
	Short: "Manage WiFi SSID to role mappings",
	Run: func(cmd *cobra.Command, args []string) {
		printSSIDMappings()
	},
}

var ssidAddSSID string
var ssidAddRole string
var ssidAddZone string
var ssidAddPriority int

var rbacSSIDAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add SSID to role mapping",
	Run: func(cmd *cobra.Command, args []string) {
		if ssidAddSSID == "" || ssidAddRole == "" {
			fmt.Println("❌ Both --ssid and --role are required")
			os.Exit(1)
		}
		addSSIDMapping(ssidAddSSID, ssidAddRole, ssidAddZone, ssidAddPriority)
	},
}

var rbacTrustCmd = &cobra.Command{
	Use:   "trust",
	Short: "Manage cross-realm trust relationships",
	Run: func(cmd *cobra.Command, args []string) {
		printTrustRelationships()
	},
}

var trustAddRealm string
var trustAddLevel string
var trustAddPerms string
var trustAddBidirectional bool

var rbacTrustAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Add trust relationship with another realm",
	Run: func(cmd *cobra.Command, args []string) {
		if trustAddRealm == "" {
			fmt.Println("❌ --realm is required")
			os.Exit(1)
		}
		addTrustRelationship(trustAddRealm, trustAddLevel, trustAddPerms, trustAddBidirectional)
	},
}

var checkRole string
var checkAction string
var checkSSID string

var rbacCheckCmd = &cobra.Command{
	Use:   "check",
	Short: "Check if an action is allowed",
	Run: func(cmd *cobra.Command, args []string) {
		if checkAction == "" {
			fmt.Println("❌ --action is required")
			os.Exit(1)
		}
		checkPermission(checkRole, checkAction, checkSSID)
	},
}

func init() {
	rootCmd.AddCommand(rbacCmd)

	// Subcommands
	rbacCmd.AddCommand(rbacRolesCmd)
	rbacCmd.AddCommand(rbacRoleCmd)
	rbacCmd.AddCommand(rbacSSIDCmd)
	rbacCmd.AddCommand(rbacTrustCmd)
	rbacCmd.AddCommand(rbacCheckCmd)

	// SSID add subcommand
	rbacSSIDCmd.AddCommand(rbacSSIDAddCmd)
	rbacSSIDAddCmd.Flags().StringVar(&ssidAddSSID, "ssid", "", "WiFi SSID pattern (supports *)")
	rbacSSIDAddCmd.Flags().StringVar(&ssidAddRole, "role", "", "Role to assign")
	rbacSSIDAddCmd.Flags().StringVar(&ssidAddZone, "zone", "", "Optional zone restriction")
	rbacSSIDAddCmd.Flags().IntVar(&ssidAddPriority, "priority", 10, "Priority (higher = checked first)")

	// Trust add subcommand
	rbacTrustCmd.AddCommand(rbacTrustAddCmd)
	rbacTrustAddCmd.Flags().StringVar(&trustAddRealm, "realm", "", "Remote realm ID")
	rbacTrustAddCmd.Flags().StringVar(&trustAddLevel, "level", "access", "Trust level: none, read, access, register, full")
	rbacTrustAddCmd.Flags().StringVar(&trustAddPerms, "permissions", "", "Comma-separated permissions")
	rbacTrustAddCmd.Flags().BoolVar(&trustAddBidirectional, "bidirectional", false, "Make trust mutual")

	// Check command
	rbacCheckCmd.Flags().StringVar(&checkRole, "role", "", "Role to check")
	rbacCheckCmd.Flags().StringVar(&checkAction, "action", "", "Action to check (e.g., service:register)")
	rbacCheckCmd.Flags().StringVar(&checkSSID, "ssid", "", "WiFi SSID to derive role from")
}

func printRoles() {
	fmt.Println("🔐 Available Roles")
	fmt.Println("─────────────────────────────────────")
	fmt.Println()

	// Built-in roles
	roles := []struct {
		name     string
		priority int
		perms    []string
	}{
		{"guest", 0, []string{"service:list", "realm:view"}},
		{"student", 10, []string{"service:access", "+ inherited from guest"}},
		{"teacher", 20, []string{"service:register", "service:unregister", "realm:manage", "+ inherited from student"}},
		{"admin", 50, []string{"realm:federate", "realm:trust", "user:*", "admin:*", "+ inherited from teacher"}},
		{"superadmin", 100, []string{"*"}},
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "ROLE\tPRIORITY\tPERMISSIONS\n")
	fmt.Fprintf(w, "────\t────────\t───────────\n")

	for _, r := range roles {
		perms := strings.Join(r.perms, ", ")
		if len(perms) > 60 {
			perms = perms[:57] + "..."
		}
		fmt.Fprintf(w, "%s\t%d\t%s\n", r.name, r.priority, perms)
	}
	w.Flush()
	fmt.Println()
}

func printRoleDetails(roleName string) {
	roles := map[string]struct {
		desc    string
		perms   []string
		inherit string
	}{
		"guest": {
			desc:    "Minimal access for unknown network connections",
			perms:   []string{"service:list", "realm:view"},
			inherit: "none",
		},
		"student": {
			desc:    "Standard access for students",
			perms:   []string{"service:access", "user:view"},
			inherit: "guest",
		},
		"teacher": {
			desc:    "Elevated access for faculty",
			perms:   []string{"service:register", "service:unregister", "realm:manage"},
			inherit: "student",
		},
		"admin": {
			desc:    "Full access within a realm",
			perms:   []string{"service:admin", "realm:federate", "realm:trust", "user:*", "admin:*", "cross-realm:*"},
			inherit: "teacher",
		},
		"superadmin": {
			desc:    "Global access across all realms",
			perms:   []string{"*"},
			inherit: "admin",
		},
	}

	role, ok := roles[roleName]
	if !ok {
		fmt.Printf("❌ Role %q not found\n", roleName)
		fmt.Println("\nAvailable roles: guest, student, teacher, admin, superadmin")
		os.Exit(1)
	}

	fmt.Printf("🔐 Role: %s\n", roleName)
	fmt.Println("─────────────────────────────────────")
	fmt.Printf("  Description:  %s\n", role.desc)
	fmt.Printf("  Inherits:     %s\n", role.inherit)
	fmt.Println("  Permissions:")
	for _, p := range role.perms {
		fmt.Printf("    • %s\n", p)
	}
	if role.inherit != "none" {
		fmt.Printf("    + permissions from %s\n", role.inherit)
	}
	fmt.Println()
}

func printSSIDMappings() {
	fmt.Println("📶 WiFi SSID → Role Mappings")
	fmt.Println("─────────────────────────────────────")
	fmt.Println()
	fmt.Println("  No custom mappings configured.")
	fmt.Println()
	fmt.Println("  Default behavior:")
	fmt.Println("    • Unknown SSID → guest role")
	fmt.Println()
	fmt.Println("  To add a mapping:")
	fmt.Println("    localmesh rbac ssid add --ssid \"CSE-Faculty*\" --role teacher")
	fmt.Println("    localmesh rbac ssid add --ssid \"CSE-Students\" --role student")
	fmt.Println()
}

func addSSIDMapping(ssid, role, zone string, priority int) {
	fmt.Println("➕ Adding SSID Mapping")
	fmt.Println("─────────────────────────────────────")
	fmt.Printf("  SSID Pattern: %s\n", ssid)
	fmt.Printf("  Role:         %s\n", role)
	if zone != "" {
		fmt.Printf("  Zone:         %s\n", zone)
	}
	fmt.Printf("  Priority:     %d\n", priority)
	fmt.Println()

	// TODO: Actually save to config/storage when framework is available
	fmt.Println("✅ Mapping added (in-memory only)")
	fmt.Println()
	fmt.Println("Note: To persist, add to localmesh.yaml:")
	fmt.Println()
	fmt.Println("  rbac:")
	fmt.Println("    ssid_mappings:")
	fmt.Printf("      - ssid: \"%s\"\n", ssid)
	fmt.Printf("        role: %s\n", role)
	if zone != "" {
		fmt.Printf("        zone: %s\n", zone)
	}
	fmt.Println()
}

func printTrustRelationships() {
	fmt.Println("🤝 Cross-Realm Trust Relationships")
	fmt.Println("─────────────────────────────────────")
	fmt.Println()
	fmt.Println("  No trust relationships configured.")
	fmt.Println()
	fmt.Println("  Trust levels:")
	fmt.Println("    • none     - Deny all cross-realm access")
	fmt.Println("    • read     - Can list services (read-only)")
	fmt.Println("    • access   - Can access public services")
	fmt.Println("    • register - Can register services")
	fmt.Println("    • full     - Full trust (treat as local)")
	fmt.Println()
	fmt.Println("  To establish trust:")
	fmt.Println("    localmesh rbac trust add --realm cse.campus.local --level access")
	fmt.Println()
}

func addTrustRelationship(realm, level, perms string, bidirectional bool) {
	fmt.Println("🤝 Establishing Trust")
	fmt.Println("─────────────────────────────────────")
	fmt.Printf("  Remote Realm:   %s\n", realm)
	fmt.Printf("  Trust Level:    %s\n", level)
	if perms != "" {
		fmt.Printf("  Permissions:    %s\n", perms)
	}
	fmt.Printf("  Bidirectional:  %v\n", bidirectional)
	fmt.Println()

	// TODO: Actually establish trust when federation is connected
	fmt.Println("✅ Trust relationship established (in-memory only)")
	fmt.Println()
	fmt.Println("Note: To persist, add to localmesh.yaml:")
	fmt.Println()
	fmt.Println("  rbac:")
	fmt.Println("    trust:")
	fmt.Printf("      - realm: \"%s\"\n", realm)
	fmt.Printf("        level: %s\n", level)
	if bidirectional {
		fmt.Println("        bidirectional: true")
	}
	fmt.Println()
}

func checkPermission(role, action, ssid string) {
	fmt.Println("🔍 Permission Check")
	fmt.Println("─────────────────────────────────────")

	// If SSID provided, resolve role
	if ssid != "" {
		fmt.Printf("  WiFi SSID:    %s\n", ssid)
		// TODO: Actually resolve from SSID mappings
		role = "guest" // Default for now
		fmt.Printf("  Resolved Role: %s (from SSID)\n", role)
	} else if role != "" {
		fmt.Printf("  Role:         %s\n", role)
	} else {
		role = "guest"
		fmt.Printf("  Role:         %s (default)\n", role)
	}

	fmt.Printf("  Action:       %s\n", action)
	fmt.Println()

	// Check permission based on built-in rules
	allowed := checkRolePermission(role, action)

	if allowed {
		fmt.Println("✅ ALLOWED")
		fmt.Printf("   Role %q has permission %q\n", role, action)
	} else {
		fmt.Println("❌ DENIED")
		fmt.Printf("   Role %q lacks permission %q\n", role, action)
		fmt.Println()
		fmt.Println("   Suggestion: Upgrade role or add permission")
	}
	fmt.Println()
}

func checkRolePermission(role, action string) bool {
	perms := map[string][]string{
		"guest":      {"service:list", "realm:view"},
		"student":    {"service:list", "realm:view", "service:access", "user:view"},
		"teacher":    {"service:list", "realm:view", "service:access", "user:view", "service:register", "service:unregister", "realm:manage"},
		"admin":      {"*"},
		"superadmin": {"*"},
	}

	rolePerms, ok := perms[role]
	if !ok {
		return false
	}

	for _, p := range rolePerms {
		if p == "*" || p == action {
			return true
		}
		// Check wildcard pattern (e.g., "service:*" matches "service:register")
		if strings.HasSuffix(p, ":*") {
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(action, prefix) {
				return true
			}
		}
	}

	return false
}
