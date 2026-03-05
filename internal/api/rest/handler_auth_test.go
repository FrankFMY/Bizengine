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
	organizations map[uuid.UUID]*types.Organization
	members       map[string]*types.Labor
}

func newMockAuthRepo() *mockAuthRepo {
	return &mockAuthRepo{
		users:         make(map[string]*types.User),
		usersById:     make(map[uuid.UUID]*types.User),
		organizations: make(map[uuid.UUID]*types.Organization),
		members:       make(map[string]*types.Labor),
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

func (m *mockAuthRepo) CreateOrganization(_ context.Context, org *types.Organization) error {
	cp := *org
	m.organizations[org.ID] = &cp
	return nil
}

func (m *mockAuthRepo) GetOrganization(_ context.Context, id uuid.UUID) (*types.Organization, error) {
	org, ok := m.organizations[id]
	if !ok {
		return nil, errs.NewNotFound("organization not found")
	}
	cp := *org
	return &cp, nil
}

func (m *mockAuthRepo) GetOrganizationBySlug(_ context.Context, slug string) (*types.Organization, error) {
	for _, org := range m.organizations {
		if org.Slug == slug {
			cp := *org
			return &cp, nil
		}
	}
	return nil, errs.NewNotFound("organization not found")
}

func (m *mockAuthRepo) ListUserOrganizations(_ context.Context, userID uuid.UUID) ([]types.Organization, error) {
	var result []types.Organization
	for _, mem := range m.members {
		if mem.UserID == userID {
			if org, ok := m.organizations[mem.OrganizationID]; ok {
				result = append(result, *org)
			}
		}
	}
	return result, nil
}

func (m *mockAuthRepo) UpdateOrganization(_ context.Context, org *types.Organization) error {
	m.organizations[org.ID] = org
	return nil
}

func (m *mockAuthRepo) AddMember(_ context.Context, mem *types.Labor) error {
	key := mem.OrganizationID.String() + ":" + mem.UserID.String()
	cp := *mem
	m.members[key] = &cp
	return nil
}

func (m *mockAuthRepo) GetMember(_ context.Context, orgID, userID uuid.UUID) (*types.Labor, error) {
	key := orgID.String() + ":" + userID.String()
	mem, ok := m.members[key]
	if !ok {
		return nil, errs.NewNotFound("member not found")
	}
	cp := *mem
	return &cp, nil
}

func (m *mockAuthRepo) ListMembers(_ context.Context, _ uuid.UUID) ([]types.Labor, error) {
	return nil, nil
}

func (m *mockAuthRepo) UpdateMemberRole(_ context.Context, _, _ uuid.UUID, _ string) error {
	return nil
}

func (m *mockAuthRepo) RemoveMember(_ context.Context, _, _ uuid.UUID) error {
	return nil
}

// --- mock session store ---

type mockSessionStore struct {
	sessions map[string]*auth.Session
	seances  map[string]*auth.Seance
	sesSet   map[string][]string // sessionID -> seanceIDs
}

func newMockSessionStore() *mockSessionStore {
	return &mockSessionStore{
		sessions: make(map[string]*auth.Session),
		seances:  make(map[string]*auth.Seance),
		sesSet:   make(map[string][]string),
	}
}

func (m *mockSessionStore) CreateSession(_ context.Context, s *auth.Session, _ time.Duration) error {
	cp := *s
	m.sessions[s.ID] = &cp
	return nil
}

func (m *mockSessionStore) GetSession(_ context.Context, id string) (*auth.Session, error) {
	s, ok := m.sessions[id]
	if !ok {
		return nil, errs.NewUnauthorized("session expired")
	}
	cp := *s
	return &cp, nil
}

func (m *mockSessionStore) UpdateSession(_ context.Context, s *auth.Session, _ time.Duration) error {
	cp := *s
	m.sessions[s.ID] = &cp
	return nil
}

func (m *mockSessionStore) DeleteSession(_ context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func (m *mockSessionStore) CreateSeance(_ context.Context, s *auth.Seance, _ time.Duration) error {
	cp := *s
	m.seances[s.ID] = &cp
	m.sesSet[s.SessionID] = append(m.sesSet[s.SessionID], s.ID)
	return nil
}

func (m *mockSessionStore) GetSeance(_ context.Context, id string) (*auth.Seance, error) {
	s, ok := m.seances[id]
	if !ok {
		return nil, errs.NewUnauthorized("seance expired")
	}
	cp := *s
	return &cp, nil
}

func (m *mockSessionStore) SlideSeance(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (m *mockSessionStore) DeleteSessionSeances(_ context.Context, sessionID string) error {
	for _, id := range m.sesSet[sessionID] {
		delete(m.seances, id)
	}
	delete(m.sesSet, sessionID)
	return nil
}

// --- test helpers ---

func setupAuthRouter(repo *mockAuthRepo) (*auth.Service, http.Handler) {
	store := newMockSessionStore()
	authSvc := auth.NewService(repo, store, 72*time.Hour, 10*time.Minute)
	r := chi.NewRouter()
	h := NewAuthHandler(authSvc, false, nil, nil)
	r.Post("/auth/register", h.Register)
	r.Post("/auth/login", h.Login)

	r.Group(func(r chi.Router) {
		r.Use(auth.SessionOnlyMiddleware(authSvc))
		r.Post("/auth/logout", h.Logout)
		r.Post("/auth/unlock", h.Unlock)
	})

	r.Group(func(r chi.Router) {
		r.Use(auth.Middleware(authSvc))
		r.Get("/auth/check", h.Check)
	})

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

func doPostWithCookies(handler http.Handler, path string, body any, cookies []*http.Cookie) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func extractCookies(w *httptest.ResponseRecorder) []*http.Cookie {
	resp := http.Response{Header: w.Header()}
	return resp.Cookies()
}

func findCookie(cookies []*http.Cookie, name string) *http.Cookie {
	for _, c := range cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// unwrapOK unmarshals a types.Response, asserts ok==true, and returns .Data as map.
func unwrapOK(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var resp types.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.True(t, resp.OK, "expected ok=true, got body: %s", w.Body.String())
	if resp.Data == nil {
		return nil
	}
	data, ok := resp.Data.(map[string]any)
	require.True(t, ok, "expected data to be map, got %T", resp.Data)
	return data
}

// unwrapErr unmarshals a types.Response, asserts ok==false, and returns the error code.
func unwrapErr(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp types.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.False(t, resp.OK, "expected ok=false")
	require.NotNil(t, resp.Error)
	return resp.Error.Code
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

	cookies := extractCookies(w)
	assert.NotNil(t, findCookie(cookies, "teco_session"))
	assert.NotNil(t, findCookie(cookies, "teco_seance"))

	data := unwrapOK(t, w)
	assert.Contains(t, data, "user")
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
	assert.Equal(t, "CONFLICT", unwrapErr(t, w))
}

func TestRegisterMissingFields(t *testing.T) {
	repo := newMockAuthRepo()
	_, router := setupAuthRouter(repo)

	w := doPost(router, "/auth/register", map[string]string{
		"email": "", "password": "",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "BAD_REQUEST", unwrapErr(t, w))
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

	cookies := extractCookies(w)
	assert.NotNil(t, findCookie(cookies, "teco_session"))
	assert.NotNil(t, findCookie(cookies, "teco_seance"))

	data := unwrapOK(t, w)
	assert.Contains(t, data, "user")
	assert.Contains(t, data, "organizations")
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
	assert.Equal(t, "UNAUTHORIZED", unwrapErr(t, w))
}

func TestLoginUnknownEmail(t *testing.T) {
	repo := newMockAuthRepo()
	_, router := setupAuthRouter(repo)

	w := doPost(router, "/auth/login", map[string]string{
		"email": "unknown@example.com", "password": "password123",
	})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "UNAUTHORIZED", unwrapErr(t, w))
}

func TestLogoutSuccess(t *testing.T) {
	repo := newMockAuthRepo()
	_, router := setupAuthRouter(repo)

	w := doPost(router, "/auth/register", map[string]string{
		"email": "logout@example.com", "password": "password123", "full_name": "User",
	})
	cookies := extractCookies(w)

	w = doPostWithCookies(router, "/auth/logout", nil, cookies)
	assert.Equal(t, http.StatusOK, w.Code)
	unwrapOK(t, w)

	clearedCookies := extractCookies(w)
	sc := findCookie(clearedCookies, "teco_session")
	assert.NotNil(t, sc)
	assert.Equal(t, -1, sc.MaxAge)
}

func TestCheckWithAuth(t *testing.T) {
	repo := newMockAuthRepo()
	_, router := setupAuthRouter(repo)

	w := doPost(router, "/auth/register", map[string]string{
		"email": "check@example.com", "password": "password123", "full_name": "Check User",
	})
	cookies := extractCookies(w)

	req := httptest.NewRequest("GET", "/auth/check", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	data := unwrapOK(t, w)
	assert.Contains(t, data, "user_id")
	assert.Contains(t, data, "session_id")
	assert.Contains(t, data, "seance_id")
	assert.Contains(t, data, "organization_id")
	assert.Contains(t, data, "admin")
}

func TestCheckWithoutAuth(t *testing.T) {
	repo := newMockAuthRepo()
	_, router := setupAuthRouter(repo)

	req := httptest.NewRequest("GET", "/auth/check", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Auth middleware returns plain text "SESSION" — stays unchanged
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, `"SESSION"`, w.Body.String())
}

func TestUnlockSuccess(t *testing.T) {
	repo := newMockAuthRepo()
	_, router := setupAuthRouter(repo)

	w := doPost(router, "/auth/register", map[string]string{
		"email": "unlock@example.com", "password": "password123", "full_name": "User",
	})
	cookies := extractCookies(w)

	// Only session cookie (simulating expired seance)
	sessCookie := findCookie(cookies, "teco_session")

	w = doPostWithCookies(router, "/auth/unlock", map[string]string{
		"password": "password123",
	}, []*http.Cookie{sessCookie})

	assert.Equal(t, http.StatusOK, w.Code)
	unwrapOK(t, w)

	newCookies := extractCookies(w)
	newSeance := findCookie(newCookies, "teco_seance")
	assert.NotNil(t, newSeance)
	assert.NotEmpty(t, newSeance.Value)
}
