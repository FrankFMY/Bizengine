package settings

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// Service provides settings operations.
type Service struct {
	repo Repository
	bus  event.Bus
}

// NewService creates a new settings service.
func NewService(repo Repository, bus event.Bus) *Service {
	return &Service{repo: repo, bus: bus}
}

// Get returns current settings for an organization, creating defaults if missing.
func (s *Service) Get(ctx context.Context, orgID uuid.UUID) (*Settings, error) {
	settings, err := s.repo.Get(ctx, orgID)
	if err != nil {
		defaults := s.defaults(orgID)
		if uErr := s.repo.Upsert(ctx, defaults); uErr != nil {
			return nil, uErr
		}
		return defaults, nil
	}
	return settings, nil
}

// Update modifies organization settings.
func (s *Service) Update(ctx context.Context, orgID uuid.UUID, input UpdateInput, actorID *uuid.UUID) (*Settings, error) {
	settings, err := s.Get(ctx, orgID)
	if err != nil {
		return nil, err
	}

	if input.Currency != nil {
		if len(*input.Currency) != 3 {
			return nil, errs.NewBadRequest("currency must be a 3-letter code")
		}
		settings.Currency = *input.Currency
	}
	if input.Timezone != nil {
		settings.Timezone = *input.Timezone
	}
	if input.OrderNumberFormat != nil {
		settings.OrderNumberFormat = *input.OrderNumberFormat
	}
	if input.Requisites != nil {
		settings.Requisites = input.Requisites
	}
	if input.Features != nil {
		settings.Features = input.Features
	}

	settings.Version++
	settings.UpdatedAt = time.Now()

	if err := s.repo.Upsert(ctx, settings); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, "settings.updated", map[string]any{
		"organization_id": orgID,
	}, actorID)

	return settings, nil
}

// UpdateIntegrations updates integration API keys.
func (s *Service) UpdateIntegrations(ctx context.Context, orgID uuid.UUID, input UpdateIntegrationsInput, actorID *uuid.UUID) (*Settings, error) {
	settings, err := s.Get(ctx, orgID)
	if err != nil {
		return nil, err
	}

	if input.Integrations == nil {
		return nil, errs.NewBadRequest("integrations is required")
	}

	settings.Integrations = input.Integrations
	settings.Version++
	settings.UpdatedAt = time.Now()

	if err := s.repo.Upsert(ctx, settings); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, "settings.integrations.updated", map[string]any{
		"organization_id": orgID,
	}, actorID)

	return settings, nil
}

// SetLogo sets the logo file ID.
func (s *Service) SetLogo(ctx context.Context, orgID uuid.UUID, fileID uuid.UUID, actorID *uuid.UUID) (*Settings, error) {
	settings, err := s.Get(ctx, orgID)
	if err != nil {
		return nil, err
	}

	settings.LogoFileID = &fileID
	settings.Version++
	settings.UpdatedAt = time.Now()

	if err := s.repo.Upsert(ctx, settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// EnsureDefaults creates default settings for a new organization.
func (s *Service) EnsureDefaults(ctx context.Context, orgID uuid.UUID) error {
	_, err := s.repo.Get(ctx, orgID)
	if err != nil {
		defaults := s.defaults(orgID)
		return s.repo.Upsert(ctx, defaults)
	}
	return nil
}

func (s *Service) defaults(orgID uuid.UUID) *Settings {
	now := time.Now()
	return &Settings{
		OrganizationID:    orgID,
		Currency:          "RUB",
		Timezone:          "Europe/Moscow",
		OrderNumberFormat: "ORD-{SEQ}",
		Requisites:        json.RawMessage(`{}`),
		Integrations:      json.RawMessage(`{}`),
		Features:          json.RawMessage(`{}`),
		Version:           1,
		UpdatedAt:         now,
		CreatedAt:         now,
	}
}

func (s *Service) publishEvent(ctx context.Context, orgID uuid.UUID, eventType string, data map[string]any, actorID *uuid.UUID) {
	payload, _ := json.Marshal(data)
	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Type:           eventType,
		Data:           payload,
		Timestamp:      time.Now(),
	}
	if actorID != nil {
		ev.ActorID = actorID
	}
	s.bus.Publish(ctx, ev)
}
