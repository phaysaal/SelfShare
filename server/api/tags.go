package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/faisal/selfshare/store"
)

// TagHandler holds dependencies for tag-related API handlers.
type TagHandler struct {
	DB *store.DB
}

// CreateTag handles POST /api/v1/tags
func (h *TagHandler) CreateTag(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}

	tag, err := h.DB.CreateTag(req.Name, req.Color)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create tag")
		log.Printf("CreateTag error: %v", err)
		return
	}
	writeJSON(w, http.StatusCreated, tag)
}

// ListTags handles GET /api/v1/tags
func (h *TagHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags, err := h.DB.ListTags()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list tags")
		return
	}
	if tags == nil {
		tags = []store.Tag{}
	}
	writeJSON(w, http.StatusOK, tags)
}

// UpdateTag handles PUT /api/v1/tags/{id}
func (h *TagHandler) UpdateTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req struct {
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := h.DB.UpdateTag(id, req.Name, req.Color); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update tag")
		return
	}
	tag, _ := h.DB.GetTag(id)
	writeJSON(w, http.StatusOK, tag)
}

// DeleteTag handles DELETE /api/v1/tags/{id}
func (h *TagHandler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := h.DB.DeleteTag(id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete tag")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// TagFile handles POST /api/v1/files/{id}/tags
func (h *TagHandler) TagFile(w http.ResponseWriter, r *http.Request) {
	fileID := r.PathValue("id")
	var req struct {
		TagID string `json:"tag_id"`
		Name  string `json:"name"`
		Color string `json:"color"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return
	}

	// Allow tagging by tag_id or by name (auto-create)
	tagID := req.TagID
	if tagID == "" && req.Name != "" {
		tag, err := h.DB.CreateTag(req.Name, req.Color)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to create tag")
			return
		}
		tagID = tag.ID
	}
	if tagID == "" {
		writeError(w, http.StatusBadRequest, "tag_id or name required")
		return
	}

	if err := h.DB.TagFile(fileID, tagID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to tag file")
		return
	}

	tags, _ := h.DB.GetFileTags(fileID)
	writeJSON(w, http.StatusOK, tags)
}

// UntagFile handles DELETE /api/v1/files/{id}/tags/{tagId}
func (h *TagHandler) UntagFile(w http.ResponseWriter, r *http.Request) {
	fileID := r.PathValue("id")
	tagID := r.PathValue("tagId")

	if err := h.DB.UntagFile(fileID, tagID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to untag file")
		return
	}

	tags, _ := h.DB.GetFileTags(fileID)
	writeJSON(w, http.StatusOK, tags)
}

// GetFileTags handles GET /api/v1/files/{id}/tags
func (h *TagHandler) GetFileTags(w http.ResponseWriter, r *http.Request) {
	fileID := r.PathValue("id")
	tags, err := h.DB.GetFileTags(fileID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get tags")
		return
	}
	if tags == nil {
		tags = []store.Tag{}
	}
	writeJSON(w, http.StatusOK, tags)
}

// ListFilesByTag handles GET /api/v1/tags/{id}/files
func (h *TagHandler) ListFilesByTag(w http.ResponseWriter, r *http.Request) {
	tagID := r.PathValue("id")
	files, err := h.DB.ListFilesByTag(tagID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list files")
		return
	}
	writeJSON(w, http.StatusOK, filesToResponse(files))
}
