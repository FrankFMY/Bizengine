package rest

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// WorkspaceSeedFunc is called after a workspace is created to seed initial data.
type WorkspaceSeedFunc func(ctx context.Context, wsID uuid.UUID) error

// WorkspaceHandler handles workspace endpoints.
type WorkspaceHandler struct {
	authSvc *auth.Service
	repo    auth.Repository
	seeders []WorkspaceSeedFunc
}

// NewWorkspaceHandler creates a new WorkspaceHandler.
func NewWorkspaceHandler(authSvc *auth.Service, repo auth.Repository, seeders ...WorkspaceSeedFunc) *WorkspaceHandler {
	return &WorkspaceHandler{authSvc: authSvc, repo: repo, seeders: seeders}
}

// Create handles POST /api/v1/workspaces.
func (h *WorkspaceHandler) Create(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromCtx(r.Context())
	if !ok {
		respondError(w, errs.NewUnauthorized("not authenticated"))
		return
	}

	var input struct {
		Name string `json:"name"`
		Slug string `json:"slug"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	ws, err := h.authSvc.CreateWorkspace(r.Context(), userID, input.Name, input.Slug)
	if err != nil {
		respondError(w, err)
		return
	}

	for _, seed := range h.seeders {
		if err := seed(r.Context(), ws.ID); err != nil {
			log.Error().Err(err).Str("workspace_id", ws.ID.String()).Msg("workspace seed failed")
		}
	}

	respondCreated(w,ws)
}

// List handles GET /api/v1/workspaces.
func (h *WorkspaceHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromCtx(r.Context())
	if !ok {
		respondError(w, errs.NewUnauthorized("not authenticated"))
		return
	}

	workspaces, err := h.repo.ListUserWorkspaces(r.Context(), userID)
	if err != nil {
		respondError(w, err)
		return
	}

	if workspaces == nil {
		workspaces = []types.Workspace{}
	}

	respondOK(w, http.StatusOK,workspaces)
}

// Get handles GET /api/v1/workspaces/{wsID}.
func (h *WorkspaceHandler) Get(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	ws, err := h.repo.GetWorkspace(r.Context(), wsID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,ws)
}

// Update handles PUT /api/v1/workspaces/{wsID}.
func (h *WorkspaceHandler) Update(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	ws, err := h.repo.GetWorkspace(r.Context(), wsID)
	if err != nil {
		respondError(w, err)
		return
	}

	var input struct {
		Name     *string         `json:"name"`
		Settings json.RawMessage `json:"settings"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	if input.Name != nil {
		ws.Name = *input.Name
	}
	if input.Settings != nil {
		ws.Settings = input.Settings
	}

	if err := h.repo.UpdateWorkspace(r.Context(), ws); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,ws)
}

// AddMember handles POST /api/v1/workspaces/{wsID}/members.
func (h *WorkspaceHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	var input struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	user, err := h.repo.GetUserByEmail(r.Context(), input.Email)
	if err != nil {
		respondError(w, err)
		return
	}

	member := &types.WorkspaceMember{
		WorkspaceID: wsID,
		UserID:      user.ID,
		Role:        input.Role,
	}
	if err := h.repo.AddMember(r.Context(), member); err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w,member)
}

// ListMembers handles GET /api/v1/workspaces/{wsID}/members.
func (h *WorkspaceHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	members, err := h.repo.ListMembers(r.Context(), wsID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,members)
}

// UpdateMember handles PUT /api/v1/workspaces/{wsID}/members/{userID}.
func (h *WorkspaceHandler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, err := parseUUID(r, "userID")
	if err != nil {
		respondError(w, err)
		return
	}

	var input struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	if err := h.repo.UpdateMemberRole(r.Context(), wsID, userID, input.Role); err != nil {
		respondError(w, err)
		return
	}

	member, _ := h.repo.GetMember(r.Context(), wsID, userID)
	respondOK(w, http.StatusOK,member)
}

// RemoveMember handles DELETE /api/v1/workspaces/{wsID}/members/{userID}.
func (h *WorkspaceHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, err := parseUUID(r, "userID")
	if err != nil {
		respondError(w, err)
		return
	}

	if err := h.repo.RemoveMember(r.Context(), wsID, userID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, nil)
}
