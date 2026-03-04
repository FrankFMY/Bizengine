package rest

import (
	"net/http"

	"github.com/bizengine/engine/internal/core/auth"
)

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	authSvc *auth.Service
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authSvc *auth.Service) *AuthHandler {
	return &AuthHandler{authSvc: authSvc}
}

// Register handles POST /api/v1/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var input auth.RegisterInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}

	result, err := h.authSvc.Register(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input auth.LoginInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}

	result, err := h.authSvc.Login(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Refresh handles POST /api/v1/auth/refresh.
func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}

	result, err := h.authSvc.Refresh(r.Context(), input.RefreshToken)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// Logout handles POST /api/v1/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}

	if err := h.authSvc.Logout(r.Context(), input.RefreshToken); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// SwitchWorkspace handles POST /api/v1/auth/switch.
func (h *AuthHandler) SwitchWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromCtx(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"code": "UNAUTHORIZED", "message": "not authenticated"})
		return
	}

	var input struct {
		WorkspaceID string `json:"workspace_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}

	wsID, err := parseUUIDString(input.WorkspaceID)
	if err != nil {
		writeError(w, err)
		return
	}

	token, err := h.authSvc.SwitchWorkspace(r.Context(), userID, wsID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"access_token": token})
}
