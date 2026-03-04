// Package auth provides JWT authentication, RBAC authorization, and HTTP middleware.
package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// Claims represents JWT claims for an access token.
type Claims struct {
	jwt.RegisteredClaims
	Email string `json:"email"`
	Name  string `json:"name"`
	WsID  string `json:"ws"`
	Role  string `json:"role"`
}

// Service provides authentication operations.
type Service struct {
	repo       Repository
	jwtSecret  []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewService creates a new auth service.
func NewService(repo Repository, jwtSecret string, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		repo:       repo,
		jwtSecret:  []byte(jwtSecret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// RegisterInput is the input for user registration.
type RegisterInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

// LoginInput is the input for user login.
type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResult is the response for register/login/refresh.
type AuthResult struct {
	User         *types.User      `json:"user"`
	AccessToken  string           `json:"access_token"`
	RefreshToken string           `json:"refresh_token"`
	Workspaces   []types.Workspace `json:"workspaces,omitempty"`
}

// Register creates a new user account.
func (s *Service) Register(ctx context.Context, input RegisterInput) (*AuthResult, error) {
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

	// Generate tokens (no workspace yet)
	accessToken, err := s.issueAccessToken(user, nil, "")
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.issueRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// Login authenticates a user with email and password.
func (s *Service) Login(ctx context.Context, input LoginInput) (*AuthResult, error) {
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

	// Issue token for first workspace if available
	var wsID *uuid.UUID
	var role string
	if len(workspaces) > 0 {
		wsID = &workspaces[0].ID
		member, err := s.repo.GetMember(ctx, workspaces[0].ID, user.ID)
		if err == nil {
			role = member.Role
		}
	}

	accessToken, err := s.issueAccessToken(user, wsID, role)
	if err != nil {
		return nil, err
	}

	refreshToken, err := s.issueRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Workspaces:   workspaces,
	}, nil
}

// Refresh generates a new access token from a refresh token.
func (s *Service) Refresh(ctx context.Context, refreshTokenRaw string) (*AuthResult, error) {
	hash := hashToken(refreshTokenRaw)
	rt, err := s.repo.GetRefreshToken(ctx, hash)
	if err != nil {
		return nil, errs.NewUnauthorized("invalid refresh token")
	}

	if rt.RevokedAt != nil {
		return nil, errs.NewUnauthorized("refresh token revoked")
	}

	if time.Now().After(rt.ExpiresAt) {
		return nil, errs.NewUnauthorized("refresh token expired")
	}

	user, err := s.repo.GetUserByID(ctx, rt.UserID)
	if err != nil {
		return nil, err
	}

	// Revoke old, issue new (rotation)
	s.repo.RevokeRefreshToken(ctx, hash)

	accessToken, err := s.issueAccessToken(user, nil, "")
	if err != nil {
		return nil, err
	}

	newRefreshToken, err := s.issueRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	return &AuthResult{
		User:         user,
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

// Logout revokes a refresh token.
func (s *Service) Logout(ctx context.Context, refreshTokenRaw string) error {
	hash := hashToken(refreshTokenRaw)
	return s.repo.RevokeRefreshToken(ctx, hash)
}

// SwitchWorkspace issues a new access token for a different workspace.
func (s *Service) SwitchWorkspace(ctx context.Context, userID, wsID uuid.UUID) (string, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		return "", err
	}

	member, err := s.repo.GetMember(ctx, wsID, userID)
	if err != nil {
		return "", errs.NewForbidden("not a member of this workspace")
	}

	return s.issueAccessToken(user, &wsID, member.Role)
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

// VerifyToken parses and validates a JWT access token.
func (s *Service) VerifyToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errs.NewUnauthorized("unexpected signing method")
		}
		return s.jwtSecret, nil
	})
	if err != nil {
		return nil, errs.NewUnauthorized("invalid token")
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errs.NewUnauthorized("invalid token claims")
	}
	return claims, nil
}

func (s *Service) issueAccessToken(user *types.User, wsID *uuid.UUID, role string) (string, error) {
	now := time.Now()
	claims := &Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   user.ID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL)),
		},
		Email: user.Email,
		Name:  user.FullName,
		Role:  role,
	}
	if wsID != nil {
		claims.WsID = wsID.String()
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}

func (s *Service) issueRefreshToken(ctx context.Context, userID uuid.UUID) (string, error) {
	raw := uuid.New().String()
	hash := hashToken(raw)
	expiresAt := time.Now().Add(s.refreshTTL)

	if _, err := s.repo.CreateRefreshToken(ctx, userID, hash, expiresAt); err != nil {
		return "", err
	}

	return raw, nil
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}
