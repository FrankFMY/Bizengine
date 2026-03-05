package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/core/entity"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// EntityRepo implements entity.Repository using PostgreSQL.
type EntityRepo struct {
	pool *pgxpool.Pool
}

// NewEntityRepo creates a new EntityRepo.
func NewEntityRepo(pool *pgxpool.Pool) *EntityRepo {
	return &EntityRepo{pool: pool}
}

// Create inserts a new entity.
func (r *EntityRepo) Create(ctx context.Context, e *types.Entity) error {
	return r.createWith(ctx, r.pool, e)
}

// CreateTx inserts a new entity within an existing transaction.
func (r *EntityRepo) CreateTx(ctx context.Context, tx pgx.Tx, e *types.Entity) error {
	return r.createWith(ctx, tx, e)
}

type queryable interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// ensure pgxpool.Pool implements queryable via its methods
// We need a common interface for pool and tx

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (interface{ RowsAffected() int64 }, error)
}

func (r *EntityRepo) createWith(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, e *types.Entity) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.Meta == nil {
		e.Meta = json.RawMessage(`{}`)
	}
	now := time.Now()
	e.CreatedAt = now
	e.UpdatedAt = now
	e.Status = "active"

	return q.QueryRow(ctx,
		`INSERT INTO entities (id, organization_id, kind, name, status, parent_id, meta, sort_order, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 RETURNING id, created_at, updated_at`,
		e.ID, e.OrganizationID, e.Kind, e.Name, e.Status, e.ParentID, e.Meta, e.SortOrder, e.CreatedAt, e.UpdatedAt,
	).Scan(&e.ID, &e.CreatedAt, &e.UpdatedAt)
}

// GetByID returns a single entity by ID within an organization.
func (r *EntityRepo) GetByID(ctx context.Context, orgID, id uuid.UUID) (*types.Entity, error) {
	var e types.Entity
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, kind, name, status, parent_id, meta, sort_order, created_at, updated_at, deleted_at
		 FROM entities
		 WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, id,
	).Scan(&e.ID, &e.OrganizationID, &e.Kind, &e.Name, &e.Status, &e.ParentID, &e.Meta, &e.SortOrder, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("entity not found")
		}
		return nil, err
	}
	return &e, nil
}

// List returns entities matching the filter.
func (r *EntityRepo) List(ctx context.Context, orgID uuid.UUID, filter entity.ListFilter) ([]types.Entity, int, error) {
	filter.Page.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("organization_id = $%d", argIdx))
	args = append(args, orgID)
	argIdx++

	conditions = append(conditions, "deleted_at IS NULL")

	if filter.Kind != nil {
		conditions = append(conditions, fmt.Sprintf("kind = $%d", argIdx))
		args = append(args, *filter.Kind)
		argIdx++
	}
	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.ParentID != nil {
		conditions = append(conditions, fmt.Sprintf("parent_id = $%d", argIdx))
		args = append(args, *filter.ParentID)
		argIdx++
	}
	if filter.Search != nil && *filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("name ILIKE '%%' || $%d || '%%'", argIdx))
		args = append(args, *filter.Search)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	// Count
	var total int
	countQuery := "SELECT COUNT(*) FROM entities WHERE " + where
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Validate sort column
	sortCol := "created_at"
	switch filter.Page.Sort {
	case "name", "kind", "status", "sort_order", "created_at", "updated_at":
		sortCol = filter.Page.Sort
	}

	orderDir := "DESC"
	if filter.Page.Order == "asc" {
		orderDir = "ASC"
	}

	query := fmt.Sprintf(
		`SELECT id, organization_id, kind, name, status, parent_id, meta, sort_order, created_at, updated_at, deleted_at
		 FROM entities WHERE %s ORDER BY %s %s LIMIT $%d OFFSET $%d`,
		where, sortCol, orderDir, argIdx, argIdx+1,
	)
	args = append(args, filter.Page.Limit, filter.Page.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var entities []types.Entity
	for rows.Next() {
		var e types.Entity
		if err := rows.Scan(&e.ID, &e.OrganizationID, &e.Kind, &e.Name, &e.Status, &e.ParentID, &e.Meta, &e.SortOrder, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt); err != nil {
			return nil, 0, err
		}
		entities = append(entities, e)
	}

	return entities, total, rows.Err()
}

