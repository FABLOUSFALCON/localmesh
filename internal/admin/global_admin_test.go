package admin

import (
	"context"
	"testing"
	"time"
)

func TestNewGlobalAdmin(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	if admin == nil {
		t.Fatal("NewGlobalAdmin returned nil")
	}

	if admin.realmID != "admin-realm" {
		t.Errorf("realmID = %q, want %q", admin.realmID, "admin-realm")
	}

	if admin.realmName != "Global Admin" {
		t.Errorf("realmName = %q, want %q", admin.realmName, "Global Admin")
	}
}

func TestGlobalAdmin_RegisterRealm(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	realm := &RealmInfo{
		ID:       "test-realm",
		Name:     "Test Realm",
		Endpoint: "localhost:9000",
	}

	err := admin.RegisterRealm(realm)
	if err != nil {
		t.Fatalf("RegisterRealm failed: %v", err)
	}

	// Verify realm was registered
	got, ok := admin.GetRealm("test-realm")
	if !ok {
		t.Fatal("realm not found after registration")
	}

	if got.Name != "Test Realm" {
		t.Errorf("Name = %q, want %q", got.Name, "Test Realm")
	}

	if got.Status != RealmStatusOnline {
		t.Errorf("Status = %q, want %q", got.Status, RealmStatusOnline)
	}
}

func TestGlobalAdmin_RegisterRealm_MissingID(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	realm := &RealmInfo{
		Name:     "Test Realm",
		Endpoint: "localhost:9000",
	}

	err := admin.RegisterRealm(realm)
	if err == nil {
		t.Error("RegisterRealm should fail without realm ID")
	}
}

func TestGlobalAdmin_UnregisterRealm(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	realm := &RealmInfo{
		ID:   "test-realm",
		Name: "Test Realm",
	}
	admin.RegisterRealm(realm)

	// Unregister
	ok := admin.UnregisterRealm("test-realm")
	if !ok {
		t.Error("UnregisterRealm should return true")
	}

	// Verify realm was removed
	_, ok = admin.GetRealm("test-realm")
	if ok {
		t.Error("realm should not exist after unregistration")
	}
}

func TestGlobalAdmin_UnregisterRealm_Unknown(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	ok := admin.UnregisterRealm("unknown-realm")
	if ok {
		t.Error("UnregisterRealm should return false for unknown realm")
	}
}

func TestGlobalAdmin_ListRealms(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	admin.RegisterRealm(&RealmInfo{ID: "realm-1", Name: "Realm 1"})
	admin.RegisterRealm(&RealmInfo{ID: "realm-2", Name: "Realm 2"})
	admin.RegisterRealm(&RealmInfo{ID: "realm-3", Name: "Realm 3"})

	realms := admin.ListRealms()

	if len(realms) != 3 {
		t.Errorf("ListRealms() returned %d realms, want 3", len(realms))
	}

	realmIDs := make(map[string]bool)
	for _, r := range realms {
		realmIDs[r.ID] = true
	}

	if !realmIDs["realm-1"] || !realmIDs["realm-2"] || !realmIDs["realm-3"] {
		t.Errorf("expected realms not found: %v", realmIDs)
	}
}

func TestGlobalAdmin_AddService(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	admin.RegisterRealm(&RealmInfo{ID: "test-realm", Name: "Test Realm"})

	svc := &ServiceInfo{
		Name:      "my-service",
		RealmID:   "test-realm",
		RealmName: "Test Realm",
		Hostname:  "localhost",
		Port:      8080,
		Healthy:   true,
	}

	admin.AddService(svc)

	// List services and verify
	services := admin.ListServices("test-realm")
	if len(services) != 1 {
		t.Errorf("ListServices() returned %d services, want 1", len(services))
	}

	if services[0].Name != "my-service" {
		t.Errorf("Service name = %q, want %q", services[0].Name, "my-service")
	}
}

