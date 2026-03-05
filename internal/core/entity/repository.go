// Package entity provides the core Entity + Component service.
package entity

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/bizengine/engine/pkg/types"
)

// Repository defines storage operations for entities and components.
type Repository interface {
	// Entity CRUD
	Create(ctx context.Context, e *types.Entity) error
	GetByID(ctx context.Context, orgID, id uuid.UUID) (*types.Entity, error)
	List(ctx context.Context, orgID uuid.UUID, filter ListFilter) ([]types.Entity, int, error)
	Update(ctx context.Context, e *types.Entity) error
	SoftDelete(ctx context.Context, orgID, id uuid.UUID) error

	// Component CRUD
	SetComponent(ctx context.Context, c *types.Component) error
	GetComponent(ctx context.Context, orgID, entityID uuid.UUID, compType string) (*types.Component, error)
	ListComponents(ctx context.Context, orgID, entityID uuid.UUID) ([]types.Component, error)
	DeleteComponent(ctx context.Context, orgID, entityID uuid.UUID, compType string) error

	// Transactional support
	WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error
	CreateTx(ctx context.Context, tx pgx.Tx, e *types.Entity) error
	SetComponentTx(ctx context.Context, tx pgx.Tx, c *types.Component) error
}

// ListFilter defines entity listing parameters.
type ListFilter struct {
	Kind     *string
	Status   *string
	ParentID *uuid.UUID
	Search   *string
	Page     types.PageRequest
}

// CreateEntityInput is the input for creating an entity.
type CreateEntityInput struct {
	Kind     string          `json:"kind"`
	Name     string          `json:"name"`
	ParentID *uuid.UUID      `json:"parent_id,omitempty"`
	Meta     json.RawMessage `json:"meta,omitempty"`
}

// UpdateEntityInput is the input for updating an entity.
type UpdateEntityInput struct {
	Name      *string         `json:"name,omitempty"`
	Status    *string         `json:"status,omitempty"`
	ParentID  *uuid.UUID      `json:"parent_id,omitempty"`
	Meta      json.RawMessage `json:"meta,omitempty"`
	SortOrder *int            `json:"sort_order,omitempty"`
}
