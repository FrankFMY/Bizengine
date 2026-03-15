package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/module/messenger"
	"github.com/bizengine/engine/pkg/errs"
)

// MessengerRepo implements messenger.Repository with PostgreSQL.
type MessengerRepo struct {
	pool *pgxpool.Pool
}

// NewMessengerRepo creates a new MessengerRepo.
func NewMessengerRepo(pool *pgxpool.Pool) *MessengerRepo {
	return &MessengerRepo{pool: pool}
}

// CreateConversation inserts a new conversation.
func (r *MessengerRepo) CreateConversation(ctx context.Context, tx pgx.Tx, c *messenger.Conversation) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO conversations (id, organization_id, type, name, reference_type, reference_id, avatar_file_id, created_by, ver, upd, iat)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, now(), now())`,
		c.ID, c.OrganizationID, c.Type, c.Name, c.ReferenceType, c.ReferenceID, c.AvatarFileID, c.CreatedBy)
	return err
}

// GetConversation returns a conversation by ID.
func (r *MessengerRepo) GetConversation(ctx context.Context, orgID, convID uuid.UUID) (*messenger.Conversation, error) {
	query := `SELECT id, organization_id, type, name, reference_type, reference_id, avatar_file_id, created_by,
		last_message_id, last_message_at, last_message_preview, ver, upd, iat
		FROM conversations WHERE id = $1`
	args := []any{convID}

	if orgID != uuid.Nil {
		query += ` AND organization_id = $2`
		args = append(args, orgID)
	}

	var c messenger.Conversation
	err := r.pool.QueryRow(ctx, query, args...).Scan(
		&c.ID, &c.OrganizationID, &c.Type, &c.Name, &c.ReferenceType, &c.ReferenceID,
		&c.AvatarFileID, &c.CreatedBy, &c.LastMessageID, &c.LastMessageAt, &c.LastMessagePreview,
		&c.Ver, &c.Upd, &c.Iat)
	if err != nil {
		return nil, errs.NewNotFound("conversation not found")
	}
	return &c, nil
}

// ListConversations returns conversations the user belongs to.
func (r *MessengerRepo) ListConversations(ctx context.Context, orgID, userID uuid.UUID, filter messenger.ConversationFilter) ([]messenger.ConversationWithUnread, int, error) {
	baseQuery := `FROM conversations c
		JOIN conversation_members cm ON cm.conversation_id = c.id AND cm.user_id = $2
		WHERE c.organization_id = $1`
	args := []any{orgID, userID}
	argIdx := 3

	if filter.Type != "" {
		baseQuery += fmt.Sprintf(` AND c.type = $%d`, argIdx)
		args = append(args, filter.Type)
		argIdx++
	}
	if filter.Search != "" {
		baseQuery += fmt.Sprintf(` AND (c.name ILIKE $%d OR c.last_message_preview ILIKE $%d)`, argIdx, argIdx)
		args = append(args, "%"+filter.Search+"%")
		argIdx++
	}

	var total int
	countArgs := make([]any, len(args))
	copy(countArgs, args)
	_ = r.pool.QueryRow(ctx, `SELECT COUNT(*) `+baseQuery, countArgs...).Scan(&total)

	selectQuery := fmt.Sprintf(`SELECT c.id, c.organization_id, c.type, c.name, c.reference_type, c.reference_id,
		c.avatar_file_id, c.created_by, c.last_message_id, c.last_message_at, c.last_message_preview,
		c.ver, c.upd, c.iat,
		COALESCE((
			SELECT COUNT(*) FROM messages m
			WHERE m.conversation_id = c.id AND m.deleted_at IS NULL
			AND m.iat > COALESCE(cm.last_read_at, '1970-01-01')
		), 0) AS unread_count
		%s ORDER BY COALESCE(c.last_message_at, c.iat) DESC LIMIT $%d OFFSET $%d`,
		baseQuery, argIdx, argIdx+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, selectQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var convs []messenger.ConversationWithUnread
	for rows.Next() {
		var cw messenger.ConversationWithUnread
		if err := rows.Scan(
			&cw.ID, &cw.OrganizationID, &cw.Type, &cw.Name, &cw.ReferenceType, &cw.ReferenceID,
			&cw.AvatarFileID, &cw.CreatedBy, &cw.LastMessageID, &cw.LastMessageAt, &cw.LastMessagePreview,
			&cw.Ver, &cw.Upd, &cw.Iat, &cw.UnreadCount); err != nil {
			return nil, 0, err
		}
		convs = append(convs, cw)
	}
	if convs == nil {
		convs = []messenger.ConversationWithUnread{}
	}
	return convs, total, rows.Err()
}

// FindDirectConversation finds an existing direct conversation between two users.
func (r *MessengerRepo) FindDirectConversation(ctx context.Context, orgID, userA, userB uuid.UUID) (*messenger.Conversation, error) {
	var c messenger.Conversation
	err := r.pool.QueryRow(ctx,
		`SELECT c.id, c.organization_id, c.type, c.name, c.reference_type, c.reference_id,
			c.avatar_file_id, c.created_by, c.last_message_id, c.last_message_at, c.last_message_preview,
			c.ver, c.upd, c.iat
		 FROM conversations c
		 WHERE c.organization_id = $1 AND c.type = 'direct'
		 AND EXISTS (SELECT 1 FROM conversation_members WHERE conversation_id = c.id AND user_id = $2)
		 AND EXISTS (SELECT 1 FROM conversation_members WHERE conversation_id = c.id AND user_id = $3)`,
		orgID, userA, userB).Scan(
		&c.ID, &c.OrganizationID, &c.Type, &c.Name, &c.ReferenceType, &c.ReferenceID,
		&c.AvatarFileID, &c.CreatedBy, &c.LastMessageID, &c.LastMessageAt, &c.LastMessagePreview,
		&c.Ver, &c.Upd, &c.Iat)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// FindEntityConversation finds a conversation for a business entity.
func (r *MessengerRepo) FindEntityConversation(ctx context.Context, orgID uuid.UUID, refType string, refID uuid.UUID) (*messenger.Conversation, error) {
	var c messenger.Conversation
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, type, name, reference_type, reference_id, avatar_file_id, created_by,
			last_message_id, last_message_at, last_message_preview, ver, upd, iat
		 FROM conversations WHERE organization_id = $1 AND reference_type = $2 AND reference_id = $3`,
		orgID, refType, refID).Scan(
		&c.ID, &c.OrganizationID, &c.Type, &c.Name, &c.ReferenceType, &c.ReferenceID,
		&c.AvatarFileID, &c.CreatedBy, &c.LastMessageID, &c.LastMessageAt, &c.LastMessagePreview,
		&c.Ver, &c.Upd, &c.Iat)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateLastMessage updates the last message info on a conversation.