func TestGlobalAdmin_FindService(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	admin.RegisterRealm(&RealmInfo{ID: "realm-1", Name: "Realm 1"})
	admin.RegisterRealm(&RealmInfo{ID: "realm-2", Name: "Realm 2"})

	admin.AddService(&ServiceInfo{Name: "web-server", RealmID: "realm-1", Tags: []string{"http", "public"}})
	admin.AddService(&ServiceInfo{Name: "api-server", RealmID: "realm-1", Tags: []string{"http", "api"}})
	admin.AddService(&ServiceInfo{Name: "database", RealmID: "realm-2", Tags: []string{"storage"}})

	// FindService matches exact name
	results := admin.FindService("web-server")
	if len(results) != 1 {
		t.Errorf("FindService(web-server) returned %d services, want 1", len(results))
	}

	// Find specific service
	results = admin.FindService("database")
	if len(results) != 1 {
		t.Errorf("FindService(database) returned %d services, want 1", len(results))
	}

	// Unknown service
	results = admin.FindService("unknown")
	if len(results) != 0 {
		t.Errorf("FindService(unknown) returned %d services, want 0", len(results))
	}
}

func TestGlobalAdmin_FireAlert(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	admin.RegisterRealm(&RealmInfo{ID: "test-realm", Name: "Test Realm"})

	alert := &Alert{
		RealmID: "test-realm",
		Level:   AlertLevelWarning,
		Message: "High CPU usage",
		Source:  "monitor",
	}

	admin.FireAlert(alert)

	alerts := admin.ListAlerts("", "", false)
	if len(alerts) != 1 {
		t.Errorf("ListAlerts() returned %d alerts, want 1", len(alerts))
	}

	if alerts[0].Message != "High CPU usage" {
		t.Errorf("Alert message = %q, want %q", alerts[0].Message, "High CPU usage")
	}

	if alerts[0].ID == "" {
		t.Error("Alert ID should be generated")
	}
}

func TestGlobalAdmin_AckAlert(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	alert := &Alert{
		RealmID: "test-realm",
		Level:   AlertLevelError,
		Message: "Service down",
	}
	admin.FireAlert(alert)

	alerts := admin.ListAlerts("", "", false)
	alertID := alerts[0].ID

	ok := admin.AckAlert(alertID, "admin-user")
	if !ok {
		t.Error("AckAlert should return true")
	}

	// Verify alert was acknowledged - we don't have GetAlert, so check via ListAlerts
	alerts = admin.ListAlerts("", "", false)
	found := false
	for _, a := range alerts {
		if a.ID == alertID {
			found = true
			if a.AckedAt == nil {
				t.Error("AckedAt should be set")
			}
			if a.AckedBy != "admin-user" {
				t.Errorf("AckedBy = %q, want %q", a.AckedBy, "admin-user")
			}
		}
	}
	if !found {
		t.Error("Alert not found")
	}
}

func TestGlobalAdmin_AddPolicy(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	policy := &Policy{
		ID:          "policy-001",
		Name:        "Default RBAC",
		Description: "Default RBAC policy",
		Type:        "rbac",
		Enabled:     true,
		Content: map[string]any{
			"default_role": "guest",
		},
	}

	err := admin.AddPolicy(policy)
	if err != nil {
		t.Fatalf("AddPolicy failed: %v", err)
	}

	policies := admin.ListPolicies()
	if len(policies) != 1 {
		t.Errorf("ListPolicies() returned %d policies, want 1", len(policies))
	}

	if policies[0].Version != 1 {
		t.Errorf("Version = %d, want 1", policies[0].Version)
	}
}

func TestGlobalAdmin_AddPolicy_MissingID(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	policy := &Policy{
		Name:    "Test Policy",
		Type:    "rbac",
		Enabled: true,
	}

	err := admin.AddPolicy(policy)
	if err == nil {
		t.Error("AddPolicy should fail without policy ID")
	}
}

func TestGlobalAdmin_DeletePolicy(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	policy := &Policy{
		ID:      "policy-001",
		Name:    "Test Policy",
		Type:    "network",
		Enabled: true,
	}
	err := admin.AddPolicy(policy)
	if err != nil {
		t.Fatalf("AddPolicy failed: %v", err)
	}

	ok := admin.DeletePolicy(policy.ID)
	if !ok {
		t.Error("DeletePolicy should return true")
	}

	// Verify deletion
	policies := admin.ListPolicies()
	if len(policies) != 0 {
		t.Errorf("expected 0 policies after deletion, got %d", len(policies))
	}
}

