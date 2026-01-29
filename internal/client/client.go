// Package client provides a gRPC client for connecting to LocalMesh servers.
package client

import (
	"context"
	"fmt"
	"time"

	agentv1 "github.com/FABLOUSFALCON/localmesh/api/gen/agent/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps the gRPC client for the AgentService.
type Client struct {
	conn    *grpc.ClientConn
	service agentv1.AgentServiceClient
	timeout time.Duration
}

// Options configures the client.
type Options struct {
	// ServerAddr is the address of the LocalMesh server (host:port).
	ServerAddr string
	// AgentID is a unique identifier for this agent (unused in current proto, but kept for future).
	AgentID string
	// Timeout for RPC calls.
	Timeout time.Duration
}

// New creates a new gRPC client.
func New(opts Options) (*Client, error) {
	if opts.Timeout == 0 {
		opts.Timeout = 10 * time.Second
	}

	ctx, cancel := context.WithTimeout(context.Background(), opts.Timeout)
	defer cancel()

	// Connect to the server
	// TODO: Add TLS support
	conn, err := grpc.DialContext(ctx, opts.ServerAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to %s: %w", opts.ServerAddr, err)
	}

	return &Client{
		conn:    conn,
		service: agentv1.NewAgentServiceClient(conn),
		timeout: opts.Timeout,
	}, nil
}

// Close closes the client connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// RegisterOptions contains service registration parameters.
type RegisterOptions struct {
	Name           string
	Port           int32
	IP             string
	HealthEndpoint string
	Description    string
	Tags           []string
	Metadata       map[string]string
}

// RegisterResult contains the registration result.
type RegisterResult struct {
	Success           bool
	Hostname          string
	URL               string
	RegistrationID    string
	HeartbeatInterval int32
	Error             string
}

// Register registers a service with the LocalMesh server.
func (c *Client) Register(ctx context.Context, opts RegisterOptions) (*RegisterResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req := &agentv1.RegisterRequest{
		Name:        opts.Name,
		Port:        opts.Port,
		Ip:          opts.IP,
		Description: opts.Description,
		HealthPath:  opts.HealthEndpoint,
		Tags:        opts.Tags,
		Metadata:    opts.Metadata,
	}

	resp, err := c.service.Register(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("registration failed: %w", err)
	}

	return &RegisterResult{
		Success:           resp.Success,
		Hostname:          resp.Hostname,
		URL:               resp.Url,
		RegistrationID:    resp.RegistrationId,
		HeartbeatInterval: resp.HeartbeatInterval,
		Error:             resp.Error,
	}, nil
}

// Unregister unregisters a service.
func (c *Client) Unregister(ctx context.Context, name string, registrationID string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req := &agentv1.UnregisterRequest{
		Name:           name,
		RegistrationId: registrationID,
	}

	resp, err := c.service.Unregister(ctx, req)
	if err != nil {
		return fmt.Errorf("unregister failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("unregister failed: %s", resp.Error)
	}

	return nil
}

// ServiceStatus represents the status of a service.
type ServiceStatus struct {
	Name           string
	Hostname       string
	Port           int32
	IP             string
	URL            string
	Description    string
	Tags           []string
	Healthy        bool
	RegisteredAt   time.Time
	LastHeartbeat  time.Time
	RegistrationID string
}

// SendHeartbeat sends a heartbeat to the server.
func (c *Client) SendHeartbeat(ctx context.Context, name, registrationID string, healthy bool, statusMessage string) (*HeartbeatResult, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req := &agentv1.HeartbeatRequest{
		Name:           name,
		RegistrationId: registrationID,
		Healthy:        healthy,
		StatusMessage:  statusMessage,
	}

	resp, err := c.service.Heartbeat(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("heartbeat failed: %w", err)
	}

	return &HeartbeatResult{
		Success:           resp.Success,
		RegistrationValid: resp.RegistrationValid,
		ServerTime:        time.Unix(resp.ServerTime, 0),
		Error:             resp.Error,
	}, nil
}

// HeartbeatResult contains the heartbeat response.
type HeartbeatResult struct {
	Success           bool
	RegistrationValid bool
	ServerTime        time.Time
	Error             string
}

// ListServices lists all services registered on the server.
func (c *Client) ListServices(ctx context.Context, tags []string) ([]ServiceStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req := &agentv1.ListServicesRequest{
		Tags: tags,
	}

	resp, err := c.service.ListServices(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("list services failed: %w", err)
	}

	services := make([]ServiceStatus, len(resp.Services))
	for i, svc := range resp.Services {
		services[i] = ServiceStatus{
			Name:           svc.Name,
			Hostname:       svc.Hostname,
			Port:           svc.Port,
			IP:             svc.Ip,
			URL:            svc.Url,
			Description:    svc.Description,
			Tags:           svc.Tags,
			Healthy:        svc.Healthy,
			RegisteredAt:   time.Unix(svc.RegisteredAt, 0),
			LastHeartbeat:  time.Unix(svc.LastHeartbeat, 0),
			RegistrationID: svc.RegistrationId,
		}
	}

	return services, nil
}

// GetServiceStatus gets the status of a specific service.
func (c *Client) GetServiceStatus(ctx context.Context, name string) (*ServiceStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	req := &agentv1.GetServiceStatusRequest{
		Name: name,
	}

	resp, err := c.service.GetServiceStatus(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("get service status failed: %w", err)
	}

	if resp.Service == nil {
		return nil, nil
	}

	svc := resp.Service
	return &ServiceStatus{
		Name:           svc.Name,
		Hostname:       svc.Hostname,
		Port:           svc.Port,
		IP:             svc.Ip,
		URL:            svc.Url,
		Description:    svc.Description,
		Tags:           svc.Tags,
		Healthy:        svc.Healthy,
		RegisteredAt:   time.Unix(svc.RegisteredAt, 0),
		LastHeartbeat:  time.Unix(svc.LastHeartbeat, 0),
		RegistrationID: svc.RegistrationId,
	}, nil
}
