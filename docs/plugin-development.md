# Plugin Development Guide

This guide explains how to build plugins for LocalMesh.

## Overview

Plugins are self-contained services that run on top of the LocalMesh framework. The framework handles:

- Service discovery (mDNS)
- Routing (HTTP gateway)
- Authentication & Authorization
- Storage
- Cloud sync

Your plugin focuses on **business logic**.

## Plugin Structure

```
my-plugin/
├── go.mod
├── main.go           # Entry point
├── handlers.go       # HTTP handlers
├── models.go         # Data models
├── storage.go        # Storage operations
└── ui/               # Frontend (optional)
    ├── index.html
    ├── styles.css
    └── app.js
```

## Creating a Plugin

### 1. Scaffold a New Plugin

```bash
localmesh plugin scaffold my-plugin
```

This generates a basic plugin structure.

### 2. Implement the Plugin Interface

```go
package main

import (
    "context"
    "net/http"

    "github.com/FABLOUSFALCON/localmesh/pkg/sdk"
)

type MyPlugin struct {
    sdk.BasePlugin
}

// Info returns plugin metadata
func (p *MyPlugin) Info() sdk.PluginInfo {
    return sdk.PluginInfo{
        Name:        "my-plugin",
        Version:     "1.0.0",
        Description: "My awesome plugin",
        Author:      "Your Name",
    }
}

// Init is called when the plugin loads
func (p *MyPlugin) Init(ctx context.Context, cfg sdk.PluginConfig) error {
    // Call base init
    if err := p.BasePlugin.Init(ctx, cfg); err != nil {
        return err
    }
    
    // Your initialization code
    cfg.Logger.Info("my-plugin initialized")
    return nil
}

// Routes defines HTTP endpoints
func (p *MyPlugin) Routes() []sdk.Route {
    return []sdk.Route{
        {
            Method:      "GET",
            Path:        "/",
            Handler:     p.handleIndex,
            RequireAuth: false,
            Description: "Plugin home page",
        },
        {
            Method:      "GET",
            Path:        "/data",
            Handler:     p.handleGetData,
            RequireAuth: true,
            Description: "Get user data",
        },
        {
            Method:      "POST",
            Path:        "/data",
            Handler:     p.handlePostData,
            RequireAuth: true,
            AllowedZones: []string{"cs-department"},
            Description: "Submit data (CS only)",
        },
    }
}

// RequiredZones defines which zones can access this plugin
func (p *MyPlugin) RequiredZones() []string {
    return []string{"general"}  // Accessible from general zone
}

// Handlers
func (p *MyPlugin) handleIndex(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("Welcome to My Plugin!"))
}

func (p *MyPlugin) handleGetData(w http.ResponseWriter, r *http.Request) {
    // Get request context with network info
    ctx, ok := sdk.GetRequestContextFromRequest(r)
    if !ok {
        http.Error(w, "no context", http.StatusInternalServerError)
        return
    }
    
    // Use context info
    p.Config().Logger.Info("request from zone", "zone", ctx.NetworkZone)
    
    w.Write([]byte("Your data here"))
}

func (p *MyPlugin) handlePostData(w http.ResponseWriter, r *http.Request) {
    // This endpoint is only accessible from cs-department zone
    // Framework enforces this automatically
    w.Write([]byte("Data saved"))
}

// Export the plugin
var Plugin MyPlugin
```

### 3. Build the Plugin

```bash
cd plugins/my-plugin
go build -buildmode=plugin -o my-plugin.so
```

### 4. Install and Enable

```bash
localmesh plugin install ./plugins/my-plugin
localmesh plugin enable my-plugin
```

## Using the Request Context

Every request includes network identity information:

