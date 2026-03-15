package messenger

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"

	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// Service provides messenger operations.
type Service struct {
	repo Repository
	bus  event.Bus
}

// NewService creates a new messenger service.
func NewService(repo Repository, bus event.Bus) *Service {
	return &Service{repo: repo, bus: bus}
}

// CreateDirectConversation creates or returns an existing direct conversation.
func (s *Service) CreateDirectConversation(ctx context.Context, orgID, userID, peerID uuid.UUID) (*Conversation, error) {
	if userID == peerID {
		return nil, errs.NewBadRequest("cannot create direct conversation with yourself")
	}

	existing, err := s.repo.FindDirectConversation(ctx, orgID, userID, peerID)
	if err == nil && existing != nil {
		return existing, nil
	}

	conv := &Conversation{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Type:           "direct",
		CreatedBy:      userID,
	}

	err = s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.CreateConversation(ctx, tx, conv); err != nil {
			return err
		}
		if err := s.repo.AddMember(ctx, tx, conv.ID, userID, "member"); err != nil {
			return err
		}
		return s.repo.AddMember(ctx, tx, conv.ID, peerID, "member")
	})
	if err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, "messenger.conversation.created", map[string]any{
		"conversation_id": conv.ID,
		"type":            "direct",
	})

	return conv, nil
}

// CreateGroupConversation creates a new group conversation.
func (s *Service) CreateGroupConversation(ctx context.Context, orgID, creatorID uuid.UUID, name string, memberIDs []uuid.UUID) (*Conversation, error) {
	if name == "" {
		return nil, errs.NewBadRequest("name is required for group conversations")
	}

	conv := &Conversation{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Type:           "group",
		Name:           &name,
		CreatedBy:      creatorID,
	}

	err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.CreateConversation(ctx, tx, conv); err != nil {
			return err
		}
		if err := s.repo.AddMember(ctx, tx, conv.ID, creatorID, "admin"); err != nil {
			return err
		}
		for _, mid := range memberIDs {
			if mid == creatorID {
				continue
			}
			if err := s.repo.AddMember(ctx, tx, conv.ID, mid, "member"); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, "messenger.conversation.created", map[string]any{
		"conversation_id": conv.ID,
		"type":            "group",
		"name":            name,
	})

	return conv, nil
}

// GetOrCreateEntityConversation returns or creates a conversation for a business entity.
func (s *Service) GetOrCreateEntityConversation(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID, creatorID uuid.UUID) (*Conversation, error) {
	existing, err := s.repo.FindEntityConversation(ctx, orgID, refType, refID)
	if err == nil && existing != nil {
		return existing, nil
	}

	name := refType + " discussion"
	conv := &Conversation{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Type:           "entity",
		Name:           &name,
		ReferenceType:  &refType,
		ReferenceID:    &refID,
		CreatedBy:      creatorID,
	}

	err = s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.CreateConversation(ctx, tx, conv); err != nil {
			return err
		}
		return s.repo.AddMember(ctx, tx, conv.ID, creatorID, "admin")
	})
	if err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, "messenger.conversation.created", map[string]any{
		"conversation_id": conv.ID,
		"type":            "entity",
		"reference_type":  refType,
		"reference_id":    refID,
	})

	return conv, nil
}

// GetConversation returns a conversation by ID.
func (s *Service) GetConversation(ctx context.Context, orgID, convID uuid.UUID) (*Conversation, error) {
	return s.repo.GetConversation(ctx, orgID, convID)
}

// ListConversations returns conversations the user is a member of.
func (s *Service) ListConversations(ctx context.Context, orgID, userID uuid.UUID, filter ConversationFilter) ([]ConversationWithUnread, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	}
	if filter.Limit > 200 {
		filter.Limit = 200
	}
	return s.repo.ListConversations(ctx, orgID, userID, filter)
}

// AddMember adds a user to a conversation.
func (s *Service) AddMember(ctx context.Context, convID, userID uuid.UUID) error {
	err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		return s.repo.AddMember(ctx, tx, convID, userID, "member")
	})
	if err != nil {
		return err
	}

	conv, _ := s.repo.GetConversation(ctx, uuid.Nil, convID)
	if conv != nil {
		s.publishConvEvent(ctx, conv.OrganizationID, convID, "messenger.member.added", map[string]any{
			"conversation_id": convID,
			"user_id":         userID,
		})
	}
	return nil
}

