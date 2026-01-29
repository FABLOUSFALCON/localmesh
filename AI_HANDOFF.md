# 🤖 AI Assistant Handoff Document

> **READ THIS FIRST** - This document is designed for AI assistants (GitHub Copilot, Claude, etc.) to understand the LocalMesh project context when the user switches accounts or starts a new conversation.

**Last Updated:** January 30, 2026  
**Current Phase:** Phase 2.2 - Federation Protocol (COMPLETE)  
**Project Maturity:** ~85% core complete, federation ready

---

## 🚨 CRITICAL: READ THESE FILES FIRST

```bash
# Priority order for understanding the project:
1. /AI_HANDOFF.md          # You're reading it (context + mistakes to avoid)
2. /PLAN.md                 # Complete roadmap with ASCII diagrams
3. /docs/LEARNING.md        # Codebase walkthrough (8-10 hour guide)
4. /aiSkills/*.md          # Coding rules and patterns to follow
5. /localmesh.yaml         # Current configuration
```

---

## 📋 PROJECT SUMMARY

**LocalMesh** is a campus mesh network framework for WiFi-based service discovery. Think "local Kubernetes" without internet dependency.

### Core Features (Implemented ✅)
- **CLI** - Cobra-based with `start`, `network scan`, `plugin list` commands
- **TUI** - Bubble Tea dashboard with real-time stats
- **Gateway** - HTTP gateway on port 8080 with security headers
- **mDNS** - Hostname advertising via `avahi-publish-address` (campus.local)
- **DNS Server** - For Android/enterprise setups (binds to WiFi IP)
- **Service Registry** - SQLite + Badger for service registration
- **Auth Engine** - PASETO tokens + Argon2 password hashing
- **Plugin SDK** - Go plugin system for extensibility
- **Network Identity** - Detects WiFi SSID for zone-based auth

### Upcoming Features (Planned 🔜)
- **Phase 1:** Network interface selection, configurable hostname, service registration CLI/TUI
- **Phase 2:** `localmesh-agent` binary, gRPC for inter-process communication
- **Phase 3:** Federation between LocalMesh instances, cross-realm access

---

## 🏗️ ARCHITECTURE AT A GLANCE

