// Package grpc provides gRPC servers for LocalMesh communication.
package grpc

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	federationv1 "github.com/FABLOUSFALCON/localmesh/api/gen/federation/v1"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/FABLOUSFALCON/localmesh/internal/auth"
)

// FederationServer implements the FederationService gRPC interface.
type FederationServer struct {
	federationv1.UnimplementedFederationServiceServer

	realmID    string
	realmName  string
	endpoint   string
	publicKey  ed25519.PublicKey
	privateKey ed25519.PrivateKey

	// Federation state
	federationID string
	peers        map[string]*peerRealm
	services     map[string]*serviceSummary // local services for sync
	mu           sync.RWMutex

	// RBAC integration
	rbac       *auth.RBACEngine
	crossRealm *auth.CrossRealmAuthorizer

	// Callbacks
	onPeerJoined  func(realm *peerRealm)
	onPeerLeft    func(realmID string)
	onServiceSync func(services []*serviceSummary)
}

// peerRealm represents a connected peer in the federation.
type peerRealm struct {
	ID           string
	Name         string
	Endpoint     string
	PublicKey    []byte
	JoinedAt     time.Time
	LastSeen     time.Time
	Status       string
	ServiceCount int32
	TrustToken   []byte
	Permissions  []string
	client       federationv1.FederationServiceClient
	conn         *grpc.ClientConn
}

// serviceSummary represents a service for federation sync.
type serviceSummary struct {
	Name         string
	Realm        string
	Hostname     string
	Healthy      bool
	Tags         []string
	Public       bool
	AllowedZones []string
	UpdatedAt    time.Time
}

// FederationServerConfig configures the federation server.
type FederationServerConfig struct {
	RealmID   string
	RealmName string
	Endpoint  string
}

// NewFederationServer creates a new FederationServer.
func NewFederationServer(cfg FederationServerConfig) (*FederationServer, error) {
	// Generate keypair for realm identity
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generating keypair: %w", err)
	}

	realmID := cfg.RealmID
	if realmID == "" {
		realmID = uuid.New().String()
	}

	return &FederationServer{
		realmID:    realmID,
		realmName:  cfg.RealmName,
		endpoint:   cfg.Endpoint,
		publicKey:  pub,
		privateKey: priv,
		peers:      make(map[string]*peerRealm),
		services:   make(map[string]*serviceSummary),
	}, nil
}

