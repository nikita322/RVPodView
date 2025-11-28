package api

import (
	"encoding/json"
	"net/http"

	"rvpodview/internal/auth"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	pamAuth    *auth.PAMAuth
	jwtManager *auth.JWTManager
}

// NewAuthHandler creates new auth handler
func NewAuthHandler(pamAuth *auth.PAMAuth, jwtManager *auth.JWTManager) *AuthHandler {
	return &AuthHandler{
		pamAuth:    pamAuth,
		jwtManager: jwtManager,
	}
}

// LoginRequest represents login request body
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginResponse represents login response
type LoginResponse struct {
	Success bool       `json:"success"`
	Message string     `json:"message,omitempty"`
	User    *auth.User `json:"user,omitempty"`
}

// Login handles POST /api/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
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

	// Authenticate via PAM
	user, err := h.pamAuth.Authenticate(req.Username, req.Password)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, LoginResponse{
			Success: false,
			Message: "Invalid username or password",
		})
		return
	}

	// Generate JWT token
	token, err := h.jwtManager.GenerateToken(user)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, LoginResponse{
			Success: false,
			Message: "Failed to generate token",
		})
		return
	}

	// Set cookie (24 hours)
	auth.SetAuthCookie(w, token, 86400)

	writeJSON(w, http.StatusOK, LoginResponse{
		Success: true,
		User:    user,
	})
}

// Logout handles POST /api/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	auth.ClearAuthCookie(w)
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
