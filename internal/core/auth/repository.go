package auth

import (
	"context"

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
}
