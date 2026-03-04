package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/core/process"
	"github.com/bizengine/engine/pkg/errs"
)

// ProcessRepo implements process.Repository using PostgreSQL.
type ProcessRepo struct {
	pool *pgxpool.Pool
}

// NewProcessRepo creates a new ProcessRepo.
func NewProcessRepo(pool *pgxpool.Pool) *ProcessRepo {
	return &ProcessRepo{pool: pool}
}

// UpsertDefinition creates or updates a process definition.
func (r *ProcessRepo) UpsertDefinition(ctx context.Context, def *process.DefinitionRecord) error {
	defJSON, err := json.Marshal(def.Definition)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`INSERT INTO process_definitions (id, workspace_id, name, description, definition, is_active, version, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 ON CONFLICT (id, COALESCE(workspace_id, '00000000-0000-0000-0000-000000000000'))
		 DO UPDATE SET name = $3, description = $4, definition = $5, is_active = $6, version = $7, updated_at = $9`,
		def.ID, def.WorkspaceID, def.Name, def.Description, defJSON, def.IsActive, def.Version, def.CreatedAt, def.UpdatedAt,
	)
	return err
}

// GetDefinition returns a process definition by ID and optional workspace.
func (r *ProcessRepo) GetDefinition(ctx context.Context, id string, wsID *uuid.UUID) (*process.DefinitionRecord, error) {
	var def process.DefinitionRecord
	var defJSON []byte
	err := r.pool.QueryRow(ctx,
		`SELECT id, workspace_id, name, description, definition, is_active, version, created_at, updated_at
		 FROM process_definitions
		 WHERE id = $1 AND (workspace_id = $2 OR workspace_id IS NULL)
		 ORDER BY workspace_id DESC NULLS LAST LIMIT 1`,
		id, wsID,
	).Scan(&def.ID, &def.WorkspaceID, &def.Name, &def.Description, &defJSON, &def.IsActive, &def.Version, &def.CreatedAt, &def.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("process definition not found")
		}
		return nil, err
	}
	if err := json.Unmarshal(defJSON, &def.Definition); err != nil {
		return nil, err
	}
	return &def, nil
}

// ListDefinitions returns all definitions accessible to a workspace (own + system).
func (r *ProcessRepo) ListDefinitions(ctx context.Context, wsID uuid.UUID) ([]process.DefinitionRecord, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, workspace_id, name, description, definition, is_active, version, created_at, updated_at
		 FROM process_definitions
		 WHERE workspace_id = $1 OR workspace_id IS NULL
		 ORDER BY name`,
		wsID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var defs []process.DefinitionRecord
	for rows.Next() {
		var def process.DefinitionRecord
		var defJSON []byte
		if err := rows.Scan(&def.ID, &def.WorkspaceID, &def.Name, &def.Description, &defJSON, &def.IsActive, &def.Version, &def.CreatedAt, &def.UpdatedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(defJSON, &def.Definition); err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, rows.Err()
}

// CreateInstance creates a new process instance.
func (r *ProcessRepo) CreateInstance(ctx context.Context, inst *process.Instance) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO process_instances (id, workspace_id, definition_id, entity_id, current_state, status, context, history, started_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		inst.ID, inst.WorkspaceID, inst.DefinitionID, inst.EntityID, inst.CurrentState,
		inst.Status, inst.Context, inst.History, inst.StartedAt, inst.UpdatedAt,
	)
	return err
}

// GetInstance returns a process instance by ID.
func (r *ProcessRepo) GetInstance(ctx context.Context, id uuid.UUID) (*process.Instance, error) {
	var inst process.Instance
	err := r.pool.QueryRow(ctx,
		`SELECT id, workspace_id, definition_id, entity_id, current_state, status, context, history, started_at, updated_at, completed_at
		 FROM process_instances WHERE id = $1`,
		id,
	).Scan(&inst.ID, &inst.WorkspaceID, &inst.DefinitionID, &inst.EntityID, &inst.CurrentState,
		&inst.Status, &inst.Context, &inst.History, &inst.StartedAt, &inst.UpdatedAt, &inst.CompletedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("process instance not found")
		}
		return nil, err
	}
	return &inst, nil
}

// GetActiveByEntity returns active process instances for an entity.
func (r *ProcessRepo) GetActiveByEntity(ctx context.Context, entityID uuid.UUID) ([]process.Instance, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, workspace_id, definition_id, entity_id, current_state, status, context, history, started_at, updated_at, completed_at
		 FROM process_instances WHERE entity_id = $1 AND status = 'active'`,
		entityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []process.Instance
	for rows.Next() {
		var inst process.Instance
		if err := rows.Scan(&inst.ID, &inst.WorkspaceID, &inst.DefinitionID, &inst.EntityID, &inst.CurrentState,
			&inst.Status, &inst.Context, &inst.History, &inst.StartedAt, &inst.UpdatedAt, &inst.CompletedAt); err != nil {
			return nil, err
		}
		instances = append(instances, inst)
	}
	return instances, rows.Err()
}

// ListInstances returns process instances for a workspace.
func (r *ProcessRepo) ListInstances(ctx context.Context, wsID uuid.UUID, status *string, limit, offset int) ([]process.Instance, int, error) {
	baseWhere := "workspace_id = $1"
	args := []any{wsID}
	argIdx := 2

	if status != nil {
		baseWhere += " AND status = $2"
		args = append(args, *status)
		argIdx++
	}

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM process_instances WHERE "+baseWhere, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := "SELECT id, workspace_id, definition_id, entity_id, current_state, status, context, history, started_at, updated_at, completed_at FROM process_instances WHERE " +
		baseWhere + " ORDER BY started_at DESC LIMIT $" + itoa(argIdx) + " OFFSET $" + itoa(argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var instances []process.Instance
	for rows.Next() {
		var inst process.Instance
		if err := rows.Scan(&inst.ID, &inst.WorkspaceID, &inst.DefinitionID, &inst.EntityID, &inst.CurrentState,
			&inst.Status, &inst.Context, &inst.History, &inst.StartedAt, &inst.UpdatedAt, &inst.CompletedAt); err != nil {
			return nil, 0, err
		}
		instances = append(instances, inst)
	}
	return instances, total, rows.Err()
}

// UpdateInstance updates a process instance.
func (r *ProcessRepo) UpdateInstance(ctx context.Context, inst *process.Instance) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE process_instances SET current_state = $2, status = $3, context = $4, history = $5, updated_at = $6, completed_at = $7
		 WHERE id = $1`,
		inst.ID, inst.CurrentState, inst.Status, inst.Context, inst.History, inst.UpdatedAt, inst.CompletedAt,
	)
	return err
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}
