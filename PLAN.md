# LocalMesh - Campus Mesh Network Framework

> A secure, offline-first framework for building location-aware services on local networks.
> No internet. No GPS. Just WiFi-based identity and blazing fast local services.

---

## 🚀 NEXT PHASE: Dynamic mDNS Hostname Assignment

### Core Feature: Register Any Service with a Friendly URL

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              THE VISION                                      │
│                                                                              │
│   Developer starts Next.js:     $ npm run dev -- --port 3000                │
│                                                                              │
│   Developer registers:          $ localmesh register myapp --port 3000      │
│                                                                              │
│   LocalMesh:                                                                 │
│     ✓ Checks if "myapp" is available                                        │
│     ✓ Gets developer's IP automatically                                     │
│     ✓ Registers myapp.campus.local → 192.168.1.50:3000                     │
│     ✓ Advertises via mDNS (Avahi)                                          │
│                                                                              │
│   Any user on WiFi opens:       http://myapp.campus.local:3000              │
│                                 ✅ Works on Android Chrome!                 │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

### Priority Features (In Order)

#### 1. Network Interface Selection
```
$ localmesh start

🔍 Available network interfaces:
   [1] lo        - 127.0.0.1 (loopback)
   [2] wlan0     - 192.168.1.50 (WiFi)
   [3] eth0      - 10.0.0.5 (Ethernet)
   [4] docker0   - 172.17.0.1 (Docker)

? Select interfaces for LocalMesh (comma-separated): 2,3
✅ LocalMesh will operate on: wlan0, eth0
```

**Configuration options:**
- CLI flag: `--interfaces wlan0,eth0`
- Config YAML: `interfaces: [wlan0, eth0]`
- Environment: `LOCALMESH_INTERFACES=wlan0,eth0`
- TUI: Interactive selection

#### 2. Configurable Gateway Hostname
```yaml
# localmesh.yaml
gateway:
  hostname: campus     # becomes campus.local
  # OR
  hostname: myschool   # becomes myschool.local
```

**All config methods:**
- CLI: `localmesh start --hostname myschool`
- Env: `LOCALMESH_HOSTNAME=myschool`
- YAML: `gateway.hostname: myschool`
- TUI: Settings panel

#### 3. Service Registration Commands
```bash
# Register a service
$ localmesh register myapp --port 3000
✅ Registered: http://myapp.campus.local:3000

# List registered services
$ localmesh services
NAME      PORT   URL                           STATUS
myapp     3000   http://myapp.campus.local     ✅ healthy
lecture   8080   http://lecture.campus.local   ✅ healthy

# Unregister
$ localmesh unregister myapp
✅ Unregistered: myapp
```

#### 4. TUI Enhancements (Using Bubbles Components)
```
┌─────────────────────────────────────────────────────────────────────────────┐
│ LocalMesh Dashboard                                          campus.local  │
├───────────────────┬─────────────────────────────────────────────────────────┤
│                   │                                                         │
│ [Services]        │  📦 Register New Service                               │
│  Network          │  ─────────────────────────────                         │
│  Logs             │                                                         │
│  Settings    ←    │  Service Name: [myapp____________]                     │
│                   │  Port:         [3000______________]                     │
│                   │  Description:  [My Next.js App____]                     │
│                   │                                                         │
│                   │  Interfaces:                                            │
│                   │  [✓] wlan0 (192.168.1.50)                              │
│                   │  [ ] eth0  (10.0.0.5)                                  │
│                   │                                                         │
│                   │  [Register Service]   [Cancel]                         │
│                   │                                                         │
├───────────────────┴─────────────────────────────────────────────────────────┤
│ Status: Ready │ Services: 3 │ Nodes: 2 │ wlan0: 192.168.1.50              │
└─────────────────────────────────────────────────────────────────────────────┘
```

**Bubbles components to use:**
- `textinput` - Form fields
- `list` - Service/interface selection  
- `table` - Service listings
- `spinner` - Loading states
- `progress` - Health checks
- `help` - Keyboard shortcuts (we have this!)
- `viewport` - Scrollable logs

