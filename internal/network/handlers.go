// Package network provides HTTP handlers for network identity.
package network

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler provides HTTP handlers for network identity endpoints
type Handler struct {
	service *Service
}

// NewHandler creates a new network handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers network routes on a mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/network/identity", h.GetLocalIdentity)
	mux.HandleFunc("GET /api/v1/network/identity/{ip}", h.GetIdentityByIP)
	mux.HandleFunc("POST /api/v1/network/verify", h.VerifyIdentity)
	mux.HandleFunc("GET /api/v1/network/mappings", h.GetMappings)
	mux.HandleFunc("POST /api/v1/network/mappings", h.AddMapping)
	mux.HandleFunc("POST /api/v1/network/refresh", h.RefreshIdentity)
}

// GetLocalIdentity returns the local node's network identity
func (h *Handler) GetLocalIdentity(w http.ResponseWriter, r *http.Request) {
	identity := h.service.GetLocalIdentity()
	if identity == nil {
		jsonError(w, "network identity not detected", http.StatusServiceUnavailable)
		return
	}
	jsonResponse(w, identity, http.StatusOK)
}

// GetIdentityByIP returns network identity for a specific IP
func (h *Handler) GetIdentityByIP(w http.ResponseWriter, r *http.Request) {
	ip := r.PathValue("ip")
	if ip == "" {
		jsonError(w, "IP address required", http.StatusBadRequest)
		return
	}

	identity, err := h.service.DetectIdentity(r.Context(), ip)
	if err != nil {
		jsonError(w, "detection failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, identity, http.StatusOK)
}

// VerifyRequest is the request body for verification
type VerifyRequestBody struct {
	ClientIP     string `json:"client_ip"`
	ClaimedZone  string `json:"claimed_zone"`
	ClaimedSSID  string `json:"claimed_ssid"`
	ClaimedBSSID string `json:"claimed_bssid"`
}

// VerifyIdentity verifies claimed network identity
func (h *Handler) VerifyIdentity(w http.ResponseWriter, r *http.Request) {
	var body VerifyRequestBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// If no client IP provided, use the request's remote address
	if body.ClientIP == "" {
		body.ClientIP = getClientIP(r)
	}

	result, err := h.service.VerifyIdentity(r.Context(), VerifyRequest{
		ClientIP:     body.ClientIP,
		ClaimedZone:  body.ClaimedZone,
		ClaimedSSID:  body.ClaimedSSID,
		ClaimedBSSID: body.ClaimedBSSID,
	})
	if err != nil {
		jsonError(w, "verification failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, result, http.StatusOK)
}

// GetMappings returns all zone mappings
func (h *Handler) GetMappings(w http.ResponseWriter, r *http.Request) {
	mappings := h.service.GetZoneMappings()
	jsonResponse(w, map[string]interface{}{
		"mappings": mappings,
		"count":    len(mappings),
	}, http.StatusOK)
}

// AddMappingRequest is the request body for adding a mapping
type AddMappingRequest struct {
	ID          string   `json:"id"`
	Zone        string   `json:"zone"`
	SSIDs       []string `json:"ssids,omitempty"`
	Subnets     []string `json:"subnets,omitempty"`
	BSSIDs      []string `json:"bssids,omitempty"`
	Description string   `json:"description,omitempty"`
	Priority    int      `json:"priority"`
}

// AddMapping adds a new zone mapping
func (h *Handler) AddMapping(w http.ResponseWriter, r *http.Request) {
	var req AddMappingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Zone == "" {
		jsonError(w, "zone is required", http.StatusBadRequest)
		return
	}

	h.service.AddZoneMapping(ZoneMapping{
		ID:          req.ID,
		Zone:        req.Zone,
		SSIDs:       req.SSIDs,
		Subnets:     req.Subnets,
		BSSIDs:      req.BSSIDs,
		Description: req.Description,
		Priority:    req.Priority,
	})

	jsonResponse(w, map[string]string{"status": "mapping added"}, http.StatusCreated)
}

// RefreshIdentity forces a refresh of local identity
func (h *Handler) RefreshIdentity(w http.ResponseWriter, r *http.Request) {
	identity, err := h.service.RefreshLocalIdentity(r.Context())
	if err != nil {
		jsonError(w, "refresh failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, identity, http.StatusOK)
}

// Helper functions

func jsonResponse(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}

	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return strings.Trim(ip, "[]")
}