// RemoveMember removes a user from a conversation.
func (s *Service) RemoveMember(ctx context.Context, convID, userID uuid.UUID) error {
	if err := s.repo.RemoveMember(ctx, convID, userID); err != nil {
		return err
	}

	conv, _ := s.repo.GetConversation(ctx, uuid.Nil, convID)
	if conv != nil {
		s.publishConvEvent(ctx, conv.OrganizationID, convID, "messenger.member.removed", map[string]any{
			"conversation_id": convID,
			"user_id":         userID,
		})
	}
	return nil
}

// MuteConversation toggles muting for a user.
func (s *Service) MuteConversation(ctx context.Context, convID, userID uuid.UUID, muted bool) error {
	return s.repo.SetMuted(ctx, convID, userID, muted)
}

// SendMessage sends a message to a conversation.
func (s *Service) SendMessage(ctx context.Context, orgID uuid.UUID, req SendMessageRequest) (*Message, error) {
	if req.Content == "" && len(req.Attachments) == 0 {
		return nil, errs.NewBadRequest("content or attachments required")
	}

	isMember, err := s.repo.IsMember(ctx, req.ConversationID, req.SenderID)
	if err != nil {
		return nil, err
	}
	if !isMember {
		return nil, errs.NewForbidden("not a member of this conversation")
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = "text"
	}

	msg := &Message{
		ID:             uuid.New(),
		ConversationID: req.ConversationID,
		OrganizationID: orgID,
		SenderID:       req.SenderID,
		Content:        &req.Content,
		ContentType:    contentType,
		Attachments:    req.Attachments,
		ReplyToID:      req.ReplyToID,
	}
	if msg.Attachments == nil {
		msg.Attachments = []Attachment{}
	}

	now := time.Now()
	preview := req.Content
	if len(preview) > 100 {
		preview = preview[:100]
	}

	err = s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.CreateMessage(ctx, tx, msg); err != nil {
			return err
		}
		return s.repo.UpdateLastMessage(ctx, tx, req.ConversationID, msg.ID, preview, now)
	})
	if err != nil {
		return nil, err
	}

	s.publishConvEvent(ctx, orgID, req.ConversationID, "messenger.message.sent", map[string]any{
		"conversation_id": req.ConversationID,
		"message_id":      msg.ID,
		"sender_id":       req.SenderID,
		"content_type":    contentType,
		"preview":         preview,
	})

	return msg, nil
}

// SendSystemMessage sends a system message to an entity conversation.
func (s *Service) SendSystemMessage(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID, content string) error {
	conv, err := s.repo.FindEntityConversation(ctx, orgID, refType, refID)
	if err != nil || conv == nil {
		return nil
	}

	msg := &Message{
		ID:             uuid.New(),
		ConversationID: conv.ID,
		OrganizationID: orgID,
		SenderID:       conv.CreatedBy,
		Content:        &content,
		ContentType:    "system",
		Attachments:    []Attachment{},
	}

	now := time.Now()
	preview := content
	if len(preview) > 100 {
		preview = preview[:100]
	}

	err = s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.CreateMessage(ctx, tx, msg); err != nil {
			return err
		}
		return s.repo.UpdateLastMessage(ctx, tx, conv.ID, msg.ID, preview, now)
	})
	if err != nil {
		return err
	}

	s.publishConvEvent(ctx, orgID, conv.ID, "messenger.message.sent", map[string]any{
		"conversation_id": conv.ID,
		"message_id":      msg.ID,
		"content_type":    "system",
		"preview":         preview,
	})

	return nil
}

// EditMessage edits an existing message.
func (s *Service) EditMessage(ctx context.Context, orgID, messageID uuid.UUID, content string) (*Message, error) {
	if content == "" {
		return nil, errs.NewBadRequest("content is required")
	}

	msg, err := s.repo.GetMessage(ctx, orgID, messageID)
	if err != nil {
		return nil, err
	}

	if err := s.repo.UpdateMessage(ctx, messageID, content); err != nil {
		return nil, err
	}

	msg.Content = &content
	now := time.Now()
	msg.EditedAt = &now

	s.publishConvEvent(ctx, orgID, msg.ConversationID, "messenger.message.edited", map[string]any{
		"conversation_id": msg.ConversationID,
		"message_id":      messageID,
		"content":         content,
	})

	return msg, nil
}

