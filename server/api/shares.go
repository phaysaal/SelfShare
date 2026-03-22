package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/faisal/selfshare/store"
)

// ShareHandler holds dependencies for share-related API handlers.
type ShareHandler struct {
	DB      *store.DB
	BaseURL string // e.g. "https://selfshare.example.com" — empty uses relative URLs
}

// CreateShare handles POST /api/v1/shares
func (h *ShareHandler) CreateShare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FileID       string `json:"file_id"`
		Password     string `json:"password"`
		ExpiresIn    *int   `json:"expires_in"`
		MaxDownloads *int   `json:"max_downloads"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.FileID == "" {
		writeError(w, http.StatusBadRequest, "file_id required")
		return
	}

	// Verify file exists
	f, err := h.DB.GetFile(req.FileID)
	if err != nil || f == nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	share, err := h.DB.CreateShare(req.FileID, req.Password, req.ExpiresIn, req.MaxDownloads)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create share")
		log.Printf("CreateShare error: %v", err)
		return
	}

	writeJSON(w, http.StatusCreated, shareToResponse(share, f.Name, h.BaseURL))
}

// ListShares handles GET /api/v1/shares
func (h *ShareHandler) ListShares(w http.ResponseWriter, r *http.Request) {
	shares, err := h.DB.ListShares()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list shares")
		log.Printf("ListShares error: %v", err)
		return
	}

	resp := make([]map[string]any, len(shares))
	for i, s := range shares {
		resp[i] = shareWithFileToResponse(&s, h.BaseURL)
	}

	writeJSON(w, http.StatusOK, resp)
}

// RevokeShare handles DELETE /api/v1/shares/{id}
func (h *ShareHandler) RevokeShare(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	share, err := h.DB.GetShare(id)
	if err != nil || share == nil {
		writeError(w, http.StatusNotFound, "share not found")
		return
	}

	if err := h.DB.RevokeShare(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to revoke share")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "revoked"})
}

func shareToResponse(s *store.Share, fileName, baseURL string) map[string]any {
	resp := map[string]any{
		"id":             s.ID,
		"file_id":        s.FileID,
		"file_name":      fileName,
		"token":          s.Token,
		"url":            shareURL(s.Token, baseURL),
		"has_password":   s.HasPassword(),
		"download_count": s.DownloadCount,
		"created_at":     s.CreatedAt.Format(time.RFC3339),
	}
	if s.ExpiresAt != nil {
		resp["expires_at"] = s.ExpiresAt.Format(time.RFC3339)
	}
	if s.MaxDownloads != nil {
		resp["max_downloads"] = *s.MaxDownloads
	}
	return resp
}

func shareWithFileToResponse(s *store.ShareWithFile, baseURL string) map[string]any {
	resp := map[string]any{
		"id":             s.ID,
		"file_id":        s.FileID,
		"file_name":      s.FileName,
		"file_is_dir":    s.FileIsDir,
		"file_size":      s.FileSize,
		"token":          s.Token,
		"url":            shareURL(s.Token, baseURL),
		"has_password":   s.HasPassword(),
		"download_count": s.DownloadCount,
		"created_at":     s.CreatedAt.Format(time.RFC3339),
	}
	if s.ExpiresAt != nil {
		resp["expires_at"] = s.ExpiresAt.Format(time.RFC3339)
	}
	if s.MaxDownloads != nil {
		resp["max_downloads"] = *s.MaxDownloads
	}
	return resp
}

func shareURL(token, baseURL string) string {
	return baseURL + "/s/" + token
}
