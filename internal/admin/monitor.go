// Package admin provides cross-realm monitoring capabilities.
package admin

import (
	"context"
	"log/slog"
	"sync"
	"time"

	federationv1 "github.com/FABLOUSFALCON/localmesh/api/gen/federation/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// RealmMonitor monitors the health and status of all managed realms.
type RealmMonitor struct {
	admin           *GlobalAdmin
	checkInterval   time.Duration
	timeout         time.Duration
	logger          *slog.Logger
	clients         map[string]*realmClient
	mu              sync.RWMutex
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

type realmClient struct {
	conn   *grpc.ClientConn
	client federationv1.FederationServiceClient
}

// MonitorConfig configures the realm monitor.
type MonitorConfig struct {
	CheckInterval time.Duration
	Timeout       time.Duration
	Logger        *slog.Logger
}

// NewRealmMonitor creates a new realm monitor.
func NewRealmMonitor(admin *GlobalAdmin, cfg MonitorConfig) *RealmMonitor {
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 30 * time.Second
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 5 * time.Second
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return &RealmMonitor{
		admin:         admin,
		checkInterval: cfg.CheckInterval,
		timeout:       cfg.Timeout,
		logger:        cfg.Logger,
		clients:       make(map[string]*realmClient),
		stopCh:        make(chan struct{}),
	}
}

// Start begins the monitoring loop.
func (m *RealmMonitor) Start(ctx context.Context) {
	m.wg.Add(1)
	go m.monitorLoop(ctx)
	m.logger.Info("realm monitor started", "interval", m.checkInterval)
}

// Stop halts the monitoring loop.
func (m *RealmMonitor) Stop() {
	close(m.stopCh)
	m.wg.Wait()

	// Close all client connections
	m.mu.Lock()
	for _, client := range m.clients {
		if client.conn != nil {
			client.conn.Close()
		}
	}
	m.clients = make(map[string]*realmClient)
	m.mu.Unlock()

	m.logger.Info("realm monitor stopped")
}

func (m *RealmMonitor) monitorLoop(ctx context.Context) {
	defer m.wg.Done()

	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	// Initial check
	m.checkAllRealms(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkAllRealms(ctx)
		}
	}
}

func (m *RealmMonitor) checkAllRealms(ctx context.Context) {
	realms := m.admin.ListRealms()

	for _, realm := range realms {
		go m.checkRealm(ctx, realm)
	}
}

func (m *RealmMonitor) checkRealm(ctx context.Context, realm *RealmInfo) {
	ctx, cancel := context.WithTimeout(ctx, m.timeout)
	defer cancel()

	client, err := m.getOrCreateClient(realm)
	if err != nil {
		m.handleRealmDown(realm, err)
		return
	}

	// Send ping
	resp, err := client.Ping(ctx, &federationv1.PingRequest{
		RealmId:   m.admin.RealmID(),
		Timestamp: time.Now().Unix(),
	})

	if err != nil {
		m.handleRealmDown(realm, err)
		return
	}

	// Update realm status
	status := RealmStatusOnline
	if !resp.Healthy {
		status = RealmStatusDegraded
	}

	m.admin.UpdateRealmStatus(realm.ID, status, int(resp.ServiceCount), int(resp.PeerCount))

	m.logger.Debug("realm health check",
		"realm", realm.Name,
		"status", status,
		"services", resp.ServiceCount,
		"latency_ms", time.Now().UnixMilli()-resp.Timestamp,
	)
}

func (m *RealmMonitor) getOrCreateClient(realm *RealmInfo) (federationv1.FederationServiceClient, error) {
	m.mu.RLock()
	if client, ok := m.clients[realm.ID]; ok {
		m.mu.RUnlock()
		return client.client, nil
	}
	m.mu.RUnlock()

	// Create new connection
	conn, err := grpc.NewClient(
		realm.Endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	client := federationv1.NewFederationServiceClient(conn)

	m.mu.Lock()
	m.clients[realm.ID] = &realmClient{
		conn:   conn,
		client: client,
	}
	m.mu.Unlock()

	return client, nil
}

func (m *RealmMonitor) handleRealmDown(realm *RealmInfo, err error) {
	m.logger.Warn("realm unreachable",
		"realm", realm.Name,
		"endpoint", realm.Endpoint,
		"error", err,
	)

	// Update status
	m.admin.UpdateRealmStatus(realm.ID, RealmStatusUnreachable, 0, 0)

	// Fire alert if this is a state change
	if realm.Status != RealmStatusUnreachable {
		m.admin.FireAlert(&Alert{
			RealmID:   realm.ID,
			RealmName: realm.Name,
			Level:     AlertLevelError,
			Message:   "Realm became unreachable: " + err.Error(),
			Source:    "monitor",
			Metadata: map[string]string{
				"endpoint": realm.Endpoint,
			},
		})
	}

	// Remove stale client
	m.mu.Lock()
	if client, ok := m.clients[realm.ID]; ok {
		if client.conn != nil {
			client.conn.Close()
		}
		delete(m.clients, realm.ID)
	}
	m.mu.Unlock()
}

// SyncRealmServices fetches and updates services from a realm.
func (m *RealmMonitor) SyncRealmServices(ctx context.Context, realm *RealmInfo) error {
	client, err := m.getOrCreateClient(realm)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, m.timeout*2)
	defer cancel()

	resp, err := client.SyncServices(ctx, &federationv1.SyncRequest{
		RealmId:   m.admin.RealmID(),
		Services:  nil, // We're just requesting, not sending
		Timestamp: time.Now().Unix(),
	})

	if err != nil {
		return err
	}

	// Update services
	for _, svc := range resp.Services {
		m.admin.AddService(&ServiceInfo{
			Name:      svc.Name,
			RealmID:   realm.ID,
			RealmName: realm.Name,
			Hostname:  svc.Hostname,
			Healthy:   svc.Healthy,
			Public:    svc.Public,
			Tags:      svc.Tags,
		})
	}

	m.logger.Info("synced services from realm",
		"realm", realm.Name,
		"count", len(resp.Services),
	)

	return nil
}

// SyncAllRealms syncs services from all realms.
func (m *RealmMonitor) SyncAllRealms(ctx context.Context) {
	realms := m.admin.ListRealms()

	var wg sync.WaitGroup
	for _, realm := range realms {
		if realm.Status == RealmStatusUnreachable {
			continue
		}

		wg.Add(1)
		go func(r *RealmInfo) {
			defer wg.Done()
			if err := m.SyncRealmServices(ctx, r); err != nil {
				m.logger.Error("failed to sync realm services",
					"realm", r.Name,
					"error", err,
				)
			}
		}(realm)
	}
	wg.Wait()
}
