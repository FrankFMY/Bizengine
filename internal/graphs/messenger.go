package graphs

import (
	"context"
	"fmt"

	"github.com/FrankFMY/arcana"
)

var chatConversationsList = arcana.GraphDef{
	Key: "chat_conversations_list",
	Deps: []arcana.TableDep{
		{Table: "conversations", Columns: []string{"name", "last_message_at", "last_message_preview"}},
		{Table: "messages", Columns: []string{"content", "iat"}},
		{Table: "conversation_members", Columns: []string{"last_read_message_id"}},
	},
	Params: arcana.ParamSchema{
		"type":   arcana.ParamString().Build(),
		"search": arcana.ParamString().Build(),
		"limit":  arcana.ParamInt().Default(50),
		"offset": arcana.ParamInt().Default(0),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		identity := arcana.User(ctx)
		userID := identity.UserID
		limit := p.Int("limit")
		offset := p.Int("offset")
		convType := p.String("type")
		search := p.String("search")

		baseQuery := `FROM conversations c
			JOIN conversation_members cm ON cm.conversation_id = c.id AND cm.user_id = $2
			WHERE c.organization_id = $1`
		args := []any{orgID, userID}
		argIdx := 3

		if convType != "" {
			baseQuery += fmt.Sprintf(` AND c.type = $%d`, argIdx)
			args = append(args, convType)
			argIdx++
		}
		if search != "" {
			baseQuery += fmt.Sprintf(` AND (c.name ILIKE $%d OR c.last_message_preview ILIKE $%d)`, argIdx, argIdx)
			args = append(args, "%"+search+"%")
			argIdx++
		}

		query := fmt.Sprintf(`SELECT c.id, c.type, c.name, c.reference_type, c.reference_id,
			c.last_message_at, c.last_message_preview,
			COALESCE((
				SELECT COUNT(*) FROM messages m
				WHERE m.conversation_id = c.id AND m.deleted_at IS NULL
				AND m.iat > COALESCE(cm.last_read_at, '1970-01-01')
			), 0) AS unread_count,
			COUNT(*) OVER() AS total_count
			%s ORDER BY COALESCE(c.last_message_at, c.iat) DESC LIMIT $%d OFFSET $%d`,
			baseQuery, argIdx, argIdx+1)
		args = append(args, limit, offset)

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()
		for rows.Next() {
			var id, convT string
			var name, refType, refID, lastMsgAt, lastMsgPreview *string
			var unread, cnt int
			if err := rows.Scan(&id, &convT, &name, &refType, &refID,
				&lastMsgAt, &lastMsgPreview, &unread, &cnt); err != nil {
				return nil, err
			}
			result.SetTotal(cnt)
			result.AddRef(arcana.Ref{Table: "conversations", ID: id})
			result.AddRow("conversations", id, map[string]any{
				"id": id, "type": convT, "name": name,
				"reference_type": refType, "reference_id": refID,
				"last_message_at": lastMsgAt, "last_message_preview": lastMsgPreview,
				"unread_count": unread,
			})
		}
		return result, rows.Err()
	},
}

var chatMessages = arcana.GraphDef{
	Key: "chat_messages",
	Deps: []arcana.TableDep{
		{Table: "messages", Columns: []string{"content", "iat"}},
		{Table: "message_reactions", Columns: []string{"emoji"}},
	},
	Params: arcana.ParamSchema{
		"conversation_id": arcana.ParamUUID().Required(),
		"before":          arcana.ParamString().Build(),
		"limit":           arcana.ParamInt().Default(50),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		convID := p.String("conversation_id")
		before := p.String("before")
		limit := p.Int("limit")

		query := `SELECT id, sender_id, content, content_type, attachments, reply_to_id, edited_at, iat
			FROM messages WHERE conversation_id = $1 AND organization_id = $2 AND deleted_at IS NULL`
		args := []any{convID, orgID}
		argIdx := 3

		if before != "" {
			query += fmt.Sprintf(` AND iat < (SELECT iat FROM messages WHERE id = $%d)`, argIdx)
			args = append(args, before)
			argIdx++
		}

		query += fmt.Sprintf(` ORDER BY iat DESC LIMIT $%d`, argIdx)
		args = append(args, limit)

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()
		for rows.Next() {
			var id, senderID, contentType string
			var content *string
			var attachments []byte
			var replyToID *string
			var editedAt, iat any
			if err := rows.Scan(&id, &senderID, &content, &contentType,
				&attachments, &replyToID, &editedAt, &iat); err != nil {
				return nil, err
			}
			result.AddRef(arcana.Ref{Table: "messages", ID: id})
			result.AddRow("messages", id, map[string]any{
				"id": id, "sender_id": senderID, "content": content,
				"content_type": contentType, "attachments": string(attachments),
				"reply_to_id": replyToID, "edited_at": editedAt, "iat": iat,
			})
		}
		return result, rows.Err()
	},
}

var chatUnreadTotal = arcana.GraphDef{
	Key: "chat_unread_total",
	Deps: []arcana.TableDep{
		{Table: "conversation_members", Columns: []string{"last_read_message_id"}},
		{Table: "messages", Columns: []string{"iat"}},
	},
	Params: arcana.ParamSchema{},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		identity := arcana.User(ctx)
		userID := identity.UserID

		var total int
		err := q.QueryRow(ctx,
			`SELECT COALESCE(SUM(sub.unread), 0) FROM (
				SELECT COUNT(m.id) AS unread
				FROM conversation_members cm
				JOIN conversations c ON c.id = cm.conversation_id AND c.organization_id = $1
				LEFT JOIN messages m ON m.conversation_id = cm.conversation_id
					AND m.deleted_at IS NULL
					AND m.iat > COALESCE(cm.last_read_at, '1970-01-01')
				WHERE cm.user_id = $2
				GROUP BY cm.conversation_id
			) sub`, orgID, userID).Scan(&total)
		if err != nil {
			return nil, err
		}

		result := arcana.NewResult()
		result.AddRow("chat_unread", "total", map[string]any{
			"unread": total,
		})
		return result, nil
	},
}
