package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/faisal/selfshare/storage"
	"github.com/faisal/selfshare/store"
	"github.com/faisal/selfshare/tasks"
)

// FileHandler holds dependencies for file-related API handlers.
type FileHandler struct {
	DB          *store.DB
	Files       *storage.FileStore
	ThumbWorker *tasks.ThumbWorker
}

// ListRoot handles GET /api/v1/files — list the root folder contents.
func (h *FileHandler) ListRoot(w http.ResponseWriter, r *http.Request) {
	files, err := h.DB.ListChildren("root")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list files")
		log.Printf("ListRoot error: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, filesToResponse(files))
}

// GetFile handles GET /api/v1/files/{id} — get file/folder metadata.
func (h *FileHandler) GetFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f, err := h.DB.GetFile(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		log.Printf("GetFile error: %v", err)
		return
	}
	if f == nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	writeJSON(w, http.StatusOK, fileToResponse(f))
}

// ListChildren handles GET /api/v1/files/{id}/children — list folder contents.
func (h *FileHandler) ListChildren(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	parent, err := h.DB.GetFile(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if parent == nil {
		writeError(w, http.StatusNotFound, "folder not found")
		return
	}
	if !parent.IsDir {
		writeError(w, http.StatusBadRequest, "not a folder")
		return
	}

	files, err := h.DB.ListChildren(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list files")
		log.Printf("ListChildren error: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, filesToResponse(files))
}

// Upload handles POST /api/v1/files — upload a file or create a folder.
func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header.Get("Content-Type")

	// JSON body = create folder
	if strings.HasPrefix(contentType, "application/json") {
		h.createFolder(w, r)
		return
	}

	// Multipart = file upload
	if strings.HasPrefix(contentType, "multipart/form-data") {
		h.uploadFile(w, r)
		return
	}

	writeError(w, http.StatusBadRequest, "expected multipart/form-data or application/json")
}

func (h *FileHandler) createFolder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ParentID string `json:"parent_id"`
		Name     string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if req.ParentID == "" {
		req.ParentID = "root"
	}

	if err := validateFilename(req.Name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check parent exists and is a directory
	parent, err := h.DB.GetFile(req.ParentID)
	if err != nil || parent == nil || !parent.IsDir {
		writeError(w, http.StatusBadRequest, "invalid parent folder")
		return
	}

	// Check for duplicate name
	existing, err := h.DB.GetByParentAndName(req.ParentID, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if existing != nil {
		writeError(w, http.StatusConflict, "a file or folder with that name already exists")
		return
	}

	// Compute disk path and create actual directory
	diskPath, err := h.DB.GetDiskPath(req.ParentID, req.Name)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute path")
		return
	}
	if err := h.Files.MkdirAll(diskPath); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create directory")
		log.Printf("MkdirAll error: %v", err)
		return
	}

	folder, err := h.DB.CreateFolder(req.ParentID, req.Name, diskPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create folder")
		log.Printf("CreateFolder error: %v", err)
		return
	}

	writeJSON(w, http.StatusCreated, fileToResponse(folder))
}

func (h *FileHandler) uploadFile(w http.ResponseWriter, r *http.Request) {
	// Limit to 100MB for non-chunked uploads
	r.Body = http.MaxBytesReader(w, r.Body, 100<<20)

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse multipart form")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing 'file' field")
		return
	}
	defer file.Close()

	parentID := r.FormValue("parent_id")
	if parentID == "" {
		parentID = "root"
	}

	filename := header.Filename
	if filename == "" {
		writeError(w, http.StatusBadRequest, "missing filename")
		return
	}

	if err := validateFilename(filename); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Check parent exists and is a directory
	parent, err := h.DB.GetFile(parentID)
	if err != nil || parent == nil || !parent.IsDir {
		writeError(w, http.StatusBadRequest, "invalid parent folder")
		return
	}

	// Check for duplicate name
	existing, err := h.DB.GetByParentAndName(parentID, filename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if existing != nil {
		writeError(w, http.StatusConflict, "a file with that name already exists")
		return
	}

	// Compute disk path and store
	diskPath, err := h.DB.GetDiskPath(parentID, filename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to compute path")
		return
	}

	hash, size, err := h.Files.Store(diskPath, file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store file")
		log.Printf("Store error: %v", err)
		return
	}

	mimeType := detectMimeType(filename)

	f, err := h.DB.CreateFile(parentID, filename, mimeType, hash, diskPath, size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save file record")
		log.Printf("CreateFile error: %v", err)
		return
	}

	// Trigger EXIF extraction and thumbnail generation for media files
	if isMedia(mimeType) {
		absPath := h.Files.AbsPath(diskPath)
		go tasks.ExtractAndSaveMeta(h.DB, f.ID, absPath, mimeType)
		if h.ThumbWorker != nil {
			h.ThumbWorker.Enqueue(tasks.ThumbJob{FileID: f.ID, AbsPath: absPath, MimeType: mimeType})
		}
	}

	writeJSON(w, http.StatusCreated, fileToResponse(f))
}

