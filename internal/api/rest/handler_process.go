package rest

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/bizengine/engine/internal/core/process"
)

// ProcessHandler handles process engine endpoints.
type ProcessHandler struct {
	engine *process.Engine
}

// NewProcessHandler creates a new ProcessHandler.
func NewProcessHandler(engine *process.Engine) *ProcessHandler {
	return &ProcessHandler{engine: engine}
}

// ListDefinitions handles GET /api/v1/workspaces/{wsID}/processes/definitions.
func (h *ProcessHandler) ListDefinitions(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}

	defs, err := h.engine.ListDefinitions(r.Context(), wsID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, defs)
}

// GetDefinition handles GET /api/v1/workspaces/{wsID}/processes/definitions/{defID}.
func (h *ProcessHandler) GetDefinition(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	defID := chi.URLParam(r, "defID")

	def, err := h.engine.GetDefinition(r.Context(), defID, &wsID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, def)
}

// ListInstances handles GET /api/v1/workspaces/{wsID}/processes/instances.
func (h *ProcessHandler) ListInstances(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}

	page := parsePage(r)
	status := queryString(r, "status")

	instances, total, err := h.engine.ListInstances(r.Context(), wsID, status, page.Limit, page.Offset)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  instances,
		"total":  total,
		"limit":  page.Limit,
		"offset": page.Offset,
	})
}

// GetInstance handles GET /api/v1/workspaces/{wsID}/processes/instances/{instID}.
func (h *ProcessHandler) GetInstance(w http.ResponseWriter, r *http.Request) {
	instID, err := parseUUID(r, "instID")
	if err != nil {
		writeError(w, err)
		return
	}

	inst, err := h.engine.GetInstance(r.Context(), instID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, inst)
}

// GetByEntity handles GET /api/v1/workspaces/{wsID}/processes/entity/{entityID}.
func (h *ProcessHandler) GetByEntity(w http.ResponseWriter, r *http.Request) {
	entityID, err := parseUUID(r, "entityID")
	if err != nil {
		writeError(w, err)
		return
	}

	instances, err := h.engine.GetInstancesByEntity(r.Context(), entityID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, instances)
}

// Trigger handles POST /api/v1/workspaces/{wsID}/processes/trigger.
func (h *ProcessHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		EntityID  string          `json:"entity_id"`
		EventType string          `json:"event_type"`
		Data      json.RawMessage `json:"data,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}

	entityID, err := parseUUIDString(input.EntityID)
	if err != nil {
		writeError(w, err)
		return
	}

	if err := h.engine.TriggerManual(r.Context(), wsID, entityID, input.EventType, input.Data); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
