package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// AuthRepo implements auth.Repository using PostgreSQL.
type AuthRepo struct {
	pool *pgxpool.Pool
}

// NewAuthRepo creates a new AuthRepo.
func NewAuthRepo(pool *pgxpool.Pool) *AuthRepo {
	return &AuthRepo{pool: pool}
}

// CreateUser inserts a new user.
func (r *AuthRepo) CreateUser(ctx context.Context, u *types.User) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now

	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name, phone, is_active, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		u.ID, u.Email, u.PasswordHash, u.FullName, u.Phone, u.IsActive, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return errs.NewConflict("email already registered")
		}
		return err
	}
	return nil
}

// GetUserByEmail returns a user by email.
func (r *AuthRepo) GetUserByEmail(ctx context.Context, email string) (*types.User, error) {
	var u types.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, full_name, phone, is_active, created_at, updated_at
		 FROM users WHERE lower(email) = lower($1)`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.Phone, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("user not found")
		}
		return nil, err
	}
	return &u, nil
}

// GetUserByID returns a user by ID.
func (r *AuthRepo) GetUserByID(ctx context.Context, id uuid.UUID) (*types.User, error) {
	var u types.User
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, full_name, phone, is_active, created_at, updated_at
		 FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.Phone, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("user not found")
		}
		return nil, err
	}
	return &u, nil
}

// CreateWorkspace inserts a new workspace.
func (r *AuthRepo) CreateWorkspace(ctx context.Context, ws *types.Workspace) error {
	if ws.ID == uuid.Nil {
		ws.ID = uuid.New()
	}
	now := time.Now()
	ws.CreatedAt = now
	ws.UpdatedAt = now
	if ws.Settings == nil {
		ws.Settings = json.RawMessage(`{}`)
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO workspaces (id, name, slug, owner_id, plan, settings, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		ws.ID, ws.Name, ws.Slug, ws.OwnerID, ws.Plan, ws.Settings, ws.CreatedAt, ws.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return errs.NewConflict("workspace slug already taken")
		}
		return err
	}
	return nil
}

// GetWorkspace returns a workspace by ID.
func (r *AuthRepo) GetWorkspace(ctx context.Context, id uuid.UUID) (*types.Workspace, error) {
	var ws types.Workspace
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, owner_id, plan, settings, created_at, updated_at
		 FROM workspaces WHERE id = $1`, id,
	).Scan(&ws.ID, &ws.Name, &ws.Slug, &ws.OwnerID, &ws.Plan, &ws.Settings, &ws.CreatedAt, &ws.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("workspace not found")
		}
		return nil, err
	}
	return &ws, nil
}

// GetWorkspaceBySlug returns a workspace by slug.
func (r *AuthRepo) GetWorkspaceBySlug(ctx context.Context, slug string) (*types.Workspace, error) {
	var ws types.Workspace
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, owner_id, plan, settings, created_at, updated_at
		 FROM workspaces WHERE slug = $1`, slug,
	).Scan(&ws.ID, &ws.Name, &ws.Slug, &ws.OwnerID, &ws.Plan, &ws.Settings, &ws.CreatedAt, &ws.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("workspace not found")
		}
		return nil, err
	}
	return &ws, nil
}

// ListUserWorkspaces returns all workspaces a user belongs to.
func (r *AuthRepo) ListUserWorkspaces(ctx context.Context, userID uuid.UUID) ([]types.Workspace, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT w.id, w.name, w.slug, w.owner_id, w.plan, w.settings, w.created_at, w.updated_at
		 FROM workspaces w
		 JOIN workspace_members wm ON w.id = wm.workspace_id
		 WHERE wm.user_id = $1
		 ORDER BY w.name`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workspaces []types.Workspace
	for rows.Next() {
		var ws types.Workspace
		if err := rows.Scan(&ws.ID, &ws.Name, &ws.Slug, &ws.OwnerID, &ws.Plan, &ws.Settings, &ws.CreatedAt, &ws.UpdatedAt); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, ws)
	}
	return workspaces, rows.Err()
}

