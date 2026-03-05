package entity

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// Service provides business logic for entities and components.
type Service struct {
	repo       Repository
	eventStore event.Store
	eventBus   event.Bus
}

// NewService creates a new entity service.
func NewService(repo Repository, eventStore event.Store, eventBus event.Bus) *Service {
	return &Service{
		repo:       repo,
		eventStore: eventStore,
		eventBus:   eventBus,
	}
}

// Create creates a new entity and publishes entity.created.
func (s *Service) Create(ctx context.Context, orgID uuid.UUID, input CreateEntityInput, actorID *uuid.UUID) (*types.Entity, error) {
	if input.Kind == "" {
		return nil, errs.NewBadRequest("kind is required")
	}
	if input.Name == "" {
		return nil, errs.NewBadRequest("name is required")
	}

	e := &types.Entity{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Kind:           input.Kind,
		Name:           input.Name,
		ParentID:       input.ParentID,
		Meta:           input.Meta,
	}
	if e.Meta == nil {
		e.Meta = json.RawMessage(`{}`)
	}

	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EntityID:       &e.ID,
		Type:           "entity.created",
		ActorID:        actorID,
		Timestamp:      time.Now(),
		Version:        1,
	}
	ev.Data, _ = json.Marshal(map[string]any{
		"id":     e.ID,
		"kind":   e.Kind,
		"name":   e.Name,
		"status": "active",
	})

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.CreateTx(ctx, tx, e); err != nil {
			return err
		}
		return s.eventStore.AppendTx(ctx, tx, ev)
	}); err != nil {
		return nil, err
	}

	s.eventBus.Publish(ctx, ev)
	return e, nil
}

// CreateWithComponents creates an entity and sets multiple components in a single transaction.
func (s *Service) CreateWithComponents(ctx context.Context, orgID uuid.UUID, input CreateEntityInput, components map[string]json.RawMessage, actorID *uuid.UUID) (*types.Entity, error) {
	if input.Kind == "" {
		return nil, errs.NewBadRequest("kind is required")
	}
	if input.Name == "" {
		return nil, errs.NewBadRequest("name is required")
	}

	e := &types.Entity{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Kind:           input.Kind,
		Name:           input.Name,
		ParentID:       input.ParentID,
		Meta:           input.Meta,
	}
	if e.Meta == nil {
		e.Meta = json.RawMessage(`{}`)
	}

	entityEv := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EntityID:       &e.ID,
		Type:           "entity.created",
		ActorID:        actorID,
		Timestamp:      time.Now(),
		Version:        1,
	}
	entityEv.Data, _ = json.Marshal(map[string]any{
		"id":     e.ID,
		"kind":   e.Kind,
		"name":   e.Name,
		"status": "active",
	})

	var compEvents []types.Event
	var createdComponents []types.Component

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.CreateTx(ctx, tx, e); err != nil {
			return err
		}
		if err := s.eventStore.AppendTx(ctx, tx, entityEv); err != nil {
			return err
		}

		for compType, data := range components {
			c := &types.Component{
				ID:             uuid.New(),
				EntityID:       e.ID,
				OrganizationID: orgID,
				Type:           compType,
				Data:           data,
			}
			if err := s.repo.SetComponentTx(ctx, tx, c); err != nil {
				return err
			}

			ev := types.Event{
				ID:             uuid.New(),
				OrganizationID: orgID,
				EntityID:       &e.ID,
				Type:           "component.set",
				ActorID:        actorID,
				Timestamp:      time.Now(),
				Version:        1,
			}
			ev.Data, _ = json.Marshal(map[string]any{
				"entity_id": e.ID,
				"type":      compType,
				"data":      data,
			})
			if err := s.eventStore.AppendTx(ctx, tx, ev); err != nil {
				return err
			}
			compEvents = append(compEvents, ev)
			createdComponents = append(createdComponents, *c)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	s.eventBus.Publish(ctx, entityEv)
	for _, ev := range compEvents {
		s.eventBus.Publish(ctx, ev)
	}

	e.Components = createdComponents
	return e, nil
}

// Get returns an entity by ID with optional components.
func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID, includeComponents bool) (*types.Entity, error) {
	e, err := s.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	if includeComponents {
		comps, err := s.repo.ListComponents(ctx, orgID, id)
		if err != nil {
			return nil, err
		}
		e.Components = comps
	}
	return e, nil
}

// List returns entities matching the filter.
func (s *Service) List(ctx context.Context, orgID uuid.UUID, filter ListFilter, includeComponents bool) (*types.PageResponse[types.Entity], error) {
	filter.Page.Normalize()
	entities, total, err := s.repo.List(ctx, orgID, filter)
	if err != nil {
		return nil, err
	}

	if includeComponents && len(entities) > 0 {
		for i := range entities {
			comps, err := s.repo.ListComponents(ctx, orgID, entities[i].ID)
			if err != nil {
				return nil, err
			}
			entities[i].Components = comps
		}
	}

	if entities == nil {
		entities = []types.Entity{}
	}

	return &types.PageResponse[types.Entity]{
		Items:  entities,
		Total:  total,
		Limit:  filter.Page.Limit,
		Offset: filter.Page.Offset,
	}, nil
}