#### 5. Easy Onboarding for Any Framework
```bash
# Works with ANY tech stack:

# Next.js
$ npm run dev -- --port 3000
$ localmesh register frontend --port 3000

# Python Flask
$ flask run --port 5000
$ localmesh register api --port 5000

# Go server
$ go run main.go  # listening on :8080
$ localmesh register backend --port 8080

# Static files
$ python -m http.server 9000
$ localmesh register docs --port 9000
```

### Implementation Checklist

- [ ] **Interface Selection**
  - [ ] Detect available network interfaces
  - [ ] CLI flag `--interfaces`
  - [ ] YAML config `interfaces: []`
  - [ ] TUI interactive picker
  - [ ] Validate interface exists and is up

- [ ] **Hostname Configuration**  
  - [ ] CLI flag `--hostname`
  - [ ] Env var `LOCALMESH_HOSTNAME`
  - [ ] YAML config `gateway.hostname`
  - [ ] TUI settings panel
  - [ ] Validate hostname (no special chars)

- [ ] **Service Registration**
  - [ ] `localmesh register <name> --port <port>` command
  - [ ] `localmesh unregister <name>` command
  - [ ] `localmesh services` list command
  - [ ] Hostname availability check
  - [ ] Avahi integration for each service
  - [ ] Health monitoring for registered services
  - [ ] Auto-cleanup on disconnect

- [ ] **TUI Improvements**
  - [ ] Service registration form
  - [ ] Interface selection checkboxes
  - [ ] Settings panel for hostname
  - [ ] Real-time service status
  - [ ] Use more Bubbles components

- [ ] **CLI/TUI Feature Parity**
  - [ ] Every CLI command available in TUI
  - [ ] Every TUI action available in CLI
  - [ ] Consistent behavior across both

---

## 🎯 Vision

Build a **production-grade framework** that enables universities, enterprises, and large campuses to run secure, local-only services where:

1. **Location = Identity** - Your WiFi connection determines what you can access
2. **Zero Internet Dependency** - Everything runs on local mesh
3. **Plugin Architecture** - Developers build services on top of our framework
4. **Security First** - No CVEs, no shortcuts, audit-ready code
5. **Cloud Sync** - Periodic backup to survive hardware failures

---

## 🏗️ Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              LOCALMESH FRAMEWORK                             │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─────────────┐    ┌─────────────┐    ┌─────────────┐    ┌─────────────┐  │
│  │   Plugin    │    │   Plugin    │    │   Plugin    │    │   Plugin    │  │
│  │ Attendance  │    │   Lecture   │    │   Notices   │    │  Your App   │  │
│  └──────┬──────┘    └──────┬──────┘    └──────┬──────┘    └──────┬──────┘  │
│         │                  │                  │                  │         │
│  ───────┴──────────────────┴──────────────────┴──────────────────┴───────  │
│                          PLUGIN SDK / API                                   │
│  ────────────────────────────────────────────────────────────────────────  │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                         CORE FRAMEWORK                                │  │
│  │                                                                       │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │  │
│  │  │   Gateway   │  │    Auth     │  │  Service    │  │    Mesh     │  │  │
│  │  │   Router    │  │   Engine    │  │  Registry   │  │  Discovery  │  │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘  │  │
│  │                                                                       │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  │  │
│  │  │   Network   │  │   Storage   │  │    Sync     │  │   Crypto    │  │  │
│  │  │  Identity   │  │   Engine    │  │   Engine    │  │   Module    │  │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘  │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  ┌──────────────────────────────────────────────────────────────────────┐  │
│  │                              CLI / TUI                                │  │
│  │         localmesh init | start | plugin | sync | status              │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
└─────────────────────────────────────────────────────────────────────────────┘
                                      │
                                      ▼
                    ┌─────────────────────────────────┐
                    │       MESH NETWORK (LAN)        │
                    │  ┌───────┐ ┌───────┐ ┌───────┐  │
                    │  │ Node  │ │ Node  │ │ Node  │  │
                    │  │ (CS)  │ │(Mech) │ │(Civil)│  │
                    │  └───────┘ └───────┘ └───────┘  │
                    └─────────────────────────────────┘
