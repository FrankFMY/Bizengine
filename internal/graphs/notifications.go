package graphs

import (
	"context"

	"github.com/FrankFMY/arcana"
)

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
