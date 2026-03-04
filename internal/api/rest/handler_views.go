package rest

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/views"
	"github.com/bizengine/engine/pkg/errs"
)

// ViewManager is the interface consumed by the views handler.
type ViewManager interface {
	Subscribe(ctx context.Context, seanceID string, wsID, userID uuid.UUID, viewKey string, params map[string]any) (*views.SubscribeResult, error)
	Unsubscribe(seanceID, paramsHash string)
	ActiveSubscriptions(seanceID string) []views.ActiveSub
	Sync(ctx context.Context, seanceID string, wsID uuid.UUID, req views.SyncRequest) error
}

// ViewsHandler handles view subscription REST endpoints.
type ViewsHandler struct {
	mgr ViewManager
}

// NewViewsHandler creates a new ViewsHandler.
func NewViewsHandler(mgr ViewManager) *ViewsHandler {
	return &ViewsHandler{mgr: mgr}
}

// Subscribe handles POST /api/v1/workspaces/{wsID}/views/subscribe.
func (h *ViewsHandler) Subscribe(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	seanceID, ok := auth.SeanceIDFromCtx(r.Context())
	if !ok {
		respondError(w, errs.NewUnauthorized("no seance"))
		return
	}

	userID, _ := auth.UserIDFromCtx(r.Context())

	var input struct {
		View   string         `json:"view"`
		Params map[string]any `json:"params"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	if input.View == "" {
		respondError(w, errs.NewBadRequest("view is required"))
		return
	}

	result, err := h.mgr.Subscribe(r.Context(), seanceID, wsID, userID, input.View, input.Params)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, result)
}

// Unsubscribe handles POST /api/v1/workspaces/{wsID}/views/unsubscribe.
func (h *ViewsHandler) Unsubscribe(w http.ResponseWriter, r *http.Request) {
	seanceID, ok := auth.SeanceIDFromCtx(r.Context())
	if !ok {
		respondError(w, errs.NewUnauthorized("no seance"))
		return
	}

	var input struct {
		View       string `json:"view"`
		ParamsHash string `json:"params_hash"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	if input.ParamsHash == "" {
		respondError(w, errs.NewBadRequest("params_hash is required"))
		return
	}

	h.mgr.Unsubscribe(seanceID, input.ParamsHash)
	respondOK(w, http.StatusOK, nil)
}

// Active handles GET /api/v1/workspaces/{wsID}/views/active.
func (h *ViewsHandler) Active(w http.ResponseWriter, r *http.Request) {
	seanceID, ok := auth.SeanceIDFromCtx(r.Context())
	if !ok {
		respondError(w, errs.NewUnauthorized("no seance"))
		return
	}

	subs := h.mgr.ActiveSubscriptions(seanceID)
	respondOK(w, http.StatusOK, map[string]any{"views": subs})
}

// Sync handles POST /api/v1/workspaces/{wsID}/views/sync.
func (h *ViewsHandler) Sync(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	seanceID, ok := auth.SeanceIDFromCtx(r.Context())
	if !ok {
		respondError(w, errs.NewUnauthorized("no seance"))
		return
	}

	var input views.SyncRequest
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	if err := h.mgr.Sync(r.Context(), seanceID, wsID, input); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]bool{"synced": true})
}
