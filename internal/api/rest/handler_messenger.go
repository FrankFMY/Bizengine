package rest

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/module/messenger"
	"github.com/bizengine/engine/pkg/errs"
)

// MessengerHandler handles messenger endpoints.
type MessengerHandler struct {
	svc *messenger.Service
}

// NewMessengerHandler creates a new MessengerHandler.
func NewMessengerHandler(svc *messenger.Service) *MessengerHandler {
	return &MessengerHandler{svc: svc}
}

// CreateConversation handles POST /messenger/conversations.
func (h *MessengerHandler) CreateConversation(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input struct {
		Type          string      `json:"type"`
		PeerID        *uuid.UUID  `json:"peer_id,omitempty"`
		Name          string      `json:"name,omitempty"`
		MemberIDs     []uuid.UUID `json:"member_ids,omitempty"`
		ReferenceType string      `json:"reference_type,omitempty"`
		ReferenceID   *uuid.UUID  `json:"reference_id,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	switch input.Type {
	case "direct":
		if input.PeerID == nil {
			respondError(w, errs.NewBadRequest("peer_id is required for direct conversations"))
			return
		}
		conv, err := h.svc.CreateDirectConversation(r.Context(), orgID, userID, *input.PeerID)
		if err != nil {
			respondError(w, err)
			return
		}
		respondCreated(w, conv)

	case "group":
		conv, err := h.svc.CreateGroupConversation(r.Context(), orgID, userID, input.Name, input.MemberIDs)
		if err != nil {
			respondError(w, err)
			return
		}
		respondCreated(w, conv)

	case "entity":
		if input.ReferenceType == "" || input.ReferenceID == nil {
			respondError(w, errs.NewBadRequest("reference_type and reference_id required for entity conversations"))
			return
		}
		conv, err := h.svc.GetOrCreateEntityConversation(r.Context(), orgID, input.ReferenceType, *input.ReferenceID, userID)
		if err != nil {
			respondError(w, err)
			return
		}
		respondCreated(w, conv)

	default:
		respondError(w, errs.NewBadRequest("type must be direct, group, or entity"))
	}
}

// ListConversations handles GET /messenger/conversations.
func (h *MessengerHandler) ListConversations(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}

	filter := messenger.ConversationFilter{
		Type:   r.URL.Query().Get("type"),
		Search: r.URL.Query().Get("search"),
		Limit:  limit,
		Offset: offset,
	}

	convs, total, err := h.svc.ListConversations(r.Context(), orgID, userID, filter)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]any{
		"items":  convs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetConversation handles GET /messenger/conversations/{convID}.
func (h *MessengerHandler) GetConversation(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	convID, err := parseUUID(r, "convID")
	if err != nil {
		respondError(w, err)
		return
	}

	conv, err := h.svc.GetConversation(r.Context(), orgID, convID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, conv)
}

// AddMember handles POST /messenger/conversations/{convID}/members.
func (h *MessengerHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	convID, err := parseUUID(r, "convID")
	if err != nil {
		respondError(w, err)
		return
	}

	var input struct {
		UserID uuid.UUID `json:"user_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	if err := h.svc.AddMember(r.Context(), convID, input.UserID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "added"})
}

// RemoveMember handles DELETE /messenger/conversations/{convID}/members/{userID}.
func (h *MessengerHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	convID, err := parseUUID(r, "convID")
	if err != nil {
		respondError(w, err)
		return
	}
	memberUserID, err := parseUUID(r, "userID")
	if err != nil {
		respondError(w, err)
		return
	}

	if err := h.svc.RemoveMember(r.Context(), convID, memberUserID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "removed"})
}

