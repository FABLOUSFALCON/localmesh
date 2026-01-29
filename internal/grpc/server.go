// Package grpc provides the gRPC server for agent communication.
package grpc

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	agentv1 "github.com/FABLOUSFALCON/localmesh/api/gen/agent/v1"
	federationv1 "github.com/FABLOUSFALCON/localmesh/api/gen/federation/v1"
	"github.com/FABLOUSFALCON/localmesh/internal/registry"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AgentServer implements the AgentService gRPC interface.
type AgentServer struct {
	agentv1.UnimplementedAgentServiceServer

	registry *registry.MDNSRegistry
	realm    string
	services map[string]*serviceState // keyed by registration ID
	mu       sync.RWMutex
}

// serviceState tracks individual service state.
type serviceState struct {
	registrationID string
	name           string
	port           int32
	ip             string
	hostname       string
	url            string
	description    string
	healthPath     string
	tags           []string
	metadata       map[string]string
	healthy        bool
	registeredAt   time.Time
	lastHeartbeat  time.Time
}

// ServerOption configures the AgentServer.
type ServerOption func(*AgentServer)

// WithRegistry sets the mDNS registry for the server.
func WithRegistry(r *registry.MDNSRegistry) ServerOption {
	return func(s *AgentServer) {
		s.registry = r
	}
}

// WithRealm sets the realm name for hostname generation.
func WithRealm(realm string) ServerOption {
	return func(s *AgentServer) {
		s.realm = realm
	}
}