func (r *MessengerRepo) UpdateLastMessage(ctx context.Context, tx pgx.Tx, convID, msgID uuid.UUID, preview string, at time.Time) error {
	_, err := tx.Exec(ctx,
		`UPDATE conversations SET last_message_id = $1, last_message_at = $2, last_message_preview = $3 WHERE id = $4`,
		msgID, at, preview, convID)
	return err
}

// AddMember adds a user to a conversation.
func (r *MessengerRepo) AddMember(ctx context.Context, tx pgx.Tx, convID, userID uuid.UUID, role string) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO conversation_members (id, conversation_id, user_id, role, joined_at, ver, upd, iat)
		 VALUES ($1, $2, $3, $4, now(), 1, now(), now()) ON CONFLICT (conversation_id, user_id) DO NOTHING`,
		uuid.New(), convID, userID, role)
	return err
}

// RemoveMember removes a user from a conversation.
func (r *MessengerRepo) RemoveMember(ctx context.Context, convID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM conversation_members WHERE conversation_id = $1 AND user_id = $2`, convID, userID)
	return err
}

// IsMember checks if a user is a member of a conversation.
func (r *MessengerRepo) IsMember(ctx context.Context, convID, userID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id = $1 AND user_id = $2)`,
		convID, userID).Scan(&exists)
	return exists, err
}

// GetMembers returns all members of a conversation.
func (r *MessengerRepo) GetMembers(ctx context.Context, convID uuid.UUID) ([]messenger.ConversationMember, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, conversation_id, user_id, role, muted, last_read_message_id, last_read_at, joined_at
		 FROM conversation_members WHERE conversation_id = $1 ORDER BY joined_at`, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []messenger.ConversationMember
	for rows.Next() {
		var m messenger.ConversationMember
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.UserID, &m.Role, &m.Muted,
			&m.LastReadMessageID, &m.LastReadAt, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	if members == nil {
		members = []messenger.ConversationMember{}
	}
	return members, rows.Err()
}

