package rest

import (
	"context"
	"encoding/json"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/core/entity"
	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// --- entity repo mock ---

type mockEntityRepo struct {
	entities   map[uuid.UUID]*types.Entity
	components map[string]*types.Component
}

func newMockEntityRepo() *mockEntityRepo {
	return &mockEntityRepo{
		entities:   make(map[uuid.UUID]*types.Entity),
		components: make(map[string]*types.Component),
	}
}

func (m *mockEntityRepo) Create(_ context.Context, e *types.Entity) error {
	cp := *e
	m.entities[e.ID] = &cp
	return nil
}

func (m *mockEntityRepo) CreateTx(_ context.Context, _ pgx.Tx, e *types.Entity) error {
	cp := *e
	m.entities[e.ID] = &cp
	return nil
}

func (m *mockEntityRepo) GetByID(_ context.Context, wsID, id uuid.UUID) (*types.Entity, error) {
	e, ok := m.entities[id]
	if !ok || e.WorkspaceID != wsID {
		return nil, errs.NewNotFound("entity not found")
	}
	cp := *e
	return &cp, nil
}

func (m *mockEntityRepo) List(_ context.Context, wsID uuid.UUID, filter entity.ListFilter) ([]types.Entity, int, error) {
	var result []types.Entity
	for _, e := range m.entities {
		if e.WorkspaceID != wsID {
			continue
		}
		if filter.Kind != nil && e.Kind != *filter.Kind {
			continue
		}
		result = append(result, *e)
	}
	return result, len(result), nil
}

func (m *mockEntityRepo) Update(_ context.Context, e *types.Entity) error {
	if _, ok := m.entities[e.ID]; !ok {
		return errs.NewNotFound("entity not found")
	}
	cp := *e
	m.entities[e.ID] = &cp
	return nil
}

func (m *mockEntityRepo) SoftDelete(_ context.Context, wsID, id uuid.UUID) error {
	e, ok := m.entities[id]
	if !ok || e.WorkspaceID != wsID {
		return errs.NewNotFound("entity not found")
	}
	delete(m.entities, id)
	return nil
}

func (m *mockEntityRepo) SetComponent(_ context.Context, c *types.Component) error {
	key := c.EntityID.String() + ":" + c.Type
	cp := *c
	m.components[key] = &cp
	return nil
}

func (m *mockEntityRepo) SetComponentTx(_ context.Context, _ pgx.Tx, c *types.Component) error {
	key := c.EntityID.String() + ":" + c.Type
	cp := *c
	m.components[key] = &cp
	return nil
}

func (m *mockEntityRepo) GetComponent(_ context.Context, _, entityID uuid.UUID, compType string) (*types.Component, error) {
	key := entityID.String() + ":" + compType
	c, ok := m.components[key]
	if !ok {
		return nil, errs.NewNotFound("component not found")
	}
	cp := *c
	return &cp, nil
}

func (m *mockEntityRepo) ListComponents(_ context.Context, _, entityID uuid.UUID) ([]types.Component, error) {
	var result []types.Component
	prefix := entityID.String() + ":"
	for k, c := range m.components {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			result = append(result, *c)
		}
	}
	return result, nil
}

func (m *mockEntityRepo) DeleteComponent(_ context.Context, _, entityID uuid.UUID, compType string) error {
	key := entityID.String() + ":" + compType
	if _, ok := m.components[key]; !ok {
		return errs.NewNotFound("component not found")
	}
	delete(m.components, key)
	return nil
}

func (m *mockEntityRepo) WithTx(_ context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
}

// --- event store mock ---

type mockEntityEventStore struct {
	events []types.Event
}

func (m *mockEntityEventStore) Append(_ context.Context, ev types.Event) error {
	m.events = append(m.events, ev)
	return nil
}

func (m *mockEntityEventStore) AppendTx(_ context.Context, _ pgx.Tx, ev types.Event) error {
	m.events = append(m.events, ev)
	return nil
}

