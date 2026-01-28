package auth

import (
	"encoding/json"
	"net/http"
)

type Handler struct {
	auth *Service
}

func NewHandler(auth *Service) *Handler {
	return &Handler{auth: auth}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.Refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", h.Logout)
	mux.HandleFunc("GET /api/v1/auth/me", h.Me)
	mux.HandleFunc("GET /api/v1/auth/sessions", h.Sessions)
	mux.HandleFunc("DELETE /api/v1/auth/sessions/{id}", h.RevokeSession)
	mux.HandleFunc("GET /api/v1/zones", h.ListZones)
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	req.IPAddress = getClientIP(r)
	req.UserAgent = r.UserAgent()

	resp, err := h.auth.Login(r.Context(), &req)
	if err != nil {
		if err == ErrInvalidCredentials {
			jsonError(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		if err == ErrMaxSessionsReached {
			jsonError(w, "maximum sessions reached", http.StatusTooManyRequests)
			return
		}
		jsonError(w, "login failed", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, resp, http.StatusOK)
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	resp, err := h.auth.Refresh(r.Context(), &req)
	if err != nil {
		if err == ErrTokenExpired || err == ErrInvalidToken {
			jsonError(w, "invalid or expired refresh token", http.StatusUnauthorized)
			return
		}
		if err == ErrSessionNotFound {
			jsonError(w, "session not found", http.StatusUnauthorized)
			return
		}
		jsonError(w, "refresh failed", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, resp, http.StatusOK)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if err := h.auth.Logout(r.Context(), claims.SessionID); err != nil {
		jsonError(w, "logout failed", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "logged out"}, http.StatusOK)
}

func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	user, err := h.auth.GetUser(r.Context(), claims.Subject)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	user.PasswordHash = ""

	jsonResponse(w, map[string]interface{}{
		"user":       user,
		"session_id": claims.SessionID,
		"zone":       claims.Zone,
		"zones":      claims.Zones,
		"role":       claims.Role,
		"expires_at": claims.ExpiresAt,
	}, http.StatusOK)
}

func (h *Handler) Sessions(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sessions, err := h.auth.GetUserSessions(claims.Subject)
	if err != nil {
		jsonError(w, "failed to get sessions", http.StatusInternalServerError)
		return
	}

	type sessionResponse struct {
		*Session
		Current bool `json:"current"`
	}

	var resp []sessionResponse
	for _, s := range sessions {
		resp = append(resp, sessionResponse{
			Session: s,
			Current: s.ID == claims.SessionID,
		})
	}

	jsonResponse(w, map[string]interface{}{
		"sessions": resp,
		"count":    len(resp),
	}, http.StatusOK)
}

func (h *Handler) RevokeSession(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sessionID := r.PathValue("id")
	if sessionID == "" {
		jsonError(w, "session id required", http.StatusBadRequest)
		return
	}

	session, err := h.auth.GetSession(sessionID)
	if err != nil {
		jsonError(w, "session not found", http.StatusNotFound)
		return
	}

	if session.UserID != claims.Subject && claims.Role != "admin" {
		jsonError(w, "permission denied", http.StatusForbidden)
		return
	}

	if err := h.auth.Logout(r.Context(), sessionID); err != nil {
		jsonError(w, "failed to revoke session", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"status": "session revoked"}, http.StatusOK)
}

func (h *Handler) ListZones(w http.ResponseWriter, r *http.Request) {
	zones := h.auth.GetZones()
	jsonResponse(w, map[string]interface{}{
		"zones": zones,
		"count": len(zones),
	}, http.StatusOK)
}

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