// SetMuted sets the muted status for a member.
func (r *MessengerRepo) SetMuted(ctx context.Context, convID, userID uuid.UUID, muted bool) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE conversation_members SET muted = $1 WHERE conversation_id = $2 AND user_id = $3`,
		muted, convID, userID)
	return err
}

// GetUserConversationIDs returns all conversation IDs for a user.
func (r *MessengerRepo) GetUserConversationIDs(ctx context.Context, userID, orgID uuid.UUID) ([]uuid.UUID, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT cm.conversation_id FROM conversation_members cm
		 JOIN conversations c ON c.id = cm.conversation_id
		 WHERE cm.user_id = $1 AND c.organization_id = $2`, userID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// CreateMessage inserts a new message.
func (r *MessengerRepo) CreateMessage(ctx context.Context, tx pgx.Tx, m *messenger.Message) error {
	attachJSON, _ := json.Marshal(m.Attachments)
	_, err := tx.Exec(ctx,
		`INSERT INTO messages (id, conversation_id, organization_id, sender_id, content, content_type, attachments, reply_to_id, ver, upd, iat)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, now(), now())`,
		m.ID, m.ConversationID, m.OrganizationID, m.SenderID, m.Content, m.ContentType, attachJSON, m.ReplyToID)
	return err
}

// GetMessage returns a message by ID.
func (r *MessengerRepo) GetMessage(ctx context.Context, orgID, messageID uuid.UUID) (*messenger.Message, error) {
	var m messenger.Message
	var attachJSON []byte
	err := r.pool.QueryRow(ctx,
		`SELECT id, conversation_id, organization_id, sender_id, content, content_type, attachments, reply_to_id, edited_at, deleted_at, ver, upd, iat
		 FROM messages WHERE id = $1 AND organization_id = $2`,
		messageID, orgID).Scan(
		&m.ID, &m.ConversationID, &m.OrganizationID, &m.SenderID, &m.Content, &m.ContentType,
		&attachJSON, &m.ReplyToID, &m.EditedAt, &m.DeletedAt, &m.Ver, &m.Upd, &m.Iat)
	if err != nil {
		return nil, errs.NewNotFound("message not found")
	}
	json.Unmarshal(attachJSON, &m.Attachments)
	if m.Attachments == nil {
		m.Attachments = []messenger.Attachment{}
	}
	return &m, nil
}

// UpdateMessage updates message content.
func (r *MessengerRepo) UpdateMessage(ctx context.Context, messageID uuid.UUID, content string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE messages SET content = $1, edited_at = now() WHERE id = $2`,
		content, messageID)
	return err
}

// SoftDeleteMessage sets deleted_at on a message.
func (r *MessengerRepo) SoftDeleteMessage(ctx context.Context, messageID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE messages SET deleted_at = now(), content = NULL WHERE id = $1`, messageID)
	return err
}