```go
func (p *MyPlugin) handleRequest(w http.ResponseWriter, r *http.Request) {
    ctx, ok := sdk.GetRequestContextFromRequest(r)
    if !ok {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }
    
    // Network info
    zone := ctx.NetworkZone      // "cs-department"
    network := ctx.NetworkID     // "CS-DEPT-WIFI"
    clientIP := ctx.ClientIP     // "10.10.5.42"
    verified := ctx.IsVerified   // true
    
    // User info (if authenticated)
    userID := ctx.UserID         // "user-123"
    roles := ctx.Roles           // ["student"]
    
    // Tracing
    traceID := ctx.TraceID       // "abc123..."
}
```

## Using Storage

Plugins get isolated storage:

```go
func (p *MyPlugin) Init(ctx context.Context, cfg sdk.PluginConfig) error {
    // Store data
    err := cfg.Storage.Set(ctx, "key", []byte("value"))
    
    // Retrieve data
    data, err := cfg.Storage.Get(ctx, "key")
    
    // List keys
    keys, err := cfg.Storage.List(ctx, "prefix:")
    
    // Transaction
    err = cfg.Storage.Transaction(ctx, func(tx sdk.StorageTx) error {
        val, _ := tx.Get("counter")
        // ... modify val
        return tx.Set("counter", newVal)
    })
    
    return nil
}
```

## Zone-Based Access Control

Define access at plugin or route level:

```go
// Plugin-level: all routes require "general" zone
func (p *MyPlugin) RequiredZones() []string {
    return []string{"general"}
}

// Route-level: override for specific routes
func (p *MyPlugin) Routes() []sdk.Route {
    return []sdk.Route{
        {
            Path:         "/public",
            AllowedZones: []string{},  // Empty = inherit from plugin
        },
        {
            Path:         "/admin",
            AllowedZones: []string{"admin-zone"},  // Override
        },
    }
}
```

## Emitting Events

Plugins can emit events for other plugins to consume:

```go
// In your handler
func (p *MyPlugin) handleAttendanceMarked(w http.ResponseWriter, r *http.Request) {
    // ... mark attendance
    
    // Emit event
    p.emitter.Emit(r.Context(), "attendance.marked", map[string]any{
        "student_id": studentID,
        "class_id":   classID,
        "timestamp":  time.Now().Unix(),
    })
}
```

## Health Checks

Implement custom health checks:

```go
func (p *MyPlugin) Health() sdk.HealthStatus {
    // Check dependencies
    if !p.dbHealthy() {
        return sdk.HealthStatus{
            Status:  sdk.HealthStatusUnhealthy,
            Message: "database connection failed",
        }
    }
    
    return sdk.HealthStatus{
        Status: sdk.HealthStatusHealthy,
        Details: map[string]any{
            "connections": p.activeConnections,
            "uptime":      p.uptime(),
        },
    }
}
```

## Frontend Assets

Serve static files from the `ui/` directory:

```go
func (p *MyPlugin) Routes() []sdk.Route {
    return []sdk.Route{
        {
            Method:  "GET",
            Path:    "/ui/*",
            Handler: http.StripPrefix("/plugins/my-plugin/ui/", 
                http.FileServer(http.Dir("ui/"))).ServeHTTP,
        },
    }
}
```

## Testing

```go
// plugin_test.go
package main

import (
    "testing"
    "net/http/httptest"
)

func TestHandleIndex(t *testing.T) {
    p := &MyPlugin{}
    
    req := httptest.NewRequest("GET", "/", nil)
    w := httptest.NewRecorder()
    
    p.handleIndex(w, req)
    
    if w.Code != 200 {
        t.Errorf("expected 200, got %d", w.Code)
    }
}
```

## Best Practices

1. **Never trust input** - Validate everything
2. **Use parameterized queries** - Never concatenate SQL
3. **Log appropriately** - Use structured logging
4. **Handle errors** - Don't ignore errors, log and respond appropriately
5. **Keep handlers small** - Extract logic to helper functions
6. **Write tests** - Aim for 80%+ coverage

## Example Plugins

See the `plugins/` directory for complete examples:

- `attendance/` - Location-verified attendance marking
- `lecture/` - Real-time lecture broadcasting
- `notices/` - Announcement board
