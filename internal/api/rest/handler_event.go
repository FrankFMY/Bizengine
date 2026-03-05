package rest

import (
	"net/http"
	"time"

	"github.com/bizengine/engine/internal/core/event"
)

// EventHandler handles event query endpoints.
type EventHandler struct {
	store event.Store
}

// NewEventHandler creates a new EventHandler.
func NewEventHandler(store event.Store) *EventHandler {
	return &EventHandler{store: store}
}

// List handles GET /api/v1/organizations/{orgID}/events.
func (h *EventHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	page := parsePage(r)
	eventType := queryString(r, "type")
	sinceStr := queryString(r, "since")

	if eventType != nil {
		var since *time.Time
		if sinceStr != nil {
			t, err := time.Parse(time.RFC3339, *sinceStr)
			if err == nil {
				since = &t
			}
		}
		events, err := h.store.GetByType(r.Context(), orgID, *eventType, since, page.Limit)
		if err != nil {
			respondError(w, err)
			return
		}
		respondOK(w, http.StatusOK,events)
		return
	}

	events, total, err := h.store.GetByOrganization(r.Context(), orgID, page.Limit, page.Offset)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,map[string]any{
		"items":  events,
		"total":  total,
		"limit":  page.Limit,
		"offset": page.Offset,
	})
}

// GetByEntity handles GET /api/v1/organizations/{orgID}/events/entity/{entityID}.
func (h *EventHandler) GetByEntity(w http.ResponseWriter, r *http.Request) {
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

	page := parsePage(r)
	var since *time.Time
	if s := queryString(r, "since"); s != nil {
		t, err := time.Parse(time.RFC3339, *s)
		if err == nil {
			since = &t
		}
	}

	events, err := h.store.GetByEntity(r.Context(), orgID, entityID, since, page.Limit)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,events)
}
