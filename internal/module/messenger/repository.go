// Package messenger provides the messenger business module.
package messenger

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Conversation represents a chat conversation.
type Conversation struct {
	ID                 uuid.UUID  `json:"id"`
	OrganizationID     uuid.UUID  `json:"organization_id"`
	Type               string     `json:"type"`
	Name               *string    `json:"name,omitempty"`
	ReferenceType      *string    `json:"reference_type,omitempty"`
	ReferenceID        *uuid.UUID `json:"reference_id,omitempty"`
	AvatarFileID       *uuid.UUID `json:"avatar_file_id,omitempty"`
	CreatedBy          uuid.UUID  `json:"created_by"`
	LastMessageID      *uuid.UUID `json:"last_message_id,omitempty"`
	LastMessageAt      *time.Time `json:"last_message_at,omitempty"`
	LastMessagePreview *string    `json:"last_message_preview,omitempty"`
	Ver                int        `json:"ver"`
	Upd                time.Time  `json:"upd"`
	Iat                time.Time  `json:"iat"`
}

// ConversationWithUnread adds unread count to a conversation.
type ConversationWithUnread struct {
	Conversation
	UnreadCount int `json:"unread_count"`
}

// ConversationMember represents a user in a conversation.
type ConversationMember struct {
	ID                uuid.UUID  `json:"id"`
	ConversationID    uuid.UUID  `json:"conversation_id"`
	UserID            uuid.UUID  `json:"user_id"`
	Role              string     `json:"role"`
	Muted             bool       `json:"muted"`
	LastReadMessageID *uuid.UUID `json:"last_read_message_id,omitempty"`
	LastReadAt        *time.Time `json:"last_read_at,omitempty"`
	JoinedAt          time.Time  `json:"joined_at"`
}

// Message represents a chat message.
type Message struct {
	ID             uuid.UUID    `json:"id"`
	ConversationID uuid.UUID    `json:"conversation_id"`
	OrganizationID uuid.UUID    `json:"organization_id"`
	SenderID       uuid.UUID    `json:"sender_id"`
	Content        *string      `json:"content,omitempty"`
	ContentType    string       `json:"content_type"`
	Attachments    []Attachment `json:"attachments"`
	ReplyToID      *uuid.UUID   `json:"reply_to_id,omitempty"`
	EditedAt       *time.Time   `json:"edited_at,omitempty"`
	DeletedAt      *time.Time   `json:"deleted_at,omitempty"`
	Ver            int          `json:"ver"`
	Upd            time.Time    `json:"upd"`
	Iat            time.Time    `json:"iat"`
	Reactions      []Reaction   `json:"reactions,omitempty"`
}

// Attachment is a message attachment.
type Attachment struct {
	Type         string         `json:"type"`
	FileID       *uuid.UUID     `json:"file_id,omitempty"`
	Filename     string         `json:"filename,omitempty"`
	Size         int64          `json:"size,omitempty"`
	ContentType  string         `json:"content_type,omitempty"`
	EntityKind   string         `json:"entity_kind,omitempty"`
	EntityID     *uuid.UUID     `json:"entity_id,omitempty"`
	Preview      map[string]any `json:"preview,omitempty"`
	Latitude     *float64       `json:"latitude,omitempty"`
	Longitude    *float64       `json:"longitude,omitempty"`
	Label        string         `json:"label,omitempty"`
	DocumentType string         `json:"document_type,omitempty"`
	OrderID      *uuid.UUID     `json:"order_id,omitempty"`
}

// Reaction is a message reaction.
type Reaction struct {
	MessageID uuid.UUID `json:"message_id"`
	UserID    uuid.UUID `json:"user_id"`
	Emoji     string    `json:"emoji"`
	Iat       time.Time `json:"iat"`
}

// SendMessageRequest is the input for sending a message.
type SendMessageRequest struct {
	ConversationID uuid.UUID    `json:"conversation_id"`
	SenderID       uuid.UUID    `json:"sender_id"`
	Content        string       `json:"content"`
	ContentType    string       `json:"content_type"`
	Attachments    []Attachment `json:"attachments"`
	ReplyToID      *uuid.UUID   `json:"reply_to_id,omitempty"`
}

// ConversationFilter holds query params for listing conversations.
type ConversationFilter struct {
	Type   string `json:"type"`
	Search string `json:"search"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// Repository defines data access for the messenger module.
type Repository interface {
	// Conversations
	CreateConversation(ctx context.Context, tx pgx.Tx, c *Conversation) error
	GetConversation(ctx context.Context, orgID, convID uuid.UUID) (*Conversation, error)
	ListConversations(ctx context.Context, orgID, userID uuid.UUID, filter ConversationFilter) ([]ConversationWithUnread, int, error)
	FindDirectConversation(ctx context.Context, orgID, userA, userB uuid.UUID) (*Conversation, error)
	FindEntityConversation(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID) (*Conversation, error)
	UpdateLastMessage(ctx context.Context, tx pgx.Tx, convID uuid.UUID, msgID uuid.UUID, preview string, at time.Time) error

	// Members
	AddMember(ctx context.Context, tx pgx.Tx, convID, userID uuid.UUID, role string) error
	RemoveMember(ctx context.Context, convID, userID uuid.UUID) error
	IsMember(ctx context.Context, convID, userID uuid.UUID) (bool, error)
	GetMembers(ctx context.Context, convID uuid.UUID) ([]ConversationMember, error)
	SetMuted(ctx context.Context, convID, userID uuid.UUID, muted bool) error
	GetUserConversationIDs(ctx context.Context, userID, orgID uuid.UUID) ([]uuid.UUID, error)

	// Messages
	CreateMessage(ctx context.Context, tx pgx.Tx, m *Message) error
	GetMessage(ctx context.Context, orgID, messageID uuid.UUID) (*Message, error)
	UpdateMessage(ctx context.Context, messageID uuid.UUID, content string) error
	SoftDeleteMessage(ctx context.Context, messageID uuid.UUID) error
	ListMessages(ctx context.Context, orgID, convID uuid.UUID, before *uuid.UUID, limit int) ([]Message, error)
	SearchMessages(ctx context.Context, orgID uuid.UUID, query string, convID *uuid.UUID, limit int) ([]Message, error)

	// Read tracking
	MarkRead(ctx context.Context, convID, userID, messageID uuid.UUID) error
	GetUnreadCounts(ctx context.Context, orgID, userID uuid.UUID) (map[uuid.UUID]int, error)

	// Reactions
	AddReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error
	RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error
	GetReactions(ctx context.Context, messageID uuid.UUID) ([]Reaction, error)

	WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}