```

---

## 📦 Project Structure

```
localmesh/
├── cmd/
│   └── localmesh/              # CLI entry point
│       └── main.go
│
├── internal/                    # Private framework code
│   ├── gateway/                 # HTTP gateway & reverse proxy
│   │   ├── router.go
│   │   ├── middleware.go
│   │   └── proxy.go
│   │
│   ├── auth/                    # Authentication & Authorization
│   │   ├── engine.go
│   │   ├── network_identity.go  # WiFi-based identity
│   │   ├── token.go             # JWT/PASETO tokens
│   │   └── policies.go          # Access control policies
│   │
│   ├── mesh/                    # Mesh network operations
│   │   ├── discovery.go         # mDNS/DNS-SD service discovery
│   │   ├── node.go              # Node representation
│   │   ├── heartbeat.go         # Health monitoring
│   │   └── topology.go          # Network topology
│   │
│   ├── registry/                # Service registry
│   │   ├── registry.go
│   │   ├── service.go
│   │   └── health.go
│   │
│   ├── storage/                 # Storage abstraction
│   │   ├── engine.go
│   │   ├── sqlite.go
│   │   ├── badger.go            # For high-performance KV
│   │   └── migrations.go
│   │
│   ├── sync/                    # Cloud sync engine
│   │   ├── engine.go
│   │   ├── snapshot.go
│   │   ├── restore.go
│   │   └── providers/
│   │       ├── s3.go
│   │       └── gcs.go
│   │
│   ├── crypto/                  # Cryptographic operations
│   │   ├── keys.go
│   │   ├── signing.go
│   │   └── encryption.go
│   │
│   └── config/                  # Configuration management
│       ├── config.go
│       └── validation.go
│
├── pkg/                         # PUBLIC SDK for plugin developers
│   ├── sdk/
│   │   ├── plugin.go            # Plugin interface
│   │   ├── context.go           # Request context with network info
│   │   ├── storage.go           # Storage helpers
│   │   ├── auth.go              # Auth helpers
│   │   └── events.go            # Event system
│   │
│   └── types/                   # Shared types
│       ├── service.go
│       ├── user.go
│       └── network.go
│
├── plugins/                     # Demo plugins (separate modules)
│   ├── attendance/
│   │   ├── go.mod
│   │   ├── main.go
│   │   ├── handlers.go
│   │   └── ui/
│   │
│   ├── lecture/
│   │   ├── go.mod
│   │   ├── main.go
│   │   ├── websocket.go
│   │   └── ui/
│   │
│   └── notices/
│       ├── go.mod
│       ├── main.go
│       └── ui/
│
├── web/                         # Admin dashboard (optional)
│   └── dashboard/
│
├── scripts/                     # Build & deployment scripts
│   ├── build.sh
│   ├── install.sh
│   └── security-audit.sh
│
├── configs/                     # Configuration templates
│   ├── localmesh.example.yaml
│   └── policies.example.yaml
│
├── docs/                        # Documentation
│   ├── getting-started.md
│   ├── plugin-development.md
│   └── security.md
│
├── test/                        # Integration & E2E tests
│   ├── integration/
│   └── e2e/
│
├── go.mod
├── go.sum
├── Makefile
├── .golangci.yml                # Linter configuration
├── .goreleaser.yml              # Release automation
└── PLAN.md                      # This file
```

---

## 🔐 Security Architecture

### Threat Model

| Threat | Mitigation |
|--------|------------|
| Unauthorized network access | WiFi-based identity verification |
| Token theft | Short-lived tokens + refresh rotation |
| Service spoofing | mTLS between services, signed service manifests |
| Data tampering | HMAC signatures on critical data |
| Replay attacks | Nonce + timestamp validation |
| SQL injection | Parameterized queries only, no raw SQL |
| Path traversal | Strict input validation, allowlists |
| Privilege escalation | RBAC with least privilege |
| Supply chain attacks | Dependency vendoring, hash verification |

### Security Practices

1. **No Dynamic SQL** - All queries parameterized via sqlc or similar
2. **Input Validation** - Every external input validated at boundary
3. **Output Encoding** - Context-aware encoding for all outputs
4. **Cryptography** - Use only audited libraries (stdlib crypto, nacl)
5. **Secrets Management** - Never in code, environment or vault only
6. **Dependency Scanning** - govulncheck in CI pipeline
7. **Static Analysis** - golangci-lint with security linters enabled
8. **Fuzzing** - Go's native fuzzing for parsers and handlers

### Network Identity System

```
┌──────────────────────────────────────────────────────────────────────┐
│                     NETWORK IDENTITY FLOW                            │
├──────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  1. Client connects to WiFi (e.g., "CS-DEPT-WIFI")                  │
│                          ↓                                           │
│  2. Client requests token from Gateway                               │
│     → Sends: MAC address, requested SSID info                        │
│                          ↓                                           │
│  3. Gateway verifies client is actually on claimed network           │
│     → Checks ARP table / DHCP leases / AP verification               │
│                          ↓                                           │
│  4. Gateway issues JWT/PASETO token with claims:                     │
│     {                                                                │
│       "network_zone": "cs-department",                               │
│       "network_id": "CS-DEPT-WIFI",                                  │
│       "allowed_services": ["attendance", "lectures", "notices"],     │
│       "location_verified": true,                                     │
│       "issued_at": 1706400000,                                       │
│       "expires_at": 1706403600  // 1 hour                            │
│     }                                                                │
│                          ↓                                           │
│  5. Client includes token in all requests                            │
│                          ↓                                           │
│  6. Services validate token + check allowed_services                 │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 🔌 Plugin System Design

