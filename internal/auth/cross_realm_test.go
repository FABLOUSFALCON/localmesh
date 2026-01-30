package auth

import (
	"context"
	"testing"
)

func TestNewCrossRealmAuthorizer(t *testing.T) {
	rbac := NewRBACEngine()
	auth := NewCrossRealmAuthorizer("realm-001", rbac)

	if auth == nil {
		t.Fatal("NewCrossRealmAuthorizer returned nil")
	}
}

func TestCrossRealmAuthorizer_EstablishTrust(t *testing.T) {
	rbac := NewRBACEngine()
	auth := NewCrossRealmAuthorizer("realm-001", rbac)

	trust := &RealmTrust{
		RemoteRealmID:   "realm-002",
		RemoteRealmName: "Remote Realm",
		TrustLevel:      TrustAccess,
	}

	err := auth.EstablishTrust(trust)
	if err != nil {
		t.Fatalf("EstablishTrust failed: %v", err)
	}

	got, ok := auth.GetTrust("realm-002")
	if !ok {
		t.Fatal("trust not found after EstablishTrust")
	}

	if got.TrustLevel != TrustAccess {
		t.Errorf("TrustLevel = %v, want %v", got.TrustLevel, TrustAccess)
	}

	if got.RemoteRealmName != "Remote Realm" {
		t.Errorf("RemoteRealmName = %q, want %q", got.RemoteRealmName, "Remote Realm")
	}
}

func TestCrossRealmAuthorizer_TrustLevels(t *testing.T) {
	rbac := NewRBACEngine()
	auth := NewCrossRealmAuthorizer("realm-001", rbac)

	// Set up realms with different trust levels
	trusts := []*RealmTrust{
		{RemoteRealmID: "read-only", TrustLevel: TrustRead},
		{RemoteRealmID: "access", TrustLevel: TrustAccess},
		{RemoteRealmID: "register", TrustLevel: TrustRegister},
		{RemoteRealmID: "full", TrustLevel: TrustFull},
		{RemoteRealmID: "none", TrustLevel: TrustNone},
	}

	for _, trust := range trusts {
		if err := auth.EstablishTrust(trust); err != nil {
			t.Fatalf("EstablishTrust failed: %v", err)
		}
	}

	tests := []struct {
		realmID    string
		action     string
		userRole   string
		wantAccess bool
	}{
		// Read-only realm - can only read/list, not access
		{"read-only", "service.list", "student", true},

		// Access realm - can access services
		{"access", "service.list", "student", true},
		{"access", "service.access", "student", true},

		// Register realm - can register services (with teacher role)
		{"register", "service.list", "teacher", true},
		{"register", "service.access", "teacher", true},
		{"register", "service.register", "teacher", true},

		// Full trust realm - can do everything (with teacher role for register)
		{"full", "service.list", "student", true},
		{"full", "service.access", "student", true},
		{"full", "service.register", "teacher", true},

		// Unknown realm - no trust established
		{"unknown", "service.list", "student", false},
	}

	for _, tt := range tests {
		t.Run(tt.realmID+"_"+tt.action, func(t *testing.T) {
			req := &CrossRealmRequest{
				SourceRealmID: tt.realmID,
				Action:        tt.action,
				UserRole:      tt.userRole,
			}
			decision := auth.Authorize(context.Background(), req)
			if decision.Allowed != tt.wantAccess {
				t.Errorf("Authorize(%q, %q) = %v, want %v (reason: %s)",
					tt.realmID, tt.action, decision.Allowed, tt.wantAccess, decision.Reason)
			}
		})
	}
}

func TestCrossRealmAuthorizer_RoleMapping(t *testing.T) {
	rbac := NewRBACEngine()
	auth := NewCrossRealmAuthorizer("realm-001", rbac)

	// Establish trust with role mapping
	trust := &RealmTrust{
		RemoteRealmID: "realm-002",
		TrustLevel:    TrustAccess,
		RoleMapping: map[string]string{
			"professor": "teacher",
			"staff":     "admin",
		},
	}
	if err := auth.EstablishTrust(trust); err != nil {
		t.Fatalf("EstablishTrust failed: %v", err)
	}

	// Test that trust was established
	got, ok := auth.GetTrust("realm-002")
	if !ok {
		t.Fatal("trust not found")
	}

	// Check role mapping was preserved
	if got.RoleMapping["professor"] != "teacher" {
		t.Errorf("RoleMapping[professor] = %q, want teacher", got.RoleMapping["professor"])
	}
	if got.RoleMapping["staff"] != "admin" {
		t.Errorf("RoleMapping[staff] = %q, want admin", got.RoleMapping["staff"])
	}
}

func TestCrossRealmAuthorizer_RevokeTrust(t *testing.T) {
	rbac := NewRBACEngine()
	auth := NewCrossRealmAuthorizer("realm-001", rbac)

	trust := &RealmTrust{
		RemoteRealmID: "realm-002",
		TrustLevel:    TrustFull,
	}
	if err := auth.EstablishTrust(trust); err != nil {
		t.Fatalf("EstablishTrust failed: %v", err)
	}

	_, ok := auth.GetTrust("realm-002")
	if !ok {
		t.Fatal("trust should exist before revoke")
	}

	revoked := auth.RevokeTrust("realm-002")
	if !revoked {
		t.Error("RevokeTrust should return true for existing trust")
	}

	_, ok = auth.GetTrust("realm-002")
	if ok {
		t.Error("trust should not exist after revoke")
	}
}

func TestCrossRealmAuthorizer_ListTrusts(t *testing.T) {
	rbac := NewRBACEngine()
	auth := NewCrossRealmAuthorizer("realm-001", rbac)

	trust1 := &RealmTrust{RemoteRealmID: "realm-002", TrustLevel: TrustRead}
	trust2 := &RealmTrust{RemoteRealmID: "realm-003", TrustLevel: TrustAccess}

	auth.EstablishTrust(trust1)
	auth.EstablishTrust(trust2)

	trusts := auth.ListTrusts()

	if len(trusts) != 2 {
		t.Errorf("ListTrusts() returned %d trusts, want 2", len(trusts))
	}

	foundRealms := make(map[string]bool)
	for _, r := range trusts {
		foundRealms[r.RemoteRealmID] = true
	}

	if !foundRealms["realm-002"] || !foundRealms["realm-003"] {
		t.Errorf("expected realms not found: %v", foundRealms)
	}
}

func TestTrustLevelValues(t *testing.T) {
	// Test that trust levels have correct ordering
	if TrustNone >= TrustRead {
		t.Error("TrustNone should be less than TrustRead")
	}
	if TrustRead >= TrustAccess {
		t.Error("TrustRead should be less than TrustAccess")
	}
	if TrustAccess >= TrustRegister {
		t.Error("TrustAccess should be less than TrustRegister")
	}
	if TrustRegister >= TrustFull {
		t.Error("TrustRegister should be less than TrustFull")
	}
}
