// Package process provides the state machine (process) engine.
package process

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/pkg/dsl"
)

// Repository defines storage operations for process definitions and instances.
type Repository interface {
	// Definitions
	UpsertDefinition(ctx context.Context, def *DefinitionRecord) error
	GetDefinition(ctx context.Context, id string, orgID *uuid.UUID) (*DefinitionRecord, error)
	ListDefinitions(ctx context.Context, orgID uuid.UUID) ([]DefinitionRecord, error)

	// Instances
	CreateInstance(ctx context.Context, inst *Instance) error
	GetInstance(ctx context.Context, id uuid.UUID) (*Instance, error)
	GetActiveByEntity(ctx context.Context, entityID uuid.UUID) ([]Instance, error)
	ListInstances(ctx context.Context, orgID uuid.UUID, status *string, limit, offset int) ([]Instance, int, error)
	UpdateInstance(ctx context.Context, inst *Instance) error
}

// DefinitionRecord is the DB representation of a process definition.
type DefinitionRecord struct {
	ID         string                `json:"id"`
	OrganizationID *uuid.UUID           `json:"organization_id,omitempty"`
	Name       string                `json:"name"`
	Description string               `json:"description"`
	Definition dsl.ProcessDefinition `json:"definition"`
	IsActive   bool                  `json:"is_active"`
	Version    int                   `json:"version"`
	CreatedAt  time.Time             `json:"created_at"`
	UpdatedAt  time.Time             `json:"updated_at"`
}

// Instance represents a running process instance.
type Instance struct {
	ID           uuid.UUID       `json:"id"`
	OrganizationID  uuid.UUID       `json:"organization_id"`
	DefinitionID string          `json:"definition_id"`
	EntityID     uuid.UUID       `json:"entity_id"`
	CurrentState string          `json:"current_state"`
	Status       string          `json:"status"`
	Context      json.RawMessage `json:"context"`
	History      json.RawMessage `json:"history"`
	StartedAt    time.Time       `json:"started_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
	CompletedAt  *time.Time      `json:"completed_at,omitempty"`
}

// HistoryEntry records a single state transition.
type HistoryEntry struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
}