// Update modifies an existing entity.
func (r *EntityRepo) Update(ctx context.Context, e *types.Entity) error {
	e.UpdatedAt = time.Now()
	tag, err := r.pool.Exec(ctx,
		`UPDATE entities SET name = $3, status = $4, parent_id = $5, meta = $6, sort_order = $7, updated_at = $8
		 WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL`,
		e.OrganizationID, e.ID, e.Name, e.Status, e.ParentID, e.Meta, e.SortOrder, e.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NewNotFound("entity not found")
	}
	return nil
}

// SoftDelete marks an entity as deleted.
func (r *EntityRepo) SoftDelete(ctx context.Context, orgID, id uuid.UUID) error {
	now := time.Now()
	tag, err := r.pool.Exec(ctx,
		`UPDATE entities SET deleted_at = $3, updated_at = $3 WHERE organization_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, id, now,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NewNotFound("entity not found")
	}
	return nil
}

// SetComponent upserts a component on an entity.
func (r *EntityRepo) SetComponent(ctx context.Context, c *types.Component) error {
	return r.setComponentWith(ctx, r.pool, c)
}

// SetComponentTx upserts a component within a transaction.
func (r *EntityRepo) SetComponentTx(ctx context.Context, tx pgx.Tx, c *types.Component) error {
	return r.setComponentWith(ctx, tx, c)
}

func (r *EntityRepo) setComponentWith(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, c *types.Component) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	now := time.Now()
	c.CreatedAt = now
	c.UpdatedAt = now

	return q.QueryRow(ctx,
		`INSERT INTO components (id, entity_id, organization_id, type, data, version, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, 1, $6, $7)
		 ON CONFLICT (entity_id, type) DO UPDATE SET
			data = EXCLUDED.data,
			version = components.version + 1,
			updated_at = EXCLUDED.updated_at
		 RETURNING id, version, created_at, updated_at`,
		c.ID, c.EntityID, c.OrganizationID, c.Type, c.Data, c.CreatedAt, c.UpdatedAt,
	).Scan(&c.ID, &c.Version, &c.CreatedAt, &c.UpdatedAt)
}

// GetComponent returns a specific component by type.
func (r *EntityRepo) GetComponent(ctx context.Context, orgID, entityID uuid.UUID, compType string) (*types.Component, error) {
	var c types.Component
	err := r.pool.QueryRow(ctx,
		`SELECT id, entity_id, organization_id, type, data, version, created_at, updated_at
		 FROM components
		 WHERE organization_id = $1 AND entity_id = $2 AND type = $3`,
		orgID, entityID, compType,
	).Scan(&c.ID, &c.EntityID, &c.OrganizationID, &c.Type, &c.Data, &c.Version, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("component not found")
		}
		return nil, err
	}
	return &c, nil
}

// ListComponents returns all components for an entity.
func (r *EntityRepo) ListComponents(ctx context.Context, orgID, entityID uuid.UUID) ([]types.Component, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, entity_id, organization_id, type, data, version, created_at, updated_at
		 FROM components
		 WHERE organization_id = $1 AND entity_id = $2
		 ORDER BY type`,
		orgID, entityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var components []types.Component
	for rows.Next() {
		var c types.Component
		if err := rows.Scan(&c.ID, &c.EntityID, &c.OrganizationID, &c.Type, &c.Data, &c.Version, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		components = append(components, c)
	}
	return components, rows.Err()
}

// DeleteComponent removes a component from an entity.
func (r *EntityRepo) DeleteComponent(ctx context.Context, orgID, entityID uuid.UUID, compType string) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM components WHERE organization_id = $1 AND entity_id = $2 AND type = $3`,
		orgID, entityID, compType,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NewNotFound("component not found")
	}
	return nil
}

// WithTx runs fn within a database transaction.
func (r *EntityRepo) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
