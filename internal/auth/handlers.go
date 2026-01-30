package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type Handler struct {
	auth *Service
}

func NewHandler(auth *Service) *Handler {
	return &Handler{auth: auth}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	// Auth endpoints
	mux.HandleFunc("POST /api/v1/auth/login", h.Login)
	mux.HandleFunc("POST /api/v1/auth/register", h.Register)
	mux.HandleFunc("POST /api/v1/auth/refresh", h.Refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", h.Logout)
	mux.HandleFunc("GET /api/v1/auth/me", h.Me)
	mux.HandleFunc("GET /api/v1/auth/sessions", h.Sessions)
	mux.HandleFunc("DELETE /api/v1/auth/sessions/{id}", h.RevokeSession)
	mux.HandleFunc("PUT /api/v1/auth/password", h.ChangePassword)

	// Zone endpoints
	mux.HandleFunc("GET /api/v1/zones", h.ListZones)

	// User management endpoints (admin only)
	mux.HandleFunc("GET /api/v1/users", h.ListUsers)
	mux.HandleFunc("POST /api/v1/users", h.CreateUser)
	mux.HandleFunc("GET /api/v1/users/{id}", h.GetUser)
	mux.HandleFunc("PUT /api/v1/users/{id}", h.UpdateUser)
	mux.HandleFunc("DELETE /api/v1/users/{id}", h.DeleteUser)
	mux.HandleFunc("PUT /api/v1/users/{id}/password", h.ResetUserPassword)

	// Role management endpoints
	mux.HandleFunc("GET /api/v1/roles", h.ListRoles)
	mux.HandleFunc("GET /api/v1/roles/{name}", h.GetRole)
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

// Register handles user self-registration
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Username == "" || req.Password == "" {
		jsonError(w, "username and password are required", http.StatusBadRequest)
		return
	}

	if len(req.Password) < 8 {
		jsonError(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Check if username already exists
	exists, err := h.auth.UserExists(r.Context(), req.Username)
	if err != nil {
		jsonError(w, "failed to check username", http.StatusInternalServerError)
		return
	}
	if exists {
		jsonError(w, "username already taken", http.StatusConflict)
		return
	}

	// Create user with default role
	user := &User{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Role:        "user", // Default role for self-registration
		Zone:        "default",
	}

	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}

	if err := h.auth.CreateUser(r.Context(), user, req.Password); err != nil {
		jsonError(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	user.PasswordHash = "" // Don't expose hash

	jsonResponse(w, map[string]interface{}{
		"message": "user registered successfully",
		"user":    user,
	}, http.StatusCreated)
}

// ChangePassword allows a user to change their own password
func (h *Handler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 8 {
		jsonError(w, "new password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Verify current password
	user, err := h.auth.GetUser(r.Context(), claims.Subject)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	if !VerifyPassword(req.CurrentPassword, user.PasswordHash) {
		jsonError(w, "current password is incorrect", http.StatusUnauthorized)
		return
	}

	// Update password
	if err := h.auth.UpdatePassword(r.Context(), claims.Subject, req.NewPassword); err != nil {
		jsonError(w, "failed to update password", http.StatusInternalServerError)
		return
	}

	// Optionally logout all other sessions
	h.auth.LogoutAll(r.Context(), claims.Subject)

	jsonResponse(w, map[string]string{"message": "password changed successfully"}, http.StatusOK)
}

// ListUsers returns all users (admin only)
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims == nil || (claims.Role != "admin" && claims.Role != "superadmin") {
		jsonError(w, "admin access required", http.StatusForbidden)
		return
	}

	limit := 50
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := parseInt(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := parseInt(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	users, err := h.auth.ListUsers(r.Context(), limit, offset)
	if err != nil {
		jsonError(w, "failed to list users", http.StatusInternalServerError)
		return
	}

	// Strip password hashes
	for _, u := range users {
		u.PasswordHash = ""
	}

	count, _ := h.auth.UserCount(r.Context())

	jsonResponse(w, map[string]interface{}{
		"users":  users,
		"count":  len(users),
		"total":  count,
		"limit":  limit,
		"offset": offset,
	}, http.StatusOK)
}

// CreateUser creates a new user (admin only)
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims == nil || (claims.Role != "admin" && claims.Role != "superadmin") {
		jsonError(w, "admin access required", http.StatusForbidden)
		return
	}

	var req struct {
		Username    string `json:"username"`
		Password    string `json:"password"`
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		Zone        string `json:"zone"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Username == "" || req.Password == "" {
		jsonError(w, "username and password are required", http.StatusBadRequest)
		return
	}

	// Check if username exists
	exists, _ := h.auth.UserExists(r.Context(), req.Username)
	if exists {
		jsonError(w, "username already taken", http.StatusConflict)
		return
	}

	user := &User{
		Username:    req.Username,
		DisplayName: req.DisplayName,
		Email:       req.Email,
		Role:        req.Role,
		Zone:        req.Zone,
	}

	if user.DisplayName == "" {
		user.DisplayName = user.Username
	}
	if user.Role == "" {
		user.Role = "user"
	}
	if user.Zone == "" {
		user.Zone = "default"
	}

	// Prevent non-superadmin from creating superadmin
	if user.Role == "superadmin" && claims.Role != "superadmin" {
		jsonError(w, "only superadmin can create superadmin users", http.StatusForbidden)
		return
	}

	if err := h.auth.CreateUser(r.Context(), user, req.Password); err != nil {
		jsonError(w, "failed to create user", http.StatusInternalServerError)
		return
	}

	user.PasswordHash = ""
	jsonResponse(w, user, http.StatusCreated)
}

// GetUser returns a specific user (admin only)
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims == nil {
		jsonError(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	userID := r.PathValue("id")

	// Users can view themselves, admins can view anyone
	if userID != claims.Subject && claims.Role != "admin" && claims.Role != "superadmin" {
		jsonError(w, "admin access required", http.StatusForbidden)
		return
	}

	user, err := h.auth.GetUser(r.Context(), userID)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	user.PasswordHash = ""
	jsonResponse(w, user, http.StatusOK)
}

// UpdateUser updates a user (admin only)
func (h *Handler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims == nil || (claims.Role != "admin" && claims.Role != "superadmin") {
		jsonError(w, "admin access required", http.StatusForbidden)
		return
	}

	userID := r.PathValue("id")

	var req struct {
		DisplayName string `json:"display_name"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		Zone        string `json:"zone"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	user, err := h.auth.GetUser(r.Context(), userID)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	// Prevent non-superadmin from modifying superadmin users
	if user.Role == "superadmin" && claims.Role != "superadmin" {
		jsonError(w, "only superadmin can modify superadmin users", http.StatusForbidden)
		return
	}

	// Prevent role elevation to superadmin by non-superadmin
	if req.Role == "superadmin" && claims.Role != "superadmin" {
		jsonError(w, "only superadmin can assign superadmin role", http.StatusForbidden)
		return
	}

	// Update fields if provided
	if req.DisplayName != "" {
		user.DisplayName = req.DisplayName
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Role != "" {
		user.Role = req.Role
	}
	if req.Zone != "" {
		user.Zone = req.Zone
	}

	if err := h.auth.UpdateUser(r.Context(), user); err != nil {
		jsonError(w, "failed to update user", http.StatusInternalServerError)
		return
	}

	user.PasswordHash = ""
	jsonResponse(w, user, http.StatusOK)
}

// DeleteUser deletes a user (admin only)
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims == nil || (claims.Role != "admin" && claims.Role != "superadmin") {
		jsonError(w, "admin access required", http.StatusForbidden)
		return
	}

	userID := r.PathValue("id")

	// Prevent self-deletion
	if userID == claims.Subject {
		jsonError(w, "cannot delete your own account", http.StatusBadRequest)
		return
	}

	// Check if user exists and get their role
	user, err := h.auth.GetUser(r.Context(), userID)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	// Prevent non-superadmin from deleting superadmin users
	if user.Role == "superadmin" && claims.Role != "superadmin" {
		jsonError(w, "only superadmin can delete superadmin users", http.StatusForbidden)
		return
	}

	if err := h.auth.DeleteUser(r.Context(), userID); err != nil {
		jsonError(w, "failed to delete user", http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]string{"message": "user deleted"}, http.StatusOK)
}

// ResetUserPassword allows admin to reset a user's password
func (h *Handler) ResetUserPassword(w http.ResponseWriter, r *http.Request) {
	claims := GetClaims(r.Context())
	if claims == nil || (claims.Role != "admin" && claims.Role != "superadmin") {
		jsonError(w, "admin access required", http.StatusForbidden)
		return
	}

	userID := r.PathValue("id")

	var req struct {
		NewPassword string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 8 {
		jsonError(w, "password must be at least 8 characters", http.StatusBadRequest)
		return
	}

	// Check if user exists
	user, err := h.auth.GetUser(r.Context(), userID)
	if err != nil {
		jsonError(w, "user not found", http.StatusNotFound)
		return
	}

	// Prevent non-superadmin from resetting superadmin password
	if user.Role == "superadmin" && claims.Role != "superadmin" {
		jsonError(w, "only superadmin can reset superadmin passwords", http.StatusForbidden)
		return
	}

	if err := h.auth.UpdatePassword(r.Context(), userID, req.NewPassword); err != nil {
		jsonError(w, "failed to reset password", http.StatusInternalServerError)
		return
	}

	// Logout all sessions for this user
	h.auth.LogoutAll(r.Context(), userID)

	jsonResponse(w, map[string]string{"message": "password reset successfully"}, http.StatusOK)
}

// ListRoles returns all available roles
func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles := []map[string]interface{}{
		{"name": "guest", "description": "Limited read access", "level": 0},
		{"name": "user", "description": "Standard user access", "level": 1},
		{"name": "student", "description": "Student access", "level": 1},
		{"name": "teacher", "description": "Teacher access with additional permissions", "level": 2},
		{"name": "admin", "description": "Administrative access", "level": 3},
		{"name": "superadmin", "description": "Full system access", "level": 4},
	}

	jsonResponse(w, map[string]interface{}{
		"roles": roles,
		"count": len(roles),
	}, http.StatusOK)
}

// GetRole returns a specific role with its permissions
func (h *Handler) GetRole(w http.ResponseWriter, r *http.Request) {
	roleName := r.PathValue("name")

	roleInfo := map[string]interface{}{
		"name":        roleName,
		"permissions": getDefaultPermissionsForRole(roleName),
	}

	jsonResponse(w, roleInfo, http.StatusOK)
}

func getDefaultPermissionsForRole(role string) []string {
	switch role {
	case "guest":
		return []string{"services:read"}
	case "user", "student":
		return []string{"services:read", "services:list", "profile:read", "profile:update"}
	case "teacher":
		return []string{"services:read", "services:list", "services:register", "profile:read", "profile:update", "zones:read"}
	case "admin":
		return []string{
			"services:*", "users:read", "users:create", "users:update", "users:delete",
			"zones:*", "roles:read", "profile:*",
		}
	case "superadmin":
		return []string{"*"}
	default:
		return []string{"services:read"}
	}
}

func parseInt(s string) (int, error) {
	var i int
	_, err := fmt.Sscanf(s, "%d", &i)
	return i, err
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
