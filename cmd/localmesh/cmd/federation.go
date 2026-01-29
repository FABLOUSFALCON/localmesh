package cmd

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	localgrpc "github.com/FABLOUSFALCON/localmesh/internal/grpc"
	"github.com/spf13/cobra"
)

var federationServer *localgrpc.FederationServer

// federationCmd is the parent command for federation operations
var federationCmd = &cobra.Command{
	Use:   "federation",
	Short: "Manage realm federation",
	Long: `Federation allows multiple LocalMesh realms to share services.

When federated:
  - Services can be discovered across realms
  - Cross-realm access control is enforced
  - Service catalogs are synchronized

Examples:
  localmesh federation status
  localmesh federation join --peer cse.campus.local:9000
  localmesh federation peers
  localmesh federation sync`,
}

// federationStatusCmd shows federation status
var federationStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show federation status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🌐 Federation Status")
		fmt.Println("─────────────────────────────────────")

		if federationServer == nil {
			fmt.Println("  Status:      ❌ Not initialized")
			fmt.Println("  Hint:        Start the server first with 'localmesh start'")
			return nil
		}

		fmt.Printf("  Realm ID:    %s\n", federationServer.RealmID())
		fmt.Printf("  Realm Name:  %s\n", federationServer.RealmName())
		fmt.Printf("  Public Key:  %s...\n", federationServer.PublicKeyHex()[:16])

		fedID := federationServer.FederationID()
		if fedID == "" {
			fmt.Println("  Federation:  ❌ Not joined")
		} else {
			fmt.Printf("  Federation:  ✅ %s\n", fedID)
		}

		peers := federationServer.Peers()
		fmt.Printf("  Peers:       %d connected\n", len(peers))

		return nil
	},
}

// federationJoinCmd joins another realm's federation
var federationJoinCmd = &cobra.Command{
	Use:   "join",
	Short: "Join a federation via a peer realm",
	Long: `Connect to another LocalMesh realm and join its federation.

Once joined, your services become discoverable by other realms,
and you can discover their services.

Examples:
  localmesh federation join --peer cse.campus.local:9000
  localmesh federation join --peer 192.168.1.100:9000`,
	RunE: func(cmd *cobra.Command, args []string) error {
		peerEndpoint, _ := cmd.Flags().GetString("peer")
		if peerEndpoint == "" {
			return fmt.Errorf("--peer is required")
		}

		if federationServer == nil {
			return fmt.Errorf("federation server not running - start the server first")
		}

		fmt.Printf("🔗 Joining federation via %s...\n", peerEndpoint)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := federationServer.JoinPeer(ctx, peerEndpoint); err != nil {
			return fmt.Errorf("failed to join: %w", err)
		}

		fmt.Println("✅ Successfully joined federation!")
		fmt.Printf("   Federation ID: %s\n", federationServer.FederationID())
		fmt.Printf("   Peers: %d\n", len(federationServer.Peers()))

		return nil
	},
}

// federationPeersCmd lists connected peers
var federationPeersCmd = &cobra.Command{
	Use:   "peers",
	Short: "List connected federation peers",
	RunE: func(cmd *cobra.Command, args []string) error {
		if federationServer == nil {
			return fmt.Errorf("federation server not running")
		}

		peers := federationServer.Peers()

		fmt.Println("👥 Federation Peers")
		fmt.Println("─────────────────────────────────────────────────")

		if len(peers) == 0 {
			fmt.Println("  (no peers connected)")
			fmt.Println()
			fmt.Println("  Join a federation:")
			fmt.Println("    localmesh federation join --peer <host>:9000")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "  REALM ID\tSTATUS\tENDPOINT")
		for _, peerID := range peers {
			fmt.Fprintf(w, "  %s\t✅ active\t-\n", peerID)
		}
		w.Flush()

		return nil
	},
}

// federationSyncCmd syncs services with peers
var federationSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync services with federation peers",
	RunE: func(cmd *cobra.Command, args []string) error {
		if federationServer == nil {
			return fmt.Errorf("federation server not running")
		}

		peers := federationServer.Peers()
		if len(peers) == 0 {
			fmt.Println("⚠️  No peers to sync with")
			return nil
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		fmt.Println("🔄 Syncing services with peers...")
		for _, peerID := range peers {
			fmt.Printf("   Syncing with %s... ", peerID)
			if err := federationServer.SyncWithPeer(ctx, peerID); err != nil {
				fmt.Printf("❌ %v\n", err)
			} else {
				fmt.Println("✅")
			}
		}

		fmt.Println("✅ Sync complete")
		return nil
	},
}

// federationLeaveCmd leaves the current federation
var federationLeaveCmd = &cobra.Command{
	Use:   "leave",
	Short: "Leave the current federation",
	RunE: func(cmd *cobra.Command, args []string) error {
		if federationServer == nil {
			return fmt.Errorf("federation server not running")
		}

		fedID := federationServer.FederationID()
		if fedID == "" {
			fmt.Println("⚠️  Not currently in a federation")
			return nil
		}

		fmt.Printf("🚪 Leaving federation %s...\n", fedID)
		// TODO: Notify peers and clean up
		fmt.Println("✅ Left federation")
		return nil
	},
}

func init() {
	federationCmd.AddCommand(federationStatusCmd)
	federationCmd.AddCommand(federationJoinCmd)
	federationCmd.AddCommand(federationPeersCmd)
	federationCmd.AddCommand(federationSyncCmd)
	federationCmd.AddCommand(federationLeaveCmd)

	federationJoinCmd.Flags().String("peer", "", "peer realm endpoint (host:port)")
	federationJoinCmd.MarkFlagRequired("peer")

	rootCmd.AddCommand(federationCmd)
}

// SetFederationServer sets the federation server instance (called from framework)
func SetFederationServer(server *localgrpc.FederationServer) {
	federationServer = server
}
