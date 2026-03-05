package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

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

// CreateOrganization inserts a new organization.
func (r *AuthRepo) CreateOrganization(ctx context.Context, org *types.Organization) error {
	if org.ID == uuid.Nil {
		org.ID = uuid.New()
	}
	now := time.Now()
	org.CreatedAt = now
	org.UpdatedAt = now
	if org.Settings == nil {
		org.Settings = json.RawMessage(`{}`)
	}

	_, err := r.pool.Exec(ctx,
		`INSERT INTO organizations (id, name, slug, owner_id, plan, settings, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		org.ID, org.Name, org.Slug, org.OwnerID, org.Plan, org.Settings, org.CreatedAt, org.UpdatedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return errs.NewConflict("organization slug already taken")
		}
		return err
	}
	return nil
}

// GetOrganization returns an organization by ID.
func (r *AuthRepo) GetOrganization(ctx context.Context, id uuid.UUID) (*types.Organization, error) {
	var org types.Organization
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, owner_id, plan, settings, created_at, updated_at
		 FROM organizations WHERE id = $1`, id,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.OwnerID, &org.Plan, &org.Settings, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("organization not found")
		}
		return nil, err
	}
	return &org, nil
}

// GetOrganizationBySlug returns an organization by slug.
func (r *AuthRepo) GetOrganizationBySlug(ctx context.Context, slug string) (*types.Organization, error) {
	var org types.Organization
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, owner_id, plan, settings, created_at, updated_at
		 FROM organizations WHERE slug = $1`, slug,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.OwnerID, &org.Plan, &org.Settings, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("organization not found")
		}
		return nil, err
	}
	return &org, nil
}

// ListUserOrganizations returns all organizations a user belongs to.
func (r *AuthRepo) ListUserOrganizations(ctx context.Context, userID uuid.UUID) ([]types.Organization, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT w.id, w.name, w.slug, w.owner_id, w.plan, w.settings, w.created_at, w.updated_at
		 FROM organizations w
		 JOIN labors wm ON w.id = wm.organization_id
		 WHERE wm.user_id = $1
		 ORDER BY w.name`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var organizations []types.Organization
	for rows.Next() {
		var org types.Organization
		if err := rows.Scan(&org.ID, &org.Name, &org.Slug, &org.OwnerID, &org.Plan, &org.Settings, &org.CreatedAt, &org.UpdatedAt); err != nil {
			return nil, err
		}
		organizations = append(organizations, org)
	}
	return organizations, rows.Err()
}

// UpdateOrganization updates organization fields.
func (r *AuthRepo) UpdateOrganization(ctx context.Context, org *types.Organization) error {
	org.UpdatedAt = time.Now()
	tag, err := r.pool.Exec(ctx,
		`UPDATE organizations SET name = $2, settings = $3, updated_at = $4 WHERE id = $1`,
		org.ID, org.Name, org.Settings, org.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NewNotFound("organization not found")
	}
	return nil
}

// AddMember adds a user to an organization.
func (r *AuthRepo) AddMember(ctx context.Context, m *types.Labor) error {
	if m.Permissions == nil {
		m.Permissions = json.RawMessage(`[]`)
	}
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	m.JoinedAt = time.Now()
	_, err := r.pool.Exec(ctx,
		`INSERT INTO labors (id, organization_id, user_id, role, permissions, admin, joined_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		m.ID, m.OrganizationID, m.UserID, m.Role, m.Permissions, m.Admin, m.JoinedAt,
	)
	if err != nil {
		if isDuplicateKey(err) {
			return errs.NewConflict("user already a member")
		}
		return err
	}
	return nil
}

// GetMember returns an organization member.
func (r *AuthRepo) GetMember(ctx context.Context, orgID, userID uuid.UUID) (*types.Labor, error) {
	var m types.Labor
	err := r.pool.QueryRow(ctx,
		`SELECT organization_id, user_id, role, permissions, admin, joined_at
		 FROM labors WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&m.OrganizationID, &m.UserID, &m.Role, &m.Permissions, &m.Admin, &m.JoinedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("member not found")
		}
		return nil, err
	}
	return &m, nil
}

// ListMembers returns all members of an organization.
func (r *AuthRepo) ListMembers(ctx context.Context, orgID uuid.UUID) ([]types.Labor, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT organization_id, user_id, role, permissions, admin, joined_at
		 FROM labors WHERE organization_id = $1
		 ORDER BY joined_at`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []types.Labor
	for rows.Next() {
		var m types.Labor
		if err := rows.Scan(&m.OrganizationID, &m.UserID, &m.Role, &m.Permissions, &m.Admin, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

// UpdateMemberRole changes a member's role.
func (r *AuthRepo) UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role string) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE labors SET role = $3 WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID, role,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NewNotFound("member not found")
	}
	return nil
}

// RemoveMember removes a user from an organization.
func (r *AuthRepo) RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM labors WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NewNotFound("member not found")
	}
	return nil
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
