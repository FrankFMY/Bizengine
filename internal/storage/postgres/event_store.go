package postgres

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/pkg/types"
)

// EventStore implements event.Store using PostgreSQL.
type EventStore struct {
	pool *pgxpool.Pool
}

// NewEventStore creates a new EventStore.
func NewEventStore(pool *pgxpool.Pool) *EventStore {
	return &EventStore{pool: pool}
}

// Append inserts a new event into the append-only event store.
func (s *EventStore) Append(ctx context.Context, ev types.Event) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO events (id, organization_id, entity_id, type, data, actor_id, timestamp, version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (id, "timestamp") DO NOTHING`,
		ev.ID, ev.OrganizationID, ev.EntityID, ev.Type, ev.Data, ev.ActorID, ev.Timestamp, ev.Version,
	)
	return err
}

// AppendTx inserts a new event within an existing transaction.
func (s *EventStore) AppendTx(ctx context.Context, tx pgx.Tx, ev types.Event) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO events (id, organization_id, entity_id, type, data, actor_id, timestamp, version)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		ev.ID, ev.OrganizationID, ev.EntityID, ev.Type, ev.Data, ev.ActorID, ev.Timestamp, ev.Version,
	)
	return err
}

// GetByEntity returns events for a specific entity, ordered by timestamp DESC.
func (s *EventStore) GetByEntity(ctx context.Context, orgID, entityID uuid.UUID, since *time.Time, limit int) ([]types.Event, error) {
	if limit <= 0 {
		limit = 100
	}

	var rows pgx.Rows
	var err error
	if since != nil {
		rows, err = s.pool.Query(ctx,
			`SELECT id, organization_id, entity_id, type, data, actor_id, timestamp, version
			 FROM events
			 WHERE organization_id = $1 AND entity_id = $2 AND timestamp > $3
			 ORDER BY timestamp DESC LIMIT $4`,
			orgID, entityID, *since, limit,
		)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT id, organization_id, entity_id, type, data, actor_id, timestamp, version
			 FROM events
			 WHERE organization_id = $1 AND entity_id = $2
			 ORDER BY timestamp DESC LIMIT $3`,
			orgID, entityID, limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEvents(rows)
}

// GetByOrganization returns events for an organization, ordered by timestamp DESC.
func (s *EventStore) GetByOrganization(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]types.Event, int, error) {
	if limit <= 0 {
		limit = 100
	}

	var total int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM events WHERE organization_id = $1`, orgID,
	).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := s.pool.Query(ctx,
		`SELECT id, organization_id, entity_id, type, data, actor_id, timestamp, version
		 FROM events
		 WHERE organization_id = $1
		 ORDER BY timestamp DESC LIMIT $2 OFFSET $3`,
		orgID, limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	events, err := scanEvents(rows)
	return events, total, err
}

// GetByType returns events of a specific type, ordered by timestamp DESC.
func (s *EventStore) GetByType(ctx context.Context, orgID uuid.UUID, eventType string, since *time.Time, limit int) ([]types.Event, error) {
	if limit <= 0 {
		limit = 100
	}

	var rows pgx.Rows
	var err error
	if since != nil {
		rows, err = s.pool.Query(ctx,
			`SELECT id, organization_id, entity_id, type, data, actor_id, timestamp, version
			 FROM events
			 WHERE organization_id = $1 AND type = $2 AND timestamp > $3
			 ORDER BY timestamp DESC LIMIT $4`,
			orgID, eventType, *since, limit,
		)
	} else {
		rows, err = s.pool.Query(ctx,
			`SELECT id, organization_id, entity_id, type, data, actor_id, timestamp, version
			 FROM events
			 WHERE organization_id = $1 AND type = $2
			 ORDER BY timestamp DESC LIMIT $3`,
			orgID, eventType, limit,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanEvents(rows)
}

func scanEvents(rows pgx.Rows) ([]types.Event, error) {
	var events []types.Event
	for rows.Next() {
		var ev types.Event
		if err := rows.Scan(
			&ev.ID, &ev.OrganizationID, &ev.EntityID, &ev.Type,
			&ev.Data, &ev.ActorID, &ev.Timestamp, &ev.Version,
		); err != nil {
			return nil, err
		}
		events = append(events, ev)
	}
	return events, rows.Err()
}
