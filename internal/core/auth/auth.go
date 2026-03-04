// Package auth provides session-based authentication, RBAC authorization, and HTTP middleware.
package auth

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

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
	User       *types.User       `json:"user"`
	Workspaces []types.Workspace `json:"workspaces,omitempty"`
	Session    *Session          `json:"-"`
	Seance     *Seance           `json:"-"`
}

// Register creates a new user account with a session.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*LoginResult, error) {
	if input.Email == "" || input.Password == "" {
		return nil, errs.NewBadRequest("email and password are required")
	}
	if len(input.Password) < 8 {
		return nil, errs.NewBadRequest("password must be at least 8 characters")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(input.Password), 12)
	if err != nil {
		return nil, errs.Wrap(err, errs.CodeInternal, "failed to hash password")
	}

	user := &types.User{
		ID:           uuid.New(),
		Email:        input.Email,
		PasswordHash: string(hash),
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

	sess, seance, err := s.createSessionAndSeance(ctx, user, phoneID, uuid.Nil, "")
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

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return nil, errs.NewUnauthorized("invalid credentials")
	}

	workspaces, err := s.repo.ListUserWorkspaces(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	var wsID uuid.UUID
	var role string
	if len(workspaces) > 0 {
		wsID = workspaces[0].ID
		member, err := s.repo.GetMember(ctx, workspaces[0].ID, user.ID)
		if err == nil {
			role = member.Role
		}
	}

	phoneID := input.PhoneID
	if phoneID == uuid.Nil {
		phoneID = uuid.New()
	}

	sess, seance, err := s.createSessionAndSeance(ctx, user, phoneID, wsID, role)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		User:       user,
		Workspaces: workspaces,
		Session:    sess,
		Seance:     seance,
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

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
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

// SwitchWorkspace updates the session's workspace and role.
func (s *Service) SwitchWorkspace(ctx context.Context, sessionID string, wsID uuid.UUID) (*Session, error) {
	sess, err := s.sessionStore.GetSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	member, err := s.repo.GetMember(ctx, wsID, sess.UserID)
	if err != nil {
		return nil, errs.NewForbidden("not a member of this workspace")
	}

	sess.WorkspaceID = wsID
	sess.Role = member.Role

	if err := s.sessionStore.UpdateSession(ctx, sess, s.sessionTTL); err != nil {
		return nil, err
	}

	return sess, nil
}

// GetSessionInfo returns session data for the /check endpoint.
func (s *Service) GetSessionInfo(ctx context.Context, sessionID string) (*Session, error) {
	return s.sessionStore.GetSession(ctx, sessionID)
}

// CreateWorkspace creates a new workspace and adds the owner as a member.
func (s *Service) CreateWorkspace(ctx context.Context, userID uuid.UUID, name, slug string) (*types.Workspace, error) {
	if name == "" || slug == "" {
		return nil, errs.NewBadRequest("name and slug are required")
	}

	ws := &types.Workspace{
		ID:       uuid.New(),
		Name:     name,
		Slug:     slug,
		OwnerID:  userID,
		Plan:     "free",
		Settings: json.RawMessage(`{}`),
	}

	if err := s.repo.CreateWorkspace(ctx, ws); err != nil {
		return nil, err
	}

	member := &types.WorkspaceMember{
		WorkspaceID: ws.ID,
		UserID:      userID,
		Role:        "owner",
		Permissions: json.RawMessage(`[]`),
	}
	if err := s.repo.AddMember(ctx, member); err != nil {
		return nil, err
	}

	return ws, nil
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

func (s *Service) createSessionAndSeance(ctx context.Context, user *types.User, phoneID, wsID uuid.UUID, role string) (*Session, *Seance, error) {
	sess := &Session{
		ID:          uuid.New().String(),
		UserID:      user.ID,
		PhoneID:     phoneID,
		WorkspaceID: wsID,
		Role:        role,
		Email:       user.Email,
		FullName:    user.FullName,
		CreatedAt:   time.Now(),
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
