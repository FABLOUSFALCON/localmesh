package auth

import (
	"context"
	"testing"
)

func TestNewRBACEngine(t *testing.T) {
	engine := NewRBACEngine()

	if engine == nil {
		t.Fatal("NewRBACEngine returned nil")
	}

	// Check default roles exist
	expectedRoles := []string{"guest", "student", "teacher", "admin", "superadmin"}
	for _, roleID := range expectedRoles {
		role, ok := engine.GetRole(roleID)
		if !ok {
			t.Errorf("default role %q not found", roleID)
		}
		if role.ID != roleID {
			t.Errorf("role ID mismatch: got %q, want %q", role.ID, roleID)
		}
	}
}

func TestRBACEngine_HasPermission(t *testing.T) {
	engine := NewRBACEngine()

	tests := []struct {
		name       string
		roleID     string
		permission Permission
		want       bool
	}{
		{"guest can list services", "guest", PermServiceList, true},
		{"guest can view realm", "guest", PermRealmView, true},
		{"guest cannot register services", "guest", PermServiceRegister, false},
		{"student can access services", "student", PermServiceAccess, true},
		{"student cannot register services", "student", PermServiceRegister, false},
		{"teacher can register services", "teacher", PermServiceRegister, true},
		{"teacher can unregister services", "teacher", PermServiceUnregister, true},
		{"admin can federate", "admin", PermRealmFederate, true},
		{"admin can manage users", "admin", PermUserCreate, true},
		{"superadmin has all permissions", "superadmin", PermServiceRegister, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := engine.HasPermission(tt.roleID, tt.permission)
			if got != tt.want {
				t.Errorf("HasPermission(%q, %q) = %v, want %v", tt.roleID, tt.permission, got, tt.want)
			}
		})
	}
}

func TestRBACEngine_SSIDMapping(t *testing.T) {
	engine := NewRBACEngine()

	err := engine.AddSSIDMapping(SSIDRoleMapping{
		ID:       "faculty-wifi",
		SSIDs:    []string{"CSE-Faculty", "CSE-Faculty-5G"},
		RoleID:   "teacher",
		Priority: 10,
	})
	if err != nil {
		t.Fatalf("AddSSIDMapping failed: %v", err)
	}

	err = engine.AddSSIDMapping(SSIDRoleMapping{
		ID:       "student-wifi",
		SSIDs:    []string{"CSE-Students", "CSE-Lab*"},
		RoleID:   "student",
		Priority: 5,
	})
	if err != nil {
		t.Fatalf("AddSSIDMapping failed: %v", err)
	}

	tests := []struct {
		ssid         string
		expectedRole string
		shouldMatch  bool
	}{
		{"CSE-Faculty", "teacher", true},
		{"CSE-Faculty-5G", "teacher", true},
		{"CSE-Students", "student", true},
		{"CSE-Lab-101", "student", true},
		{"Unknown-WiFi", "guest", false},
	}

	for _, tt := range tests {
		t.Run(tt.ssid, func(t *testing.T) {
			role, matched := engine.ResolveRoleFromSSID(tt.ssid, "")
			if role != tt.expectedRole {
				t.Errorf("ResolveRoleFromSSID(%q) = %q, want %q", tt.ssid, role, tt.expectedRole)
			}
			if matched != tt.shouldMatch {
				t.Errorf("matched = %v, want %v", matched, tt.shouldMatch)
			}
		})
	}
}

func TestRBACEngine_Evaluate(t *testing.T) {
	engine := NewRBACEngine()

	engine.AddSSIDMapping(SSIDRoleMapping{
		ID:     "faculty",
		SSIDs:  []string{"Faculty-WiFi"},
		RoleID: "teacher",
	})

	tests := []struct {
		name    string
		ctx     *PolicyContext
		allowed bool
	}{
		{
			name:    "student can access service",
			ctx:     &PolicyContext{Role: "student", Action: "service.access"},
			allowed: true,
		},
		{
			name:    "student cannot register service",
			ctx:     &PolicyContext{Role: "student", Action: "service.register"},
			allowed: false,
		},
		{
			name:    "teacher can register service",
			ctx:     &PolicyContext{Role: "teacher", Action: "register"},
			allowed: true,
		},
		{
			name:    "role from SSID - faculty wifi",
			ctx:     &PolicyContext{SSID: "Faculty-WiFi", Action: "register"},
			allowed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decision := engine.Evaluate(context.Background(), tt.ctx)
			if decision.Allowed != tt.allowed {
				t.Errorf("Evaluate() allowed = %v, want %v (reason: %s)", decision.Allowed, tt.allowed, decision.Reason)
			}
		})
	}
}

func TestMatchSSIDPattern(t *testing.T) {
	tests := []struct {
		ssid    string
		pattern string
		want    bool
	}{
		{"CSE-Faculty", "CSE-Faculty", true},
		{"CSE-Faculty", "cse-faculty", true},
		{"CSE-Lab-101", "CSE-Lab*", true},
		{"CSE-Lab-Advanced", "CSE-Lab*", true},
		{"Faculty-Lab", "CSE-Lab*", false},
		{"Anything", "*", true},
		{"CSE-Guest", "CSE-Faculty", false},
	}

	for _, tt := range tests {
		t.Run(tt.ssid+"_"+tt.pattern, func(t *testing.T) {
			got := matchSSIDPattern(tt.ssid, tt.pattern)
			if got != tt.want {
				t.Errorf("matchSSIDPattern(%q, %q) = %v, want %v", tt.ssid, tt.pattern, got, tt.want)
			}
		})
	}
}
