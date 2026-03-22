package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/faisal/selfshare/storage"
	"github.com/faisal/selfshare/store"
	"github.com/faisal/selfshare/tasks"
)

// UploadHandler holds dependencies for chunked upload handlers.
type UploadHandler struct {
	DB          *store.DB
	Files       *storage.FileStore
	TempDir     string
	ThumbWorker *tasks.ThumbWorker
}

// Initiate handles POST /api/v1/uploads — start a chunked upload session.
func (h *UploadHandler) Initiate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ParentID  string `json:"parent_id"`
		Filename  string `json:"filename"`
		TotalSize int64  `json:"total_size"`
		ChunkSize int    `json:"chunk_size"`
		SHA256    string `json:"sha256"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	if req.Filename == "" || req.TotalSize <= 0 {
		writeError(w, http.StatusBadRequest, "filename and total_size required")
		return
	}
	if err := validateFilename(req.Filename); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ParentID == "" {
		req.ParentID = "root"
	}

	// Validate parent exists and is a directory
	parent, err := h.DB.GetFile(req.ParentID)
	if err != nil || parent == nil || !parent.IsDir {
		writeError(w, http.StatusBadRequest, "invalid parent folder")
		return
	}

	// Check for duplicate name
	existing, err := h.DB.GetByParentAndName(req.ParentID, req.Filename)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if existing != nil {
		writeError(w, http.StatusConflict, "a file with that name already exists")
		return
	}

	// Create upload session
	upload, err := h.DB.CreateUpload(req.ParentID, req.Filename, req.TotalSize, req.ChunkSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create upload session")
		log.Printf("CreateUpload error: %v", err)
		return
	}

	// Create temp directory for chunks
	chunkDir := filepath.Join(h.TempDir, upload.ID)
	if err := os.MkdirAll(chunkDir, 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create temp directory")
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"id":           upload.ID,
		"chunk_size":   upload.ChunkSize,
		"total_chunks": upload.TotalChunks,
		"expires_at":   upload.ExpiresAt.Format("2006-01-02T15:04:05Z"),
	})
}

// UploadChunk handles PUT /api/v1/uploads/{id}/{chunk} — upload a single chunk.
func (h *UploadHandler) UploadChunk(w http.ResponseWriter, r *http.Request) {
	uploadID := r.PathValue("id")
	chunkStr := r.PathValue("chunk")

	chunkNum, err := strconv.Atoi(chunkStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chunk number")
		return
	}

	upload, err := h.DB.GetUpload(uploadID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if upload == nil {
		writeError(w, http.StatusNotFound, "upload session not found")
		return
	}
	if upload.Status != "active" {
		writeError(w, http.StatusConflict, "upload is not active")
		return
	}
	if chunkNum < 0 || chunkNum >= upload.TotalChunks {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("chunk number must be 0-%d", upload.TotalChunks-1))
		return
	}

	// Limit chunk body to chunk_size + small buffer
	maxSize := int64(upload.ChunkSize) + 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	// Write chunk to temp file
	chunkPath := filepath.Join(h.TempDir, uploadID, fmt.Sprintf("chunk_%06d", chunkNum))
	f, err := os.Create(chunkPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create chunk file")
		return
	}

	n, err := io.Copy(f, r.Body)
	f.Close()
	if err != nil {
		os.Remove(chunkPath)
		writeError(w, http.StatusInternalServerError, "failed to write chunk")
		return
	}

	// Mark chunk as received
	if err := h.DB.MarkChunkReceived(uploadID, chunkNum); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update upload state")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"chunk":    chunkNum,
		"size":     n,
		"received": len(upload.Received) + 1,
		"total":    upload.TotalChunks,
	})
}

// Complete handles POST /api/v1/uploads/{id}/complete — assemble chunks into a file.
func (h *UploadHandler) Complete(w http.ResponseWriter, r *http.Request) {
	uploadID := r.PathValue("id")

	upload, err := h.DB.GetUpload(uploadID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if upload == nil {
		writeError(w, http.StatusNotFound, "upload session not found")
		return
	}
	if upload.Status != "active" {
		writeError(w, http.StatusConflict, "upload is not active")
		return
	}

	// Refresh to get latest received list
	upload, _ = h.DB.GetUpload(uploadID)
	if !upload.IsComplete() {
		missing := upload.MissingChunks()
		writeJSON(w, http.StatusBadRequest, map[string]any{
			"error":          "missing chunks",
			"missing_chunks": missing,
		})
		return
	}

	h.DB.SetUploadStatus(uploadID, "completing")

	// Assemble chunks into a temp file while computing SHA256
	chunkDir := filepath.Join(h.TempDir, uploadID)
	assembled, err := os.CreateTemp(h.TempDir, "assembled-*")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create temp file")
		return
	}
	assembledPath := assembled.Name()
	defer func() {
		os.Remove(assembledPath)
	}()

	hasher := sha256.New()
	writer := io.MultiWriter(assembled, hasher)
	var totalBytes int64

	for i := 0; i < upload.TotalChunks; i++ {
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%06d", i))
		chunk, err := os.Open(chunkPath)
		if err != nil {
			assembled.Close()
			h.DB.SetUploadStatus(uploadID, "active")
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to read chunk %d", i))
			return
		}
		n, err := io.Copy(writer, chunk)
		chunk.Close()
		if err != nil {
			assembled.Close()
			h.DB.SetUploadStatus(uploadID, "active")
			writeError(w, http.StatusInternalServerError, fmt.Sprintf("failed to assemble chunk %d", i))
			return
		}
		totalBytes += n
	}
	assembled.Close()

	hash := hex.EncodeToString(hasher.Sum(nil))

	// Compute real disk path
	diskPath, err := h.DB.GetDiskPath(upload.ParentID, upload.Filename)
	if err != nil {
		h.DB.SetUploadStatus(uploadID, "active")
		writeError(w, http.StatusInternalServerError, "failed to compute path")
		return
	}

	// Store at real path via FileStore
	assembledFile, err := os.Open(assembledPath)
	if err != nil {
		h.DB.SetUploadStatus(uploadID, "active")
		writeError(w, http.StatusInternalServerError, "failed to read assembled file")
		return
	}
	storedHash, size, err := h.Files.Store(diskPath, assembledFile)
	assembledFile.Close()
	if err != nil {
		h.DB.SetUploadStatus(uploadID, "active")
		writeError(w, http.StatusInternalServerError, "failed to store file")
		log.Printf("FileStore error: %v", err)
		return
	}

	// Verify hash matches
	if storedHash != hash {
		log.Printf("WARNING: hash mismatch during assembly: computed=%s stored=%s", hash, storedHash)
	}

	// Create file record
	mimeType := detectMimeType(upload.Filename)

	file, err := h.DB.CreateFile(upload.ParentID, upload.Filename, mimeType, storedHash, diskPath, size)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create file record")
		log.Printf("CreateFile error: %v", err)
		return
	}

	// Trigger EXIF + thumbnail for media files
	if isMedia(mimeType) {
		absPath := h.Files.AbsPath(diskPath)
		go tasks.ExtractAndSaveMeta(h.DB, file.ID, absPath, mimeType)
		if h.ThumbWorker != nil {
			h.ThumbWorker.Enqueue(tasks.ThumbJob{FileID: file.ID, AbsPath: absPath, MimeType: mimeType})
		}
	}

	// Cleanup: delete chunks and upload session
	os.RemoveAll(chunkDir)
	h.DB.SetUploadStatus(uploadID, "done")
	h.DB.DeleteUpload(uploadID)

	writeJSON(w, http.StatusCreated, map[string]any{
		"status": "complete",
		"sha256": storedHash,
		"size":   size,
		"file":   fileToResponse(file),
	})
}

// Cancel handles DELETE /api/v1/uploads/{id} — cancel an upload session.
func (h *UploadHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	uploadID := r.PathValue("id")

	upload, err := h.DB.GetUpload(uploadID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if upload == nil {
		writeError(w, http.StatusNotFound, "upload session not found")
		return
	}

	// Cleanup chunks and session
	chunkDir := filepath.Join(h.TempDir, uploadID)
	os.RemoveAll(chunkDir)
	h.DB.DeleteUpload(uploadID)

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// GetStatus handles GET /api/v1/uploads/{id} — get upload session status.
func (h *UploadHandler) GetStatus(w http.ResponseWriter, r *http.Request) {
	uploadID := r.PathValue("id")

	upload, err := h.DB.GetUpload(uploadID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "database error")
		return
	}
	if upload == nil {
		writeError(w, http.StatusNotFound, "upload session not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"id":             upload.ID,
		"filename":       upload.Filename,
		"total_size":     upload.TotalSize,
		"chunk_size":     upload.ChunkSize,
		"total_chunks":   upload.TotalChunks,
		"received":       upload.Received,
		"missing_chunks": upload.MissingChunks(),
		"status":         upload.Status,
		"expires_at":     upload.ExpiresAt.Format("2006-01-02T15:04:05Z"),
	})
}

// CleanupStaleUploads removes expired upload sessions and their chunk files.
func (h *UploadHandler) CleanupStaleUploads() {
	ids, err := h.DB.CleanStaleUploads()
	if err != nil {
		log.Printf("Stale upload cleanup error: %v", err)
		return
	}
	for _, id := range ids {
		chunkDir := filepath.Join(h.TempDir, id)
		os.RemoveAll(chunkDir)
		log.Printf("Cleaned up stale upload: %s", id)
	}
}