func TestGlobalAdmin_GetStats(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	admin.RegisterRealm(&RealmInfo{ID: "realm-1", Status: RealmStatusOnline})
	admin.RegisterRealm(&RealmInfo{ID: "realm-2", Status: RealmStatusOnline})
	admin.RegisterRealm(&RealmInfo{ID: "realm-3", Status: RealmStatusOffline})

	admin.AddService(&ServiceInfo{Name: "svc-1", RealmID: "realm-1", Healthy: true})
	admin.AddService(&ServiceInfo{Name: "svc-2", RealmID: "realm-1", Healthy: false})
	admin.AddService(&ServiceInfo{Name: "svc-3", RealmID: "realm-2", Healthy: true})

	admin.FireAlert(&Alert{RealmID: "realm-1", Level: AlertLevelWarning})
	admin.FireAlert(&Alert{RealmID: "realm-2", Level: AlertLevelError})

	stats := admin.GetStats(context.Background())

	if stats.TotalRealms != 3 {
		t.Errorf("TotalRealms = %d, want 3", stats.TotalRealms)
	}

	if stats.OnlineRealms != 2 {
		t.Errorf("OnlineRealms = %d, want 2", stats.OnlineRealms)
	}

	if stats.TotalServices != 3 {
		t.Errorf("TotalServices = %d, want 3", stats.TotalServices)
	}

	if stats.HealthyServices != 2 {
		t.Errorf("HealthyServices = %d, want 2", stats.HealthyServices)
	}

	if stats.TotalAlerts != 2 {
		t.Errorf("TotalAlerts = %d, want 2", stats.TotalAlerts)
	}
}

func TestRealmStatus_Values(t *testing.T) {
	tests := []struct {
		status RealmStatus
		want   string
	}{
		{RealmStatusOnline, "online"},
		{RealmStatusOffline, "offline"},
		{RealmStatusDegraded, "degraded"},
		{RealmStatusUnreachable, "unreachable"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.status) != tt.want {
				t.Errorf("RealmStatus = %q, want %q", tt.status, tt.want)
			}
		})
	}
}

func TestAlertLevel_Values(t *testing.T) {
	tests := []struct {
		level AlertLevel
		want  string
	}{
		{AlertLevelInfo, "info"},
		{AlertLevelWarning, "warning"},
		{AlertLevelError, "error"},
		{AlertLevelCritical, "critical"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if string(tt.level) != tt.want {
				t.Errorf("AlertLevel = %q, want %q", tt.level, tt.want)
			}
		})
	}
}

func TestGlobalAdmin_UpdateRealmStatus(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	admin.RegisterRealm(&RealmInfo{ID: "test-realm", Status: RealmStatusOnline})

	admin.UpdateRealmStatus("test-realm", RealmStatusDegraded, 5, 2)

	realm, _ := admin.GetRealm("test-realm")
	if realm.Status != RealmStatusDegraded {
		t.Errorf("Status = %q, want %q", realm.Status, RealmStatusDegraded)
	}
	if realm.ServiceCount != 5 {
		t.Errorf("ServiceCount = %d, want 5", realm.ServiceCount)
	}
	if realm.PeerCount != 2 {
		t.Errorf("PeerCount = %d, want 2", realm.PeerCount)
	}
}

func TestGlobalAdmin_RemoveService(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	admin.RegisterRealm(&RealmInfo{ID: "test-realm"})
	admin.AddService(&ServiceInfo{Name: "my-service", RealmID: "test-realm"})

	// Verify service exists
	services := admin.ListServices("test-realm")
	if len(services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(services))
	}

	// Remove service
	admin.RemoveService("test-realm", "my-service")

	// Verify service removed
	services = admin.ListServices("test-realm")
	if len(services) != 0 {
		t.Errorf("expected 0 services after removal, got %d", len(services))
	}
}

func TestGlobalAdmin_Timestamps(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	before := time.Now()
	admin.RegisterRealm(&RealmInfo{ID: "test-realm"})
	after := time.Now()

	realm, _ := admin.GetRealm("test-realm")

	if realm.JoinedAt.Before(before) || realm.JoinedAt.After(after) {
		t.Error("JoinedAt should be set to current time")
	}

	if realm.LastSeen.Before(before) || realm.LastSeen.After(after) {
		t.Error("LastSeen should be set to current time")
	}
}

func TestGlobalAdmin_Accessors(t *testing.T) {
	admin := NewGlobalAdmin("admin-realm", "Global Admin")

	if admin.RealmID() != "admin-realm" {
		t.Errorf("RealmID() = %q, want %q", admin.RealmID(), "admin-realm")
	}

	if admin.RealmName() != "Global Admin" {
		t.Errorf("RealmName() = %q, want %q", admin.RealmName(), "Global Admin")
	}
}