### Plugin Interface

```go
// pkg/sdk/plugin.go

type Plugin interface {
    // Metadata
    Info() PluginInfo
    
    // Lifecycle
    Init(ctx context.Context, cfg PluginConfig) error
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    
    // HTTP handlers (mounted at /plugins/{plugin-name}/)
    Routes() []Route
    
    // Access requirements
    RequiredZones() []string  // Which network zones can access this
    
    // Health
    Health() HealthStatus
}

type PluginInfo struct {
    Name        string
    Version     string
    Description string
    Author      string
    MinFrameworkVersion string
}

type Route struct {
    Method      string
    Path        string
    Handler     http.HandlerFunc
    RequireAuth bool
    AllowedZones []string  // Override plugin-level zones for specific routes
}
```

### Plugin Context

```go
// Every request handler receives rich context

type RequestContext struct {
    // Network identity
    NetworkZone    string
    NetworkID      string
    ClientIP       net.IP
    IsVerified     bool
    
    // User (if authenticated)
    UserID         string
    Roles          []string
    
    // Storage access (scoped to plugin)
    DB             PluginStorage
    
    // Logging
    Logger         *slog.Logger
    
    // Tracing
    TraceID        string
}
```

---

## 🌐 Mesh Discovery Protocol

### Service Advertisement

```yaml
# Each service advertises via mDNS/DNS-SD

_localmesh._tcp.local.
  attendance._localmesh._tcp.local.
    - host: node-cs-01.local
    - port: 8081
    - txt:
        version: 1.0.0
        zones: cs-department,general
        health: /health
        
  lecture._localmesh._tcp.local.
    - host: node-main-01.local
    - port: 8082
    - txt:
        version: 1.0.0
        zones: general
        health: /health
```

### Node Discovery Flow

```
1. Node starts → Broadcasts presence via mDNS
2. Gateway discovers nodes → Adds to registry
3. Gateway monitors heartbeats → Removes dead nodes
4. Clients query gateway → Get service locations
5. Gateway proxies OR redirects based on config
```

---

## ☁️ Cloud Sync Architecture

### Sync Strategy

```
┌─────────────────────────────────────────────────────────────────┐
│                      SYNC ARCHITECTURE                          │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  LOCAL                           CLOUD                          │
│  ┌─────────────────┐            ┌─────────────────┐            │
│  │   SQLite DB     │───WAL────▶│   S3/GCS        │            │
│  │   Badger KV     │───Snap───▶│   Object Store  │            │
│  │   Config Files  │───Enc────▶│   Encrypted     │            │
│  └─────────────────┘            └─────────────────┘            │
│                                                                 │
│  Sync Modes:                                                    │
│  • Continuous: Stream WAL changes (near real-time)              │
│  • Periodic: Snapshot every N minutes (configurable)            │
│  • Manual: On-demand via CLI                                    │
│                                                                 │
│  Recovery:                                                      │
│  $ localmesh restore --from s3://backup/latest                  │
│  → Downloads encrypted snapshot                                 │
│  → Decrypts with local key                                      │
│  → Restores to local storage                                    │
│  → Validates integrity                                          │
│  → Restarts services                                            │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 🖥️ CLI Design

```bash
# Core Commands
localmesh init                    # Initialize new LocalMesh node
localmesh start                   # Start the framework
localmesh stop                    # Graceful shutdown
localmesh status                  # Show running services, nodes, health

