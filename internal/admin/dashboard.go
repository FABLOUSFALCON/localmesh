// Package admin provides HTTP dashboard handlers for the global admin.
package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/FABLOUSFALCON/localmesh/internal/auth"
)

// DashboardHandlers provides HTTP handlers for the admin dashboard.
type DashboardHandlers struct {
	admin *GlobalAdmin
}

// NewDashboardHandlers creates new dashboard handlers.
func NewDashboardHandlers(admin *GlobalAdmin) *DashboardHandlers {
	return &DashboardHandlers{admin: admin}
}

// RegisterRoutes registers the dashboard routes on a mux.
func (h *DashboardHandlers) RegisterRoutes(mux *http.ServeMux) {
	// Dashboard overview
	mux.HandleFunc("/api/admin/dashboard", h.requireAdmin(h.handleDashboard))
	mux.HandleFunc("/api/admin/stats", h.requireAdmin(h.handleStats))

	// Realm management
	mux.HandleFunc("/api/admin/realms", h.requireAdmin(h.handleRealms))
	mux.HandleFunc("/api/admin/realm/", h.requireAdmin(h.handleRealm))

	// Service overview
	mux.HandleFunc("/api/admin/services", h.requireAdmin(h.handleServices))
	mux.HandleFunc("/api/admin/service/", h.requireAdmin(h.handleService))

	// Alerts
	mux.HandleFunc("/api/admin/alerts", h.requireAdmin(h.handleAlerts))
	mux.HandleFunc("/api/admin/alert/", h.requireAdmin(h.handleAlert))

	// Policies
	mux.HandleFunc("/api/admin/policies", h.requireAdmin(h.handlePolicies))
	mux.HandleFunc("/api/admin/policy/", h.requireAdmin(h.handlePolicy))
}

