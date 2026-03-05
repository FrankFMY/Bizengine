package rest

import (
	"net/http"

	"github.com/bizengine/engine/internal/webhook"
)

type WebhookHandler struct {
	svc *webhook.Service
}

func NewWebhookHandler(svc *webhook.Service) *WebhookHandler {
	return &WebhookHandler{svc: svc}
}

// Create handles POST /webhooks.
func (h *WebhookHandler) Create(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	var input webhook.CreateWebhookInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	hook, err := h.svc.Create(r.Context(), orgID, input.URL, input.Events, input.Secret)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, hook)
}

// List handles GET /webhooks.
func (h *WebhookHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	hooks, err := h.svc.List(r.Context(), orgID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, hooks)
}

// Delete handles DELETE /webhooks/{id}.
func (h *WebhookHandler) Delete(w http.ResponseWriter, r *http.Request) {
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

	if err := h.svc.Delete(r.Context(), orgID, id); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Toggle handles POST /webhooks/{id}/toggle.
func (h *WebhookHandler) Toggle(w http.ResponseWriter, r *http.Request) {
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

	var input struct {
		Active bool `json:"active"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	hook, err := h.svc.Toggle(r.Context(), orgID, id, input.Active)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, hook)
}

// Test handles POST /webhooks/{id}/test.
func (h *WebhookHandler) Test(w http.ResponseWriter, r *http.Request) {
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

	if err := h.svc.Test(r.Context(), orgID, id); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "delivered"})
}