func (m *mockEntityEventStore) GetByEntity(_ context.Context, _, _ uuid.UUID, _ *time.Time, _ int) ([]types.Event, error) {
	return nil, nil
}

func (m *mockEntityEventStore) GetByWorkspace(_ context.Context, _ uuid.UUID, _, _ int) ([]types.Event, int, error) {
	return nil, 0, nil
}

func (m *mockEntityEventStore) GetByType(_ context.Context, _ uuid.UUID, _ string, _ *time.Time, _ int) ([]types.Event, error) {
	return nil, nil
}

// --- event bus mock ---

type mockEntityBus struct{}

func (m *mockEntityBus) Publish(_ context.Context, _ types.Event) error          { return nil }
func (m *mockEntityBus) Subscribe(_ string, _ event.Subscriber)                  {}
func (m *mockEntityBus) SubscribePattern(_ string, _ event.Subscriber)           {}
func (m *mockEntityBus) SubscribeAll(_ event.Subscriber)                         {}

// --- helpers ---

func setupEntityRouter(repo *mockEntityRepo) (http.Handler, uuid.UUID) {
	store := &mockEntityEventStore{}
	bus := &mockEntityBus{}
	svc := entity.NewService(repo, store, bus)
	h := NewEntityHandler(svc)
	wsID := uuid.New()

	// Create a chi router with entity routes, injecting auth context via middleware
	authRepo := newMockAuthRepo()
	authSvc := auth.NewService(authRepo, "test-secret-key-32-bytes-long!!!", 15*time.Minute, 7*24*time.Hour)

	// Register a test user and get a token
	result, _ := authSvc.Register(context.Background(), auth.RegisterInput{
		Email:    "entity-test@example.com",
		Password: "password123",
		FullName: "Test User",
	})

	r := chi.NewRouter()
	r.Use(auth.Middleware(authSvc))
	r.Route("/workspaces/{wsID}", func(r chi.Router) {
		r.Post("/entities", h.Create)
		r.Get("/entities", h.List)
		r.Get("/entities/{id}", h.Get)
		r.Put("/entities/{id}", h.Update)
		r.Delete("/entities/{id}", h.Delete)
	})

	// Store the access token in the router context via a wrapper
	return &authedHandler{handler: r, token: result.AccessToken}, wsID
}

type authedHandler struct {
	handler http.Handler
	token   string
}

func (h *authedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Authorization") == "" {
		r.Header.Set("Authorization", "Bearer "+h.token)
	}
	h.handler.ServeHTTP(w, r)
}