// Update modifies an entity and publishes entity.updated.
func (s *Service) Update(ctx context.Context, orgID, id uuid.UUID, input UpdateEntityInput, actorID *uuid.UUID) (*types.Entity, error) {
	e, err := s.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return nil, err
	}

	changes := make(map[string]any)
	if input.Name != nil && *input.Name != e.Name {
		changes["name"] = map[string]any{"old": e.Name, "new": *input.Name}
		e.Name = *input.Name
	}
	if input.Status != nil && *input.Status != e.Status {
		changes["status"] = map[string]any{"old": e.Status, "new": *input.Status}
		e.Status = *input.Status
	}
	if input.ParentID != nil {
		changes["parent_id"] = map[string]any{"old": e.ParentID, "new": *input.ParentID}
		e.ParentID = input.ParentID
	}
	if input.Meta != nil {
		changes["meta"] = map[string]any{"old": e.Meta, "new": input.Meta}
		e.Meta = input.Meta
	}
	if input.SortOrder != nil && *input.SortOrder != e.SortOrder {
		changes["sort_order"] = map[string]any{"old": e.SortOrder, "new": *input.SortOrder}
		e.SortOrder = *input.SortOrder
	}

	if len(changes) == 0 {
		return e, nil
	}

	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EntityID:       &id,
		Type:           "entity.updated",
		ActorID:        actorID,
		Timestamp:      time.Now(),
		Version:        1,
	}
	ev.Data, _ = json.Marshal(map[string]any{
		"id":      id,
		"changes": changes,
	})

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.Update(ctx, e); err != nil {
			return err
		}
		return s.eventStore.AppendTx(ctx, tx, ev)
	}); err != nil {
		return nil, err
	}

	s.eventBus.Publish(ctx, ev)
	return e, nil
}

// Delete soft-deletes an entity and publishes entity.deleted.
func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID, actorID *uuid.UUID) error {
	e, err := s.repo.GetByID(ctx, orgID, id)
	if err != nil {
		return err
	}

	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EntityID:       &id,
		Type:           "entity.deleted",
		ActorID:        actorID,
		Timestamp:      time.Now(),
		Version:        1,
	}
	ev.Data, _ = json.Marshal(map[string]any{
		"id":   id,
		"kind": e.Kind,
	})

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.SoftDelete(ctx, orgID, id); err != nil {
			return err
		}
		return s.eventStore.AppendTx(ctx, tx, ev)
	}); err != nil {
		return err
	}

	s.eventBus.Publish(ctx, ev)
	return nil
}

// SetComponent sets a component on an entity and publishes component.set.
func (s *Service) SetComponent(ctx context.Context, orgID, entityID uuid.UUID, compType string, data json.RawMessage, actorID *uuid.UUID) (*types.Component, error) {
	// Verify entity exists
	if _, err := s.repo.GetByID(ctx, orgID, entityID); err != nil {
		return nil, err
	}

	c := &types.Component{
		ID:             uuid.New(),
		EntityID:       entityID,
		OrganizationID: orgID,
		Type:           compType,
		Data:           data,
	}

	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EntityID:       &entityID,
		Type:           "component.set",
		ActorID:        actorID,
		Timestamp:      time.Now(),
		Version:        1,
	}
	ev.Data, _ = json.Marshal(map[string]any{
		"entity_id": entityID,
		"type":      compType,
		"data":      data,
	})

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.SetComponentTx(ctx, tx, c); err != nil {
			return err
		}
		return s.eventStore.AppendTx(ctx, tx, ev)
	}); err != nil {
		return nil, err
	}

	ev.Data, _ = json.Marshal(map[string]any{
		"entity_id": entityID,
		"type":      compType,
		"data":      data,
		"version":   c.Version,
	})
	s.eventBus.Publish(ctx, ev)
	return c, nil
}

// GetComponent returns a specific component.
func (s *Service) GetComponent(ctx context.Context, orgID, entityID uuid.UUID, compType string) (*types.Component, error) {
	return s.repo.GetComponent(ctx, orgID, entityID, compType)
}

// ListComponents returns all components for an entity.
func (s *Service) ListComponents(ctx context.Context, orgID, entityID uuid.UUID) ([]types.Component, error) {
	return s.repo.ListComponents(ctx, orgID, entityID)
}

// DeleteComponent removes a component and publishes component.removed.
func (s *Service) DeleteComponent(ctx context.Context, orgID, entityID uuid.UUID, compType string, actorID *uuid.UUID) error {
	if _, err := s.repo.GetByID(ctx, orgID, entityID); err != nil {
		return err
	}

	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EntityID:       &entityID,
		Type:           "component.removed",
		ActorID:        actorID,
		Timestamp:      time.Now(),
		Version:        1,
	}
	ev.Data, _ = json.Marshal(map[string]any{
		"entity_id": entityID,
		"type":      compType,
	})

	if err := s.repo.DeleteComponent(ctx, orgID, entityID, compType); err != nil {
		return err
	}

	s.eventBus.Publish(ctx, ev)
	return nil
}
