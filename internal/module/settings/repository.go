package settings

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// Settings represents organization-level configuration.
type Settings struct {
	OrganizationID    uuid.UUID       `json:"organization_id"`
	Currency          string          `json:"currency"`
	Timezone          string          `json:"timezone"`
	OrderNumberFormat string          `json:"order_number_format"`
	LogoFileID        *uuid.UUID      `json:"logo_file_id,omitempty"`
	Requisites        json.RawMessage `json:"requisites"`
	Integrations      json.RawMessage `json:"integrations"`
	Features          json.RawMessage `json:"features"`
	Version           int             `json:"ver"`
	UpdatedAt         time.Time       `json:"upd"`
	CreatedAt         time.Time       `json:"iat"`
}

// UpdateInput is the input for updating settings.
type UpdateInput struct {
	Currency          *string         `json:"currency"`
	Timezone          *string         `json:"timezone"`
	OrderNumberFormat *string         `json:"order_number_format"`
	Requisites        json.RawMessage `json:"requisites"`
	Features          json.RawMessage `json:"features"`
}

// UpdateIntegrationsInput is the input for updating integration keys.
type UpdateIntegrationsInput struct {
	Integrations json.RawMessage `json:"integrations"`
}

// Repository defines data access for settings.
type Repository interface {
	Get(ctx context.Context, orgID uuid.UUID) (*Settings, error)
	Upsert(ctx context.Context, s *Settings) error
}
