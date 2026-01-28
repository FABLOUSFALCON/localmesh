# LocalMesh - Campus Mesh Network Framework

> A secure, offline-first framework for building location-aware services on local networks.
> No internet. No GPS. Just WiFi-based identity and blazing fast local services.

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

## 📅 Development Phases

### Phase 1: Foundation (Weeks 1-3)
- [ ] Project scaffolding & tooling setup
- [ ] Core configuration system
- [ ] Basic CLI structure (cobra)
- [ ] Logging & error handling patterns
- [ ] mDNS discovery implementation
- [ ] Service registry (in-memory)
- [ ] Basic HTTP gateway

### Phase 2: Security Core (Weeks 4-5)
- [ ] Network identity detection
- [ ] PASETO token generation & validation
- [ ] Zone-based access control
- [ ] Policy engine
- [ ] Crypto key management

### Phase 3: Plugin System (Weeks 6-7)
- [ ] Plugin interface definition
- [ ] Plugin loader & lifecycle
- [ ] Plugin isolation & resource limits
- [ ] Plugin storage abstraction
- [ ] Plugin scaffold generator

### Phase 4: Storage & Sync (Weeks 8-9)
- [ ] SQLite integration with migrations
- [ ] Badger KV for hot data
- [ ] Snapshot system
- [ ] S3/GCS sync providers
- [ ] Restore functionality

### Phase 5: Demo Plugins (Weeks 10-11)
- [ ] Attendance plugin
- [ ] Live lecture plugin
- [ ] Notice board plugin
- [ ] Plugin documentation

### Phase 6: Polish & Security Audit (Week 12)
- [ ] Security audit & fixes
- [ ] Performance optimization
- [ ] Documentation completion
- [ ] Demo video & presentation

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

- [ ] TUI dashboard using bubbletea
- [ ] Prometheus metrics export
- [ ] OpenTelemetry tracing
- [ ] Plugin marketplace
- [ ] Mobile SDK (Go → gomobile)
- [ ] Hardware token support (YubiKey)
- [ ] Mesh networking without central gateway (full P2P)

---

*This is a living document. Update as we build.*

**Last Updated**: 2026-01-28
**Authors**: The LocalMesh Team 🚀
