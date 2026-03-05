package rest

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/core/entity"
	"github.com/bizengine/engine/internal/module/space"
	"github.com/bizengine/engine/pkg/errs"
)

// EntityHandler handles entity CRUD endpoints.
type EntityHandler struct {
	entitySvc *entity.Service
}

// NewEntityHandler creates a new EntityHandler.
func NewEntityHandler(entitySvc *entity.Service) *EntityHandler {
	return &EntityHandler{entitySvc: entitySvc}
}

// Create handles POST /api/v1/organizations/{orgID}/entities.
func (h *EntityHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input entity.CreateEntityInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	e, err := h.entitySvc.Create(r.Context(), orgID, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, e)
}

// Get handles GET /api/v1/organizations/{orgID}/entities/{id}.
func (h *EntityHandler) Get(w http.ResponseWriter, r *http.Request) {
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

	include := r.URL.Query().Get("include")
	e, err := h.entitySvc.Get(r.Context(), orgID, id, include == "components")
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, e)
}

// List handles GET /api/v1/organizations/{orgID}/entities.
func (h *EntityHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	filter := entity.ListFilter{
		Kind:     queryString(r, "kind"),
		Status:   queryString(r, "status"),
		ParentID: queryUUID(r, "parent_id"),
		Search:   queryString(r, "search"),
		Page:     parsePage(r),
	}

	include := r.URL.Query().Get("include")
	result, err := h.entitySvc.List(r.Context(), orgID, filter, include == "components")
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, result)
}

// Update handles PUT /api/v1/organizations/{orgID}/entities/{id}.
func (h *EntityHandler) Update(w http.ResponseWriter, r *http.Request) {
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
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input entity.UpdateEntityInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	e, err := h.entitySvc.Update(r.Context(), orgID, id, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, e)
}

// Delete handles DELETE /api/v1/organizations/{orgID}/entities/{id}.
func (h *EntityHandler) Delete(w http.ResponseWriter, r *http.Request) {
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
	userID, _ := auth.UserIDFromCtx(r.Context())

	if err := h.entitySvc.Delete(r.Context(), orgID, id, &userID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, nil)
}

// SetComponent handles PUT /api/v1/organizations/{orgID}/entities/{entityID}/components/{type}.
func (h *EntityHandler) SetComponent(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	entityID, err := parseUUID(r, "entityID")
	if err != nil {
		respondError(w, err)
		return
	}
	compType := chi.URLParam(r, "type")
	userID, _ := auth.UserIDFromCtx(r.Context())

	var data json.RawMessage
	if err := decodeJSON(r, &data); err != nil {
		respondError(w, err)
		return
	}

	if compType == "bounds" {
		if err := h.validateBounds(r, orgID, entityID, data); err != nil {
			var ve errs.ValidationErrors
			switch err.(type) {
			case *space.OutOfBoundsError:
				ve = errs.ValidationErrors{}
				ve.Set("bounds", "OUTSIDE_PARENT")
				respondValidation(w, ve)
				return
			case *space.CollisionError:
				ve = errs.ValidationErrors{}
				ve.Set("bounds", "COLLISION")
				respondValidation(w, ve)
				return
			default:
				respondError(w, err)
				return
			}
		}
	}

	c, err := h.entitySvc.SetComponent(r.Context(), orgID, entityID, compType, data, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, c)
}

func (h *EntityHandler) validateBounds(r *http.Request, orgID, entityID uuid.UUID, data json.RawMessage) error {
	var bounds space.AABB
	if err := json.Unmarshal(data, &bounds); err != nil {
		return errs.NewBadRequest("invalid bounds data: " + err.Error())
	}
	if !bounds.Valid() {
		return errs.NewBadRequest("invalid bounds: min must be less than max on all axes")
	}

	e, err := h.entitySvc.Get(r.Context(), orgID, entityID, false)
	if err != nil {
		return err
	}

	newObj := space.Placement{ID: entityID.String(), Box: bounds}

	var parentBounds *space.AABB
	if e.ParentID != nil {
		parentComp, err := h.entitySvc.GetComponent(r.Context(), orgID, *e.ParentID, "bounds")
		if err == nil {
			var pb space.AABB
			if json.Unmarshal(parentComp.Data, &pb) == nil && pb.Valid() {
				parentBounds = &pb
			}
		}
	}

	siblings, err := h.entitySvc.List(r.Context(), orgID, entity.ListFilter{
		ParentID: e.ParentID,
	}, false)
	if err != nil {
		return err
	}

	var sibPlacements []space.Placement
	for _, sib := range siblings.Items {
		if sib.ID == entityID {
			continue
		}
		sibComp, err := h.entitySvc.GetComponent(r.Context(), orgID, sib.ID, "bounds")
		if err != nil {
			continue
		}
		var sb space.AABB
		if json.Unmarshal(sibComp.Data, &sb) == nil && sb.Valid() {
			sibPlacements = append(sibPlacements, space.Placement{ID: sib.ID.String(), Box: sb})
		}
	}

	return space.ValidatePlacement(newObj, parentBounds, sibPlacements)
}

// GetComponent handles GET /api/v1/organizations/{orgID}/entities/{entityID}/components/{type}.
func (h *EntityHandler) GetComponent(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	entityID, err := parseUUID(r, "entityID")
	if err != nil {
		respondError(w, err)
		return
	}
	compType := chi.URLParam(r, "type")

	c, err := h.entitySvc.GetComponent(r.Context(), orgID, entityID, compType)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, c)
}

// ListComponents handles GET /api/v1/organizations/{orgID}/entities/{entityID}/components.
func (h *EntityHandler) ListComponents(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	entityID, err := parseUUID(r, "entityID")
	if err != nil {
		respondError(w, err)
		return
	}

	comps, err := h.entitySvc.ListComponents(r.Context(), orgID, entityID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, comps)
}

// DeleteComponent handles DELETE /api/v1/organizations/{orgID}/entities/{entityID}/components/{type}.
func (h *EntityHandler) DeleteComponent(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	entityID, err := parseUUID(r, "entityID")
	if err != nil {
		respondError(w, err)
		return
	}
	compType := chi.URLParam(r, "type")
	userID, _ := auth.UserIDFromCtx(r.Context())

	if err := h.entitySvc.DeleteComponent(r.Context(), orgID, entityID, compType, &userID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, nil)
}
