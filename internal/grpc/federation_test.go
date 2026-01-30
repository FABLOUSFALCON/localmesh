package grpc

import (
	"context"
	"testing"

	federationv1 "github.com/FABLOUSFALCON/localmesh/api/gen/federation/v1"
)

func TestNewFederationServer(t *testing.T) {
	cfg := FederationServerConfig{
		RealmID:   "test-realm",
		RealmName: "Test Realm",
		Endpoint:  "localhost:9000",
	}

	server, err := NewFederationServer(cfg)
	if err != nil {
		t.Fatalf("NewFederationServer failed: %v", err)
	}

	if server == nil {
		t.Fatal("NewFederationServer returned nil")
	}

	if server.realmID != "test-realm" {
		t.Errorf("realmID = %q, want %q", server.realmID, "test-realm")
	}

	if server.realmName != "Test Realm" {
		t.Errorf("realmName = %q, want %q", server.realmName, "Test Realm")
	}

	if len(server.publicKey) == 0 {
		t.Error("publicKey not generated")
	}

	if len(server.privateKey) == 0 {
		t.Error("privateKey not generated")
	}
}

func TestNewFederationServer_AutoGenerateRealmID(t *testing.T) {
	cfg := FederationServerConfig{
		RealmName: "Test Realm",
		Endpoint:  "localhost:9000",
	}

	server, err := NewFederationServer(cfg)
	if err != nil {
		t.Fatalf("NewFederationServer failed: %v", err)
	}

	if server.realmID == "" {
		t.Error("realmID should be auto-generated")
	}
}

func TestFederationServer_JoinFederation(t *testing.T) {
	server, _ := NewFederationServer(FederationServerConfig{
		RealmID:   "main-realm",
		RealmName: "Main Realm",
		Endpoint:  "localhost:9000",
	})

	// Test successful join
	req := &federationv1.JoinRequest{
		RealmId:   "test-realm",
		RealmName: "Test Realm",
		Endpoint:  "localhost:9001",
		PublicKey: []byte("test-public-key"),
	}

	resp, err := server.JoinFederation(context.Background(), req)
	if err != nil {
		t.Fatalf("JoinFederation failed: %v", err)
	}

	if !resp.Accepted {
		t.Errorf("JoinFederation not accepted: %s", resp.Error)
	}

	if resp.FederationId == "" {
		t.Error("FederationId should be set")
	}

	if len(resp.TrustToken) == 0 {
		t.Error("TrustToken should be set")
	}

	// Verify realms list includes both realms
	if len(resp.Realms) != 2 {
		t.Errorf("expected 2 realms in response, got %d", len(resp.Realms))
	}
}

func TestFederationServer_JoinFederation_MissingFields(t *testing.T) {
	server, _ := NewFederationServer(FederationServerConfig{
		RealmID:   "main-realm",
		RealmName: "Main Realm",
		Endpoint:  "localhost:9000",
	})

	tests := []struct {
		name string
		req  *federationv1.JoinRequest
	}{
		{
			name: "missing realm_id",
			req:  &federationv1.JoinRequest{Endpoint: "localhost:9001"},
		},
		{
			name: "missing endpoint",
			req:  &federationv1.JoinRequest{RealmId: "test-realm"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := server.JoinFederation(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("JoinFederation failed: %v", err)
			}
			if resp.Accepted {
				t.Error("JoinFederation should not accept invalid request")
			}
		})
	}
}

func TestFederationServer_JoinFederation_Duplicate(t *testing.T) {
	server, _ := NewFederationServer(FederationServerConfig{
		RealmID:   "main-realm",
		RealmName: "Main Realm",
		Endpoint:  "localhost:9000",
	})

	req := &federationv1.JoinRequest{
		RealmId:   "test-realm",
		RealmName: "Test Realm",
		Endpoint:  "localhost:9001",
	}

	// First join should succeed
	resp, _ := server.JoinFederation(context.Background(), req)
	if !resp.Accepted {
		t.Error("First join should be accepted")
	}

	// Second join with same ID should fail
	resp, _ = server.JoinFederation(context.Background(), req)
	if resp.Accepted {
		t.Error("Duplicate join should be rejected")
	}
}

