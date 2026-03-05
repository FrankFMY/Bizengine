package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/module/settings"
)

// SettingsRepo implements settings.Repository.
type SettingsRepo struct {
	pool *pgxpool.Pool
}

// NewSettingsRepo creates a new SettingsRepo.
func NewSettingsRepo(pool *pgxpool.Pool) *SettingsRepo {
	return &SettingsRepo{pool: pool}
}

// Get returns settings for an organization.
func (r *SettingsRepo) Get(ctx context.Context, orgID uuid.UUID) (*settings.Settings, error) {
	var s settings.Settings
	err := r.pool.QueryRow(ctx,
		`SELECT organization_id, currency, timezone, order_number_format, logo_file_id,
		        requisites, integrations, features, ver, upd, iat
		 FROM organization_settings WHERE organization_id = $1`,
		orgID,
	).Scan(
		&s.OrganizationID, &s.Currency, &s.Timezone, &s.OrderNumberFormat, &s.LogoFileID,
		&s.Requisites, &s.Integrations, &s.Features, &s.Version, &s.UpdatedAt, &s.CreatedAt,
	)
	if err == pgx.ErrNoRows {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	return &s, nil
}

// Upsert creates or updates settings.
func (r *SettingsRepo) Upsert(ctx context.Context, s *settings.Settings) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO organization_settings (organization_id, currency, timezone, order_number_format, logo_file_id,
		                                    requisites, integrations, features, ver, upd, iat)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 ON CONFLICT (organization_id) DO UPDATE SET
		   currency = EXCLUDED.currency,
		   timezone = EXCLUDED.timezone,
		   order_number_format = EXCLUDED.order_number_format,
		   logo_file_id = EXCLUDED.logo_file_id,
		   requisites = EXCLUDED.requisites,
		   integrations = EXCLUDED.integrations,
		   features = EXCLUDED.features,
		   ver = EXCLUDED.ver,
		   upd = EXCLUDED.upd`,
		s.OrganizationID, s.Currency, s.Timezone, s.OrderNumberFormat, s.LogoFileID,
		s.Requisites, s.Integrations, s.Features, s.Version, s.UpdatedAt, s.CreatedAt,
	)
	return err
}
