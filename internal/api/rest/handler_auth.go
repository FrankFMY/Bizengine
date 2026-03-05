package rest

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/pkg/errs"
)

// Disconnector disconnects a user's real-time connections.
type Disconnector interface {
	Disconnect(ctx context.Context, userID string) error
}

// AuthHandler handles authentication endpoints.
type AuthHandler struct {
	authSvc      *auth.Service
	cookieSecure bool
	disconnector Disconnector
	viewUnsub    ViewUnsubscriber
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(authSvc *auth.Service, cookieSecure bool, disconnector Disconnector, viewUnsub ViewUnsubscriber) *AuthHandler {
	return &AuthHandler{authSvc: authSvc, cookieSecure: cookieSecure, disconnector: disconnector, viewUnsub: viewUnsub}
}

// Register handles POST /api/v1/auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var input auth.RegisterInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	result, err := h.authSvc.Register(r.Context(), input)
	if err != nil {
		respondError(w, err)
		return
	}

	auth.SetAuthCookies(w, result.Session, result.Seance, h.authSvc.SessionTTL(), h.authSvc.SeanceTTL(), h.cookieSecure)

	respondCreated(w, map[string]any{
		"user": result.User,
	})
}

// Login handles POST /api/v1/auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var input auth.LoginInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	result, err := h.authSvc.Login(r.Context(), input)
	if err != nil {
		respondError(w, err)
		return
	}

	auth.SetAuthCookies(w, result.Session, result.Seance, h.authSvc.SessionTTL(), h.authSvc.SeanceTTL(), h.cookieSecure)

	respondOK(w, http.StatusOK, map[string]any{
		"user":          result.User,
		"organizations": result.Organizations,
	})
}

// Logout handles POST /api/v1/auth/logout.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := auth.SessionIDFromCtx(r.Context())
	if !ok {
		respondError(w, errs.NewUnauthorized("no session"))
		return
	}

	userID, _ := auth.UserIDFromCtx(r.Context())
	seanceID, _ := auth.SeanceIDFromCtx(r.Context())

	if err := h.authSvc.Logout(r.Context(), sessionID); err != nil {
		respondError(w, err)
		return
	}

	if h.viewUnsub != nil && seanceID != "" {
		h.viewUnsub.UnsubscribeAll(seanceID)
	}

	if h.disconnector != nil {
		h.disconnector.Disconnect(r.Context(), userID.String())
	}

	auth.ClearAuthCookies(w)
	respondOK(w, http.StatusOK, nil)
}

// Unlock handles POST /api/v1/auth/unlock.
func (h *AuthHandler) Unlock(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := auth.SessionIDFromCtx(r.Context())
	if !ok {
		respondError(w, errs.NewUnauthorized("no session"))
		return
	}

	var input struct {
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	seance, err := h.authSvc.Unlock(r.Context(), sessionID, input.Password)
	if err != nil {
		respondError(w, err)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "teco_seance",
		Value:    seance.ID,
		Path:     "/",
		MaxAge:   int(h.authSvc.SeanceTTL().Seconds()),
		HttpOnly: true,
		Secure:   h.cookieSecure,
		SameSite: http.SameSiteLaxMode,
	})

	respondOK(w, http.StatusOK, map[string]string{"status": "unlocked"})
}

// Check handles GET /api/v1/auth/check.
func (h *AuthHandler) Check(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := auth.SessionIDFromCtx(r.Context())
	if !ok {
		respondError(w, errs.NewUnauthorized("no session"))
		return
	}

	seanceID, _ := auth.SeanceIDFromCtx(r.Context())

	sess, err := h.authSvc.GetSessionInfo(r.Context(), sessionID)
	if err != nil {
		respondError(w, err)
		return
	}

	isAdmin := sess.Role == "owner" || sess.Role == "admin"
	result := map[string]any{
		"seance_id":       seanceID,
		"session_id":      sess.ID,
		"user_id":         sess.UserID,
		"organization_id": sess.OrganizationID,
		"admin":           isAdmin,
	}
	if sess.PhoneID != (uuid.UUID{}) {
		result["phone_id"] = sess.PhoneID
	}
	respondOK(w, http.StatusOK, result)
}

// SwitchOrganization handles POST /api/v1/auth/switch.
func (h *AuthHandler) SwitchOrganization(w http.ResponseWriter, r *http.Request) {
	sessionID, ok := auth.SessionIDFromCtx(r.Context())
	if !ok {
		respondError(w, errs.NewUnauthorized("not authenticated"))
		return
	}

	var input struct {
		OrganizationID string `json:"organization_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	orgID, err := parseUUIDString(input.OrganizationID)
	if err != nil {
		respondError(w, err)
		return
	}

	sess, err := h.authSvc.SwitchOrganization(r.Context(), sessionID, orgID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]any{
		"organization_id": sess.OrganizationID,
		"role":            sess.Role,
	})
}