func isMedia(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/") || strings.HasPrefix(mimeType, "video/") || strings.HasPrefix(mimeType, "audio/")
}

// Download handles GET /api/v1/files/{id}/download — download file as attachment.
func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f, err := h.DB.GetFile(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if f == nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if f.IsDir {
		writeError(w, http.StatusBadRequest, "cannot download a folder")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, f.Name))
	h.serveFile(w, r, f)
}

// View handles GET /api/v1/files/{id}/view — serve file inline (for photos, videos, audio).
// Supports HTTP Range requests for video/audio streaming and seeking.
func (h *FileHandler) View(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	f, err := h.DB.GetFile(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if f == nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	if f.IsDir {
		writeError(w, http.StatusBadRequest, "not a file")
		return
	}

	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="%s"`, f.Name))
	h.serveFile(w, r, f)
}

// serveFile serves a file with Range request support using http.ServeContent.
func (h *FileHandler) serveFile(w http.ResponseWriter, r *http.Request, f *store.File) {
	file, err := h.Files.Open(f.DiskPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open file")
		log.Printf("Open file error: %v", err)
		return
	}
	defer file.Close()

	if f.MimeType != nil {
		w.Header().Set("Content-Type", *f.MimeType)
	}

	// http.ServeContent handles:
	// - Range requests (206 Partial Content) for streaming/seeking
	// - If-Modified-Since / If-None-Match
	// - Content-Length
	http.ServeContent(w, r, f.Name, f.UpdatedAt, file)
}

// DeleteFile handles DELETE /api/v1/files/{id} — soft delete (trash).
func (h *FileHandler) DeleteFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	f, err := h.DB.GetFile(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if f == nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	if err := h.DB.SoftDeleteFile(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete")
		log.Printf("SoftDelete error: %v", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// UpdateFile handles PUT /api/v1/files/{id} — rename or move.
func (h *FileHandler) UpdateFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	var req struct {
		Name     *string `json:"name"`
		ParentID *string `json:"parent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	f, err := h.DB.GetFile(id)
	if err != nil || f == nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	if req.Name != nil {
		if err := validateFilename(*req.Name); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		oldPath, newPath, err := h.DB.RenameFile(id, *req.Name)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to rename")
			return
		}
		// Rename on disk
		if oldPath != "" {
			if err := h.Files.Rename(oldPath, newPath); err != nil {
				log.Printf("Disk rename error: %v", err)
			}
		}
	}

	if req.ParentID != nil {
		parent, err := h.DB.GetFile(*req.ParentID)
		if err != nil || parent == nil || !parent.IsDir {
			writeError(w, http.StatusBadRequest, "invalid target folder")
			return
		}
		oldPath, newPath, err := h.DB.MoveFile(id, *req.ParentID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to move")
			return
		}
		// Move on disk
		if oldPath != "" {
			if err := h.Files.Rename(oldPath, newPath); err != nil {
				log.Printf("Disk move error: %v", err)
			}
		}
	}

	updated, _ := h.DB.GetFile(id)
	writeJSON(w, http.StatusOK, fileToResponse(updated))
}

// validateFilename rejects dangerous filenames.
func validateFilename(name string) error {
	if name == "" {
		return fmt.Errorf("filename cannot be empty")
	}
	if name == "." || name == ".." {
		return fmt.Errorf("invalid filename")
	}
	if strings.ContainsAny(name, "/\\\x00") {
		return fmt.Errorf("filename contains invalid characters")
	}
	if len(name) > 255 {
		return fmt.Errorf("filename too long")
	}
	return nil
}

// detectMimeType returns a MIME type based on file extension.
func detectMimeType(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	mimeTypes := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".heic": "image/heic",
		".heif": "image/heif",
		".svg":  "image/svg+xml",
		".mp4":  "video/mp4",
		".mov":  "video/quicktime",
		".avi":  "video/x-msvideo",
		".mkv":  "video/x-matroska",
		".webm": "video/webm",
		".mp3":  "audio/mpeg",
		".wav":  "audio/wav",
		".flac": "audio/flac",
		".pdf":  "application/pdf",
		".zip":  "application/zip",
		".txt":  "text/plain",
		".json": "application/json",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
	}
	if mt, ok := mimeTypes[ext]; ok {
		return mt
	}
	return "application/octet-stream"
}
