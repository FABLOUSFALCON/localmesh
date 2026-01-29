// Package auth provides cross-realm authorization for federated LocalMesh deployments.
// This allows realms to share permissions and authorize users from trusted realms.
package auth

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TrustLevel defines how much a realm trusts another realm
type TrustLevel int

const (
	// TrustNone - No trust, deny all cross-realm access
	TrustNone TrustLevel = iota
	// TrustRead - Can read/list services but not access them
	TrustRead
	// TrustAccess - Can access public services
	TrustAccess
	// TrustRegister - Can register services in this realm
	TrustRegister
	// TrustFull - Full trust, treat as local users
	TrustFull
)

// RealmTrust defines trust relationship between realms
type RealmTrust struct {
	ID              string            `json:"id"`
	LocalRealmID    string            `json:"local_realm_id"`
	RemoteRealmID   string            `json:"remote_realm_id"`
	RemoteRealmName string            `json:"remote_realm_name"`
	TrustLevel      TrustLevel        `json:"trust_level"`
	Permissions     []Permission      `json:"permissions"`   // Explicit permissions granted
	RoleMapping     map[string]string `json:"role_mapping"`  // Remote role -> local role
	Bidirectional   bool              `json:"bidirectional"` // Is trust mutual?
	ExpiresAt       *time.Time        `json:"expires_at,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// CrossRealmRequest represents a request from another realm
type CrossRealmRequest struct {
	SourceRealmID   string            `json:"source_realm_id"`
	SourceRealmName string            `json:"source_realm_name"`
	UserID          string            `json:"user_id"`
	UserRole        string            `json:"user_role"`
	Zone            string            `json:"zone"`
	Action          string            `json:"action"`
	Resource        string            `json:"resource"`
	Metadata        map[string]string `json:"metadata"`
	TrustToken      string            `json:"trust_token"` // Signed by source realm
}

// CrossRealmResponse contains the authorization decision
type CrossRealmResponse struct {
	Allowed        bool              `json:"allowed"`
	Reason         string            `json:"reason"`
	MappedRole     string            `json:"mapped_role"`
	EffectivePerms []Permission      `json:"effective_permissions"`
	Expiry         *time.Time        `json:"expiry,omitempty"`
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// CrossRealmAuthorizer handles authorization for cross-realm requests
type CrossRealmAuthorizer struct {
	localRealmID string
	trusts       map[string]*RealmTrust // Remote realm ID -> trust
	rbac         *RBACEngine
	mu           sync.RWMutex
}

// NewCrossRealmAuthorizer creates a new cross-realm authorizer
func NewCrossRealmAuthorizer(localRealmID string, rbac *RBACEngine) *CrossRealmAuthorizer {
	return &CrossRealmAuthorizer{
		localRealmID: localRealmID,
		trusts:       make(map[string]*RealmTrust),
		rbac:         rbac,
	}
}

// EstablishTrust creates a trust relationship with another realm
func (cra *CrossRealmAuthorizer) EstablishTrust(trust *RealmTrust) error {
	if trust.RemoteRealmID == "" {
		return fmt.Errorf("remote realm ID is required")
	}
	if trust.LocalRealmID == "" {
		trust.LocalRealmID = cra.localRealmID
	}
	if trust.ID == "" {
		trust.ID = fmt.Sprintf("%s->%s", trust.LocalRealmID, trust.RemoteRealmID)
	}

	now := time.Now()
	trust.UpdatedAt = now
	if trust.CreatedAt.IsZero() {
		trust.CreatedAt = now
	}

	cra.mu.Lock()
	defer cra.mu.Unlock()

	cra.trusts[trust.RemoteRealmID] = trust
	return nil
}

// RevokeTrust removes a trust relationship
func (cra *CrossRealmAuthorizer) RevokeTrust(remoteRealmID string) bool {
	cra.mu.Lock()
	defer cra.mu.Unlock()

	if _, ok := cra.trusts[remoteRealmID]; ok {
		delete(cra.trusts, remoteRealmID)
		return true
	}
	return false
}

// GetTrust retrieves trust relationship for a realm
func (cra *CrossRealmAuthorizer) GetTrust(remoteRealmID string) (*RealmTrust, bool) {
	cra.mu.RLock()
	defer cra.mu.RUnlock()
	trust, ok := cra.trusts[remoteRealmID]
	return trust, ok
}

// ListTrusts returns all trust relationships
func (cra *CrossRealmAuthorizer) ListTrusts() []*RealmTrust {
	cra.mu.RLock()
	defer cra.mu.RUnlock()

	trusts := make([]*RealmTrust, 0, len(cra.trusts))
	for _, trust := range cra.trusts {
		trusts = append(trusts, trust)
	}
	return trusts
}

// Authorize checks if a cross-realm request is allowed
func (cra *CrossRealmAuthorizer) Authorize(ctx context.Context, req *CrossRealmRequest) *CrossRealmResponse {
	response := &CrossRealmResponse{
		Allowed:  false,
		Metadata: make(map[string]string),
	}

	// Check if we trust this realm
	cra.mu.RLock()
	trust, hasTrust := cra.trusts[req.SourceRealmID]
	cra.mu.RUnlock()

	if !hasTrust {
		response.Reason = fmt.Sprintf("no trust established with realm %q", req.SourceRealmID)
		return response
	}

	// Check if trust has expired
	if trust.ExpiresAt != nil && time.Now().After(*trust.ExpiresAt) {
		response.Reason = "trust relationship has expired"
		return response
	}

	// Determine effective role (map remote role to local role)
	localRole := cra.mapRole(trust, req.UserRole)
	response.MappedRole = localRole
	response.Metadata["original_role"] = req.UserRole
	response.Metadata["source_realm"] = req.SourceRealmID

	// Check trust level first
	requiredLevel := cra.actionToTrustLevel(req.Action)
	if trust.TrustLevel < requiredLevel {
		response.Reason = fmt.Sprintf("trust level %d insufficient (requires %d)", trust.TrustLevel, requiredLevel)
		return response
	}

	// Check explicit permissions if defined
	if len(trust.Permissions) > 0 {
		requiredPerm := cra.rbac.actionToPermission(req.Action, req.Resource)
		if requiredPerm != "" && !cra.hasExplicitPermission(trust, requiredPerm) {
			response.Reason = fmt.Sprintf("cross-realm permission %q not granted", requiredPerm)
			return response
		}
	}

	// Evaluate using local RBAC with mapped role
	pctx := &PolicyContext{
		UserID:   req.UserID,
		Role:     localRole,
		Zone:     req.Zone,
		RealmID:  req.SourceRealmID,
		Action:   req.Action,
		Resource: req.Resource,
		Metadata: req.Metadata,
	}

	decision := cra.rbac.Evaluate(ctx, pctx)
	response.Allowed = decision.Allowed
	response.Reason = decision.Reason
	response.EffectivePerms = decision.Grants

	if trust.ExpiresAt != nil {
		response.Expiry = trust.ExpiresAt
	}

	return response
}

// mapRole converts a remote role to a local role based on trust configuration
func (cra *CrossRealmAuthorizer) mapRole(trust *RealmTrust, remoteRole string) string {
	// Check explicit role mapping first
	if trust.RoleMapping != nil {
		if localRole, ok := trust.RoleMapping[remoteRole]; ok {
			return localRole
		}
		// Check for wildcard mapping
		if localRole, ok := trust.RoleMapping["*"]; ok {
			return localRole
		}
	}

	// Default role mapping based on trust level
	switch trust.TrustLevel {
	case TrustFull:
		// Keep the same role
		return remoteRole
	case TrustRegister:
		// Map to teacher at most
		if remoteRole == "admin" || remoteRole == "superadmin" {
			return "teacher"
		}
		return remoteRole
	case TrustAccess:
		// Map to student at most
		if remoteRole == "admin" || remoteRole == "superadmin" || remoteRole == "teacher" {
			return "student"
		}
		return remoteRole
	case TrustRead:
		// Guest access only
		return "guest"
	default:
		return "guest"
	}
}

// actionToTrustLevel determines minimum trust level required for an action
func (cra *CrossRealmAuthorizer) actionToTrustLevel(action string) TrustLevel {
	switch action {
	case "list", "service.list", "realm.view", "user.view":
		return TrustRead
	case "access", "service.access":
		return TrustAccess
	case "register", "service.register", "unregister", "service.unregister":
		return TrustRegister
	case "admin", "realm.manage", "realm.federate", "realm.trust":
		return TrustFull
	default:
		return TrustAccess
	}
}

// hasExplicitPermission checks if trust includes a specific permission
func (cra *CrossRealmAuthorizer) hasExplicitPermission(trust *RealmTrust, perm Permission) bool {
	for _, p := range trust.Permissions {
		if p == PermAll || p == perm {
			return true
		}
	}
	return false
}

// TrustLevelFromString parses a trust level from string
func TrustLevelFromString(s string) TrustLevel {
	switch s {
	case "none":
		return TrustNone
	case "read":
		return TrustRead
	case "access":
		return TrustAccess
	case "register":
		return TrustRegister
	case "full":
		return TrustFull
	default:
		return TrustNone
	}
}

// String returns the string representation of a trust level
func (tl TrustLevel) String() string {
	switch tl {
	case TrustNone:
		return "none"
	case TrustRead:
		return "read"
	case TrustAccess:
		return "access"
	case TrustRegister:
		return "register"
	case TrustFull:
		return "full"
	default:
		return "unknown"
	}
}
