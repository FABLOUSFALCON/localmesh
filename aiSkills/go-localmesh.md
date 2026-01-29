# LocalMesh Go Development Rules

You are an expert AI programming assistant specializing in building secure, high-performance systems with Go. You're working on LocalMesh, a campus mesh network framework for location-aware services.

## Project Context

LocalMesh is a production-grade framework that enables:
- **Location-Based Authentication** - WiFi network determines access
- **Zero Internet Dependency** - Everything runs on local mesh
- **Plugin Architecture** - Developers build services on top
- **Security First** - CVE-free by design

## Technology Stack

- **Go 1.22+** - Latest stable version
- **net/http** - Standard library HTTP (chi router allowed)
- **mDNS** - hashicorp/mdns for service discovery
- **Storage** - SQLite + Badger KV (embedded, no external deps)
- **Tokens** - PASETO v4 (preferred over JWT)
- **CLI** - cobra + bubbletea
- **Config** - viper
- **Logging** - slog (stdlib)

## Code Style & Best Practices

### General
1. Always use the latest stable Go version (1.22+)
2. Follow Go idioms and effective Go principles
3. Use meaningful variable and function names
4. Keep functions small and focused (max 50-80 lines)
5. Prefer composition over inheritance
6. Use interfaces for abstraction, but don't over-interface

### Error Handling
```go
// GOOD: Wrap errors with context
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}

// BAD: Losing error context
if err != nil {
    return err
}

// BAD: Ignoring errors
result, _ := someFunction()
```

### Security (CRITICAL)
1. **Never concatenate SQL** - Always use parameterized queries
2. **Validate all input** - At the boundary, before processing
3. **No hardcoded secrets** - Use environment variables or viper
4. **Use crypto/rand** - Never math/rand for security
5. **Time-constant comparison** - For secrets use subtle.ConstantTimeCompare
6. **PASETO over JWT** - No algorithm confusion attacks
7. **Principle of least privilege** - Minimal permissions everywhere

### Concurrency
```go
// GOOD: Use context for cancellation
func (s *Server) processRequest(ctx context.Context) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    case result := <-s.process():
        return s.handleResult(result)
    }
}

// GOOD: Use sync.WaitGroup for goroutine coordination
var wg sync.WaitGroup
for _, item := range items {
    wg.Add(1)
    go func(item Item) {
        defer wg.Done()
        process(item)
    }(item)
}
wg.Wait()
```

### HTTP Handlers
```go
// GOOD: Use http.HandlerFunc with proper error handling
func (h *Handler) handleUser(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    
    // Input validation
    userID := r.PathValue("id")
    if userID == "" {
        http.Error(w, "missing user id", http.StatusBadRequest)
        return
    }
    
    // Business logic
    user, err := h.userService.Get(ctx, userID)
    if err != nil {
        if errors.Is(err, ErrNotFound) {
            http.Error(w, "user not found", http.StatusNotFound)
            return
        }
        h.logger.Error("failed to get user", "error", err, "user_id", userID)
        http.Error(w, "internal error", http.StatusInternalServerError)
        return
    }
    
    // Response
    w.Header().Set("Content-Type", "application/json")
    if err := json.NewEncoder(w).Encode(user); err != nil {
        h.logger.Error("failed to encode response", "error", err)
    }
}
```

### Logging (slog)
```go
// GOOD: Structured logging with context
logger := slog.Default().With("component", "gateway")
logger.Info("request received", 
    "method", r.Method,
    "path", r.URL.Path,
    "remote_addr", r.RemoteAddr,
)

// GOOD: Error logging with context
logger.Error("failed to process request",
    "error", err,
    "trace_id", traceID,
)
```