func TestFederationServer_LeaveFederation(t *testing.T) {
	server, _ := NewFederationServer(FederationServerConfig{
		RealmID:   "main-realm",
		RealmName: "Main Realm",
		Endpoint:  "localhost:9000",
	})

	// Join first
	joinReq := &federationv1.JoinRequest{
		RealmId:   "test-realm",
		RealmName: "Test Realm",
		Endpoint:  "localhost:9001",
	}
	server.JoinFederation(context.Background(), joinReq)

	// Now leave
	leaveReq := &federationv1.LeaveRequest{
		RealmId: "test-realm",
	}
	resp, err := server.LeaveFederation(context.Background(), leaveReq)
	if err != nil {
		t.Fatalf("LeaveFederation failed: %v", err)
	}

	if !resp.Success {
		t.Errorf("LeaveFederation should succeed: %s", resp.Error)
	}
}

func TestFederationServer_LeaveFederation_Unknown(t *testing.T) {
	server, _ := NewFederationServer(FederationServerConfig{
		RealmID:   "main-realm",
		RealmName: "Main Realm",
		Endpoint:  "localhost:9000",
	})

	leaveReq := &federationv1.LeaveRequest{
		RealmId: "unknown-realm",
	}
	resp, _ := server.LeaveFederation(context.Background(), leaveReq)

	if resp.Success {
		t.Error("LeaveFederation should fail for unknown realm")
	}
}

func TestFederationServer_Ping(t *testing.T) {
	server, _ := NewFederationServer(FederationServerConfig{
		RealmID:   "main-realm",
		RealmName: "Main Realm",
		Endpoint:  "localhost:9000",
	})

	req := &federationv1.PingRequest{
		RealmId: "test-realm",
	}

	resp, err := server.Ping(context.Background(), req)
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if resp.RealmId != "main-realm" {
		t.Errorf("RealmId = %q, want %q", resp.RealmId, "main-realm")
	}

	if resp.Status != "healthy" {
		t.Errorf("Status = %q, want %q", resp.Status, "healthy")
	}
}

func TestFederationServer_Accessors(t *testing.T) {
	server, _ := NewFederationServer(FederationServerConfig{
		RealmID:   "main-realm",
		RealmName: "Main Realm",
		Endpoint:  "localhost:9000",
	})

	if server.RealmID() != "main-realm" {
		t.Errorf("RealmID() = %q, want %q", server.RealmID(), "main-realm")
	}

	if server.RealmName() != "Main Realm" {
		t.Errorf("RealmName() = %q, want %q", server.RealmName(), "Main Realm")
	}

	if server.PublicKeyHex() == "" {
		t.Error("PublicKeyHex() should not be empty")
	}
}

func TestFederationServer_SetRBAC(t *testing.T) {
	server, _ := NewFederationServer(FederationServerConfig{
		RealmID:   "main-realm",
		RealmName: "Main Realm",
		Endpoint:  "localhost:9000",
	})

	// Initially nil
	if server.rbac != nil {
		t.Error("rbac should be nil initially")
	}

	// Set RBAC - This just tests that SetRBAC doesn't panic
	// The actual RBAC integration is tested in the auth package
	server.SetRBAC(nil)
}

func TestFederationServer_Peers(t *testing.T) {
	server, _ := NewFederationServer(FederationServerConfig{
		RealmID:   "main-realm",
		RealmName: "Main Realm",
		Endpoint:  "localhost:9000",
	})

	// Add some peers
	req1 := &federationv1.JoinRequest{RealmId: "realm-1", RealmName: "Realm 1", Endpoint: "localhost:9001"}
	req2 := &federationv1.JoinRequest{RealmId: "realm-2", RealmName: "Realm 2", Endpoint: "localhost:9002"}

	server.JoinFederation(context.Background(), req1)
	server.JoinFederation(context.Background(), req2)

	peers := server.Peers()
	if len(peers) != 2 {
		t.Errorf("Peers() returned %d peers, want 2", len(peers))
	}

	peerIDs := make(map[string]bool)
	for _, p := range peers {
		peerIDs[p] = true
	}

	if !peerIDs["realm-1"] || !peerIDs["realm-2"] {
		t.Errorf("expected peers not found: %v", peerIDs)
	}
}
