package auth

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/pkg/types"
)

// Repository defines storage operations for authentication.
type Repository interface {
	// Users
	CreateUser(ctx context.Context, u *types.User) error
	GetUserByEmail(ctx context.Context, email string) (*types.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*types.User, error)

	// Workspaces
	CreateWorkspace(ctx context.Context, ws *types.Workspace) error
	GetWorkspace(ctx context.Context, id uuid.UUID) (*types.Workspace, error)
	GetWorkspaceBySlug(ctx context.Context, slug string) (*types.Workspace, error)
	ListUserWorkspaces(ctx context.Context, userID uuid.UUID) ([]types.Workspace, error)
	UpdateWorkspace(ctx context.Context, ws *types.Workspace) error

	// Members
	AddMember(ctx context.Context, m *types.WorkspaceMember) error
	GetMember(ctx context.Context, wsID, userID uuid.UUID) (*types.WorkspaceMember, error)
	ListMembers(ctx context.Context, wsID uuid.UUID) ([]types.WorkspaceMember, error)
	UpdateMemberRole(ctx context.Context, wsID, userID uuid.UUID, role string) error
	RemoveMember(ctx context.Context, wsID, userID uuid.UUID) error

	// Refresh Tokens
	CreateRefreshToken(ctx context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (uuid.UUID, error)
	GetRefreshToken(ctx context.Context, tokenHash string) (*RefreshToken, error)
	RevokeRefreshToken(ctx context.Context, tokenHash string) error
	RevokeAllUserTokens(ctx context.Context, userID uuid.UUID) error
}

// RefreshToken represents a stored refresh token.
type RefreshToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	CreatedAt time.Time
	RevokedAt *time.Time
}