// DeleteMessage soft-deletes a message.
func (s *Service) DeleteMessage(ctx context.Context, orgID, messageID uuid.UUID) error {
	msg, err := s.repo.GetMessage(ctx, orgID, messageID)
	if err != nil {
		return err
	}

	if err := s.repo.SoftDeleteMessage(ctx, messageID); err != nil {
		return err
	}

	s.publishConvEvent(ctx, orgID, msg.ConversationID, "messenger.message.deleted", map[string]any{
		"conversation_id": msg.ConversationID,
		"message_id":      messageID,
	})

	return nil
}

// ListMessages returns messages in a conversation.
func (s *Service) ListMessages(ctx context.Context, orgID, convID uuid.UUID, before *uuid.UUID, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	return s.repo.ListMessages(ctx, orgID, convID, before, limit)
}

// MarkRead marks messages as read up to a given message ID.
func (s *Service) MarkRead(ctx context.Context, convID, userID, messageID uuid.UUID) error {
	return s.repo.MarkRead(ctx, convID, userID, messageID)
}

// GetUnreadCount returns unread counts per conversation.
func (s *Service) GetUnreadCount(ctx context.Context, orgID, userID uuid.UUID) (map[uuid.UUID]int, error) {
	return s.repo.GetUnreadCounts(ctx, orgID, userID)
}

// AddReaction adds a reaction to a message.
func (s *Service) AddReaction(ctx context.Context, orgID uuid.UUID, messageID, userID uuid.UUID, emoji string) error {
	emoji = strings.TrimSpace(emoji)
	if emoji == "" {
		return errs.NewBadRequest("emoji is required")
	}

	if err := s.repo.AddReaction(ctx, messageID, userID, emoji); err != nil {
		return err
	}

	msg, _ := s.repo.GetMessage(ctx, orgID, messageID)
	if msg != nil {
		s.publishConvEvent(ctx, orgID, msg.ConversationID, "messenger.reaction.added", map[string]any{
			"conversation_id": msg.ConversationID,
			"message_id":      messageID,
			"user_id":         userID,
			"emoji":           emoji,
		})
	}
	return nil
}

// RemoveReaction removes a reaction from a message.
func (s *Service) RemoveReaction(ctx context.Context, orgID uuid.UUID, messageID, userID uuid.UUID, emoji string) error {
	if err := s.repo.RemoveReaction(ctx, messageID, userID, emoji); err != nil {
		return err
	}

	msg, _ := s.repo.GetMessage(ctx, orgID, messageID)
	if msg != nil {
		s.publishConvEvent(ctx, orgID, msg.ConversationID, "messenger.reaction.removed", map[string]any{
			"conversation_id": msg.ConversationID,
			"message_id":      messageID,
			"user_id":         userID,
			"emoji":           emoji,
		})
	}
	return nil
}

// SearchMessages searches messages by content.
func (s *Service) SearchMessages(ctx context.Context, orgID uuid.UUID, query string, convID *uuid.UUID, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return s.repo.SearchMessages(ctx, orgID, query, convID, limit)
}

// SetTyping publishes a typing indicator event.
func (s *Service) SetTyping(ctx context.Context, orgID, convID, userID uuid.UUID) error {
	s.publishConvEvent(ctx, orgID, convID, "messenger.typing", map[string]any{
		"conversation_id": convID,
		"user_id":         userID,
	})
	return nil
}

// GetUserConversationIDs returns all conversation IDs for a user in an org.
func (s *Service) GetUserConversationIDs(ctx context.Context, userID, orgID uuid.UUID) ([]uuid.UUID, error) {
	return s.repo.GetUserConversationIDs(ctx, userID, orgID)
}

func (s *Service) publishEvent(ctx context.Context, orgID uuid.UUID, eventType string, data map[string]any) {
	payload, _ := json.Marshal(data)
	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Type:           eventType,
		Data:           payload,
		Timestamp:      time.Now(),
	}
	if err := s.bus.Publish(ctx, ev); err != nil {
		log.Error().Err(err).Str("type", eventType).Msg("messenger: failed to publish event")
	}
}

func (s *Service) publishConvEvent(ctx context.Context, orgID, convID uuid.UUID, eventType string, data map[string]any) {
	data["_channel"] = "chat:" + convID.String()
	s.publishEvent(ctx, orgID, eventType, data)
}
