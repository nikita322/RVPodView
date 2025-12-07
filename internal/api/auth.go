package api

import (
	"encoding/json"
	"net/http"
	"time"

	"podmanview/internal/auth"
	"podmanview/internal/events"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	pamAuth      *auth.PAMAuth
	jwtManager   *auth.JWTManager
	wsTokenStore *auth.WSTokenStore
	eventStore   *events.Store
	rateLimiter  *auth.LoginRateLimiter
}

// NewAuthHandler creates new auth handler
func NewAuthHandler(pamAuth *auth.PAMAuth, jwtManager *auth.JWTManager, wsTokenStore *auth.WSTokenStore, eventStore *events.Store) *AuthHandler {
	return &AuthHandler{
		pamAuth:      pamAuth,
		jwtManager:   jwtManager,
		wsTokenStore: wsTokenStore,
		eventStore:   eventStore,
		rateLimiter:  auth.NewLoginRateLimiter(),
	}
}

// LoginRequest represents login request body
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Remember bool   `json:"remember"`
}

// LoginResponse represents login response
type LoginResponse struct {
	Success bool       `json:"success"`
	Message string     `json:"message,omitempty"`
	User    *auth.User `json:"user,omitempty"`
}

// Login handles POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	clientIP := getClientIP(r)

	// Check rate limit first - reject immediately without wasting resources
	if allowed, _ := h.rateLimiter.Allow(clientIP); !allowed {
		writeJSON(w, http.StatusTooManyRequests, LoginResponse{
			Success: false,
			Message: "Too many login attempts",
		})
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, LoginResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	if req.Username == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, LoginResponse{
			Success: false,
			Message: "Username and password are required",
		})
		return
	}

	user, err := h.pamAuth.Authenticate(req.Username, req.Password)
	if err != nil {
		h.eventStore.Add(events.EventLoginFailed, req.Username, clientIP, false, "")
		writeJSON(w, http.StatusUnauthorized, LoginResponse{
			Success: false,
			Message: "Invalid username or password",
		})
		return
	}

	h.rateLimiter.Reset(clientIP)

	// Token duration: 24 hours default, 30 days if "remember me"
	tokenDuration := 24 * time.Hour
	cookieMaxAge := 86400 // 24 hours in seconds
	if req.Remember {
		tokenDuration = 30 * 24 * time.Hour // 30 days
		cookieMaxAge = 30 * 24 * 3600       // 30 days in seconds
	}

	// Generate JWT token
	token, err := h.jwtManager.GenerateTokenWithDuration(user, tokenDuration)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, LoginResponse{
			Success: false,
			Message: "Failed to generate token",
		})
		return
	}

	// Set cookie (Secure flag auto-set for HTTPS)
	auth.SetAuthCookie(w, r, token, cookieMaxAge)

	// Log successful login
	h.eventStore.Add(events.EventLogin, user.Username, clientIP, true, "")

	writeJSON(w, http.StatusOK, LoginResponse{
		Success: true,
		User:    user,
	})
}

// Logout handles POST /api/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	username := ""
	if user != nil {
		username = user.Username
	}

	auth.ClearAuthCookie(w)

	// Log logout
	h.eventStore.Add(events.EventLogout, username, getClientIP(r), true, "")

	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// Me handles GET /api/auth/me
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Not authenticated"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"user": user,
	})
}

// WSToken handles GET /api/auth/ws-token
// Returns a one-time CSRF token for WebSocket connections
func (h *AuthHandler) WSToken(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Not authenticated"})
		return
	}

	// Only admins can get WebSocket tokens (terminals require admin)
	if !user.IsAdmin() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "Admin access required"})
		return
	}

	token, err := h.wsTokenStore.Generate(user.Username)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate token"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"token": token})
}