// ListMessages returns messages in a conversation with cursor pagination.
func (r *MessengerRepo) ListMessages(ctx context.Context, orgID, convID uuid.UUID, before *uuid.UUID, limit int) ([]messenger.Message, error) {
	query := `SELECT m.id, m.conversation_id, m.organization_id, m.sender_id, m.content, m.content_type,
		m.attachments, m.reply_to_id, m.edited_at, m.deleted_at, m.ver, m.upd, m.iat
		FROM messages m WHERE m.conversation_id = $1 AND m.organization_id = $2 AND m.deleted_at IS NULL`
	args := []any{convID, orgID}
	argIdx := 3

	if before != nil {
		query += fmt.Sprintf(` AND m.iat < (SELECT iat FROM messages WHERE id = $%d)`, argIdx)
		args = append(args, *before)
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY m.iat DESC LIMIT $%d`, argIdx)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []messenger.Message
	for rows.Next() {
		var m messenger.Message
		var attachJSON []byte
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.OrganizationID, &m.SenderID,
			&m.Content, &m.ContentType, &attachJSON, &m.ReplyToID, &m.EditedAt, &m.DeletedAt,
			&m.Ver, &m.Upd, &m.Iat); err != nil {
			return nil, err
		}
		json.Unmarshal(attachJSON, &m.Attachments)
		if m.Attachments == nil {
			m.Attachments = []messenger.Attachment{}
		}
		messages = append(messages, m)
	}
	if messages == nil {
		messages = []messenger.Message{}
	}
	return messages, rows.Err()
}

// SearchMessages searches messages by content.
func (r *MessengerRepo) SearchMessages(ctx context.Context, orgID uuid.UUID, query string, convID *uuid.UUID, limit int) ([]messenger.Message, error) {
	sql := `SELECT id, conversation_id, organization_id, sender_id, content, content_type,
		attachments, reply_to_id, edited_at, deleted_at, ver, upd, iat
		FROM messages WHERE organization_id = $1 AND deleted_at IS NULL AND content ILIKE $2`
	args := []any{orgID, "%" + query + "%"}
	argIdx := 3

	if convID != nil {
		sql += fmt.Sprintf(` AND conversation_id = $%d`, argIdx)
		args = append(args, *convID)
		argIdx++
	}

	sql += fmt.Sprintf(` ORDER BY iat DESC LIMIT $%d`, argIdx)
	args = append(args, limit)

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []messenger.Message
	for rows.Next() {
		var m messenger.Message
		var attachJSON []byte
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.OrganizationID, &m.SenderID,
			&m.Content, &m.ContentType, &attachJSON, &m.ReplyToID, &m.EditedAt, &m.DeletedAt,
			&m.Ver, &m.Upd, &m.Iat); err != nil {
			return nil, err
		}
		json.Unmarshal(attachJSON, &m.Attachments)
		if m.Attachments == nil {
			m.Attachments = []messenger.Attachment{}
		}
		messages = append(messages, m)
	}
	if messages == nil {
		messages = []messenger.Message{}
	}
	return messages, rows.Err()
}

// MarkRead updates last_read_message_id for a member.
func (r *MessengerRepo) MarkRead(ctx context.Context, convID, userID, messageID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE conversation_members SET last_read_message_id = $1, last_read_at = now()
		 WHERE conversation_id = $2 AND user_id = $3`,
		messageID, convID, userID)
	return err
}

// GetUnreadCounts returns unread message counts per conversation for a user.
func (r *MessengerRepo) GetUnreadCounts(ctx context.Context, orgID, userID uuid.UUID) (map[uuid.UUID]int, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT cm.conversation_id,
			COUNT(m.id) AS unread
		 FROM conversation_members cm
		 JOIN conversations c ON c.id = cm.conversation_id AND c.organization_id = $1
		 LEFT JOIN messages m ON m.conversation_id = cm.conversation_id
			AND m.deleted_at IS NULL
			AND m.iat > COALESCE(cm.last_read_at, '1970-01-01')
		 WHERE cm.user_id = $2
		 GROUP BY cm.conversation_id
		 HAVING COUNT(m.id) > 0`, orgID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[uuid.UUID]int)
	for rows.Next() {
		var convID uuid.UUID
		var count int
		if err := rows.Scan(&convID, &count); err != nil {
			return nil, err
		}
		counts[convID] = count
	}
	return counts, rows.Err()
}

// AddReaction adds a reaction.
func (r *MessengerRepo) AddReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO message_reactions (message_id, user_id, emoji, iat) VALUES ($1, $2, $3, now())
		 ON CONFLICT (message_id, user_id, emoji) DO NOTHING`,
		messageID, userID, emoji)
	return err
}

// RemoveReaction removes a reaction.
func (r *MessengerRepo) RemoveReaction(ctx context.Context, messageID, userID uuid.UUID, emoji string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM message_reactions WHERE message_id = $1 AND user_id = $2 AND emoji = $3`,
		messageID, userID, emoji)
	return err
}

// GetReactions returns all reactions for a message.
func (r *MessengerRepo) GetReactions(ctx context.Context, messageID uuid.UUID) ([]messenger.Reaction, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT message_id, user_id, emoji, iat FROM message_reactions WHERE message_id = $1 ORDER BY iat`,
		messageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reactions []messenger.Reaction
	for rows.Next() {
		var rx messenger.Reaction
		if err := rows.Scan(&rx.MessageID, &rx.UserID, &rx.Emoji, &rx.Iat); err != nil {
			return nil, err
		}
		reactions = append(reactions, rx)
	}
	if reactions == nil {
		reactions = []messenger.Reaction{}
	}
	return reactions, rows.Err()
}

// WithTx executes fn inside a transaction.
func (r *MessengerRepo) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
