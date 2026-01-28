// Package attendance provides a demo attendance tracking plugin.
package attendance

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/FABLOUSFALCON/localmesh/pkg/sdk"
)

// Plugin is the attendance tracking plugin
type Plugin struct {
	sdk.BasePlugin

	mu       sync.RWMutex
	records  map[string][]AttendanceRecord
	sessions map[string]*Session
}

// AttendanceRecord represents a single attendance entry
type AttendanceRecord struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	SessionID string    `json:"session_id"`
	Zone      string    `json:"zone"`
	MarkedAt  time.Time `json:"marked_at"`
	Verified  bool      `json:"verified"`
}

// Session represents an active attendance session
type Session struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Course       string    `json:"course"`
	Zone         string    `json:"zone"` // Required zone for attendance
	CreatedBy    string    `json:"created_by"`
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
	Active       bool      `json:"active"`
	Participants []string  `json:"participants"`
}

// New creates a new attendance plugin instance
func New() *Plugin {
	return &Plugin{
		records:  make(map[string][]AttendanceRecord),
		sessions: make(map[string]*Session),
	}
}

// Info returns plugin metadata
func (p *Plugin) Info() sdk.PluginInfo {
	return sdk.PluginInfo{
		Name:                "attendance",
		Version:             "1.0.0",
		Description:         "Zone-based attendance tracking for classrooms and lectures",
		Author:              "LocalMesh Team",
		MinFrameworkVersion: "0.1.0",
		License:             "MIT",
	}
}

// Routes returns the HTTP routes for this plugin
func (p *Plugin) Routes() []sdk.Route {
	return []sdk.Route{
		{
			Method:      "GET",
			Path:        "/",
			Handler:     p.handleIndex,
			RequireAuth: false,
			Description: "Plugin information",
		},
		{
			Method:      "GET",
			Path:        "/sessions",
			Handler:     p.handleListSessions,
			RequireAuth: true,
			Description: "List active attendance sessions",
		},
		{
			Method:      "POST",
			Path:        "/sessions",
			Handler:     p.handleCreateSession,
			RequireAuth: true,
			Description: "Create a new attendance session",
		},
		{
			Method:      "GET",
			Path:        "/sessions/{id}",
			Handler:     p.handleGetSession,
			RequireAuth: true,
			Description: "Get session details",
		},
		{
			Method:      "POST",
			Path:        "/sessions/{id}/mark",
			Handler:     p.handleMarkAttendance,
			RequireAuth: true,
			Description: "Mark attendance for a session",
		},
		{
			Method:      "GET",
			Path:        "/sessions/{id}/records",
			Handler:     p.handleGetRecords,
			RequireAuth: true,
			Description: "Get attendance records for a session",
		},
		{
			Method:      "DELETE",
			Path:        "/sessions/{id}",
			Handler:     p.handleEndSession,
			RequireAuth: true,
			Description: "End an attendance session",
		},
	}
}

// RequiredZones returns zones that can access this plugin
// Empty means all zones (but individual sessions have zone requirements)
func (p *Plugin) RequiredZones() []string {
	return nil // Accessible from all zones, session-level zone checking
}

// Health returns the plugin health status
func (p *Plugin) Health() sdk.HealthStatus {
	p.mu.RLock()
	activeCount := 0
	for _, s := range p.sessions {
		if s.Active {
			activeCount++
		}
	}
	p.mu.RUnlock()

	return sdk.HealthStatus{
		Status:  sdk.HealthStatusHealthy,
		Message: "Plugin running",
		Details: map[string]any{
			"active_sessions": activeCount,
			"total_records":   len(p.records),
		},
	}
}

// --- Handlers ---

func (p *Plugin) handleIndex(w http.ResponseWriter, r *http.Request) {
	p.jsonResponse(w, http.StatusOK, map[string]any{
		"name":        "attendance",
		"version":     "1.0.0",
		"description": "Zone-based attendance tracking",
		"endpoints": []string{
			"GET /sessions - List active sessions",
			"POST /sessions - Create new session",
			"GET /sessions/{id} - Get session details",
			"POST /sessions/{id}/mark - Mark attendance",
			"GET /sessions/{id}/records - Get attendance records",
			"DELETE /sessions/{id} - End session",
		},
	})
}

