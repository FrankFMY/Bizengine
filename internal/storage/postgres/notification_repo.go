package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/notification"
)

type NotificationRepo struct {
	pool *pgxpool.Pool
}

func NewNotificationRepo(pool *pgxpool.Pool) *NotificationRepo {
	return &NotificationRepo{pool: pool}
}

func (r *NotificationRepo) Create(ctx context.Context, n *notification.Notification) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO notifications (id, organization_id, user_id, title, body, severity, read, reference_type, reference_id, ver, upd, iat)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		n.ID, n.OrganizationID, n.UserID, n.Title, n.Body, n.Severity,
		n.Read, n.ReferenceType, n.ReferenceID, n.Ver, n.Upd, n.Iat,
	)
	return err
}

func (r *NotificationRepo) List(ctx context.Context, orgID, userID uuid.UUID, filter notification.ListFilter) ([]notification.Notification, int, error) {
	filter.Page.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("organization_id = $%d", argIdx))
	args = append(args, orgID)
	argIdx++

	conditions = append(conditions, fmt.Sprintf("user_id = $%d", argIdx))
	args = append(args, userID)
	argIdx++

	if filter.Read != nil {
		conditions = append(conditions, fmt.Sprintf("read = $%d", argIdx))
		args = append(args, *filter.Read)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM notifications WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT id, organization_id, user_id, title, body, severity, read, reference_type, reference_id, ver, upd, iat
		 FROM notifications WHERE %s ORDER BY iat DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, filter.Page.Limit, filter.Page.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []notification.Notification
	for rows.Next() {
		var n notification.Notification
		if err := rows.Scan(&n.ID, &n.OrganizationID, &n.UserID, &n.Title, &n.Body, &n.Severity,
			&n.Read, &n.ReferenceType, &n.ReferenceID, &n.Ver, &n.Upd, &n.Iat); err != nil {
			return nil, 0, err
		}
		items = append(items, n)
	}
	return items, total, rows.Err()
}

func (r *NotificationRepo) MarkRead(ctx context.Context, orgID, userID, notifID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE notifications SET read = true, upd = now(), ver = ver + 1
		 WHERE id = $1 AND organization_id = $2 AND user_id = $3`,
		notifID, orgID, userID,
	)
	return err
}

func (r *NotificationRepo) MarkAllRead(ctx context.Context, orgID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE notifications SET read = true, upd = now(), ver = ver + 1
		 WHERE organization_id = $1 AND user_id = $2 AND read = false`,
		orgID, userID,
	)
	return err
}

func (r *NotificationRepo) CountUnread(ctx context.Context, orgID, userID uuid.UUID) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM notifications WHERE organization_id = $1 AND user_id = $2 AND read = false`,
		orgID, userID,
	).Scan(&count)
	return count, err
}