```
┌─────────────────────────────────────────────────────────────────┐
│                        LocalMesh Node                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  cmd/localmesh/         ─── CLI Entry (Cobra)                  │
│       │                                                         │
│       ▼                                                         │
│  internal/core/         ─── Framework Orchestration            │
│       │                                                         │
│       ├──► internal/gateway/    ─── HTTP Gateway + mDNS + DNS  │
│       ├──► internal/grpc/       ─── gRPC (Agent + Federation)  │
│       │         ├─ AgentService      (service registration)    │
│       │         └─ FederationService (realm-to-realm sync)     │
│       ├──► internal/registry/   ─── Service Registry + mDNS    │
│       ├──► internal/auth/       ─── PASETO Auth Engine         │
│       ├──► internal/mesh/       ─── Node Discovery (hashicorp) │
│       ├──► internal/network/    ─── WiFi/Network Detection     │
│       ├──► internal/storage/    ─── SQLite + Badger            │
│       ├──► internal/tui/        ─── Bubble Tea Dashboard       │
│       └──► internal/plugins/    ─── Go Plugin Loader           │
│                                                                 │
│  api/proto/             ─── gRPC Proto Definitions             │
│       ├─ agent/v1/          (AgentService)                     │
│       └─ federation/v1/     (FederationService)                │
│  api/gen/               ─── Generated gRPC Code                │
│  pkg/sdk/               ─── Public SDK for plugin developers   │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                     LocalMesh Agent                             │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  cmd/localmesh-agent/   ─── Agent CLI Entry                    │
│       │                                                         │
│       ▼                                                         │
│  internal/client/       ─── gRPC Client for AgentService       │
│                                                                 │
│  Commands:                                                      │
│    register <name> --port <port>  → Service registration       │
│    unregister <name>               → Remove registration       │
│    status                          → Server connection status  │
│    list                            → List registered services  │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                     Federation Architecture                     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│    ┌─────────────┐         ┌─────────────┐                     │
│    │  Realm A    │◄───────►│  Realm B    │                     │
│    │  (campus)   │  gRPC   │  (cse)      │                     │
│    │  :9000      │ Fed Svc │  :9000      │                     │
│    └─────────────┘         └─────────────┘                     │
│          │                       │                              │
│    Ed25519 Keys            Ed25519 Keys                        │
│    Service Catalog         Service Catalog                      │
│                                                                 │
│  Federation RPCs:                                               │
│    JoinFederation   ─── Establish peer connection              │
│    LeaveFederation  ─── Disconnect from peer                   │
│    SyncServices     ─── Exchange service catalogs              │
│    ResolveService   ─── Find service across realms             │
│    ExchangeTrust    ─── Share public keys for auth             │
│    Ping             ─── Health check                           │
│                                                                 │
│  CLI Commands:                                                  │
│    federation status       ─── Show realm and federation info  │
│    federation join --peer  ─── Join another realm              │
│    federation peers        ─── List connected peers            │
│    federation sync         ─── Sync services with peers        │
│    federation leave        ─── Leave current federation        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 🔧 KEY IMPLEMENTATION DETAILS

### mDNS Hostname (campus.local)

**File:** `internal/gateway/hostname.go`

We use `avahi-publish-address` subprocess because:
1. `grandcat/zeroconf` only registers _services_, not A records for hostnames
2. Port 5353 is already used by system Avahi daemon
3. Avahi subprocess integrates with the system's mDNS

```go
// How we advertise hostname:
cmd := exec.CommandContext(ctx, "avahi-publish-address", "-R", hostname+".local", ip)
```

**Important:** The hostname defaults to "campus" (not "mesh") to avoid collision with the `_mesh._tcp` service type.

### DNS Server (for Android)

**File:** `internal/gateway/dns.go`

Android Chrome supports mDNS `.local` domains. BUT if the router's DHCP points clients to the LocalMesh server for DNS, we need a real DNS server.

```go
// Binds to WiFi IP specifically to avoid conflict with systemd-resolved on port 53
dns.ListenAndServe(wifiIP+":53", "udp", handler)
```

### Auth Engine (PASETO, not JWT)

**File:** `internal/auth/engine.go`

We use PASETO v4 instead of JWT because:
- No algorithm confusion attacks
- Built-in expiration handling
- Simpler, more secure by default

### TUI Framework (Bubble Tea)

**File:** `internal/tui/*.go`

Uses Elm Architecture: `Model → Update → View`

Available Bubbles components to use:
- `textinput` - Form fields
- `list` - Selection lists
- `table` - Data tables
- `spinner` - Loading indicators
- `progress` - Progress bars
- `help` - Keyboard shortcuts
- `viewport` - Scrollable content

---

## ⚠️ MISTAKES TO AVOID

### 1. Don't Use Zeroconf for Hostname Registration
```go
// ❌ WRONG - zeroconf registers services, not hostnames
zeroconf.Register("campus", "_http._tcp", "local.", 8080, nil, nil)

// ✅ CORRECT - use avahi-publish-address
exec.Command("avahi-publish-address", "-R", "campus.local", "192.168.1.50")
```

### 2. Don't Bind DNS Server to 0.0.0.0:53
```go
// ❌ WRONG - conflicts with systemd-resolved
dns.ListenAndServe(":53", "udp", handler)

// ✅ CORRECT - bind to specific WiFi IP
dns.ListenAndServe(wifiIP+":53", "udp", handler)
```

### 3. Don't Use Port 5353 Directly
```go
// ❌ WRONG - Avahi daemon already uses this
net.ListenUDP("udp", &net.UDPAddr{Port: 5353})

// ✅ CORRECT - use avahi-publish-address or hashicorp/mdns
```

### 4. Don't Forget UFW Rules
```bash
# Required firewall rules for LocalMesh:
sudo ufw allow 8080/tcp  # Gateway
sudo ufw allow 5353/udp  # mDNS
sudo ufw allow 53/udp    # DNS (if using DNS server)
```

### 5. Don't Test mDNS on the Hotspot Device
The device providing the hotspot cannot resolve `.local` via mDNS for services on itself. Test from a **client device** connected to the hotspot.

### 6. Always Wrap Errors with Context
```go
// ❌ WRONG
return err

// ✅ CORRECT
return fmt.Errorf("failed to start gateway: %w", err)
```

### 7. Always Use Parameterized SQL Queries
```go
// ❌ WRONG - SQL injection vulnerability
query := fmt.Sprintf("SELECT * FROM users WHERE id = '%s'", id)

// ✅ CORRECT
db.QueryRow("SELECT * FROM users WHERE id = ?", id)
```

---

## 📁 AI SKILLS TO APPLY

The `aiSkills/` folder contains coding rules. **ALWAYS read these before writing code:**

| File | Purpose |
|------|---------|
| `go-localmesh.md` | Project-specific Go patterns, error handling, testing |
| `security-first.md` | Security rules (CRITICAL for auth, SQL, tokens) |
| `go-backend-scalability.md` | Performance and scalability patterns |
| `go-temporal-dsl.md` | (If using workflows) Temporal patterns |

### Key Rules Summary:
1. **Error Handling:** Always wrap with `fmt.Errorf("context: %w", err)`
2. **Security:** Parameterized SQL, PASETO tokens, `crypto/rand`
3. **Logging:** Use `slog` with structured fields
4. **Testing:** Table-driven tests, 80%+ coverage
5. **Concurrency:** Use `context.Context` for cancellation

---

## 🗺️ CURRENT ROADMAP

### Phase 1: Dynamic mDNS Hostname Assignment ✅ COMPLETE
- [x] Network interface selection CLI (`localmesh network interfaces`)
- [x] `localmesh register <name> --port <port>` command
- [x] `localmesh unregister <name>` command
- [x] `localmesh services` list command
- [x] MDNSRegistry with avahi-publish-address integration
- [x] `--hostname` and `--interfaces` flags on `start` command
- [ ] Network interface selection in TUI - remaining for polish
- [ ] TUI service registration form - remaining for polish

### Phase 2.1: LocalMesh Agent Binary ✅ COMPLETE
- [x] Create `cmd/localmesh-agent/main.go`
- [x] Define gRPC proto files (`api/proto/agent/v1/agent.proto`)
- [x] Generated gRPC code (`api/gen/agent/v1/`)
- [x] Implement AgentService (Register, Unregister, Heartbeat, ListServices, GetServiceStatus)
- [x] Agent CLI: `register`, `unregister`, `status`, `list`
- [x] gRPC client in `internal/client/client.go`
- [x] gRPC server in `internal/grpc/server.go`
- [x] Framework integration with GRPCConfig

### Phase 2.2: Federation Protocol ✅ COMPLETE
- [x] Define gRPC FederationService proto (`api/proto/federation/v1/`)
- [x] Implement FederationServer (`internal/grpc/federation.go`)
- [x] Ed25519 keypair generation for realm identity
- [x] JoinFederation, LeaveFederation RPCs
- [x] SyncServices RPC for catalog synchronization
- [x] ResolveService RPC with zone-based access control
- [x] ExchangeTrust RPC for secure trust establishment
- [x] Federation CLI: `status`, `join`, `peers`, `sync`, `leave`
- [x] Framework integration with NewServerWithFederation()
- [ ] Automatic peer discovery via mDNS (future enhancement)
- [ ] TLS/mTLS for federation transport security (future)
- [ ] Persistent federation state across restarts (future)

### Phase 3: Enhanced RBAC 🔜 NEXT
- [ ] WiFi SSID → Role mapping
- [ ] Zone-based permissions
- [ ] Cross-realm authorization
- [ ] Policy engine for service access

---

## 🔌 RUNNING THE PROJECT

```bash
# Build all binaries
make build-all

# Run server in dev mode (requires sudo for mDNS/DNS)
sudo ./build/localmesh start --dev

# === Using localmesh CLI (legacy/direct) ===
./build/localmesh register myapp --port 3000
./build/localmesh services
./build/localmesh network interfaces

# === Using localmesh-agent (recommended) ===
# In another terminal, while server is running:
./build/localmesh-agent register myapp --port 3000 --server localhost:9000
./build/localmesh-agent list --server localhost:9000
./build/localmesh-agent status --server localhost:9000
./build/localmesh-agent unregister myapp --server localhost:9000

# === Federation Commands ===
# View federation status
./build/localmesh federation status

# Join another realm's federation
./build/localmesh federation join --peer cse.campus.local:9000

# List connected federation peers
./build/localmesh federation peers

# Sync services with all connected peers
./build/localmesh federation sync

# Leave federation
./build/localmesh federation leave

# Test mDNS resolution
getent hosts campus.local
curl http://campus.local:8080/health

# Check DNS server
dig @<WIFI_IP> campus.local +short
```

### Ports Used
- **8080:** HTTP Gateway
- **9000:** gRPC (AgentService + FederationService)
- **53:** DNS Server (bound to WiFi IP)
- **5353:** mDNS (via Avahi daemon)

---

## 🧪 TESTING CHECKLIST

Before committing:
```bash
# Lint
golangci-lint run

# Test
go test ./...

# Vulnerability check
govulncheck ./...

# Build
make build

# Manual test
sudo ./localmesh start --dev
curl http://campus.local:8080/health
```

---

## 📝 GIT CONVENTIONS

### Commit Format
```
<type>(<scope>): <description>

Types: feat, fix, docs, refactor, test, chore
Scopes: gateway, auth, tui, registry, mdns, dns, config, agent
```

### Examples
```
feat(gateway): add DNS server for Android support
fix(mdns): switch to avahi-publish-address for hostname
docs(plan): add federated architecture with gRPC
refactor(auth): use PASETO v4 instead of JWT
```

---

## 🆘 IF YOU'RE LOST

1. **Read `docs/LEARNING.md`** - 8-10 hour comprehensive guide
2. **Check `PLAN.md`** - ASCII diagrams explain everything
3. **Run with `--dev`** - See verbose logging
4. **Ask the user** - They know the vision!

---

## 👤 USER PREFERENCES

Based on conversation history:
- Prefers detailed ASCII diagrams for architecture
- Wants CLI/TUI feature parity
- Likes atomic git commits
- Values learning the "why" behind decisions
- Uses Ubuntu with UFW firewall
- Tests on phone hotspot with laptop as client

---

## 🔄 CONTINUITY PROTOCOL

When starting a new conversation:

1. **User says:** "Read AI_HANDOFF.md" or "Continue LocalMesh"
2. **You should:**
   - Read this file first
   - Read `PLAN.md` for current roadmap
   - Check `aiSkills/*.md` before writing code
   - Ask user what they want to work on next
3. **Then continue** from where the previous session left off

---

*This document should be updated whenever major architectural decisions are made or significant features are completed.*