func (p *Plugin) handleListSessions(w http.ResponseWriter, r *http.Request) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	sessions := make([]*Session, 0)
	for _, s := range p.sessions {
		if s.Active {
			sessions = append(sessions, s)
		}
	}

	p.jsonResponse(w, http.StatusOK, map[string]any{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

type CreateSessionRequest struct {
	Name     string `json:"name"`
	Course   string `json:"course"`
	Zone     string `json:"zone"`
	Duration int    `json:"duration_minutes"` // How long session is open
}

func (p *Plugin) handleCreateSession(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		p.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name == "" {
		p.jsonError(w, http.StatusBadRequest, "name is required")
		return
	}

	if req.Zone == "" {
		req.Zone = "any" // Default to any zone
	}

	if req.Duration <= 0 {
		req.Duration = 60 // Default 1 hour
	}

	// Get user from context (if authenticated)
	createdBy := "system"
	if rc, ok := sdk.GetRequestContextFromRequest(r); ok && rc != nil {
		createdBy = rc.UserID
	}

	session := &Session{
		ID:           generateID(),
		Name:         req.Name,
		Course:       req.Course,
		Zone:         req.Zone,
		CreatedBy:    createdBy,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(time.Duration(req.Duration) * time.Minute),
		Active:       true,
		Participants: []string{},
	}

	p.mu.Lock()
	p.sessions[session.ID] = session
	p.mu.Unlock()

	p.jsonResponse(w, http.StatusCreated, session)
}

func (p *Plugin) handleGetSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		p.jsonError(w, http.StatusBadRequest, "session id required")
		return
	}

	p.mu.RLock()
	session, exists := p.sessions[id]
	p.mu.RUnlock()

	if !exists {
		p.jsonError(w, http.StatusNotFound, "session not found")
		return
	}

	p.jsonResponse(w, http.StatusOK, session)
}

func (p *Plugin) handleMarkAttendance(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		p.jsonError(w, http.StatusBadRequest, "session id required")
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	session, exists := p.sessions[id]
	if !exists {
		p.jsonError(w, http.StatusNotFound, "session not found")
		return
	}

	if !session.Active {
		p.jsonError(w, http.StatusBadRequest, "session is no longer active")
		return
	}

	if time.Now().After(session.ExpiresAt) {
		session.Active = false
		p.jsonError(w, http.StatusBadRequest, "session has expired")
		return
	}

	// Get network identity from header (set by middleware)
	clientZone := r.Header.Get("X-Network-Zone")

	// Check zone requirement
	if session.Zone != "any" && session.Zone != clientZone {
		p.jsonError(w, http.StatusForbidden,
			"you must be in zone '"+session.Zone+"' to mark attendance (you are in '"+clientZone+"')")
		return
	}

	// Get user info
	userID := "anonymous"
	username := "Anonymous"
	if rc, ok := sdk.GetRequestContextFromRequest(r); ok && rc != nil {
		userID = rc.UserID
		username = rc.Username
	}

	// Check if already marked
	for _, pid := range session.Participants {
		if pid == userID {
			p.jsonError(w, http.StatusConflict, "attendance already marked")
			return
		}
	}

	// Create record
	record := AttendanceRecord{
		UserID:    userID,
		Username:  username,
		SessionID: id,
		Zone:      clientZone,
		MarkedAt:  time.Now(),
		Verified:  clientZone == session.Zone || session.Zone == "any",
	}

	p.records[id] = append(p.records[id], record)
	session.Participants = append(session.Participants, userID)

	p.jsonResponse(w, http.StatusOK, map[string]any{
		"status":   "attendance marked",
		"record":   record,
		"session":  session.Name,
		"verified": record.Verified,
	})
}

func (p *Plugin) handleGetRecords(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		p.jsonError(w, http.StatusBadRequest, "session id required")
		return
	}

	p.mu.RLock()
	session, exists := p.sessions[id]
	records := p.records[id]
	p.mu.RUnlock()

	if !exists {
		p.jsonError(w, http.StatusNotFound, "session not found")
		return
	}

	p.jsonResponse(w, http.StatusOK, map[string]any{
		"session": session,
		"records": records,
		"count":   len(records),
	})
}

func (p *Plugin) handleEndSession(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		p.jsonError(w, http.StatusBadRequest, "session id required")
		return
	}

	p.mu.Lock()
	session, exists := p.sessions[id]
	if exists {
		session.Active = false
	}
	p.mu.Unlock()

	if !exists {
		p.jsonError(w, http.StatusNotFound, "session not found")
		return
	}

	p.jsonResponse(w, http.StatusOK, map[string]any{
		"status":  "session ended",
		"session": session,
	})
}

// --- Helpers ---

func (p *Plugin) jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (p *Plugin) jsonError(w http.ResponseWriter, status int, message string) {
	p.jsonResponse(w, status, map[string]string{"error": message})
}

// Simple ID generator
func generateID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[time.Now().UnixNano()%int64(len(chars))]
		time.Sleep(time.Nanosecond)
	}
	return string(b)
}
