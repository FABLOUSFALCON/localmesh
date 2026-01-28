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