# Plugin Management
localmesh plugin list             # List installed plugins
localmesh plugin install <path>   # Install a plugin
localmesh plugin remove <name>    # Remove a plugin
localmesh plugin enable <name>    # Enable a plugin
localmesh plugin disable <name>   # Disable a plugin

# Network & Discovery
localmesh network scan            # Scan for other LocalMesh nodes
localmesh network status          # Show network topology
localmesh network zones           # List configured zones

# Sync & Backup
localmesh sync status             # Show sync status
localmesh sync now                # Trigger immediate sync
localmesh restore --from <uri>    # Restore from backup

# Security
localmesh keys generate           # Generate new keypair
localmesh keys rotate             # Rotate keys
localmesh audit                   # Run security audit

# Development
localmesh dev                     # Start in development mode
localmesh plugin scaffold <name>  # Generate plugin boilerplate
```

---

## 📊 Demo Plugins (MVP)

### 1. Attendance Plugin
- **Access**: Department WiFi only (e.g., CS students → CS-DEPT-WIFI)
- **Features**:
  - Mark attendance (QR code or button)
  - View attendance history
  - Export reports (CSV)
- **Security**: Location-verified, time-bound sessions

### 2. Live Lecture Plugin
- **Access**: General campus WiFi
- **Features**:
  - Real-time text/slide broadcast (WebSocket)
  - Q&A queue
  - Session recording (text-based)
- **Security**: Teacher auth via department WiFi

### 3. Notice Board Plugin
- **Access**: General campus WiFi
- **Features**:
  - Post announcements
  - Department-specific notices
  - Real-time updates
- **Security**: Post permissions via role

---

## 🛠️ Technology Stack

| Component | Technology | Rationale |
|-----------|------------|-----------|
| Language | Go 1.22+ | Performance, concurrency, single binary |
| HTTP Router | chi or stdlib | Lightweight, composable |
| WebSocket | gorilla/websocket or nhooyr/websocket | Battle-tested |
| mDNS | hashicorp/mdns | Proven in Consul |
| Database | SQLite + Badger | Embedded, no external deps |
| Tokens | PASETO v4 | Safer than JWT, no algorithm confusion |
| Crypto | stdlib + nacl | Audited, secure defaults |
| CLI | cobra + bubbletea | Excellent UX, TUI support |
| Config | viper | Flexible configuration |
| Logging | slog (stdlib) | Structured, fast |
| Testing | stdlib + testify | Comprehensive |
| Linting | golangci-lint | 50+ linters |
| Build | GoReleaser | Cross-platform releases |

---

## 📅 Development Phases & Progress Tracker

> **Legend:** ✅ Complete | 🔄 In Progress | ⏳ Pending

### Phase 1: Foundation (Weeks 1-3)

| Status | Task | Notes |
|:------:|------|-------|
| ✅ | Project scaffolding & directory structure | Done - all dirs created |
| ✅ | Go module initialization | `github.com/FABLOUSFALCON/localmesh` |
| ✅ | golangci-lint security config | `.golangci.yml` with 30+ linters |
| ✅ | Makefile with build/test/lint targets | Complete |
| ✅ | Basic CLI structure (cobra) | Commands: init, start, stop, status, plugin, network, sync, auth |
| ✅ | Plugin SDK interface definition | `pkg/sdk/plugin.go` |
| ✅ | Shared types package | `pkg/types/types.go` |
| ✅ | Core configuration system (viper) | `internal/config/config.go` |
| ✅ | Logging & error handling patterns (slog) | Integrated throughout |
| ✅ | mDNS discovery implementation | `internal/mesh/discovery.go` - working! |
| ✅ | Service registry (in-memory + persisted) | `internal/registry/registry.go` |
| ✅ | Basic HTTP gateway | `internal/gateway/router.go` with middleware |
| ✅ | **Interactive TUI dashboard** | Bubble Tea + Lip Gloss |
| ✅ | SQLite storage (pure Go) | `internal/storage/sqlite.go` - WAL mode, 64MB cache |
| ✅ | Badger KV store | `internal/storage/badger.go` - sessions, tokens |

### Phase 2: Security Core (Weeks 4-5)

| Status | Task | Notes |
|:------:|------|-------|
| ✅ | PASETO token generation & validation | v2 tokens with Ed25519 |
| ✅ | Zone-based access control | `internal/auth/zones.go` |
| ✅ | Crypto key management | Ed25519 keys auto-generated |
| ✅ | Rate limiting middleware | Per-IP, per-user with burst |
| ✅ | Auth middleware & handlers | Login, refresh, logout, sessions |
| ✅ | Argon2id password hashing | OWASP recommended params |
| ✅ | Session management | Max sessions, auto-expiry |
| ✅ | **Network identity detection** | WiFi SSID→Zone, BSSID, subnet detection |
| ✅ | **Network identity API** | `/api/v1/network/identity`, mappings, verify |
| ✅ | **Security headers middleware** | CSP, HSTS, X-Frame-Options, etc. |

### Phase 3: Plugin System (Weeks 6-7)

| Status | Task | Notes |
|:------:|------|-------|
| ✅ | Plugin interface definition | `sdk.Plugin` interface |
| ✅ | **Plugin loader & lifecycle** | Load, init, start, stop |
| ✅ | **Plugin route registration** | Mount at `/plugins/{name}/` |
| ⏳ | Plugin storage abstraction | Isolated KV per plugin |
| ⏳ | Plugin scaffold generator | `localmesh plugin scaffold` |
| ⏳ | Plugin hot-reload (dev mode) | Optional |

### Phase 4: Storage & Sync (Weeks 8-9)

| Status | Task | Notes |
|:------:|------|-------|
| ✅ | SQLite integration | modernc.org/sqlite (pure Go) |
| ✅ | Database migrations system | Auto-creates tables |
| ✅ | Badger KV for hot data | Sessions, tokens, cache |
| ⏳ | Snapshot system | Point-in-time backups |
| ⏳ | S3/GCS sync providers | Cloud backup |
| ⏳ | Restore functionality | `localmesh restore` |

### Phase 5: Demo Plugins (Weeks 10-11)

| Status | Task | Notes |
|:------:|------|-------|
| ✅ | **Attendance plugin** | Zone-based attendance, sessions, records |
| ⏳ | Live lecture plugin | WebSocket broadcast |
| ⏳ | Notice board plugin | Real-time updates |
| ⏳ | Plugin documentation | Examples, API docs |

### Phase 6: Polish & Security Audit (Week 12)

| Status | Task | Notes |
|:------:|------|-------|
| ⏳ | Security audit & fixes | govulncheck, gosec |
| ⏳ | Performance optimization | Benchmarks |
| ⏳ | Documentation completion | |
| ⏳ | Demo video & presentation | For submission |

---

## 🖥️ Interactive TUI Dashboard

We're building a **btop-like** interactive terminal UI using Bubble Tea + Lip Gloss.

### TUI Features

```
┌─────────────────────────────────────────────────────────────────────────────┐
│  LocalMesh Dashboard                                    v1.0.0  │ 🟢 Online │
├─────────────────────────────────────────────────────────────────────────────┤
│                                                                             │
│  ┌─ Services ────────────────────────┐  ┌─ Network Zones ───────────────┐  │
│  │                                   │  │                               │  │
│  │  🟢 attendance    10.0.1.5:8081  │  │  📍 general        12 clients │  │
│  │  🟢 lecture       10.0.1.5:8082  │  │  📍 cs-department   8 clients │  │
│  │  🟡 notices       10.0.1.6:8083  │  │  📍 mech-dept       3 clients │  │
│  │                                   │  │                               │  │
│  └───────────────────────────────────┘  └───────────────────────────────┘  │
│                                                                             │
│  ┌─ Nodes ───────────────────────────┐  ┌─ Quick Actions ───────────────┐  │
│  │                                   │  │                               │  │
│  │  🖥️  main-gateway   10.0.1.5     │  │  [s] Start service            │  │
│  │  🖥️  cs-node-01     10.0.1.6     │  │  [p] Manage plugins           │  │
│  │  ⚫ mech-node-01   offline       │  │  [l] View logs                │  │
│  │                                   │  │  [c] Configuration            │  │
│  └───────────────────────────────────┘  │  [q] Quit                     │  │
│                                         └───────────────────────────────┘  │
│                                                                             │
│  ┌─ Recent Activity ────────────────────────────────────────────────────┐  │
│  │  19:32:45  attendance  Student marked present (CS101)                │  │
│  │  19:32:41  lecture     New session started: "Data Structures"        │  │
│  │  19:32:38  gateway     Node cs-node-01 joined the mesh               │  │
│  └──────────────────────────────────────────────────────────────────────┘  │
│                                                                             │
│  CPU: ▓▓▓▓▓░░░░░ 48%   MEM: ▓▓▓▓▓▓▓░░░ 72%   SYNC: Last 5m ago          │
└─────────────────────────────────────────────────────────────────────────────┘
```

### TUI Technology Stack

| Package | Purpose |
|---------|---------|
| [charmbracelet/bubbletea](https://github.com/charmbracelet/bubbletea) | TUI framework (Elm architecture) |
| [charmbracelet/lipgloss](https://github.com/charmbracelet/lipgloss) | Styling (colors, borders, layout) |
| [charmbracelet/bubbles](https://github.com/charmbracelet/bubbles) | Pre-built components (tables, spinners, etc.) |
| [charmbracelet/log](https://github.com/charmbracelet/log) | Beautiful logging |

### TUI Views/Screens

1. **Dashboard** (default) - Overview of everything
2. **Services** - Detailed service list, start/stop/restart
3. **Plugins** - Install, enable, disable, configure
4. **Logs** - Real-time log viewer with filtering
5. **Network** - Node discovery, zone visualization
6. **Config** - Edit configuration interactively
7. **Sync** - Cloud sync status, trigger backup/restore

### Example Projects for Inspiration

- [btop](https://github.com/aristocratos/btop) - System monitor
- [lazygit](https://github.com/jesseduffield/lazygit) - Git TUI
- [k9s](https://github.com/derailed/k9s) - Kubernetes TUI
- [glow](https://github.com/charmbracelet/glow) - Markdown reader
- [soft-serve](https://github.com/charmbracelet/soft-serve) - Git server TUI

---

## 🧪 Testing Strategy

```
├── Unit Tests
│   └── Every package has _test.go files
│   └── Table-driven tests
│   └── 80%+ coverage target
│
├── Integration Tests
│   └── test/integration/
│   └── Docker-based multi-node tests
│   └── Database integration tests
│
├── E2E Tests
│   └── test/e2e/
│   └── Full flow tests with real network
│
├── Fuzz Tests
│   └── Input parsers
│   └── Token validation
│   └── Network identity verification
│
├── Security Tests
│   └── govulncheck
│   └── gosec
│   └── Manual penetration testing
│
└── Performance Tests
    └── Benchmark critical paths
    └── Load testing with k6
