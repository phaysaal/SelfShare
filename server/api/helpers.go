package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/faisal/selfshare/store"
)

// JSON response helpers

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Error encoding JSON: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// FileResponse is the JSON representation of a file/folder.
type FileResponse struct {
	ID        string  `json:"id"`
	ParentID  *string `json:"parent_id"`
	Name      string  `json:"name"`
	IsDir     bool    `json:"is_dir"`
	SizeBytes int64   `json:"size_bytes"`
	MimeType  *string `json:"mime_type,omitempty"`
	SHA256    *string `json:"sha256,omitempty"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

func fileToResponse(f *store.File) FileResponse {
	return FileResponse{
		ID:        f.ID,
		ParentID:  f.ParentID,
		Name:      f.Name,
		IsDir:     f.IsDir,
		SizeBytes: f.SizeBytes,
		MimeType:  f.MimeType,
		SHA256:    f.SHA256,
		CreatedAt: f.CreatedAt.Format(time.RFC3339),
		UpdatedAt: f.UpdatedAt.Format(time.RFC3339),
	}
}

func filesToResponse(files []*store.File) []FileResponse {
	resp := make([]FileResponse, len(files))
	for i, f := range files {
		resp[i] = fileToResponse(f)
	}
	return resp
}
