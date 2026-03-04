package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// --- auth repo mock ---

type mockAuthRepo struct {
	users         map[string]*types.User
	usersById     map[uuid.UUID]*types.User
	workspaces    map[uuid.UUID]*types.Workspace
	members       map[string]*types.WorkspaceMember
	refreshTokens map[string]*auth.RefreshToken
}

func newMockAuthRepo() *mockAuthRepo {
	return &mockAuthRepo{
		users:         make(map[string]*types.User),
		usersById:     make(map[uuid.UUID]*types.User),
		workspaces:    make(map[uuid.UUID]*types.Workspace),
		members:       make(map[string]*types.WorkspaceMember),
		refreshTokens: make(map[string]*auth.RefreshToken),
	}
}

func (m *mockAuthRepo) CreateUser(_ context.Context, u *types.User) error {
	if _, exists := m.users[u.Email]; exists {
		return errs.NewConflict("email already exists")
	}
	cp := *u
	m.users[u.Email] = &cp
	m.usersById[u.ID] = &cp
	return nil
}

func (m *mockAuthRepo) GetUserByEmail(_ context.Context, email string) (*types.User, error) {
	u, ok := m.users[email]
	if !ok {
		return nil, errs.NewNotFound("user not found")
	}
	cp := *u
	return &cp, nil
}

func (m *mockAuthRepo) GetUserByID(_ context.Context, id uuid.UUID) (*types.User, error) {
	u, ok := m.usersById[id]
	if !ok {
		return nil, errs.NewNotFound("user not found")
	}
	cp := *u
	return &cp, nil
}

func (m *mockAuthRepo) CreateWorkspace(_ context.Context, ws *types.Workspace) error {
	cp := *ws
	m.workspaces[ws.ID] = &cp
	return nil
}

func (m *mockAuthRepo) GetWorkspace(_ context.Context, id uuid.UUID) (*types.Workspace, error) {
	ws, ok := m.workspaces[id]
	if !ok {
		return nil, errs.NewNotFound("workspace not found")
	}
	cp := *ws
	return &cp, nil
}

func (m *mockAuthRepo) GetWorkspaceBySlug(_ context.Context, slug string) (*types.Workspace, error) {
	for _, ws := range m.workspaces {
		if ws.Slug == slug {
			cp := *ws
			return &cp, nil
		}
	}
	return nil, errs.NewNotFound("workspace not found")
}

func (m *mockAuthRepo) ListUserWorkspaces(_ context.Context, userID uuid.UUID) ([]types.Workspace, error) {
	var result []types.Workspace
	for key, mem := range m.members {
		_ = key
		if mem.UserID == userID {
			if ws, ok := m.workspaces[mem.WorkspaceID]; ok {
				result = append(result, *ws)
			}
		}
	}
	return result, nil
}

func (m *mockAuthRepo) UpdateWorkspace(_ context.Context, ws *types.Workspace) error {
	m.workspaces[ws.ID] = ws
	return nil
}

func (m *mockAuthRepo) AddMember(_ context.Context, mem *types.WorkspaceMember) error {
	key := mem.WorkspaceID.String() + ":" + mem.UserID.String()
	cp := *mem
	m.members[key] = &cp
	return nil
}

func (m *mockAuthRepo) GetMember(_ context.Context, wsID, userID uuid.UUID) (*types.WorkspaceMember, error) {
	key := wsID.String() + ":" + userID.String()
	mem, ok := m.members[key]
	if !ok {
		return nil, errs.NewNotFound("member not found")
	}
	cp := *mem
	return &cp, nil
}

func (m *mockAuthRepo) ListMembers(_ context.Context, wsID uuid.UUID) ([]types.WorkspaceMember, error) {
	return nil, nil
}

func (m *mockAuthRepo) UpdateMemberRole(_ context.Context, wsID, userID uuid.UUID, role string) error {
	return nil
}

func (m *mockAuthRepo) RemoveMember(_ context.Context, wsID, userID uuid.UUID) error {
	return nil
}

func (m *mockAuthRepo) CreateRefreshToken(_ context.Context, userID uuid.UUID, tokenHash string, expiresAt time.Time) (uuid.UUID, error) {
	id := uuid.New()
	now := time.Now()
	m.refreshTokens[tokenHash] = &auth.RefreshToken{
		ID:        id,
		UserID:    userID,
		TokenHash: tokenHash,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}
	return id, nil
}