// JoinFederation handles a request from another realm to join.
func (s *FederationServer) JoinFederation(ctx context.Context, req *federationv1.JoinRequest) (*federationv1.JoinResponse, error) {
	if req.RealmId == "" || req.Endpoint == "" {
		return &federationv1.JoinResponse{
			Accepted: false,
			Error:    "realm_id and endpoint are required",
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already a peer
	if _, exists := s.peers[req.RealmId]; exists {
		return &federationv1.JoinResponse{
			Accepted: false,
			Error:    "realm already in federation",
		}, nil
	}

	// Generate federation ID if this is the first peer
	if s.federationID == "" {
		s.federationID = uuid.New().String()
	}

	// Generate trust token
	trustToken := make([]byte, 32)
	if _, err := rand.Read(trustToken); err != nil {
		return nil, status.Errorf(codes.Internal, "generating trust token: %v", err)
	}

	// Add peer
	peer := &peerRealm{
		ID:           req.RealmId,
		Name:         req.RealmName,
		Endpoint:     req.Endpoint,
		PublicKey:    req.PublicKey,
		JoinedAt:     time.Now(),
		LastSeen:     time.Now(),
		Status:       "active",
		TrustToken:   trustToken,
		ServiceCount: 0,
	}
	s.peers[req.RealmId] = peer

	// Build list of all realms for response
	var realms []*federationv1.RealmInfo
	for _, p := range s.peers {
		realms = append(realms, &federationv1.RealmInfo{
			RealmId:      p.ID,
			RealmName:    p.Name,
			Endpoint:     p.Endpoint,
			PublicKey:    p.PublicKey,
			JoinedAt:     p.JoinedAt.Unix(),
			Status:       p.Status,
			ServiceCount: p.ServiceCount,
		})
	}

	// Add ourselves
	realms = append(realms, &federationv1.RealmInfo{
		RealmId:   s.realmID,
		RealmName: s.realmName,
		Endpoint:  s.endpoint,
		PublicKey: s.publicKey,
		JoinedAt:  time.Now().Unix(),
		Status:    "active",
	})

	// Callback
	if s.onPeerJoined != nil {
		go s.onPeerJoined(peer)
	}

	return &federationv1.JoinResponse{
		Accepted:     true,
		FederationId: s.federationID,
		Realms:       realms,
		TrustToken:   trustToken,
	}, nil
}

// LeaveFederation handles a realm leaving the federation.
func (s *FederationServer) LeaveFederation(ctx context.Context, req *federationv1.LeaveRequest) (*federationv1.LeaveResponse, error) {
	if req.RealmId == "" {
		return &federationv1.LeaveResponse{
			Success: false,
			Error:   "realm_id is required",
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	peer, exists := s.peers[req.RealmId]
	if !exists {
		return &federationv1.LeaveResponse{
			Success: false,
			Error:   "realm not in federation",
		}, nil
	}

	// Close connection if any
	if peer.conn != nil {
		peer.conn.Close()
	}

	delete(s.peers, req.RealmId)

	// Callback
	if s.onPeerLeft != nil {
		go s.onPeerLeft(req.RealmId)
	}

	return &federationv1.LeaveResponse{
		Success: true,
	}, nil
}

// SyncServices exchanges service catalogs with a peer.
func (s *FederationServer) SyncServices(ctx context.Context, req *federationv1.SyncRequest) (*federationv1.SyncResponse, error) {
	if req.RealmId == "" {
		return &federationv1.SyncResponse{
			Success: false,
			Error:   "realm_id is required",
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify peer exists
	peer, exists := s.peers[req.RealmId]
	if !exists {
		return &federationv1.SyncResponse{
			Success: false,
			Error:   "unknown realm - join federation first",
		}, nil
	}

	// Update peer's last seen
	peer.LastSeen = time.Now()
	peer.ServiceCount = int32(len(req.Services))

	// Store received services (with realm prefix)
	for _, svc := range req.Services {
		key := fmt.Sprintf("%s/%s", req.RealmId, svc.Name)
		s.services[key] = &serviceSummary{
			Name:         svc.Name,
			Realm:        req.RealmId,
			Hostname:     svc.Hostname,
			Healthy:      svc.Healthy,
			Tags:         svc.Tags,
			Public:       svc.Public,
			AllowedZones: svc.AllowedZones,
			UpdatedAt:    time.Unix(svc.UpdatedAt, 0),
		}
	}

	// Return our local services
	var localServices []*federationv1.ServiceSummary
	for _, svc := range s.services {
		if svc.Realm == s.realmID || svc.Realm == "" {
			localServices = append(localServices, &federationv1.ServiceSummary{
				Name:         svc.Name,
				Realm:        s.realmID,
				Hostname:     svc.Hostname,
				Healthy:      svc.Healthy,
				Tags:         svc.Tags,
				Public:       svc.Public,
				AllowedZones: svc.AllowedZones,
				UpdatedAt:    svc.UpdatedAt.Unix(),
			})
		}
	}

	return &federationv1.SyncResponse{
		Success:  true,
		Services: localServices,
		SyncTime: time.Now().Unix(),
	}, nil
}

// ResolveService looks up a service in the federation.
func (s *FederationServer) ResolveService(ctx context.Context, req *federationv1.ResolveRequest) (*federationv1.ResolveResponse, error) {
	if req.ServiceName == "" {
		return &federationv1.ResolveResponse{
			Found: false,
			Error: "service_name is required",
		}, nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Look for service in our catalog (local or federated)
	for key, svc := range s.services {
		if svc.Name == req.ServiceName || key == req.ServiceName {
			// Check access control
			if !svc.Public && len(svc.AllowedZones) > 0 {
				allowed := false
				for _, zone := range svc.AllowedZones {
					if zone == req.Zone {
						allowed = true
						break
					}
				}
				if !allowed {
					return &federationv1.ResolveResponse{
						Found: false,
						Error: "access denied: zone not allowed",
					}, nil
				}
			}

			// If service is in another realm, we might need to proxy
			directAccess := svc.Realm == s.realmID || svc.Realm == ""
			var proxyEndpoint string
			if !directAccess {
				if peer, ok := s.peers[svc.Realm]; ok {
					proxyEndpoint = peer.Endpoint
				}
			}

			return &federationv1.ResolveResponse{
				Found:         true,
				Hostname:      svc.Hostname,
				DirectAccess:  directAccess,
				ProxyEndpoint: proxyEndpoint,
			}, nil
		}
	}

	// Service not found locally, query federated peers
	for _, peer := range s.peers {
		if peer.client == nil {
			continue
		}

		// Forward resolve request to peer
		resp, err := peer.client.ResolveService(ctx, req)
		if err == nil && resp.Found {
			return resp, nil
		}
	}

	return &federationv1.ResolveResponse{
		Found: false,
		Error: "service not found in federation",
	}, nil
}

// ExchangeTrust establishes trust between realms.
func (s *FederationServer) ExchangeTrust(ctx context.Context, req *federationv1.TrustRequest) (*federationv1.TrustResponse, error) {
	if req.RealmId == "" {
		return &federationv1.TrustResponse{
			Accepted: false,
			Error:    "realm_id is required",
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	peer, exists := s.peers[req.RealmId]
	if !exists {
		return &federationv1.TrustResponse{
			Accepted: false,
			Error:    "realm not in federation",
		}, nil
	}

	// For now, grant all requested permissions
	// In production, this would involve policy checks
	peer.Permissions = req.RequestedPermissions
	peer.PublicKey = req.PublicKey

	return &federationv1.TrustResponse{
		Accepted:           true,
		GrantedPermissions: req.RequestedPermissions,
		ExpiresAt:          time.Now().Add(24 * time.Hour).Unix(),
	}, nil
}

// Ping checks if this realm is alive.
func (s *FederationServer) Ping(ctx context.Context, req *federationv1.PingRequest) (*federationv1.PingResponse, error) {
	return &federationv1.PingResponse{
		RealmId:   s.realmID,
		Timestamp: time.Now().Unix(),
		Status:    "healthy",
	}, nil
}

// --- Client-side methods for initiating federation ---

// JoinPeer attempts to join another realm's federation.
func (s *FederationServer) JoinPeer(ctx context.Context, peerEndpoint string) error {
	// Connect to peer
	conn, err := grpc.DialContext(ctx, peerEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("connecting to peer %s: %w", peerEndpoint, err)
	}

	client := federationv1.NewFederationServiceClient(conn)

	// Send join request
	resp, err := client.JoinFederation(ctx, &federationv1.JoinRequest{
		RealmId:         s.realmID,
		RealmName:       s.realmName,
		Endpoint:        s.endpoint,
		PublicKey:       s.publicKey,
		ProtocolVersion: "1.0",
	})
	if err != nil {
		conn.Close()
		return fmt.Errorf("join request failed: %w", err)
	}

	if !resp.Accepted {
		conn.Close()
		return fmt.Errorf("join rejected: %s", resp.Error)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.federationID = resp.FederationId

	// Store peer info from response
	for _, realm := range resp.Realms {
		if realm.RealmId == s.realmID {
			continue // Skip ourselves
		}

		s.peers[realm.RealmId] = &peerRealm{
			ID:           realm.RealmId,
			Name:         realm.RealmName,
			Endpoint:     realm.Endpoint,
			PublicKey:    realm.PublicKey,
			JoinedAt:     time.Unix(realm.JoinedAt, 0),
			LastSeen:     time.Now(),
			Status:       realm.Status,
			ServiceCount: realm.ServiceCount,
		}
	}

	// Keep connection to the peer we joined through
	// Find peer by endpoint
	for _, p := range s.peers {
		if p.Endpoint == peerEndpoint {
			p.conn = conn
			p.client = client
			p.TrustToken = resp.TrustToken
			break
		}
	}

	return nil
}

// SyncWithPeer syncs services with a specific peer.
func (s *FederationServer) SyncWithPeer(ctx context.Context, peerID string) error {
	s.mu.RLock()
	peer, exists := s.peers[peerID]
	if !exists {
		s.mu.RUnlock()
		return fmt.Errorf("peer %s not found", peerID)
	}

	if peer.client == nil {
		s.mu.RUnlock()
		return fmt.Errorf("no connection to peer %s", peerID)
	}

	// Collect local services
	var localServices []*federationv1.ServiceSummary
	for _, svc := range s.services {
		if svc.Realm == s.realmID || svc.Realm == "" {
			localServices = append(localServices, &federationv1.ServiceSummary{
				Name:         svc.Name,
				Realm:        s.realmID,
				Hostname:     svc.Hostname,
				Healthy:      svc.Healthy,
				Tags:         svc.Tags,
				Public:       svc.Public,
				AllowedZones: svc.AllowedZones,
				UpdatedAt:    svc.UpdatedAt.Unix(),
			})
		}
	}
	s.mu.RUnlock()

	// Send sync request
	resp, err := peer.client.SyncServices(ctx, &federationv1.SyncRequest{
		RealmId:  s.realmID,
		Services: localServices,
		LastSync: time.Now().Add(-5 * time.Minute).Unix(),
	})
	if err != nil {
		return fmt.Errorf("sync failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("sync rejected: %s", resp.Error)
	}

	// Store received services
	s.mu.Lock()
	for _, svc := range resp.Services {
		key := fmt.Sprintf("%s/%s", svc.Realm, svc.Name)
		s.services[key] = &serviceSummary{
			Name:         svc.Name,
			Realm:        svc.Realm,
			Hostname:     svc.Hostname,
			Healthy:      svc.Healthy,
			Tags:         svc.Tags,
			Public:       svc.Public,
			AllowedZones: svc.AllowedZones,
			UpdatedAt:    time.Unix(svc.UpdatedAt, 0),
		}
	}
	s.mu.Unlock()

	return nil
}

// --- Accessors ---

// RealmID returns this realm's ID.
func (s *FederationServer) RealmID() string {
	return s.realmID
}

// RealmName returns this realm's name.
func (s *FederationServer) RealmName() string {
	return s.realmName
}

// FederationID returns the federation ID if joined.
func (s *FederationServer) FederationID() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.federationID
}

// Peers returns the list of connected peers.
func (s *FederationServer) Peers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ids []string
	for id := range s.peers {
		ids = append(ids, id)
	}
	return ids
}

// PublicKeyHex returns the realm's public key as hex string.
func (s *FederationServer) PublicKeyHex() string {
	return hex.EncodeToString(s.publicKey)
}

// AddLocalService adds a service to the local catalog for federation sync.
func (s *FederationServer) AddLocalService(name, hostname string, healthy, public bool, tags, allowedZones []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.services[name] = &serviceSummary{
		Name:         name,
		Realm:        s.realmID,
		Hostname:     hostname,
		Healthy:      healthy,
		Public:       public,
		Tags:         tags,
		AllowedZones: allowedZones,
		UpdatedAt:    time.Now(),
	}
}

// RemoveLocalService removes a service from the local catalog.
func (s *FederationServer) RemoveLocalService(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.services, name)
}

// SetRBAC sets the RBAC engine for permission checking.
func (s *FederationServer) SetRBAC(rbac *auth.RBACEngine) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rbac = rbac
	s.crossRealm = auth.NewCrossRealmAuthorizer(s.realmID, rbac)
}

// GetCrossRealmAuthorizer returns the cross-realm authorizer.
func (s *FederationServer) GetCrossRealmAuthorizer() *auth.CrossRealmAuthorizer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.crossRealm
}

// EstablishTrust creates a trust relationship with a peer realm.
func (s *FederationServer) EstablishTrust(remoteRealmID string, level auth.TrustLevel, permissions []auth.Permission) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.crossRealm == nil {
		return fmt.Errorf("RBAC not initialized")
	}

	peer, exists := s.peers[remoteRealmID]
	if !exists {
		return fmt.Errorf("realm %q is not a federation peer", remoteRealmID)
	}

	trust := &auth.RealmTrust{
		LocalRealmID:    s.realmID,
		RemoteRealmID:   remoteRealmID,
		RemoteRealmName: peer.Name,
		TrustLevel:      level,
		Permissions:     permissions,
	}

	return s.crossRealm.EstablishTrust(trust)
}

// AuthorizeCrossRealmRequest checks if a cross-realm request is allowed.
func (s *FederationServer) AuthorizeCrossRealmRequest(ctx context.Context, sourceRealmID, userRole, action, resource string) (*auth.CrossRealmResponse, error) {
	s.mu.RLock()
	cra := s.crossRealm
	s.mu.RUnlock()

	if cra == nil {
		return nil, fmt.Errorf("RBAC not initialized")
	}

	req := &auth.CrossRealmRequest{
		SourceRealmID: sourceRealmID,
		UserRole:      userRole,
		Action:        action,
		Resource:      resource,
	}

	return cra.Authorize(ctx, req), nil
}
