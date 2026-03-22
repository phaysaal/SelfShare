package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/faisal/selfshare/auth"
	"github.com/faisal/selfshare/store"
)

// AuthHandler holds dependencies for auth-related API handlers.
type AuthHandler struct {
	DB        *store.DB
	GetSecret func() string
}

// Login handles POST /api/v1/auth/login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Device   string `json:"device"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Username == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "username and password required")
		return
	}

	user, err := h.DB.Authenticate(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "authentication error")
		log.Printf("Auth error: %v", err)
		return
	}
	if user == nil {
		writeError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}

	// Generate token pair
	tokens, err := auth.CreateTokenPair(h.GetSecret(), user.ID, user.Username, user.IsAdmin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create tokens")
		log.Printf("Token error: %v", err)
		return
	}

	// Store refresh token session
	device := req.Device
	if device == "" {
		device = r.UserAgent()
	}
	_, err = h.DB.CreateSession(user.ID, tokens.RefreshToken, device)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		log.Printf("Session error: %v", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"expires_in":    tokens.ExpiresIn,
		"user": map[string]any{
			"id":           user.ID,
			"username":     user.Username,
			"display_name": user.DisplayName,
			"is_admin":     user.IsAdmin,
		},
	})
}

// Refresh handles POST /api/v1/auth/refresh
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token required")
		return
	}

	// Validate the refresh token
	session, err := h.DB.ValidateRefreshToken(req.RefreshToken)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "session error")
		log.Printf("Refresh error: %v", err)
		return
	}
	if session == nil {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	// Get the user
	user, err := h.DB.GetUser(session.UserID)
	if err != nil || user == nil {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}

	// Rotate: delete old session, create new token pair
	h.DB.DeleteSession(session.ID)

	tokens, err := auth.CreateTokenPair(h.GetSecret(), user.ID, user.Username, user.IsAdmin)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create tokens")
		return
	}

	_, err = h.DB.CreateSession(user.ID, tokens.RefreshToken, session.DeviceName)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  tokens.AccessToken,
		"refresh_token": tokens.RefreshToken,
		"expires_in":    tokens.ExpiresIn,
	})
}

// Logout handles POST /api/v1/auth/logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.RefreshToken != "" {
		h.DB.DeleteSessionByRefreshToken(req.RefreshToken)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged out"})
}
