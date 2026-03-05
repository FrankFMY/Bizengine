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

	// Organizations
	CreateOrganization(ctx context.Context, org *types.Organization) error
	GetOrganization(ctx context.Context, id uuid.UUID) (*types.Organization, error)
	GetOrganizationBySlug(ctx context.Context, slug string) (*types.Organization, error)
	ListUserOrganizations(ctx context.Context, userID uuid.UUID) ([]types.Organization, error)
	UpdateOrganization(ctx context.Context, org *types.Organization) error

	// Members
	AddMember(ctx context.Context, m *types.Labor) error
	GetMember(ctx context.Context, orgID, userID uuid.UUID) (*types.Labor, error)
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]types.Labor, error)
	UpdateMemberRole(ctx context.Context, orgID, userID uuid.UUID, role string) error
	RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error
}
