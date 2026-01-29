// LocalMesh Agent - Service Registration Client
//
// The agent is a lightweight client that registers local services with a
// LocalMesh server. It handles:
//   - Service registration with mDNS hostname assignment
//   - Heartbeat/health reporting to the server
//   - Auto-reconnection on network changes
//
// Usage:
//
//	localmesh-agent register myapp --port 3000 --server campus.local:9000
//	localmesh-agent status
//	localmesh-agent unregister myapp
package main

import (
	"fmt"
	"os"

	"github.com/FABLOUSFALCON/localmesh/cmd/localmesh-agent/cmd"
)

// Build-time variables
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, commit, date)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