### Testing
```go
// GOOD: Table-driven tests
func TestValidateZone(t *testing.T) {
    tests := []struct {
        name    string
        zone    string
        wantErr bool
    }{
        {"valid zone", "cs-department", false},
        {"empty zone", "", true},
        {"invalid chars", "zone@123", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateZone(tt.zone)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateZone() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

## Project Structure

Follow the standard Go project layout:
- `cmd/` - Main applications
- `internal/` - Private application code
- `pkg/` - Public SDK for plugin developers
- `plugins/` - Demo plugins (separate modules)

## When Writing Code

1. **Think step-by-step** - Plan the approach before coding
2. **Security first** - Consider attack vectors
3. **Write tests** - Aim for 80%+ coverage
4. **Use golangci-lint** - Run before committing
5. **Document public APIs** - GoDoc format

## Common Patterns in LocalMesh

### Network Identity Check
```go
func (a *Auth) VerifyNetworkIdentity(ctx context.Context, clientIP net.IP) (*NetworkIdentity, error) {
    // Get client's network zone from IP
    zone, err := a.zoneResolver.ResolveZone(clientIP)
    if err != nil {
        return nil, fmt.Errorf("failed to resolve zone: %w", err)
    }
    
    // Create identity
    return &NetworkIdentity{
        Zone:       zone,
        ClientIP:   clientIP,
        VerifiedAt: time.Now(),
    }, nil
}
```

### Plugin Route Registration
```go
func (r *Registry) RegisterPlugin(p sdk.Plugin) error {
    info := p.Info()
    
    // Validate plugin
    if err := r.validatePlugin(info); err != nil {
        return fmt.Errorf("invalid plugin %s: %w", info.Name, err)
    }
    
    // Register routes
    for _, route := range p.Routes() {
        path := fmt.Sprintf("/plugins/%s%s", info.Name, route.Path)
        r.mux.Handle(route.Method+" "+path, r.wrapHandler(p, route))
    }
    
    return nil
}
```

## Security Checklist

Before committing any code:
- [ ] No hardcoded secrets
- [ ] All SQL queries parameterized
- [ ] All input validated
- [ ] Errors don't leak sensitive info
- [ ] Context used for cancellation
- [ ] Resources properly closed (defer)
- [ ] No race conditions (run with -race)
- [ ] golangci-lint passes
- [ ] govulncheck clean

## Lessons Learned (Real Debugging Sessions)

### mDNS Hostname vs Service Registration

**Problem:** Needed `campus.local` to resolve to the gateway IP.

**Wrong approach:** Used `grandcat/zeroconf` - it registers services like `_http._tcp` but doesn't create A records for hostnames.

**Solution:** Use `avahi-publish-address` command:
```go
// Register hostname (A record)
cmd := exec.CommandContext(ctx, "avahi-publish-address", "-R", "campus.local", ip)
cmd.Start()
```

### Port 5353 Already in Use

**Problem:** Trying to bind to port 5353 fails because system Avahi uses it.

**Solution:** Don't bind to 5353 directly. Use `avahi-publish-address` subprocess or `hashicorp/mdns` which works through multicast.

### DNS Server Conflicts with systemd-resolved

**Problem:** Binding DNS server to `:53` conflicts with systemd-resolved.

**Solution:** Bind to specific WiFi interface IP:
```go
// Get WiFi IP first, then bind specifically
dns.ListenAndServe(wifiIP+":53", "udp", handler)
```

### Firewall Rules

LocalMesh needs these UFW rules:
```bash
sudo ufw allow 8080/tcp  # Gateway
sudo ufw allow 5353/udp  # mDNS
sudo ufw allow 53/udp    # DNS (if enabled)
```

### mDNS on Android

Android Chrome DOES support `.local` domains via mDNS. If it's not working:
1. Check you're not testing on the hotspot device itself
2. Test from a client device connected to the network
3. mDNS won't work for the device hosting the hotspot to reach itself

## mDNS/DNS Pattern for LocalMesh

```go
// The correct pattern for advertising a hostname:

// 1. Use avahi-publish-address for hostname (A record)
func advertiseHostname(ctx context.Context, hostname, ip string) error {
    cmd := exec.CommandContext(ctx, "avahi-publish-address", "-R", hostname+".local", ip)
    return cmd.Start()
}

// 2. Use hashicorp/mdns for service discovery between nodes
func discoverServices() {
    mdns.Lookup("_mesh._tcp", entriesCh)
}

// 3. Use miekg/dns for DNS server (Android/enterprise support)
func startDNSServer(wifiIP string) {
    dns.ListenAndServe(wifiIP+":53", "udp", handler)
}
```

## External Command Pattern

When running system commands:
```go
func runCommand(ctx context.Context, name string, args ...string) error {
    cmd := exec.CommandContext(ctx, name, args...)
    
    // Capture output for debugging
    var stderr bytes.Buffer
    cmd.Stderr = &stderr
    
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("%s failed: %w (stderr: %s)", name, err, stderr.String())
    }
    return nil
}
```

## Network Interface Detection

```go
// Get WiFi interface and IP
func getWiFiIP() (string, string, error) {
    ifaces, _ := net.Interfaces()
    for _, iface := range ifaces {
        // Skip loopback, docker, etc.
        if iface.Name == "lo" || strings.HasPrefix(iface.Name, "docker") {
            continue
        }
        if iface.Flags&net.FlagUp == 0 {
            continue
        }
        
        addrs, _ := iface.Addrs()
        for _, addr := range addrs {
            if ipnet, ok := addr.(*net.IPNet); ok && ipnet.IP.To4() != nil {
                return iface.Name, ipnet.IP.String(), nil
            }
        }
    }
    return "", "", errors.New("no suitable interface found")
}
