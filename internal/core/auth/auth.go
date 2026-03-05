// Package auth provides session-based authentication, RBAC authorization, and HTTP middleware.
package auth

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// Service provides authentication operations.
type Service struct {
	repo         Repository
	sessionStore SessionStore
	sessionTTL   time.Duration
	seanceTTL    time.Duration
}

// NewService creates a new auth service.
func NewService(repo Repository, store SessionStore, sessionTTL, seanceTTL time.Duration) *Service {
	return &Service{
		repo:         repo,
		sessionStore: store,
		sessionTTL:   sessionTTL,
		seanceTTL:    seanceTTL,
	}
}

// RegisterInput is the input for user registration.
type RegisterInput struct {
	Email    string    `json:"email"`
	Password string    `json:"password"`
	FullName string    `json:"full_name"`
	PhoneID  uuid.UUID `json:"phone_id"`
}

// LoginInput is the input for user login.
type LoginInput struct {
	Email    string    `json:"email"`
	Password string    `json:"password"`
	PhoneID  uuid.UUID `json:"phone_id"`
}

// LoginResult is the response for register/login.
type LoginResult struct {
	User          *types.User          `json:"user"`
	Organizations []types.Organization `json:"organizations,omitempty"`
	Session       *Session             `json:"-"`
	Seance        *Seance              `json:"-"`
}

// Register creates a new user account with a session.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*LoginResult, error) {
	if input.Email == "" || input.Password == "" {
		return nil, errs.NewBadRequest("email and password are required")
	}
	if len(input.Password) < 8 {
		return nil, errs.NewBadRequest("password must be at least 8 characters")
	}

	hash, err := HashSecret(input.Password)
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to hash password")
	}

	user := &types.User{
		ID:           uuid.New(),
		Email:        input.Email,
		PasswordHash: hash,
		FullName:     input.FullName,
		IsActive:     true,
	}

	if err := s.repo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	phoneID := input.PhoneID
	if phoneID == uuid.Nil {
		phoneID = uuid.New()
	}

	sess, seance, err := s.createSessionAndSeance(ctx, user, phoneID, uuid.Nil, "", nil)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		User:    user,
		Session: sess,
		Seance:  seance,
	}, nil
}

// Login authenticates a user with email and password.
func (s *Service) Login(ctx context.Context, input LoginInput) (*LoginResult, error) {
	user, err := s.repo.GetUserByEmail(ctx, input.Email)
	if err != nil {
		return nil, errs.NewUnauthorized("invalid credentials")
	}

	if !user.IsActive {
		return nil, errs.NewUnauthorized("account is deactivated")
	}

	match, err := VerifySecret(input.Password, user.PasswordHash)
	if err != nil || !match {
		return nil, errs.NewUnauthorized("invalid credentials")
	}

	organizations, err := s.repo.ListUserOrganizations(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	var orgID uuid.UUID
	var role string
	var perms []string
	if len(organizations) > 0 {
		orgID = organizations[0].ID
		member, err := s.repo.GetMember(ctx, organizations[0].ID, user.ID)
		if err == nil {
			role = member.Role
			if member.Admin {
				role = "owner"
			}
			perms = parseLaborPermissions(member.Permissions)
		}
	}

	phoneID := input.PhoneID
	if phoneID == uuid.Nil {
		phoneID = uuid.New()
	}

	sess, seance, err := s.createSessionAndSeance(ctx, user, phoneID, orgID, role, perms)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		User:          user,
		Organizations: organizations,
		Session:       sess,
		Seance:        seance,
	}, nil
}

// Logout deletes a session and all its seances.
func (s *Service) Logout(ctx context.Context, sessionID string) error {
	if err := s.sessionStore.DeleteSessionSeances(ctx, sessionID); err != nil {
		return err
	}
	return s.sessionStore.DeleteSession(ctx, sessionID)
}