func (m *mockAuthRepo) GetRefreshToken(_ context.Context, tokenHash string) (*auth.RefreshToken, error) {
	rt, ok := m.refreshTokens[tokenHash]
	if !ok {
		return nil, errs.NewNotFound("token not found")
	}
	cp := *rt
	return &cp, nil
}

func (m *mockAuthRepo) RevokeRefreshToken(_ context.Context, tokenHash string) error {
	if rt, ok := m.refreshTokens[tokenHash]; ok {
		now := time.Now()
		rt.RevokedAt = &now
	}
	return nil
}

func (m *mockAuthRepo) RevokeAllUserTokens(_ context.Context, userID uuid.UUID) error {
	return nil
}

// --- test helpers ---

func setupAuthRouter(repo *mockAuthRepo) (*auth.Service, http.Handler) {
	authSvc := auth.NewService(repo, "test-secret-key-32-bytes-long!!!", 15*time.Minute, 7*24*time.Hour)
	r := chi.NewRouter()
	h := NewAuthHandler(authSvc)
	r.Post("/auth/register", h.Register)
	r.Post("/auth/login", h.Login)
	r.Post("/auth/refresh", h.Refresh)
	return authSvc, r
}

func doPost(handler http.Handler, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

// --- tests ---

func TestRegisterSuccess(t *testing.T) {
	repo := newMockAuthRepo()
	_, router := setupAuthRouter(repo)

	w := doPost(router, "/auth/register", map[string]string{
		"email":     "test@example.com",
		"password":  "password123",
		"full_name": "Test User",
	})

	assert.Equal(t, http.StatusCreated, w.Code)
	var result auth.AuthResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, "test@example.com", result.User.Email)
	assert.NotEmpty(t, result.AccessToken)
	assert.NotEmpty(t, result.RefreshToken)
}

func TestRegisterDuplicateEmail(t *testing.T) {
	repo := newMockAuthRepo()
	_, router := setupAuthRouter(repo)

	doPost(router, "/auth/register", map[string]string{
		"email": "dup@example.com", "password": "password123", "full_name": "User",
	})

	w := doPost(router, "/auth/register", map[string]string{
		"email": "dup@example.com", "password": "password456", "full_name": "User2",
	})
	assert.Equal(t, http.StatusConflict, w.Code)
}

func TestRegisterMissingFields(t *testing.T) {
	repo := newMockAuthRepo()
	_, router := setupAuthRouter(repo)

	w := doPost(router, "/auth/register", map[string]string{
		"email": "", "password": "",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLoginSuccess(t *testing.T) {
	repo := newMockAuthRepo()
	_, router := setupAuthRouter(repo)

	doPost(router, "/auth/register", map[string]string{
		"email": "login@example.com", "password": "password123", "full_name": "User",
	})

	w := doPost(router, "/auth/login", map[string]string{
		"email": "login@example.com", "password": "password123",
	})
	assert.Equal(t, http.StatusOK, w.Code)
	var result auth.AuthResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.NotEmpty(t, result.AccessToken)
}

func TestLoginWrongPassword(t *testing.T) {
	repo := newMockAuthRepo()
	_, router := setupAuthRouter(repo)

	doPost(router, "/auth/register", map[string]string{
		"email": "wp@example.com", "password": "password123", "full_name": "User",
	})

	w := doPost(router, "/auth/login", map[string]string{
		"email": "wp@example.com", "password": "wrongpassword",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestLoginUnknownEmail(t *testing.T) {
	repo := newMockAuthRepo()
	_, router := setupAuthRouter(repo)

	w := doPost(router, "/auth/login", map[string]string{
		"email": "unknown@example.com", "password": "password123",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestRefreshSuccess(t *testing.T) {
	repo := newMockAuthRepo()
	_, router := setupAuthRouter(repo)

	w := doPost(router, "/auth/register", map[string]string{
		"email": "refresh@example.com", "password": "password123", "full_name": "User",
	})
	var regResult auth.AuthResult
	json.Unmarshal(w.Body.Bytes(), &regResult)

	w = doPost(router, "/auth/refresh", map[string]string{
		"refresh_token": regResult.RefreshToken,
	})
	assert.Equal(t, http.StatusOK, w.Code)

	var refreshResult auth.AuthResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &refreshResult))
	assert.NotEmpty(t, refreshResult.AccessToken)
}

func TestRefreshInvalidToken(t *testing.T) {
	repo := newMockAuthRepo()
	_, router := setupAuthRouter(repo)

	w := doPost(router, "/auth/refresh", map[string]string{
		"refresh_token": "invalid-token",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
