package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/webhook"
	"github.com/bizengine/engine/pkg/errs"
)

type WebhookRepo struct {
	pool *pgxpool.Pool
}

func NewWebhookRepo(pool *pgxpool.Pool) *WebhookRepo {
	return &WebhookRepo{pool: pool}
}

func (r *WebhookRepo) Create(ctx context.Context, w *webhook.Webhook) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO webhooks (id, organization_id, url, secret, events, active, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		w.ID, w.OrganizationID, w.URL, w.Secret, w.Events, w.Active, w.CreatedAt)
	return err
}

func (r *WebhookRepo) Get(ctx context.Context, orgID, id uuid.UUID) (*webhook.Webhook, error) {
	var w webhook.Webhook
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, url, secret, events, active, created_at
		 FROM webhooks WHERE id = $1 AND organization_id = $2`,
		id, orgID).Scan(&w.ID, &w.OrganizationID, &w.URL, &w.Secret, &w.Events, &w.Active, &w.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, errs.NewNotFound("webhook not found")
	}
	return &w, err
}

func (r *WebhookRepo) List(ctx context.Context, orgID uuid.UUID) ([]webhook.Webhook, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, url, secret, events, active, created_at
		 FROM webhooks WHERE organization_id = $1 ORDER BY created_at DESC`,
		orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hooks []webhook.Webhook
	for rows.Next() {
		var w webhook.Webhook
		if err := rows.Scan(&w.ID, &w.OrganizationID, &w.URL, &w.Secret, &w.Events, &w.Active, &w.CreatedAt); err != nil {
			return nil, err
		}
		hooks = append(hooks, w)
	}
	return hooks, rows.Err()
}

func (r *WebhookRepo) Update(ctx context.Context, w *webhook.Webhook) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE webhooks SET url = $1, secret = $2, events = $3, active = $4 WHERE id = $5 AND organization_id = $6`,
		w.URL, w.Secret, w.Events, w.Active, w.ID, w.OrganizationID)
	return err
}

func (r *WebhookRepo) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM webhooks WHERE id = $1 AND organization_id = $2`, id, orgID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NewNotFound("webhook not found")
	}
	return nil
}

func (r *WebhookRepo) ListByEvent(ctx context.Context, orgID uuid.UUID, eventType string) ([]webhook.Webhook, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, url, secret, events, active, created_at
		 FROM webhooks WHERE organization_id = $1 AND active = true AND $2 = ANY(events)`,
		orgID, eventType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hooks []webhook.Webhook
	for rows.Next() {
		var w webhook.Webhook
		if err := rows.Scan(&w.ID, &w.OrganizationID, &w.URL, &w.Secret, &w.Events, &w.Active, &w.CreatedAt); err != nil {
			return nil, err
		}
		hooks = append(hooks, w)
	}
	return hooks, rows.Err()
}
