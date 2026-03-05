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

// OrganizationSeedFunc is called after an organization is created to seed initial data.
type OrganizationSeedFunc func(ctx context.Context, orgID uuid.UUID) error

// OrganizationHandler handles organization endpoints.
type OrganizationHandler struct {
	authSvc *auth.Service
	repo    auth.Repository
	seeders []OrganizationSeedFunc
}

// NewOrganizationHandler creates a new OrganizationHandler.
func NewOrganizationHandler(authSvc *auth.Service, repo auth.Repository, seeders ...OrganizationSeedFunc) *OrganizationHandler {
	return &OrganizationHandler{authSvc: authSvc, repo: repo, seeders: seeders}
}

// Create handles POST /api/v1/organizations.
func (h *OrganizationHandler) Create(w http.ResponseWriter, r *http.Request) {
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

	org, err := h.authSvc.CreateOrganization(r.Context(), userID, input.Name, input.Slug)
	if err != nil {
		respondError(w, err)
		return
	}

	for _, seed := range h.seeders {
		if err := seed(r.Context(), org.ID); err != nil {
			log.Error().Err(err).Str("organization_id", org.ID.String()).Msg("organization seed failed")
		}
	}

	respondCreated(w,org)
}

// List handles GET /api/v1/organizations.
func (h *OrganizationHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromCtx(r.Context())
	if !ok {
		respondError(w, errs.NewUnauthorized("not authenticated"))
		return
	}

	organizations, err := h.repo.ListUserOrganizations(r.Context(), userID)
	if err != nil {
		respondError(w, err)
		return
	}

	if organizations == nil {
		organizations = []types.Organization{}
	}

	respondOK(w, http.StatusOK,organizations)
}

// Get handles GET /api/v1/organizations/{orgID}.
func (h *OrganizationHandler) Get(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	org, err := h.repo.GetOrganization(r.Context(), orgID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,org)
}

// Update handles PUT /api/v1/organizations/{orgID}.
func (h *OrganizationHandler) Update(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	org, err := h.repo.GetOrganization(r.Context(), orgID)
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
		org.Name = *input.Name
	}
	if input.Settings != nil {
		org.Settings = input.Settings
	}

	if err := h.repo.UpdateOrganization(r.Context(), org); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,org)
}

// AddMember handles POST /api/v1/organizations/{orgID}/members.
func (h *OrganizationHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
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

	member := &types.Labor{
		OrganizationID: orgID,
		UserID:      user.ID,
		Role:        input.Role,
	}
	if err := h.repo.AddMember(r.Context(), member); err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w,member)
}

// ListMembers handles GET /api/v1/organizations/{orgID}/members.
func (h *OrganizationHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	members, err := h.repo.ListMembers(r.Context(), orgID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,members)
}

// UpdateMember handles PUT /api/v1/organizations/{orgID}/members/{userID}.
func (h *OrganizationHandler) UpdateMember(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
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

	if err := h.repo.UpdateMemberRole(r.Context(), orgID, userID, input.Role); err != nil {
		respondError(w, err)
		return
	}

	member, _ := h.repo.GetMember(r.Context(), orgID, userID)
	respondOK(w, http.StatusOK,member)
}

// RemoveMember handles DELETE /api/v1/organizations/{orgID}/members/{userID}.
func (h *OrganizationHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, err := parseUUID(r, "userID")
	if err != nil {
		respondError(w, err)
		return
	}

	if err := h.repo.RemoveMember(r.Context(), orgID, userID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, nil)
}
