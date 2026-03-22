package api

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/faisal/selfshare/storage"
	"github.com/faisal/selfshare/store"
	"github.com/faisal/selfshare/tasks"
)

// PhotoHandler holds dependencies for photo-related API handlers.
type PhotoHandler struct {
	DB       *store.DB
	Files    *storage.FileStore
	ThumbDir string
}

// PhotoResponse extends FileResponse with photo metadata.
type PhotoResponse struct {
	FileResponse
	TakenAt     *string `json:"taken_at,omitempty"`
	CameraMake  string  `json:"camera_make,omitempty"`
	CameraModel string  `json:"camera_model,omitempty"`
	Width       int     `json:"width,omitempty"`
	Height      int     `json:"height,omitempty"`
	ThumbURL    string  `json:"thumb_url,omitempty"`
}

// ListPhotos handles GET /api/v1/photos — paginated photo list.
func (h *PhotoHandler) ListPhotos(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50)
	offset := queryInt(r, "offset", 0)

	if limit > 200 {
		limit = 200
	}

	photos, total, err := h.DB.ListPhotos(limit, offset)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list photos")
		log.Printf("ListPhotos error: %v", err)
		return
	}

	resp := make([]PhotoResponse, len(photos))
	for i, p := range photos {
		resp[i] = photoToResponse(&p)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"photos": resp,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// Timeline handles GET /api/v1/photos/timeline — grouped by year-month.
func (h *PhotoHandler) Timeline(w http.ResponseWriter, r *http.Request) {
	groups, err := h.DB.ListPhotoTimeline()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get timeline")
		log.Printf("Timeline error: %v", err)
		return
	}

	type timelineEntry struct {
		Year  int    `json:"year"`
		Month int    `json:"month"`
		Label string `json:"label"`
		Count int    `json:"count"`
	}

	resp := make([]timelineEntry, len(groups))
	for i, g := range groups {
		resp[i] = timelineEntry{Year: g.Year, Month: g.Month, Label: g.Label, Count: g.Count}
	}

	writeJSON(w, http.StatusOK, resp)
}

// Thumb handles GET /api/v1/files/{id}/thumb — serve thumbnail.
func (h *PhotoHandler) Thumb(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	size := r.URL.Query().Get("size")
	if size == "" {
		size = "sm"
	}

	// Validate size
	validSize := false
	for _, s := range tasks.ThumbSizes {
		if s.Name == size {
			validSize = true
			break
		}
	}
	if !validSize {
		writeError(w, http.StatusBadRequest, "size must be sm, md, or lg")
		return
	}

	// Look up thumbnail path
	thumbRelPath, err := h.DB.GetThumbPath(id, size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if thumbRelPath == "" {
		writeError(w, http.StatusNotFound, "thumbnail not available")
		return
	}

	thumbAbsPath := filepath.Join(h.Files.Root(), thumbRelPath)

	f, err := os.Open(thumbAbsPath)
	if err != nil {
		writeError(w, http.StatusNotFound, "thumbnail file not found")
		return
	}
	defer f.Close()

	stat, _ := f.Stat()
	w.Header().Set("Content-Type", "image/jpeg")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(w, r, "thumb.jpg", stat.ModTime(), f)
}

func photoToResponse(p *store.PhotoWithFile) PhotoResponse {
	pr := PhotoResponse{
		FileResponse: fileToResponse(&p.File),
		CameraMake:   p.CameraMake,
		CameraModel:  p.CameraModel,
		Width:        p.Width,
		Height:       p.Height,
	}
	if p.TakenAt != nil {
		s := p.TakenAt.Format(time.RFC3339)
		pr.TakenAt = &s
	}
	pr.ThumbURL = "/api/v1/files/" + p.ID + "/thumb?size=sm"
	return pr
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 0 {
		return defaultVal
	}
	return v
}
