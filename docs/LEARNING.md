# 🎓 LocalMesh Learning Guide

> A systematic approach to understanding the codebase from zero to contributor.

**Time Estimate:** 8-10 hours for complete understanding

---

## 📚 Table of Contents

1. [Philosophy & Architecture](#1-philosophy--architecture)
2. [Technology Stack Deep Dive](#2-technology-stack-deep-dive)
3. [Code Reading Order](#3-code-reading-order)
4. [Data Flow Walkthrough](#4-data-flow-walkthrough)
5. [Hands-On Exercises](#5-hands-on-exercises)
6. [Building Your First Service](#6-building-your-first-service)

---

## 1. Philosophy & Architecture

### What Problem Does LocalMesh Solve?

Imagine a university campus. Students connect to `CS-DEPT-WIFI`. Now:
- A lecture streaming service runs on a professor's laptop
- Students want to watch without knowing IP addresses
- If the professor changes laptop/port, students shouldn't care
- Only students ON campus WiFi should access it

**LocalMesh fills these gaps:**
```
┌─────────────────────────────────────────────────────────────────┐
│                     THE FRAMEWORK'S JOB                         │
├─────────────────────────────────────────────────────────────────┤
│  ✅ Service Discovery   - "Where is the lecture service?"       │
│  ✅ Traffic Routing     - "Route me to it, wherever it is"      │
│  ✅ Network Identity    - "Is this user on campus WiFi?"        │
│  ✅ Health Monitoring   - "Is the service alive?"               │
│  ✅ Auto-Failover       - "Service moved? Update routes"        │
├─────────────────────────────────────────────────────────────────┤
│                     NOT THE FRAMEWORK'S JOB                     │
├─────────────────────────────────────────────────────────────────┤
│  ❌ User Management     - Service's responsibility              │
│  ❌ Business Logic      - Service's responsibility              │
│  ❌ Database Schema     - Service's responsibility              │
│  ❌ UI/Frontend         - Service's responsibility              │
└─────────────────────────────────────────────────────────────────┘
```

### Core Concept: The Service Mesh

```
                    ┌─────────────────────┐
                    │   LocalMesh Node    │
                    │   (Your Laptop)     │
                    │                     │
   HTTP Request     │  ┌───────────────┐  │
   ─────────────────┼─▶│   Gateway     │  │
   "GET /lectures"  │  │   (Router)    │  │
                    │  └───────┬───────┘  │
                    │          │          │
                    │          ▼          │
                    │  ┌───────────────┐  │
                    │  │   Registry    │  │
                    │  │ "lectures" →  │  │
                    │  │ 192.168.1.5   │  │
                    │  │    :9000      │  │
                    │  └───────┬───────┘  │
                    │          │          │
                    └──────────┼──────────┘
                               │
                               ▼ Proxy/Redirect
                    ┌─────────────────────┐
                    │  Lecture Service    │
                    │  (Another Laptop)   │
                    │  192.168.1.5:9000   │
                    └─────────────────────┘
```

---

## 2. Technology Stack Deep Dive

### 2.1 Cobra - CLI Framework

**What:** Builds command-line interfaces with subcommands
**Why:** Professional CLIs like `git`, `docker`, `kubectl` use this pattern
**Docs:** https://cobra.dev/

```go
// Simple mental model:
// rootCmd is the base: "localmesh"
// subcommands are: "localmesh start", "localmesh network scan"

var startCmd = &cobra.Command{
    Use:   "start",              // The command name
    Short: "Start the framework", // One-line help
    RunE: func(cmd *cobra.Command, args []string) error {
        // This runs when user types: localmesh start
        return nil
    },
}

// Register in init():
func init() {
    rootCmd.AddCommand(startCmd)
}
```

**Read:** `cmd/localmesh/cmd/root.go` - All our commands

**Exercise:** Run `./localmesh --help` and trace each command to its code

---

### 2.2 Viper - Configuration

**What:** Reads config from YAML/JSON/ENV/flags
**Why:** One library handles all config sources
**Docs:** https://github.com/spf13/viper

```go
// We wrapped Viper in internal/config/config.go
// Config struct mirrors YAML structure

type Config struct {
    Gateway  GatewayConfig  `yaml:"gateway"`
    Storage  StorageConfig  `yaml:"storage"`
    Network  NetworkConfig  `yaml:"network"`
}

// Usage:
cfg, _ := config.Load("localmesh.yaml")
fmt.Println(cfg.Gateway.Port)  // 8080
```

**Read:** `internal/config/config.go`

**Exercise:** Create a `localmesh.yaml` and modify values, see them load

---

### 2.3 Bubble Tea - TUI Framework

**What:** Terminal User Interface using Elm Architecture
**Why:** Build interactive dashboards like lazygit, btop
**Docs:** https://github.com/charmbracelet/bubbletea

**The Elm Architecture (critical to understand):**

```
┌─────────────────────────────────────────────────────────────┐
│                    THE ELM ARCHITECTURE                     │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│   ┌─────────┐     ┌─────────┐     ┌─────────┐              │
│   │  Model  │────▶│  View   │────▶│ Screen  │              │
│   │ (state) │     │ (render)│     │ (user)  │              │
│   └────▲────┘     └─────────┘     └────┬────┘              │
│        │                               │                    │
│        │         ┌─────────┐           │                    │
│        └─────────│ Update  │◀──────────┘                    │
│                  │ (logic) │     User presses key           │
│                  └─────────┘                                │
│                                                             │
│   1. Model holds all state (current view, selected item)    │
│   2. View renders Model to string (what user sees)          │
│   3. User presses key → Message sent to Update              │
│   4. Update modifies Model → View re-renders                │
│   5. Loop continues                                         │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

```go
// In our code (internal/tui/app.go):

type App struct {
    // MODEL - all state lives here
    currentView  View           // dashboard, services, logs, etc.
    services     []ServiceInfo  // list of services to display
    selectedIdx  int            // which item is selected
    width, height int           // terminal size
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // UPDATE - handle user input, modify state
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "j":
            a.selectedIdx++  // Move down
        case "k":
            a.selectedIdx--  // Move up
        case "q":
            return a, tea.Quit
        }
    }
    return a, nil
}

func (a *App) View() string {
    // VIEW - render state to string
    // This is called after every Update
    return a.renderDashboard()  // Returns styled string
}
```

**Read Order:**
1. `internal/tui/app.go` - Main App struct, Update, View
2. `internal/tui/keys.go` - Keybindings
3. `internal/tui/theme.go` - Colors and styles
4. `internal/tui/components.go` - Reusable UI pieces

**Exercise:** Add a new keybinding 'x' that prints "Hello!" in the status bar

---

### 2.4 Lip Gloss - Terminal Styling

**What:** CSS-like styling for terminal output
**Why:** Makes TUI beautiful with colors, borders, padding
**Docs:** https://github.com/charmbracelet/lipgloss

```go
// Think of it like CSS for terminals

// Define a style (like a CSS class)
var panelStyle = lipgloss.NewStyle().
    Border(lipgloss.RoundedBorder()).  // CSS: border-radius
    BorderForeground(lipgloss.Color("#7C3AED")).  // border-color
    Padding(1, 2).  // padding: 1 top/bottom, 2 left/right
    Width(40)       // width: 40ch

// Apply style to content
output := panelStyle.Render("Hello World")
// Returns string with ANSI escape codes for colors
```

**Key Functions:**
- `Render(string)` - Apply style to text
- `Width(int)` / `Height(int)` - Size constraints
- `Border()` - Add borders
- `Foreground()` / `Background()` - Colors
- `Bold()` / `Italic()` - Text decoration
- `lipgloss.JoinHorizontal()` - Layout side by side
- `lipgloss.JoinVertical()` - Layout stacked

**Read:** `internal/tui/theme.go` - All our styles defined here

---

### 2.5 mDNS / Zeroconf - Service Discovery

**What:** Find services on local network without central server
**Why:** Services announce themselves, others discover automatically
**Docs:** https://github.com/grandcat/zeroconf

**How it works:**
```
┌──────────────────────────────────────────────────────────────┐
│                     mDNS DISCOVERY                           │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Laptop A (runs lecture service):                            │
│  ┌──────────────────────────────────────────────────┐        │
│  │ "Hey network! I'm 'lecture-service' at port 9000" │        │
│  │  Broadcasting via mDNS on 224.0.0.251:5353       │        │
│  └──────────────────────────────────────────────────┘        │
│                          │                                   │
│                          │ Multicast UDP                     │
│                          ▼                                   │
│  ┌────────────────────────────────────────────────────────┐  │
│  │               Local Network (WiFi)                     │  │
│  └────────────────────────────────────────────────────────┘  │
│                          │                                   │
│                          ▼                                   │
│  Laptop B (runs LocalMesh gateway):                          │
│  ┌──────────────────────────────────────────────────┐        │
│  │ "Oh! Found 'lecture-service' at 192.168.1.5:9000" │        │
│  │  Adding to registry...                            │        │
│  └──────────────────────────────────────────────────┘        │
│                                                              │
└──────────────────────────────────────────────────────────────┘
```

```go
// In internal/mesh/discovery.go:

// To ADVERTISE a service:
server, _ := zeroconf.Register(
    "lecture-service",      // Instance name
    "_localmesh._tcp",      // Service type
    "local.",               // Domain
    9000,                   // Port
    []string{"zone=campus"}, // Metadata as TXT records
    nil,                    // Network interfaces (nil = all)
)

// To DISCOVER services:
resolver, _ := zeroconf.NewResolver(nil)
entries := make(chan *zeroconf.ServiceEntry)

go func() {
    for entry := range entries {
        fmt.Printf("Found: %s at %s:%d\n", 
            entry.Instance, 
            entry.AddrIPv4[0], 
            entry.Port)
    }
}()

resolver.Browse(ctx, "_localmesh._tcp", "local.", entries)
```

**Read:** `internal/mesh/discovery.go`

**Exercise:** Run `./localmesh network scan` and read the code path

---

### 2.6 SQLite & Badger - Storage

**SQLite:** Relational database in a single file
**Badger:** Key-value store for fast caching

```go
// SQLite - for structured data (users, sessions, services)
// In internal/storage/sqlite.go:

db.Exec(`CREATE TABLE IF NOT EXISTS services (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    health_status TEXT
)`)

// Query:
rows, _ := db.Query("SELECT * FROM services WHERE health_status = ?", "healthy")

// Badger - for fast key-value (tokens, cache)
// In internal/storage/badger.go:

// Set with TTL:
cache.Set("token:abc123", tokenData, 15*time.Minute)

// Get:
data, err := cache.Get("token:abc123")
```

**Read:** `internal/storage/storage.go` (interface), then sqlite.go, badger.go

---

### 2.7 PASETO - Secure Tokens

**What:** Platform-Agnostic Security Tokens (better than JWT)
**Why:** No algorithm confusion attacks, simpler, more secure
**Docs:** https://paseto.io/

```go
// In internal/auth/tokens.go:

// Create token:
token, _ := paseto.NewV2().Encrypt(
    symmetricKey,
    map[string]interface{}{
        "user_id": "123",
        "zone":    "cs-department",
        "exp":     time.Now().Add(15 * time.Minute),
    },
    nil, // footer
)
// Result: v2.local.xxx... (encrypted, tamper-proof)

// Validate token:
var claims map[string]interface{}
paseto.NewV2().Decrypt(token, symmetricKey, &claims, nil)
```

**Read:** `internal/auth/tokens.go`

---

## 3. Code Reading Order

### 📖 Recommended Reading Path (8-10 hours)

**Hour 1-2: Entry Points**
```
1. cmd/localmesh/main.go          (5 min)  - Entry point
2. cmd/localmesh/cmd/root.go      (30 min) - All CLI commands
3. internal/config/config.go      (20 min) - Configuration
4. Run: ./localmesh --help        (5 min)  - See it work
```

**Hour 3-4: Core Framework**
```
5. internal/core/framework.go     (45 min) - Main orchestration
6. internal/storage/storage.go    (15 min) - Storage interface
7. internal/storage/sqlite.go     (30 min) - SQL implementation
```

**Hour 5: Service Discovery**
```
8. internal/mesh/discovery.go     (45 min) - mDNS magic
9. internal/registry/registry.go  (30 min) - Service registry
```

**Hour 6: Network & Auth**
```
10. internal/network/identity.go  (30 min) - WiFi → Zone
11. internal/auth/service.go      (30 min) - Auth orchestration
12. internal/auth/tokens.go       (15 min) - PASETO tokens
```

**Hour 7-8: Gateway & Routing**
```
13. internal/gateway/router.go    (45 min) - HTTP routing
14. internal/gateway/security.go  (20 min) - Security headers
15. internal/services/proxy.go    (30 min) - Reverse proxy
```

**Hour 9-10: TUI**
```
16. internal/tui/app.go           (60 min) - Main TUI logic
17. internal/tui/theme.go         (20 min) - Styling
18. internal/tui/components.go    (30 min) - Reusable parts
```

---

## 4. Data Flow Walkthrough

### Scenario: User accesses a service

Let's trace what happens when someone accesses `http://gateway:8080/services/lectures`:

```
┌─────────────────────────────────────────────────────────────────────────┐
│ STEP 1: HTTP Request arrives at Gateway                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Browser: GET http://192.168.1.10:8080/services/lectures                │
│                           │                                             │
│                           ▼                                             │
│  internal/gateway/router.go                                             │
│  ┌────────────────────────────────────────────────────────────────┐    │
│  │ func (g *Router) ServeHTTP(w, r *http.Request) {               │    │
│  │     // 1. Apply security headers                               │    │
│  │     g.securityMiddleware(w)                                    │    │
│  │                                                                │    │
│  │     // 2. Check rate limit                                     │    │
│  │     if g.rateLimiter.Exceeded(r.RemoteAddr) { return 429 }     │    │
│  │                                                                │    │
│  │     // 3. Route to appropriate handler                         │    │
│  │     g.mux.ServeHTTP(w, r)                                      │    │
│  │ }                                                              │    │
│  └────────────────────────────────────────────────────────────────┘    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ STEP 2: Service lookup in Registry                                     │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  internal/services/handlers.go                                          │
│  ┌────────────────────────────────────────────────────────────────┐    │
│  │ func (h *Handler) ProxyRequest(w, r *http.Request) {           │    │
│  │     serviceName := chi.URLParam(r, "service") // "lectures"    │    │
│  │                                                                │    │
│  │     // Look up in registry                                     │    │
│  │     service, err := h.registry.Get(serviceName)                │    │
│  │     // Returns: { URL: "http://192.168.1.5:9000", Health: ok } │    │
│  │ }                                                              │    │
│  └────────────────────────────────────────────────────────────────┘    │
│                                                                         │
│  internal/registry/registry.go                                          │
│  ┌────────────────────────────────────────────────────────────────┐    │
│  │ func (r *Registry) Get(name string) (*Service, error) {        │    │
│  │     r.mu.RLock()                                               │    │
│  │     defer r.mu.RUnlock()                                       │    │
│  │     return r.services[name], nil  // In-memory map             │    │
│  │ }                                                              │    │
│  └────────────────────────────────────────────────────────────────┘    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
                                    │
                                    ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ STEP 3: Reverse Proxy to actual service                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  internal/services/proxy.go                                             │
│  ┌────────────────────────────────────────────────────────────────┐    │
│  │ func (p *ReverseProxy) Forward(w, r, targetURL) {              │    │
│  │     // Create reverse proxy                                    │    │
│  │     proxy := httputil.NewSingleHostReverseProxy(targetURL)     │    │
│  │                                                                │    │
│  │     // Forward request to 192.168.1.5:9000                     │    │
│  │     proxy.ServeHTTP(w, r)                                      │    │
│  │ }                                                              │    │
│  └────────────────────────────────────────────────────────────────┘    │
│                                                                         │
│  Request: GET http://192.168.1.5:9000/                                  │
│  ─────────────────────────────────────────────▶                         │
│                                                                         │
│  Response flows back through same path                                  │
│  ◀─────────────────────────────────────────────                         │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Scenario: Service registers itself

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Service startup (e.g., lecture-service on port 9000)                    │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  lecture-service/main.go                                                │
│  ┌────────────────────────────────────────────────────────────────┐    │
│  │ // 1. Start HTTP server                                        │    │
│  │ go http.ListenAndServe(":9000", handler)                       │    │
│  │                                                                │    │
│  │ // 2. Register with LocalMesh via HTTP API                     │    │
│  │ http.Post("http://gateway:8080/api/v1/services", JSON{         │    │
│  │     "name": "lectures",                                        │    │
│  │     "url": "http://192.168.1.5:9000",                          │    │
│  │     "health_check": "/health",                                 │    │
│  │     "zones": ["campus-main"]                                   │    │
│  │ })                                                             │    │
│  └────────────────────────────────────────────────────────────────┘    │
│                           │                                             │
│                           ▼                                             │
│  LocalMesh Gateway receives registration                                │
│  ┌────────────────────────────────────────────────────────────────┐    │
│  │ internal/services/handlers.go                                  │    │
│  │                                                                │    │
│  │ func (h *Handler) Register(w, r) {                             │    │
│  │     var svc ServiceRegistration                                │    │
│  │     json.Decode(r.Body, &svc)                                  │    │
│  │                                                                │    │
│  │     // Add to registry                                         │    │
│  │     h.registry.Register(svc)                                   │    │
│  │                                                                │    │
│  │     // Start health checks                                     │    │
│  │     go h.healthChecker.Monitor(svc)                            │    │
│  │ }                                                              │    │
│  └────────────────────────────────────────────────────────────────┘    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 5. Hands-On Exercises

### Exercise 1: Run and Explore (30 min)

```bash
# 1. Initialize LocalMesh
./localmesh init

# 2. Look at generated config
cat localmesh.yaml

# 3. Start in dev mode
./localmesh start --dev

# 4. In another terminal, check network identity
./localmesh network identity

# 5. Scan for nodes
./localmesh network scan

# 6. Launch TUI
./localmesh tui
# Press: Tab (switch panels), j/k (navigate), 1-6 (views), ? (help), q (quit)
```

### Exercise 2: Add a CLI Command (1 hour)

Add `localmesh ping` command that prints "pong":

```go
// In cmd/localmesh/cmd/root.go, add:

var pingCmd = &cobra.Command{
    Use:   "ping",
    Short: "Simple connectivity test",
    Run: func(cmd *cobra.Command, args []string) {
        fmt.Println("🏓 pong!")
    },
}

func init() {
    rootCmd.AddCommand(pingCmd)
}
```

### Exercise 3: Trace a Request (1 hour)

1. Start LocalMesh: `./localmesh start --dev`
2. In browser: `http://localhost:8080/api/v1/services`
3. Add print statements in `internal/services/handlers.go` to see the flow
4. Rebuild and observe

### Exercise 4: Add a TUI Keybinding (1 hour)

Add 'r' key to "refresh" data:

```go
// In internal/tui/app.go, in Update():

case "r":
    // Trigger refresh
    a.services = a.dataProvider.GetServices()
    return a, nil
```

### Exercise 5: Register a Service via API (30 min)

```bash
# Start LocalMesh
./localmesh start --dev &

# Register a fake service
curl -X POST http://localhost:8080/api/v1/services \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-test-service",
    "url": "http://localhost:9999",
    "health_check_path": "/health",
    "zones": ["campus"]
  }'

# List services
curl http://localhost:8080/api/v1/services
```

---

## 6. Building Your First Service

### Your Test: Real-Time Lecture Streaming

Here's the architecture for your demo:

```
┌─────────────────────────────────────────────────────────────────────────┐
│                          YOUR DEMO SETUP                                │
├─────────────────────────────────────────────────────────────────────────┤
│                                                                         │
│  Phone (Hotspot: "CampusWiFi")                                          │
│  ┌─────────────────────────────────────────────────────────────────┐   │
│  │  SSID: CampusWiFi    IP: 192.168.43.1                           │   │
│  └─────────────────────────────────────────────────────────────────┘   │
│              │                            │                             │
│              │                            │                             │
│       ┌──────┴──────┐              ┌──────┴──────┐                      │
│       │             │              │             │                      │
│       ▼             ▼              ▼             │                      │
│  ┌─────────┐   ┌─────────┐   ┌─────────┐        │                      │
│  │Laptop A │   │Laptop B │   │Laptop C │        │                      │
│  │LocalMesh│   │Lecture  │   │Student  │        │                      │
│  │Gateway  │   │Service  │   │Browser  │        │                      │
│  │:8080    │   │:9000    │   │         │        │                      │
│  └────┬────┘   └────┬────┘   └────┬────┘        │                      │
│       │             │             │              │                      │
│       │   Register  │             │              │                      │
│       │◀────────────┤             │              │                      │
│       │             │             │              │                      │
│       │             │   GET /lectures            │                      │
│       │◀────────────────────────────────────────┤                      │
│       │                           │              │                      │
│       │     Proxy to Laptop B     │              │                      │
│       ├───────────────────────────┤              │                      │
│       │                           │              │                      │
│       │          Response         │              │                      │
│       ├──────────────────────────────────────────▶                      │
│                                                                         │
│  KEY FEATURE: If Laptop B changes port (9000→9001),                     │
│  LocalMesh auto-updates and routes still work!                          │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Lecture Service (Separate Project)

Create a new project: `live-lecture-service/`

```go
// main.go - Minimal lecture service
package main

import (
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "bytes"
)

var (
    currentLecture string
    clients        = make(map[chan string]bool)
    mu             sync.RWMutex
)

func main() {
    // Register with LocalMesh on startup
    go registerWithLocalMesh()
    
    // SSE endpoint for real-time updates
    http.HandleFunc("/stream", streamHandler)
    
    // Teacher broadcasts lecture content
    http.HandleFunc("/broadcast", broadcastHandler)
    
    // Health check
    http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("ok"))
    })
    
    fmt.Println("Lecture service on :9000")
    http.ListenAndServe(":9000", nil)
}

func registerWithLocalMesh() {
    payload := map[string]interface{}{
        "name": "lectures",
        "url":  "http://localhost:9000",  // Will be detected
        "health_check_path": "/health",
    }
    body, _ := json.Marshal(payload)
    http.Post("http://localhost:8080/api/v1/services", 
        "application/json", 
        bytes.NewReader(body))
}
```

---

## 7. Key Insights

### Why This Architecture?

1. **Decoupling**: Services don't know about each other, only about LocalMesh
2. **Dynamic**: Services can move, scale, restart - routing updates automatically
3. **Zero Config**: mDNS means no central server to configure
4. **Network-Aware**: Access control based on physical network location

### Common Patterns in the Code

```go
// Pattern 1: Interface-based design
type Storage interface {
    Get(key string) ([]byte, error)
    Set(key string, value []byte) error
}
// Implementations: SQLite, Badger, Memory

// Pattern 2: Functional options
func NewServer(opts ...Option) *Server

// Pattern 3: Context for cancellation
func (s *Service) Start(ctx context.Context) error

// Pattern 4: Mutex for concurrent access
type Registry struct {
    mu       sync.RWMutex
    services map[string]*Service
}
```

### Questions to Ask Yourself

As you read code:
1. "What is the input to this function?"
2. "What is the output?"
3. "What side effects does it have?" (writes to DB, sends network request)
4. "What errors can occur?"
5. "How is this connected to the rest of the system?"

---

## 8. Quick Reference

### File → Purpose Map

| File | Purpose |
|------|---------|
| `cmd/localmesh/cmd/root.go` | All CLI commands |
| `internal/config/config.go` | YAML config loading |
| `internal/core/framework.go` | Main orchestration |
| `internal/gateway/router.go` | HTTP routing |
| `internal/mesh/discovery.go` | mDNS service discovery |
| `internal/registry/registry.go` | Service registry |
| `internal/network/identity.go` | WiFi → Zone detection |
| `internal/auth/service.go` | Authentication |
| `internal/storage/storage.go` | Storage interface |
| `internal/tui/app.go` | TUI main logic |

### Running Commands

```bash
# Initialize
./localmesh init

# Start (dev mode with demo plugins)
./localmesh start --dev

# TUI dashboard
./localmesh tui

# Network scan
./localmesh network scan

# Check identity
./localmesh network identity -v
```

---

**Next Steps:**
1. Read files in the order specified in Section 3
2. Run exercises in Section 5
3. Build your lecture-service demo project
4. Ask questions as you go!

Happy learning! 🚀
