// Package auth provides the RBAC (Role-Based Access Control) engine for LocalMesh.
// This implements WiFi SSID → Role mapping and permission-based access control.
package auth

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Permission represents a granular permission in the system
type Permission string

// Core permissions for LocalMesh
const (
	// Service permissions
	PermServiceRegister   Permission = "service:register"
	PermServiceUnregister Permission = "service:unregister"
	PermServiceAccess     Permission = "service:access"
	PermServiceList       Permission = "service:list"
	PermServiceAdmin      Permission = "service:admin"

	// Realm permissions
	PermRealmView     Permission = "realm:view"
	PermRealmManage   Permission = "realm:manage"
	PermRealmFederate Permission = "realm:federate"
	PermRealmTrust    Permission = "realm:trust"

	// User permissions
	PermUserCreate Permission = "user:create"
	PermUserDelete Permission = "user:delete"
	PermUserModify Permission = "user:modify"
	PermUserView   Permission = "user:view"

	// Admin permissions
	PermAdminAll    Permission = "admin:*"
	PermAdminConfig Permission = "admin:config"
	PermAdminLogs   Permission = "admin:logs"
	PermAdminAudit  Permission = "admin:audit"

	// Cross-realm permissions
	PermCrossRealmAccess  Permission = "cross-realm:access"
	PermCrossRealmSync    Permission = "cross-realm:sync"
	PermCrossRealmResolve Permission = "cross-realm:resolve"

	// Wildcard - all permissions
	PermAll Permission = "*"
)

// Role represents a role with a set of permissions
type Role struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description,omitempty"`
	Permissions []Permission `json:"permissions"`
	Inherits    []string     `json:"inherits,omitempty"` // Other role IDs to inherit from
	Priority    int          `json:"priority"`           // Higher = more restrictive checks
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// SSIDRoleMapping maps WiFi SSIDs to roles
type SSIDRoleMapping struct {
	ID          string   `json:"id"`
	SSIDs       []string `json:"ssids"`    // WiFi SSID patterns (supports wildcards)
	RoleID      string   `json:"role_id"`  // Role to assign
	Zone        string   `json:"zone"`     // Optional zone restriction
	Priority    int      `json:"priority"` // Higher = checked first
	Description string   `json:"description,omitempty"`
}

// PolicyContext provides context for policy evaluation
type PolicyContext struct {
	UserID   string            `json:"user_id"`
	Role     string            `json:"role"`
	Zone     string            `json:"zone"`
	SSID     string            `json:"ssid,omitempty"`
	IP       string            `json:"ip"`
	RealmID  string            `json:"realm_id"`
	Service  string            `json:"service,omitempty"`
	Action   string            `json:"action"`
	Resource string            `json:"resource,omitempty"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// PolicyDecision represents the result of policy evaluation
type PolicyDecision struct {
	Allowed bool              `json:"allowed"`
	Reason  string            `json:"reason"`
	Role    string            `json:"role"`
	Grants  []Permission      `json:"grants,omitempty"`
	Denies  []Permission      `json:"denies,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
}

// RBACEngine is the core RBAC policy engine
type RBACEngine struct {
	roles        map[string]*Role
	ssidMappings []SSIDRoleMapping
	defaultRole  string
	mu           sync.RWMutex
}

// NewRBACEngine creates a new RBAC engine with default roles
func NewRBACEngine() *RBACEngine {
	engine := &RBACEngine{
		roles:        make(map[string]*Role),
		ssidMappings: make([]SSIDRoleMapping, 0),
		defaultRole:  "guest",
	}

	// Initialize default roles
	engine.initDefaultRoles()
	return engine
}