func doGet(handler http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func doPut(handler http.Handler, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("PUT", path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

func doDelete(handler http.Handler, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("DELETE", path, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

// --- tests ---

func TestEntityCreateSuccess(t *testing.T) {
	repo := newMockEntityRepo()
	router, wsID := setupEntityRouter(repo)

	w := doPost(router, fmt.Sprintf("/workspaces/%s/entities", wsID), map[string]string{
		"kind": "product",
		"name": "Widget",
	})

	assert.Equal(t, http.StatusCreated, w.Code)
	var e types.Entity
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &e))
	assert.Equal(t, "product", e.Kind)
	assert.Equal(t, "Widget", e.Name)
	assert.Equal(t, wsID, e.WorkspaceID)
}

func TestEntityCreateMissingKind(t *testing.T) {
	repo := newMockEntityRepo()
	router, wsID := setupEntityRouter(repo)

	w := doPost(router, fmt.Sprintf("/workspaces/%s/entities", wsID), map[string]string{
		"name": "Widget",
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestEntityCreateUnauthorized(t *testing.T) {
	repo := newMockEntityRepo()
	store := &mockEntityEventStore{}
	bus := &mockEntityBus{}
	svc := entity.NewService(repo, store, bus)
	h := NewEntityHandler(svc)

	authRepo := newMockAuthRepo()
	authSvc := auth.NewService(authRepo, "test-secret-key-32-bytes-long!!!", 15*time.Minute, 7*24*time.Hour)

	r := chi.NewRouter()
	r.Use(auth.Middleware(authSvc))
	r.Post("/workspaces/{wsID}/entities", h.Create)

	// Send request without Authorization header
	wsID := uuid.New()
	b, _ := json.Marshal(map[string]string{"kind": "product", "name": "W"})
	req := httptest.NewRequest("POST", fmt.Sprintf("/workspaces/%s/entities", wsID), bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestEntityListSuccess(t *testing.T) {
	repo := newMockEntityRepo()
	router, wsID := setupEntityRouter(repo)

	// Seed entities
	for i := 0; i < 3; i++ {
		id := uuid.New()
		repo.entities[id] = &types.Entity{ID: id, WorkspaceID: wsID, Kind: "product", Name: fmt.Sprintf("P%d", i)}
	}

	w := doGet(router, fmt.Sprintf("/workspaces/%s/entities", wsID))
	assert.Equal(t, http.StatusOK, w.Code)

	var result types.PageResponse[types.Entity]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, 3, result.Total)
	assert.Len(t, result.Items, 3)
}

func TestEntityListFilterByKind(t *testing.T) {
	repo := newMockEntityRepo()
	router, wsID := setupEntityRouter(repo)

	// Seed mixed entities
	for i := 0; i < 2; i++ {
		id := uuid.New()
		repo.entities[id] = &types.Entity{ID: id, WorkspaceID: wsID, Kind: "product", Name: fmt.Sprintf("P%d", i)}
	}
	otherId := uuid.New()
	repo.entities[otherId] = &types.Entity{ID: otherId, WorkspaceID: wsID, Kind: "vehicle", Name: "V"}

	w := doGet(router, fmt.Sprintf("/workspaces/%s/entities?kind=product", wsID))
	assert.Equal(t, http.StatusOK, w.Code)

	var result types.PageResponse[types.Entity]
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &result))
	assert.Equal(t, 2, result.Total)
}

func TestEntityGetSuccess(t *testing.T) {
	repo := newMockEntityRepo()
	router, wsID := setupEntityRouter(repo)

	id := uuid.New()
	repo.entities[id] = &types.Entity{ID: id, WorkspaceID: wsID, Kind: "product", Name: "Widget"}

	w := doGet(router, fmt.Sprintf("/workspaces/%s/entities/%s", wsID, id))
	assert.Equal(t, http.StatusOK, w.Code)

	var e types.Entity
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &e))
	assert.Equal(t, "Widget", e.Name)
}

func TestEntityGetNotFound(t *testing.T) {
	repo := newMockEntityRepo()
	router, wsID := setupEntityRouter(repo)

	w := doGet(router, fmt.Sprintf("/workspaces/%s/entities/%s", wsID, uuid.New()))
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestEntityUpdateSuccess(t *testing.T) {
	repo := newMockEntityRepo()
	router, wsID := setupEntityRouter(repo)

	id := uuid.New()
	repo.entities[id] = &types.Entity{ID: id, WorkspaceID: wsID, Kind: "product", Name: "Old"}

	newName := "New"
	w := doPut(router, fmt.Sprintf("/workspaces/%s/entities/%s", wsID, id), entity.UpdateEntityInput{
		Name: &newName,
	})

	assert.Equal(t, http.StatusOK, w.Code)
	var e types.Entity
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &e))
	assert.Equal(t, "New", e.Name)
}

func TestEntityDeleteSuccess(t *testing.T) {
	repo := newMockEntityRepo()
	router, wsID := setupEntityRouter(repo)

	id := uuid.New()
	repo.entities[id] = &types.Entity{ID: id, WorkspaceID: wsID, Kind: "product", Name: "W"}

	w := doDelete(router, fmt.Sprintf("/workspaces/%s/entities/%s", wsID, id))
	assert.Equal(t, http.StatusNoContent, w.Code)
}
