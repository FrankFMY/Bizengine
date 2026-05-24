package graphs

import (
	"context"
	"fmt"

	"github.com/FrankFMY/arcana"
)

var notificationsList = arcana.GraphDef{
	Key: "notifications_list",
	Deps: []arcana.TableDep{
		{Table: "notifications", Columns: []string{"severity", "read", "iat"}},
	},
	Params: arcana.ParamSchema{
		"unread_only": arcana.ParamString().Build(),
		"limit":       arcana.ParamInt().Default(50),
		"offset":      arcana.ParamInt().Default(0),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		identity := arcana.User(ctx)
		userID := identity.UserID
		limit := p.Int("limit")
		offset := p.Int("offset")
		unreadOnly := p.String("unread_only")

		query := `
			SELECT id, severity, title, body, read, iat, COUNT(*) OVER() AS total_count
			FROM notifications
			WHERE organization_id = $1 AND user_id = $2
		`
		args := []any{orgID, userID}
		argIdx := 3

		if unreadOnly == "true" {
			query += ` AND read = false`
		}

		query += fmt.Sprintf(` ORDER BY iat DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
		args = append(args, limit, offset)

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()
		for rows.Next() {
			var id, severity, title, body string
			var isRead bool
			var iat any
			var cnt int
			if err := rows.Scan(&id, &severity, &title, &body, &isRead, &iat, &cnt); err != nil {
				return nil, err
			}
			result.SetTotal(cnt)
			result.AddRef(arcana.Ref{Table: "notifications", ID: id, Fields: []string{"severity", "read", "iat"}})
			result.AddRow("notifications", id, map[string]any{
				"id": id, "severity": severity, "title": title, "body": body,
				"read": isRead, "iat": iat,
			})
		}
		return result, rows.Err()
	},
}

var notificationsUnread = arcana.GraphDef{
	Key: "notifications_unread",
	Deps: []arcana.TableDep{
		{Table: "notifications", Columns: []string{"read", "iat"}},
	},
	Params: arcana.ParamSchema{},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		identity := arcana.User(ctx)
		userID := identity.UserID

		var count int
		err := q.QueryRow(ctx,
			`SELECT COUNT(*) FROM notifications WHERE organization_id = $1 AND user_id = $2 AND read = false`,
			orgID, userID,
		).Scan(&count)
		if err != nil {
			return nil, err
		}

		result := arcana.NewResult()
		result.AddRow("notifications", "unread_count", map[string]any{
			"unread": count,
		})
		return result, nil
	},
}