// NewAgentServer creates a new AgentServer.
func NewAgentServer(opts ...ServerOption) *AgentServer {
	s := &AgentServer{
		services: make(map[string]*serviceState),
		realm:    "campus",
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Register handles service registration from an agent.
func (s *AgentServer) Register(ctx context.Context, req *agentv1.RegisterRequest) (*agentv1.RegisterResponse, error) {
	// Validate request
	if req.Name == "" {
		return &agentv1.RegisterResponse{
			Success: false,
			Error:   "service name is required",
		}, nil
	}
	if req.Port <= 0 || req.Port > 65535 {
		return &agentv1.RegisterResponse{
			Success: false,
			Error:   "port must be between 1 and 65535",
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Generate registration ID and hostname
	registrationID := uuid.New().String()
	hostname := fmt.Sprintf("%s.%s.local", req.Name, s.realm)
	url := fmt.Sprintf("http://%s:%d", hostname, req.Port)

	// Register with mDNS if we have a registry
	if s.registry != nil {
		_, err := s.registry.Register(req.Name, int(req.Port), registry.MDNSRegisterOptions{
			IP:          req.Ip,
			Description: req.Description,
			HealthPath:  req.HealthPath,
		})
		if err != nil {
			return &agentv1.RegisterResponse{
				Success: false,
				Error:   fmt.Sprintf("failed to register with mDNS: %v", err),
			}, nil
		}
	}

	// Track service
	svc := &serviceState{
		registrationID: registrationID,
		name:           req.Name,
		port:           req.Port,
		ip:             req.Ip,
		hostname:       hostname,
		url:            url,
		description:    req.Description,
		healthPath:     req.HealthPath,
		tags:           req.Tags,
		metadata:       req.Metadata,
		healthy:        true,
		registeredAt:   time.Now(),
		lastHeartbeat:  time.Now(),
	}
	s.services[registrationID] = svc

	return &agentv1.RegisterResponse{
		Success:           true,
		Hostname:          hostname,
		Url:               url,
		RegistrationId:    registrationID,
		HeartbeatInterval: 30, // 30 seconds
	}, nil
}

// Unregister handles service deregistration from an agent.
func (s *AgentServer) Unregister(ctx context.Context, req *agentv1.UnregisterRequest) (*agentv1.UnregisterResponse, error) {
	if req.Name == "" {
		return &agentv1.UnregisterResponse{
			Success: false,
			Error:   "service name is required",
		}, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Find by name or registration ID
	var toDelete string
	for id, svc := range s.services {
		if svc.name == req.Name || id == req.RegistrationId {
			toDelete = id
			break
		}
	}

	if toDelete == "" {
		return &agentv1.UnregisterResponse{
			Success: false,
			Error:   fmt.Sprintf("service %s not found", req.Name),
		}, nil
	}

	svc := s.services[toDelete]
	delete(s.services, toDelete)

	// Unregister from mDNS
	if s.registry != nil {
		s.registry.Unregister(svc.name)
	}

	return &agentv1.UnregisterResponse{
		Success: true,
	}, nil
}

// Heartbeat handles heartbeat from an agent.
func (s *AgentServer) Heartbeat(ctx context.Context, req *agentv1.HeartbeatRequest) (*agentv1.HeartbeatResponse, error) {
	if req.Name == "" && req.RegistrationId == "" {
		return nil, status.Error(codes.InvalidArgument, "name or registration_id is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Find service
	var svc *serviceState
	for id, s := range s.services {
		if s.name == req.Name || id == req.RegistrationId {
			svc = s
			break
		}
	}

	if svc == nil {
		return &agentv1.HeartbeatResponse{
			Success:           false,
			Error:             "service not found",
			RegistrationValid: false,
			ServerTime:        time.Now().Unix(),
		}, nil
	}

	// Update state
	svc.lastHeartbeat = time.Now()
	svc.healthy = req.Healthy

	return &agentv1.HeartbeatResponse{
		Success:           true,
		RegistrationValid: true,
		ServerTime:        time.Now().Unix(),
	}, nil
}

// ListServices returns all registered services.
func (s *AgentServer) ListServices(ctx context.Context, req *agentv1.ListServicesRequest) (*agentv1.ListServicesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var services []*agentv1.ServiceInfo
	for _, svc := range s.services {
		// Filter by tags if specified
		if len(req.Tags) > 0 {
			match := false
			for _, reqTag := range req.Tags {
				for _, svcTag := range svc.tags {
					if reqTag == svcTag {
						match = true
						break
					}
				}
				if match {
					break
				}
			}
			if !match {
				continue
			}
		}

		info := &agentv1.ServiceInfo{
			Name:           svc.name,
			Port:           svc.port,
			Ip:             svc.ip,
			Hostname:       svc.hostname,
			Url:            svc.url,
			Description:    svc.description,
			Tags:           svc.tags,
			Healthy:        svc.healthy,
			RegisteredAt:   svc.registeredAt.Unix(),
			LastHeartbeat:  svc.lastHeartbeat.Unix(),
			RegistrationId: svc.registrationID,
		}
		services = append(services, info)
	}

	return &agentv1.ListServicesResponse{
		Services: services,
	}, nil
}

// GetServiceStatus returns status of a specific service.
func (s *AgentServer) GetServiceStatus(ctx context.Context, req *agentv1.GetServiceStatusRequest) (*agentv1.GetServiceStatusResponse, error) {
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "service name is required")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, svc := range s.services {
		if svc.name == req.Name {
			return &agentv1.GetServiceStatusResponse{
				Service: &agentv1.ServiceInfo{
					Name:           svc.name,
					Port:           svc.port,
					Ip:             svc.ip,
					Hostname:       svc.hostname,
					Url:            svc.url,
					Description:    svc.description,
					Tags:           svc.tags,
					Healthy:        svc.healthy,
					RegisteredAt:   svc.registeredAt.Unix(),
					LastHeartbeat:  svc.lastHeartbeat.Unix(),
					RegistrationId: svc.registrationID,
				},
			}, nil
		}
	}

	return &agentv1.GetServiceStatusResponse{
		Service: nil,
	}, nil
}

// Server wraps a gRPC server with both agent and federation services.
type Server struct {
	grpcServer       *grpc.Server
	agentServer      *AgentServer
	federationServer *FederationServer
	listener         net.Listener
	port             int
}

// ServerConfig configures the combined gRPC server.
type ServerConfig struct {
	Port      int
	RealmID   string
	RealmName string
	Endpoint  string
}

// NewServer creates a new gRPC server with agent and federation services.
func NewServer(port int, opts ...ServerOption) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", port, err)
	}

	grpcServer := grpc.NewServer()
	agentServer := NewAgentServer(opts...)

	agentv1.RegisterAgentServiceServer(grpcServer, agentServer)

	return &Server{
		grpcServer:  grpcServer,
		agentServer: agentServer,
		listener:    listener,
		port:        port,
	}, nil
}

// NewServerWithFederation creates a gRPC server with both agent and federation services.
func NewServerWithFederation(cfg ServerConfig, opts ...ServerOption) (*Server, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.Port))
	if err != nil {
		return nil, fmt.Errorf("failed to listen on port %d: %w", cfg.Port, err)
	}

	grpcServer := grpc.NewServer()
	agentServer := NewAgentServer(opts...)

	// Create federation server
	fedServer, err := NewFederationServer(FederationServerConfig{
		RealmID:   cfg.RealmID,
		RealmName: cfg.RealmName,
		Endpoint:  cfg.Endpoint,
	})
	if err != nil {
		listener.Close()
		return nil, fmt.Errorf("creating federation server: %w", err)
	}

	// Register both services
	agentv1.RegisterAgentServiceServer(grpcServer, agentServer)
	federationv1.RegisterFederationServiceServer(grpcServer, fedServer)

	return &Server{
		grpcServer:       grpcServer,
		agentServer:      agentServer,
		federationServer: fedServer,
		listener:         listener,
		port:             cfg.Port,
	}, nil
}

// Start starts the gRPC server.
func (s *Server) Start() error {
	return s.grpcServer.Serve(s.listener)
}

// Stop gracefully stops the gRPC server.
func (s *Server) Stop() {
	s.grpcServer.GracefulStop()
}

// Port returns the port the server is listening on.
func (s *Server) Port() int {
	return s.port
}

// AgentServer returns the underlying agent server.
func (s *Server) AgentServer() *AgentServer {
	return s.agentServer
}

// FederationServer returns the underlying federation server.
func (s *Server) FederationServer() *FederationServer {
	return s.federationServer
}
