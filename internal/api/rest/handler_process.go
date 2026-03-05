package rest

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/core/process"
	"github.com/bizengine/engine/pkg/dsl"
)

// ProcessHandler handles process engine endpoints.
type ProcessHandler struct {
	engine *process.Engine
}

// NewProcessHandler creates a new ProcessHandler.
func NewProcessHandler(engine *process.Engine) *ProcessHandler {
	return &ProcessHandler{engine: engine}
}

// ListDefinitions handles GET /api/v1/organizations/{orgID}/processes/definitions.
func (h *ProcessHandler) ListDefinitions(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	defs, err := h.engine.ListDefinitions(r.Context(), orgID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,defs)
}

// GetDefinition handles GET /api/v1/organizations/{orgID}/processes/definitions/{defID}.
func (h *ProcessHandler) GetDefinition(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	defID := chi.URLParam(r, "defID")

	def, err := h.engine.GetDefinition(r.Context(), defID, &orgID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,def)
}

// ListInstances handles GET /api/v1/organizations/{orgID}/processes/instances.
func (h *ProcessHandler) ListInstances(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	page := parsePage(r)
	status := queryString(r, "status")

	instances, total, err := h.engine.ListInstances(r.Context(), orgID, status, page.Limit, page.Offset)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,map[string]any{
		"items":  instances,
		"total":  total,
		"limit":  page.Limit,
		"offset": page.Offset,
	})
}

// GetInstance handles GET /api/v1/organizations/{orgID}/processes/instances/{instID}.
func (h *ProcessHandler) GetInstance(w http.ResponseWriter, r *http.Request) {
	instID, err := parseUUID(r, "instID")
	if err != nil {
		respondError(w, err)
		return
	}

	inst, err := h.engine.GetInstance(r.Context(), instID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,inst)
}

// GetByEntity handles GET /api/v1/organizations/{orgID}/processes/entity/{entityID}.
func (h *ProcessHandler) GetByEntity(w http.ResponseWriter, r *http.Request) {
	entityID, err := parseUUID(r, "entityID")
	if err != nil {
		respondError(w, err)
		return
	}

	instances, err := h.engine.GetInstancesByEntity(r.Context(), entityID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,instances)
}

// Trigger handles POST /api/v1/organizations/{orgID}/processes/trigger.
func (h *ProcessHandler) Trigger(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	var input struct {
		EntityID  string          `json:"entity_id"`
		EventType string          `json:"event_type"`
		Data      json.RawMessage `json:"data,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	entityID, err := parseUUIDString(input.EntityID)
	if err != nil {
		respondError(w, err)
		return
	}

	if err := h.engine.TriggerManual(r.Context(), orgID, entityID, input.EventType, input.Data); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, nil)
}

// CreateDefinition handles POST /processes/definitions (visual builder).
func (h *ProcessHandler) CreateDefinition(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	var input struct {
		Name        string              `json:"name"`
		Description string              `json:"description"`
		Definition  dsl.ProcessDefinition `json:"definition"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	now := time.Now()
	def := &process.DefinitionRecord{
		ID:             input.Definition.ID,
		OrganizationID: &orgID,
		Name:           input.Name,
		Description:    input.Description,
		Definition:     input.Definition,
		IsActive:       true,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if def.ID == "" {
		def.ID = uuid.New().String()
		def.Definition.ID = def.ID
	}

	if err := h.engine.SaveDefinition(r.Context(), def); err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, def)
}

// UpdateDefinition handles PUT /processes/definitions/{defID} (visual builder).
func (h *ProcessHandler) UpdateDefinition(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	defID := chi.URLParam(r, "defID")

	existing, err := h.engine.GetDefinition(r.Context(), defID, &orgID)
	if err != nil {
		respondError(w, err)
		return
	}

	var input struct {
		Name        string                `json:"name"`
		Description string                `json:"description"`
		Definition  dsl.ProcessDefinition `json:"definition"`
		IsActive    *bool                 `json:"is_active,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	existing.Name = input.Name
	existing.Description = input.Description
	existing.Definition = input.Definition
	existing.Definition.ID = defID
	existing.Version++
	existing.UpdatedAt = time.Now()
	if input.IsActive != nil {
		existing.IsActive = *input.IsActive
	}

	if err := h.engine.SaveDefinition(r.Context(), existing); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, existing)
}

// DeleteDefinition handles DELETE /processes/definitions/{defID} (visual builder).
func (h *ProcessHandler) DeleteDefinition(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	defID := chi.URLParam(r, "defID")

	if err := h.engine.DeleteDefinition(r.Context(), defID, orgID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "deleted"})
}