// Unlock creates a new seance after password verification.
func (s *Service) Unlock(ctx context.Context, sessionID, password string) (*Seance, error) {
	sess, err := s.sessionStore.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}

	match, err := VerifySecret(password, user.PasswordHash)
	if err != nil || !match {
		return nil, errs.NewUnauthorized("invalid password")
	}

	seance := &Seance{
		ID:        uuid.New().String(),
		SessionID: sessionID,
		CreatedAt: time.Now(),
	}
	if err := s.sessionStore.CreateSeance(ctx, seance, s.seanceTTL); err != nil {
		return nil, err
	}

	return seance, nil
}

// SwitchOrganization updates the session's organization and role.
func (s *Service) SwitchOrganization(ctx context.Context, sessionID string, orgID uuid.UUID) (*Session, error) {
	sess, err := s.sessionStore.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	member, err := s.repo.GetMember(ctx, orgID, sess.UserID)
	if err != nil {
		return nil, errs.NewForbidden("not a member of this organization")
	}

	sess.OrganizationID = orgID
	sess.Role = member.Role
	if member.Admin {
		sess.Role = "owner"
	}
	sess.Permissions = parseLaborPermissions(member.Permissions)

	if err := s.sessionStore.UpdateSession(ctx, sess, s.sessionTTL); err != nil {
		return nil, err
	}

	return sess, nil
}

// GetSessionInfo returns session data for the /check endpoint.
func (s *Service) GetSessionInfo(ctx context.Context, sessionID string) (*Session, error) {
	return s.sessionStore.GetSession(ctx, sessionID)
}

// CreateOrganization creates a new organization and adds the owner as a member.
func (s *Service) CreateOrganization(ctx context.Context, userID uuid.UUID, name, slug string) (*types.Organization, error) {
	if name == "" || slug == "" {
		return nil, errs.NewBadRequest("name and slug are required")
	}

	org := &types.Organization{
		ID:       uuid.New(),
		Name:     name,
		Slug:     slug,
		OwnerID:  userID,
		Plan:     "free",
		Settings: json.RawMessage(`{}`),
	}

	if err := s.repo.CreateOrganization(ctx, org); err != nil {
		return nil, err
	}

	member := &types.Labor{
		OrganizationID: org.ID,
		UserID:         userID,
		Role:           "owner",
		Permissions:    json.RawMessage(`[]`),
	}
	if err := s.repo.AddMember(ctx, member); err != nil {
		return nil, err
	}

	return org, nil
}

// SessionTTL returns the configured session TTL.
func (s *Service) SessionTTL() time.Duration {
	return s.sessionTTL
}

// SeanceTTL returns the configured seance TTL (used by middleware).
func (s *Service) SeanceTTL() time.Duration {
	return s.seanceTTL
}

// Store returns the session store (used by middleware).
func (s *Service) Store() SessionStore {
	return s.sessionStore
}

// CreateSessionAndSeance creates a session and seance for the given user.
func (s *Service) CreateSessionAndSeance(ctx context.Context, user *types.User, phoneID, orgID uuid.UUID, role string, perms ...[]string) (*Session, *Seance, error) {
	var p []string
	if len(perms) > 0 {
		p = perms[0]
	}
	return s.createSessionAndSeance(ctx, user, phoneID, orgID, role, p)
}

func (s *Service) createSessionAndSeance(ctx context.Context, user *types.User, phoneID, orgID uuid.UUID, role string, perms []string) (*Session, *Seance, error) {
	sess := &Session{
		ID:             uuid.New().String(),
		UserID:         user.ID,
		PhoneID:        phoneID,
		OrganizationID: orgID,
		Role:           role,
		Permissions:    perms,
		Email:          user.Email,
		FullName:       user.FullName,
		CreatedAt:      time.Now(),
	}
	if err := s.sessionStore.CreateSession(ctx, sess, s.sessionTTL); err != nil {
		return nil, nil, err
	}

	seance := &Seance{
		ID:        uuid.New().String(),
		SessionID: sess.ID,
		CreatedAt: time.Now(),
	}
	if err := s.sessionStore.CreateSeance(ctx, seance, s.seanceTTL); err != nil {
		return nil, nil, err
	}

	return sess, seance, nil
}

func parseLaborPermissions(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var perms []string
	if err := json.Unmarshal(raw, &perms); err != nil {
		return nil
	}
	return perms
}
