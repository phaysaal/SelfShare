package api

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/faisal/selfshare/auth"
)

type contextKey string

const claimsKey contextKey = "claims"

// GetClaims extracts the JWT claims from the request context.
func GetClaims(r *http.Request) *auth.Claims {
	claims, _ := r.Context().Value(claimsKey).(*auth.Claims)
	return claims
}

// AuthMiddleware validates the JWT access token from the Authorization header.
// Skips auth for public routes (ping, login, setup, share pages).
// Takes a function to get the secret so it picks up changes after setup.
func AuthMiddleware(getSecret func() string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isPublicRoute(r) {
			next.ServeHTTP(w, r)
			return
		}

		token := extractToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization token")
			return
		}

		claims, err := auth.ValidateAccessToken(getSecret(), token)
		if err == auth.ErrTokenExpired {
			writeError(w, http.StatusUnauthorized, "token expired")
			return
		}
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoggingMiddleware logs each request with method, path, status, and duration.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

func isPublicRoute(r *http.Request) bool {
	path := r.URL.Path

	// Health/discovery
	if path == "/api/v1/ping" {
		return true
	}

	// Auth endpoints (login, refresh — logout needs auth but we allow it)
	if strings.HasPrefix(path, "/api/v1/auth/") {
		return true
	}

	// Setup page
	if path == "/setup" || path == "/api/v1/setup" {
		return true
	}

	// Share pages
	if strings.HasPrefix(path, "/s/") {
		return true
	}

	// Web UI static assets (SPA)
	if path == "/" || path == "/favicon.ico" || strings.HasPrefix(path, "/assets/") {
		return true
	}
	// SPA client-side routes (not /api, not /s/) — serve index.html
	if !strings.HasPrefix(path, "/api/") {
		return true
	}

	return false
}

func extractToken(r *http.Request) string {
	// Check Authorization header: "Bearer <token>"
	header := r.Header.Get("Authorization")
	if strings.HasPrefix(header, "Bearer ") {
		return strings.TrimPrefix(header, "Bearer ")
	}

	// Check query parameter (for download links)
	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}

	return ""
}

// statusWriter wraps ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (sw *statusWriter) WriteHeader(code int) {
	sw.status = code
	sw.ResponseWriter.WriteHeader(code)
}
