// Package event provides event sourcing primitives: Event Bus and Event Store.
package event

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/bizengine/engine/pkg/types"
)

// Store persists events in an append-only log.
type Store interface {
	// Append inserts a new event. Events are never updated or deleted.
	Append(ctx context.Context, event types.Event) error

	// AppendTx inserts a new event within an existing transaction.
	AppendTx(ctx context.Context, tx pgx.Tx, event types.Event) error

	// GetByEntity returns events for a specific entity, ordered by timestamp DESC.
	GetByEntity(ctx context.Context, orgID, entityID uuid.UUID, since *time.Time, limit int) ([]types.Event, error)

	// GetByOrganization returns events for an organization, ordered by timestamp DESC.
	GetByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]types.Event, int, error)

	// GetByType returns events of a specific type, ordered by timestamp DESC.
	GetByType(ctx context.Context, orgID uuid.UUID, eventType string, since *time.Time, limit int) ([]types.Event, error)
}