// UpdateWorkspace updates workspace fields.
func (r *AuthRepo) UpdateWorkspace(ctx context.Context, ws *types.Workspace) error {
	ws.UpdatedAt = time.Now()
	tag, err := r.pool.Exec(ctx,
		`UPDATE workspaces SET name = $2, settings = $3, updated_at = $4 WHERE id = $1`,
		ws.ID, ws.Name, ws.Settings, ws.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NewNotFound("workspace not found")
	}
	return nil
}

// AddMember adds a user to a workspace.
func (r *AuthRepo) AddMember(ctx context.Context, m *types.WorkspaceMember) error {
	if m.Permissions == nil {
		m.Permissions = json.RawMessage(`[]`)
	}
	m.JoinedAt = time.Now()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO workspace_members (workspace_id, user_id, role, permissions, joined_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		m.WorkspaceID, m.UserID, m.Role, m.Permissions, m.JoinedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return errs.NewConflict("user already a member")
		}
		return err
	}
	return nil
}

// GetMember returns a workspace member.
func (r *AuthRepo) GetMember(ctx context.Context, wsID, userID uuid.UUID) (*types.WorkspaceMember, error) {
	var m types.WorkspaceMember
	err := r.pool.QueryRow(ctx,
		`SELECT workspace_id, user_id, role, permissions, joined_at
		 FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`,
		wsID, userID,
	).Scan(&m.WorkspaceID, &m.UserID, &m.Role, &m.Permissions, &m.JoinedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("member not found")
		}
		return nil, err
	}
	return &m, nil
}

// ListMembers returns all members of a workspace.
func (r *AuthRepo) ListMembers(ctx context.Context, wsID uuid.UUID) ([]types.WorkspaceMember, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT workspace_id, user_id, role, permissions, joined_at
		 FROM workspace_members WHERE workspace_id = $1
		 ORDER BY joined_at`, wsID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []types.WorkspaceMember
	for rows.Next() {
		var m types.WorkspaceMember
		if err := rows.Scan(&m.WorkspaceID, &m.UserID, &m.Role, &m.Permissions, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// UpdateMemberRole changes a member's role.
func (r *AuthRepo) UpdateMemberRole(ctx context.Context, wsID, userID uuid.UUID, role string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE workspace_members SET role = $3 WHERE workspace_id = $1 AND user_id = $2`,
		wsID, userID, role,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NewNotFound("member not found")
	}
	return nil
}

// RemoveMember removes a user from a workspace.
func (r *AuthRepo) RemoveMember(ctx context.Context, wsID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM workspace_members WHERE workspace_id = $1 AND user_id = $2`,
		wsID, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NewNotFound("member not found")
	}
	return nil
}

// CreateRefreshToken stores a hashed refresh token.
func (r *AuthRepo) CreateRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (uuid.UUID, error) {
	id := uuid.New()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO refresh_tokens (id, user_id, token_hash, expires_at, created_at)
		 VALUES ($1, $2, $3, $4, NOW())`,
		id, userID, tokenHash, expiresAt,
	)
	return id, err
}

// GetRefreshToken returns a refresh token by hash.
func (r *AuthRepo) GetRefreshToken(ctx context.Context, tokenHash string) (*auth.RefreshToken, error) {
	var rt auth.RefreshToken
	err := r.pool.QueryRow(ctx,
		`SELECT id, user_id, token_hash, expires_at, created_at, revoked_at
		 FROM refresh_tokens WHERE token_hash = $1`, tokenHash,
	).Scan(&rt.ID, &rt.UserID, &rt.TokenHash, &rt.ExpiresAt, &rt.CreatedAt, &rt.RevokedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("refresh token not found")
		}
		return nil, err
	}
	return &rt, nil
}

// RevokeRefreshToken marks a refresh token as revoked.
func (r *AuthRepo) RevokeRefreshToken(ctx context.Context, tokenHash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE token_hash = $1 AND revoked_at IS NULL`,
		tokenHash,
	)
	return err
}

// RevokeAllUserTokens revokes all refresh tokens for a user.
func (r *AuthRepo) RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE refresh_tokens SET revoked_at = NOW() WHERE user_id = $1 AND revoked_at IS NULL`,
		userID,
	)
	return err
}

func isDuplicateKey(err error) bool {
	return err != nil && (contains(err.Error(), "duplicate key") || contains(err.Error(), "23505"))
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
