package rest

import (
	"net/http"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/module/file"
	"github.com/bizengine/engine/pkg/errs"
)

type FileHandler struct {
	fileSvc *file.Service
}

func NewFileHandler(fileSvc *file.Service) *FileHandler {
	return &FileHandler{fileSvc: fileSvc}
}

// RequestUpload handles POST /files/upload-url — returns a presigned PUT URL.
func (h *FileHandler) RequestUpload(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input file.UploadRequest
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	resp, err := h.fileSvc.RequestUpload(r.Context(), orgID, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, resp)
}

// Confirm handles POST /files/{id}/confirm — marks file as uploaded.
func (h *FileHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}

	f, err := h.fileSvc.Confirm(r.Context(), orgID, id)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, f)
}

// Download handles GET /files/{id} — returns a presigned GET URL.
func (h *FileHandler) Download(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}

	url, f, err := h.fileSvc.GetDownloadURL(r.Context(), orgID, id)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]any{
		"file":         f,
		"download_url": url,
	})
}

// Delete handles DELETE /files/{id}.
func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}

	if err := h.fileSvc.Delete(r.Context(), orgID, id); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ListByEntity handles GET /files?entity_id=...
func (h *FileHandler) ListByEntity(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	entityID := queryUUID(r, "entity_id")
	if entityID == nil {
		respondError(w, errs.NewBadRequest("entity_id query parameter is required"))
		return
	}

	files, err := h.fileSvc.ListByEntity(r.Context(), orgID, *entityID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, files)
}
