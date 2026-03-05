package rest

import (
	"net/http"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/module/settings"
)

// SettingsHandler handles settings endpoints.
type SettingsHandler struct {
	settingsSvc *settings.Service
}

// NewSettingsHandler creates a new SettingsHandler.
func NewSettingsHandler(settingsSvc *settings.Service) *SettingsHandler {
	return &SettingsHandler{settingsSvc: settingsSvc}
}

// GetSettings handles GET /api/v1/organizations/{orgID}/settings.
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	s, err := h.settingsSvc.Get(r.Context(), orgID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, s)
}

// UpdateSettings handles PUT /api/v1/organizations/{orgID}/settings.
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input settings.UpdateInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	s, err := h.settingsSvc.Update(r.Context(), orgID, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, s)
}

// UpdateIntegrations handles PUT /api/v1/organizations/{orgID}/settings/integrations.
func (h *SettingsHandler) UpdateIntegrations(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input settings.UpdateIntegrationsInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	s, err := h.settingsSvc.UpdateIntegrations(r.Context(), orgID, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, s)
}

// SetLogo handles PUT /api/v1/organizations/{orgID}/settings/logo.
func (h *SettingsHandler) SetLogo(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input struct {
		FileID string `json:"file_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	fileID, err := parseUUIDString(input.FileID)
	if err != nil {
		respondError(w, err)
		return
	}

	s, err := h.settingsSvc.SetLogo(r.Context(), orgID, fileID, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, s)
}
