package rest

import (
	"net/http"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/notification"
)

type NotificationHandler struct {
	notifSvc *notification.Service
}

func NewNotificationHandler(notifSvc *notification.Service) *NotificationHandler {
	return &NotificationHandler{notifSvc: notifSvc}
}

func (h *NotificationHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	filter := notification.ListFilter{
		Read: queryBool(r, "read"),
		Page: parsePage(r),
	}

	items, total, err := h.notifSvc.ListNotifications(r.Context(), orgID, userID, filter)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]any{
		"items": items,
		"total": total,
	})
}

func (h *NotificationHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	notifID, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	if err := h.notifSvc.MarkRead(r.Context(), orgID, userID, notifID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, nil)
}

func (h *NotificationHandler) MarkAllRead(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	if err := h.notifSvc.MarkAllRead(r.Context(), orgID, userID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, nil)
}

func (h *NotificationHandler) CountUnread(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	count, err := h.notifSvc.CountUnread(r.Context(), orgID, userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]any{
		"unread": count,
	})
}