```

---

## 📋 Quality Gates

Before any merge:

1. ✅ All tests pass
2. ✅ golangci-lint clean (strict config)
3. ✅ govulncheck clean
4. ✅ No hardcoded secrets
5. ✅ Documentation updated
6. ✅ Changelog entry added

---

## 🎓 Learning Resources

### Go Patterns & Best Practices
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Uber Go Style Guide](https://github.com/uber-go/guide)

### Security
- [OWASP Go Secure Coding](https://owasp.org/www-project-go-secure-coding-practices-guide/)
- [CWE Top 25](https://cwe.mitre.org/top25/)

### Networking
- [mDNS RFC 6762](https://datatracker.ietf.org/doc/html/rfc6762)
- [DNS-SD RFC 6763](https://datatracker.ietf.org/doc/html/rfc6763)

### Awesome Go Projects to Learn From
- [Consul](https://github.com/hashicorp/consul) - Service mesh, mDNS
- [Caddy](https://github.com/caddyserver/caddy) - HTTP server, plugins
- [Hugo](https://github.com/gohugoio/hugo) - CLI patterns
- [Litestream](https://github.com/benbjohnson/litestream) - SQLite replication

---

## 🚀 Getting Started (Next Steps)

1. **Set up tooling**: golangci-lint, govulncheck, pre-commit hooks
2. **Initialize Go module**: `go mod init github.com/FABLOUSFALCON/localmesh`
3. **Create basic CLI skeleton** with cobra
4. **Implement mDNS discovery** as proof of concept
5. **Build minimal gateway** that proxies to a test service

---

## 💡 Future Ideas (Post-MVP)

- [ ] TUI dashboard using bubbletea *(Moved to Phase 1!)*
- [ ] Prometheus metrics export
- [ ] OpenTelemetry tracing
- [ ] Plugin marketplace
- [ ] Mobile SDK (Go → gomobile)
- [ ] Hardware token support (YubiKey)
- [ ] Mesh networking without central gateway (full P2P)

---

## 🗄️ Database Strategy

### Recommended Approach: Embedded Databases

Since LocalMesh runs **offline without external dependencies**, we use **embedded databases**:

| Database | Use Case | Why |
|----------|----------|-----|
| **SQLite** | Relational data (users, attendance records, notices) | Single file, ACID, SQL queries |
| **Badger** | Fast KV store (sessions, tokens, cache) | Pure Go, LSM-tree, no CGO option |

### SQLite for Structured Data

```go
// Example: Attendance records, user data, notices
type AttendanceRecord struct {
    ID        int64     `db:"id"`
    StudentID string    `db:"student_id"`
    ClassID   string    `db:"class_id"`
    MarkedAt  time.Time `db:"marked_at"`
    Zone      string    `db:"zone"`
}
```

**Tools:**
- [mattn/go-sqlite3](https://github.com/mattn/go-sqlite3) - CGO SQLite driver
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite) - Pure Go (no CGO!)
- [sqlc](https://sqlc.dev/) - Generate type-safe Go from SQL

### Badger for Fast KV

```go
// Example: Session tokens, rate limiting counters, hot cache
key := []byte("session:" + sessionID)
value := []byte(tokenJSON)
err := db.Update(func(txn *badger.Txn) error {
    return txn.SetEntry(badger.NewEntry(key, value).WithTTL(time.Hour))
})
```

**Why Badger over Redis:**
- Embedded (no separate server)
- Pure Go
- Persistent by default
- TTL support built-in

### Database Location

```
data/
├── localmesh.db          # SQLite database
├── badger/               # Badger KV directory
│   ├── 000000.vlog
│   └── MANIFEST
└── backups/              # Local snapshots
```

### Migration Strategy

```go
// Use golang-migrate or custom migrations
migrations := []Migration{
    {Version: 1, SQL: `CREATE TABLE users (...)`},
    {Version: 2, SQL: `CREATE TABLE attendance (...)`},
    {Version: 3, SQL: `ALTER TABLE users ADD COLUMN zone TEXT`},
}
```

---

## 🔧 Development Environment (Omarchy Linux)

### Modern CLI Tools

This project is developed on Omarchy Linux with modern alternatives:

| Traditional | Modern | Usage |
|------------|--------|-------|
| `ls`, `tree` | `eza` | `eza --tree --level=3` |
| `grep` | `rg` (ripgrep) | `rg "TODO" --type go` |
| `find` | `fd` | `fd "\.go$"` |
| `cat` | `bat` | `bat file.go` |
| `du` | `dust` | `dust -d 2` |

### Useful Commands

```bash
# List project structure
eza --tree --level=3 --icons

# Find Go files
fd "\.go$"

# Search for TODO comments
rg "TODO|FIXME" --type go

# Watch for changes and rebuild
watchexec -e go "make build"
```

---

*This is a living document. Update as we build.*

**Last Updated**: 2026-01-28
**Authors**: The LocalMesh Team 🚀
