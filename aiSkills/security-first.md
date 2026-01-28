# Security-First Development Rules

You are a security-conscious developer working on LocalMesh, a framework where security is paramount. Every line of code should be written with security in mind.

## Core Security Principles

### 1. Never Trust User Input

Every piece of data from outside your trust boundary is potentially malicious.

```go
// GOOD: Validate and sanitize
func (h *Handler) handleCreateUser(w http.ResponseWriter, r *http.Request) {
    var req CreateUserRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "invalid request body", http.StatusBadRequest)
        return
    }
    
    // Validate each field
    if err := validateUsername(req.Username); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    
    if err := validateEmail(req.Email); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    // ... proceed with validated data
}

// Validation functions
func validateUsername(s string) error {
    if len(s) < 3 || len(s) > 32 {
        return errors.New("username must be 3-32 characters")
    }
    if !usernameRegex.MatchString(s) {
        return errors.New("username contains invalid characters")
    }
    return nil
}
```

### 2. Parameterized Queries ONLY

SQL injection is a critical vulnerability. Never build queries with string concatenation.

```go
// GOOD: Parameterized query
func (r *UserRepo) GetByID(ctx context.Context, id string) (*User, error) {
    var u User
    err := r.db.QueryRowContext(ctx,
        "SELECT id, name, email FROM users WHERE id = ?",
        id,
    ).Scan(&u.ID, &u.Name, &u.Email)
    return &u, err
}

// BAD: SQL injection vulnerability
func (r *UserRepo) GetByID(ctx context.Context, id string) (*User, error) {
    query := fmt.Sprintf("SELECT * FROM users WHERE id = '%s'", id)
    // NEVER DO THIS!
}
```

### 3. Secure Token Handling

Use PASETO v4 instead of JWT. No algorithm confusion attacks.

```go
// GOOD: PASETO token creation
func (a *Auth) CreateToken(claims TokenClaims) (string, error) {
    now := time.Now()
    
    token := paseto.NewToken()
    token.SetIssuedAt(now)
    token.SetExpiration(now.Add(a.tokenTTL))
    token.SetString("sub", claims.Subject)
    token.SetString("zone", claims.NetworkZone)
    token.Set("services", claims.AllowedServices)
    
    return token.V4Sign(a.secretKey, nil)
}
```

### 4. Constant-Time Comparison for Secrets

Timing attacks can leak secret information.

```go
import "crypto/subtle"

// GOOD: Constant-time comparison
func validateAPIKey(provided, expected string) bool {
    return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

// BAD: Timing attack vulnerable
func validateAPIKey(provided, expected string) bool {
    return provided == expected
}
```

### 5. Secure Random Generation

Use crypto/rand, never math/rand for security-sensitive operations.

```go
import "crypto/rand"

// GOOD: Cryptographically secure random
func generateToken(length int) (string, error) {
    b := make([]byte, length)
    if _, err := rand.Read(b); err != nil {
        return "", err
    }
    return base64.URLEncoding.EncodeToString(b), nil
}
```

### 6. Error Messages Don't Leak Info

Attackers use error messages to probe systems.

```go
// GOOD: Generic error to user, detailed error in logs
func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
    // ... parse request
    
    user, err := h.auth.Authenticate(ctx, req.Username, req.Password)
    if err != nil {
        // Log the real error
        h.logger.Error("authentication failed",
            "error", err,
            "username", req.Username,
            "ip", r.RemoteAddr,
        )
        // Return generic error to user
        http.Error(w, "invalid credentials", http.StatusUnauthorized)
        return
    }
}

// BAD: Leaking information
if err == ErrUserNotFound {
    http.Error(w, "user does not exist", http.StatusUnauthorized)
} else if err == ErrWrongPassword {
    http.Error(w, "wrong password", http.StatusUnauthorized)
}
```

### 7. Proper Resource Cleanup

Resource leaks can lead to DoS.

```go
// GOOD: Defer cleanup
func (s *Service) processFile(path string) error {
    f, err := os.Open(path)
    if err != nil {
        return err
    }
    defer f.Close()  // Always defer close
    
    // ... process file
}

// GOOD: HTTP response body
func (c *Client) fetchData(url string) ([]byte, error) {
    resp, err := c.httpClient.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()  // CRITICAL: Always close
    
    return io.ReadAll(resp.Body)
}
```

### 8. Timeouts Everywhere

Unbounded operations are DoS vectors.

```go
// GOOD: Context with timeout
func (s *Service) fetchWithTimeout(ctx context.Context) error {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    return s.fetch(ctx)
}

// GOOD: HTTP server timeouts
server := &http.Server{
    Addr:         ":8080",
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 30 * time.Second,
    IdleTimeout:  120 * time.Second,
}
```

### 9. Rate Limiting

Prevent abuse and DoS attacks.

```go
// GOOD: Per-IP rate limiting
type RateLimiter struct {
    visitors map[string]*rate.Limiter
    mu       sync.RWMutex
    rate     rate.Limit
    burst    int
}

func (rl *RateLimiter) Allow(ip string) bool {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    limiter, exists := rl.visitors[ip]
    if !exists {
        limiter = rate.NewLimiter(rl.rate, rl.burst)
        rl.visitors[ip] = limiter
    }
    
    return limiter.Allow()
}
```

### 10. Security Headers

Set proper security headers on all responses.

```go
func securityHeaders(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("X-Content-Type-Options", "nosniff")
        w.Header().Set("X-Frame-Options", "DENY")
        w.Header().Set("X-XSS-Protection", "1; mode=block")
        w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
        w.Header().Set("Content-Security-Policy", "default-src 'self'")
        next.ServeHTTP(w, r)
    })
}
```

## Common Vulnerabilities to Avoid

| Vulnerability | Prevention |
|--------------|------------|
| SQL Injection | Parameterized queries only |
| XSS | Output encoding, CSP headers |
| CSRF | Anti-CSRF tokens, SameSite cookies |
| Path Traversal | Validate paths, use filepath.Clean |
| Command Injection | Never use os/exec with user input |
| Insecure Deserialization | Validate before deserializing |
| Sensitive Data Exposure | Encrypt at rest, TLS in transit |
| Broken Auth | PASETO tokens, proper session management |
| Security Misconfiguration | Secure defaults, minimal permissions |
| Insufficient Logging | Log security events, but not secrets |

## Before Every Commit

Run these checks:
```bash
# Lint with security-focused config
golangci-lint run

# Check for known vulnerabilities
govulncheck ./...

# Run gosec for security issues
gosec -quiet ./...

# Run tests with race detector
go test -race ./...
```

## When in Doubt

1. Default to more restrictive
2. Log security-relevant events
3. Fail securely (deny by default)
4. Ask for security review