// MuteConversation handles POST /messenger/conversations/{convID}/mute.
func (h *MessengerHandler) MuteConversation(w http.ResponseWriter, r *http.Request) {
	convID, err := parseUUID(r, "convID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input struct {
		Muted bool `json:"muted"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	if err := h.svc.MuteConversation(r.Context(), convID, userID, input.Muted); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]bool{"muted": input.Muted})
}

// SendMessage handles POST /messenger/conversations/{convID}/messages.
func (h *MessengerHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	convID, err := parseUUID(r, "convID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input struct {
		Content     string                 `json:"content"`
		ContentType string                 `json:"content_type"`
		Attachments []messenger.Attachment `json:"attachments"`
		ReplyToID   *uuid.UUID             `json:"reply_to_id,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	msg, err := h.svc.SendMessage(r.Context(), orgID, messenger.SendMessageRequest{
		ConversationID: convID,
		SenderID:       userID,
		Content:        input.Content,
		ContentType:    input.ContentType,
		Attachments:    input.Attachments,
		ReplyToID:      input.ReplyToID,
	})
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, msg)
}

// EditMessage handles PUT /messenger/conversations/{convID}/messages/{msgID}.
func (h *MessengerHandler) EditMessage(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	msgID, err := parseUUID(r, "msgID")
	if err != nil {
		respondError(w, err)
		return
	}

	var input struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	msg, err := h.svc.EditMessage(r.Context(), orgID, msgID, input.Content)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, msg)
}

// DeleteMessage handles DELETE /messenger/conversations/{convID}/messages/{msgID}.
func (h *MessengerHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	msgID, err := parseUUID(r, "msgID")
	if err != nil {
		respondError(w, err)
		return
	}

	if err := h.svc.DeleteMessage(r.Context(), orgID, msgID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ListMessages handles GET /messenger/conversations/{convID}/messages.
func (h *MessengerHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	convID, err := parseUUID(r, "convID")
	if err != nil {
		respondError(w, err)
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}

	before := queryUUID(r, "before")

	msgs, err := h.svc.ListMessages(r.Context(), orgID, convID, before, limit)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]any{
		"items": msgs,
		"limit": limit,
	})
}

// MarkRead handles POST /messenger/conversations/{convID}/read.
func (h *MessengerHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	convID, err := parseUUID(r, "convID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input struct {
		MessageID uuid.UUID `json:"message_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	if err := h.svc.MarkRead(r.Context(), convID, userID, input.MessageID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "read"})
}

// GetUnreadCount handles GET /messenger/unread-count.
func (h *MessengerHandler) GetUnreadCount(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	counts, err := h.svc.GetUnreadCount(r.Context(), orgID, userID)
	if err != nil {
		respondError(w, err)
		return
	}

	total := 0
	for _, c := range counts {
		total += c
	}

	respondOK(w, http.StatusOK, map[string]any{
		"total":          total,
		"conversations":  counts,
	})
}

// AddReaction handles POST /messenger/messages/{msgID}/reactions.
func (h *MessengerHandler) AddReaction(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	msgID, err := parseUUID(r, "msgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input struct {
		Emoji string `json:"emoji"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	if err := h.svc.AddReaction(r.Context(), orgID, msgID, userID, input.Emoji); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "added"})
}

// RemoveReaction handles DELETE /messenger/messages/{msgID}/reactions/{emoji}.
func (h *MessengerHandler) RemoveReaction(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	msgID, err := parseUUID(r, "msgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())
	emoji := chi.URLParam(r, "emoji")

	if err := h.svc.RemoveReaction(r.Context(), orgID, msgID, userID, emoji); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "removed"})
}

// SearchMessages handles GET /messenger/search.
func (h *MessengerHandler) SearchMessages(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		respondError(w, errs.NewBadRequest("q is required"))
		return
	}

	limit := 20
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}

	convID := queryUUID(r, "conversation_id")

	msgs, err := h.svc.SearchMessages(r.Context(), orgID, query, convID, limit)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]any{
		"items": msgs,
		"limit": limit,
	})
}

// SetTyping handles POST /messenger/conversations/{convID}/typing.
func (h *MessengerHandler) SetTyping(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	convID, err := parseUUID(r, "convID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	_ = h.svc.SetTyping(r.Context(), orgID, convID, userID)

	respondOK(w, http.StatusOK, map[string]string{"status": "ok"})
}