// initDefaultRoles creates the built-in role hierarchy
func (e *RBACEngine) initDefaultRoles() {
	now := time.Now()

	// Guest - minimal access
	e.roles["guest"] = &Role{
		ID:          "guest",
		Name:        "Guest",
		Description: "Minimal access for unknown network connections",
		Permissions: []Permission{
			PermServiceList,
			PermRealmView,
		},
		Priority:  0,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Student - basic access
	e.roles["student"] = &Role{
		ID:          "student",
		Name:        "Student",
		Description: "Standard access for students",
		Permissions: []Permission{
			PermServiceAccess,
			PermServiceList,
			PermRealmView,
			PermUserView,
		},
		Inherits:  []string{"guest"},
		Priority:  10,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Teacher - elevated access
	e.roles["teacher"] = &Role{
		ID:          "teacher",
		Name:        "Teacher",
		Description: "Elevated access for faculty",
		Permissions: []Permission{
			PermServiceRegister,
			PermServiceUnregister,
			PermServiceAccess,
			PermServiceList,
			PermRealmView,
			PermRealmManage,
			PermUserView,
		},
		Inherits:  []string{"student"},
		Priority:  20,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Admin - full access within realm
	e.roles["admin"] = &Role{
		ID:          "admin",
		Name:        "Administrator",
		Description: "Full access within a realm",
		Permissions: []Permission{
			PermServiceAdmin,
			PermRealmManage,
			PermRealmFederate,
			PermRealmTrust,
			PermUserCreate,
			PermUserDelete,
			PermUserModify,
			PermAdminConfig,
			PermAdminLogs,
			PermAdminAudit,
			PermCrossRealmAccess,
			PermCrossRealmSync,
			PermCrossRealmResolve,
		},
		Inherits:  []string{"teacher"},
		Priority:  50,
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Super Admin - global access
	e.roles["superadmin"] = &Role{
		ID:          "superadmin",
		Name:        "Super Administrator",
		Description: "Global access across all realms",
		Permissions: []Permission{
			PermAll,
		},
		Inherits:  []string{"admin"},
		Priority:  100,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// AddRole adds or updates a role
func (e *RBACEngine) AddRole(role *Role) error {
	if role.ID == "" {
		return fmt.Errorf("role ID is required")
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	role.UpdatedAt = time.Now()
	if role.CreatedAt.IsZero() {
		role.CreatedAt = role.UpdatedAt
	}

	e.roles[role.ID] = role
	return nil
}

// GetRole retrieves a role by ID
func (e *RBACEngine) GetRole(roleID string) (*Role, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	role, ok := e.roles[roleID]
	return role, ok
}

// ListRoles returns all roles
func (e *RBACEngine) ListRoles() []*Role {
	e.mu.RLock()
	defer e.mu.RUnlock()

	roles := make([]*Role, 0, len(e.roles))
	for _, role := range e.roles {
		roles = append(roles, role)
	}
	return roles
}

// AddSSIDMapping adds a WiFi SSID to role mapping
func (e *RBACEngine) AddSSIDMapping(mapping SSIDRoleMapping) error {
	if mapping.ID == "" {
		return fmt.Errorf("mapping ID is required")
	}
	if mapping.RoleID == "" {
		return fmt.Errorf("role ID is required")
	}
	if len(mapping.SSIDs) == 0 {
		return fmt.Errorf("at least one SSID is required")
	}

	// Verify role exists
	if _, ok := e.GetRole(mapping.RoleID); !ok {
		return fmt.Errorf("role %q not found", mapping.RoleID)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Add and maintain sorted order by priority
	e.ssidMappings = append(e.ssidMappings, mapping)
	for i := len(e.ssidMappings) - 1; i > 0; i-- {
		if e.ssidMappings[i].Priority > e.ssidMappings[i-1].Priority {
			e.ssidMappings[i], e.ssidMappings[i-1] = e.ssidMappings[i-1], e.ssidMappings[i]
		}
	}

	return nil
}

// GetSSIDMappings returns all SSID mappings
func (e *RBACEngine) GetSSIDMappings() []SSIDRoleMapping {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return append([]SSIDRoleMapping(nil), e.ssidMappings...)
}

// ResolveRoleFromSSID determines the role for a given WiFi SSID
func (e *RBACEngine) ResolveRoleFromSSID(ssid, zone string) (string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, mapping := range e.ssidMappings {
		// Check zone restriction
		if mapping.Zone != "" && mapping.Zone != zone {
			continue
		}

		// Check SSID patterns
		for _, pattern := range mapping.SSIDs {
			if matchSSIDPattern(ssid, pattern) {
				return mapping.RoleID, true
			}
		}
	}

	return e.defaultRole, false
}

// GetAllPermissions returns all effective permissions for a role (including inherited)
func (e *RBACEngine) GetAllPermissions(roleID string) []Permission {
	e.mu.RLock()
	defer e.mu.RUnlock()

	seen := make(map[string]bool)
	return e.collectPermissions(roleID, seen)
}

// collectPermissions recursively collects permissions from role hierarchy
func (e *RBACEngine) collectPermissions(roleID string, seen map[string]bool) []Permission {
	if seen[roleID] {
		return nil // Prevent infinite loops
	}
	seen[roleID] = true

	role, ok := e.roles[roleID]
	if !ok {
		return nil
	}

	perms := make([]Permission, 0)

	// Collect from inherited roles first
	for _, parentID := range role.Inherits {
		perms = append(perms, e.collectPermissions(parentID, seen)...)
	}

	// Add this role's permissions
	perms = append(perms, role.Permissions...)

	return perms
}

// HasPermission checks if a role has a specific permission
func (e *RBACEngine) HasPermission(roleID string, perm Permission) bool {
	perms := e.GetAllPermissions(roleID)

	for _, p := range perms {
		if p == PermAll || p == perm {
			return true
		}

		// Check wildcard patterns (e.g., "service:*" matches "service:register")
		if strings.HasSuffix(string(p), ":*") {
			prefix := strings.TrimSuffix(string(p), "*")
			if strings.HasPrefix(string(perm), prefix) {
				return true
			}
		}
	}

	return false
}

// Evaluate evaluates a policy decision based on context
func (e *RBACEngine) Evaluate(ctx context.Context, pctx *PolicyContext) *PolicyDecision {
	decision := &PolicyDecision{
		Allowed: false,
		Role:    pctx.Role,
		Meta:    make(map[string]string),
	}

	// Resolve role from SSID if not explicitly set
	if pctx.Role == "" && pctx.SSID != "" {
		resolvedRole, found := e.ResolveRoleFromSSID(pctx.SSID, pctx.Zone)
		pctx.Role = resolvedRole
		decision.Role = resolvedRole
		if found {
			decision.Meta["role_source"] = "ssid:" + pctx.SSID
		} else {
			decision.Meta["role_source"] = "default"
		}
	}

	// Use default role if still empty
	if pctx.Role == "" {
		pctx.Role = e.defaultRole
		decision.Role = e.defaultRole
		decision.Meta["role_source"] = "default"
	}

	// Get all permissions for this role
	perms := e.GetAllPermissions(pctx.Role)
	decision.Grants = perms

	// Convert action to permission
	requiredPerm := e.actionToPermission(pctx.Action, pctx.Resource)
	if requiredPerm == "" {
		decision.Allowed = true
		decision.Reason = "no permission required for action"
		return decision
	}

	// Check if role has required permission
	if e.HasPermission(pctx.Role, requiredPerm) {
		decision.Allowed = true
		decision.Reason = fmt.Sprintf("role %q has permission %q", pctx.Role, requiredPerm)
		return decision
	}

	decision.Allowed = false
	decision.Reason = fmt.Sprintf("role %q lacks permission %q", pctx.Role, requiredPerm)
	decision.Denies = []Permission{requiredPerm}

	return decision
}

// actionToPermission maps action strings to permissions
func (e *RBACEngine) actionToPermission(action, resource string) Permission {
	// Normalize action
	action = strings.ToLower(action)
	resource = strings.ToLower(resource)

	switch action {
	// Service actions
	case "register", "service.register":
		return PermServiceRegister
	case "unregister", "service.unregister":
		return PermServiceUnregister
	case "access", "service.access":
		return PermServiceAccess
	case "list", "service.list":
		return PermServiceList
	case "service.admin":
		return PermServiceAdmin

	// Realm actions
	case "realm.view":
		return PermRealmView
	case "realm.manage":
		return PermRealmManage
	case "federate", "realm.federate":
		return PermRealmFederate
	case "trust", "realm.trust":
		return PermRealmTrust

	// User actions
	case "user.create":
		return PermUserCreate
	case "user.delete":
		return PermUserDelete
	case "user.modify":
		return PermUserModify
	case "user.view":
		return PermUserView

	// Cross-realm actions
	case "cross-realm.access":
		return PermCrossRealmAccess
	case "cross-realm.sync":
		return PermCrossRealmSync
	case "cross-realm.resolve":
		return PermCrossRealmResolve

	default:
		// Try to parse as direct permission
		if strings.Contains(action, ":") {
			return Permission(action)
		}
		return ""
	}
}

// SetDefaultRole sets the default role for unknown connections
func (e *RBACEngine) SetDefaultRole(roleID string) error {
	if _, ok := e.GetRole(roleID); !ok {
		return fmt.Errorf("role %q not found", roleID)
	}
	e.mu.Lock()
	e.defaultRole = roleID
	e.mu.Unlock()
	return nil
}

// matchSSIDPattern matches SSID against a pattern (supports * wildcard)
func matchSSIDPattern(ssid, pattern string) bool {
	if pattern == "*" {
		return true
	}

	// Case-insensitive comparison
	ssid = strings.ToLower(ssid)
	pattern = strings.ToLower(pattern)

	if !strings.Contains(pattern, "*") {
		return ssid == pattern
	}

	// Convert wildcard pattern to simple match
	parts := strings.Split(pattern, "*")
	pos := 0

	for i, part := range parts {
		if part == "" {
			continue
		}

		idx := strings.Index(ssid[pos:], part)
		if idx == -1 {
			return false
		}

		// First part must be at start if pattern doesn't start with *
		if i == 0 && pattern[0] != '*' && idx != 0 {
			return false
		}

		pos += idx + len(part)
	}

	// Last part must be at end if pattern doesn't end with *
	if len(parts) > 0 && parts[len(parts)-1] != "" && !strings.HasSuffix(pattern, "*") {
		return strings.HasSuffix(ssid, parts[len(parts)-1])
	}

	return true
}