// requireAdmin wraps a handler to require admin role
func (h *DashboardHandlers) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := auth.GetClaims(r.Context())
		if claims == nil {
			http.Error(w, `{"error": "unauthorized"}`, http.StatusUnauthorized)
			return
		}
		if claims.Role != "admin" && claims.Role != "superadmin" {
			http.Error(w, `{"error": "admin access required"}`, http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// DashboardResponse is the main dashboard response.
type DashboardResponse struct {
	RealmID      string       `json:"realm_id"`
	RealmName    string       `json:"realm_name"`
	Stats        *Stats       `json:"stats"`
	RecentAlerts []*Alert     `json:"recent_alerts"`
	RecentRealms []*RealmInfo `json:"recent_realms"`
	Timestamp    time.Time    `json:"timestamp"`
}

func (h *DashboardHandlers) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	response := DashboardResponse{
		RealmID:      h.admin.RealmID(),
		RealmName:    h.admin.RealmName(),
		Stats:        h.admin.GetStats(ctx),
		RecentAlerts: h.admin.ListAlerts("", "", true)[:min(5, len(h.admin.ListAlerts("", "", true)))],
		RecentRealms: h.admin.ListRealms(),
		Timestamp:    time.Now(),
	}

	h.writeJSON(w, response)
}

func (h *DashboardHandlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.admin.GetStats(r.Context())
	h.writeJSON(w, stats)
}

func (h *DashboardHandlers) handleRealms(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		realms := h.admin.ListRealms()
		h.writeJSON(w, map[string]any{
			"realms": realms,
			"count":  len(realms),
		})

	case http.MethodPost:
		var realm RealmInfo
		if err := json.NewDecoder(r.Body).Decode(&realm); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := h.admin.RegisterRealm(&realm); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
		h.writeJSON(w, map[string]any{
			"message": "realm registered",
			"realm":   realm,
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *DashboardHandlers) handleRealm(w http.ResponseWriter, r *http.Request) {
	realmID := r.URL.Path[len("/api/admin/realm/"):]
	if realmID == "" {
		http.Error(w, "realm ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		realm, ok := h.admin.GetRealm(realmID)
		if !ok {
			http.Error(w, "realm not found", http.StatusNotFound)
			return
		}

		// Get realm's services
		services := h.admin.ListServices(realmID)
		alerts := h.admin.ListAlerts(realmID, "", false)

		h.writeJSON(w, map[string]any{
			"realm":    realm,
			"services": services,
			"alerts":   alerts,
		})

	case http.MethodDelete:
		if h.admin.UnregisterRealm(realmID) {
			h.writeJSON(w, map[string]any{"message": "realm unregistered"})
		} else {
			http.Error(w, "realm not found", http.StatusNotFound)
		}

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *DashboardHandlers) handleServices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	realmID := r.URL.Query().Get("realm")
	services := h.admin.ListServices(realmID)

	// Count healthy
	healthy := 0
	for _, svc := range services {
		if svc.Healthy {
			healthy++
		}
	}

	h.writeJSON(w, map[string]any{
		"services":      services,
		"count":         len(services),
		"healthy_count": healthy,
	})
}

func (h *DashboardHandlers) handleService(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[len("/api/admin/service/"):]
	if name == "" {
		http.Error(w, "service name required", http.StatusBadRequest)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	matches := h.admin.FindService(name)
	if len(matches) == 0 {
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}

	h.writeJSON(w, map[string]any{
		"name":      name,
		"instances": matches,
		"count":     len(matches),
	})
}

func (h *DashboardHandlers) handleAlerts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		realmID := r.URL.Query().Get("realm")
		level := AlertLevel(r.URL.Query().Get("level"))
		activeOnly := r.URL.Query().Get("active") == "true"

		alerts := h.admin.ListAlerts(realmID, level, activeOnly)

		h.writeJSON(w, map[string]any{
			"alerts": alerts,
			"count":  len(alerts),
		})

	case http.MethodPost:
		var alert Alert
		if err := json.NewDecoder(r.Body).Decode(&alert); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		h.admin.FireAlert(&alert)

		w.WriteHeader(http.StatusCreated)
		h.writeJSON(w, map[string]any{
			"message": "alert created",
			"alert":   alert,
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *DashboardHandlers) handleAlert(w http.ResponseWriter, r *http.Request) {
	alertID := r.URL.Path[len("/api/admin/alert/"):]
	if alertID == "" {
		http.Error(w, "alert ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPost:
		// Acknowledge alert
		ackBy := r.URL.Query().Get("ack_by")
		if ackBy == "" {
			ackBy = "admin"
		}

		if h.admin.AckAlert(alertID, ackBy) {
			h.writeJSON(w, map[string]any{"message": "alert acknowledged"})
		} else {
			http.Error(w, "alert not found", http.StatusNotFound)
		}

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *DashboardHandlers) handlePolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		policies := h.admin.ListPolicies()
		h.writeJSON(w, map[string]any{
			"policies": policies,
			"count":    len(policies),
		})

	case http.MethodPost:
		var policy Policy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if err := h.admin.AddPolicy(&policy); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusCreated)
		h.writeJSON(w, map[string]any{
			"message": "policy created",
			"policy":  policy,
		})

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *DashboardHandlers) handlePolicy(w http.ResponseWriter, r *http.Request) {
	policyID := r.URL.Path[len("/api/admin/policy/"):]
	if policyID == "" {
		http.Error(w, "policy ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		policy, ok := h.admin.GetPolicy(policyID)
		if !ok {
			http.Error(w, "policy not found", http.StatusNotFound)
			return
		}
		h.writeJSON(w, policy)

	case http.MethodPut:
		var policy Policy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		policy.ID = policyID

		if err := h.admin.AddPolicy(&policy); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		h.writeJSON(w, map[string]any{
			"message": "policy updated",
			"policy":  policy,
		})

	case http.MethodDelete:
		if h.admin.DeletePolicy(policyID) {
			h.writeJSON(w, map[string]any{"message": "policy deleted"})
		} else {
			http.Error(w, "policy not found", http.StatusNotFound)
		}

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *DashboardHandlers) writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
