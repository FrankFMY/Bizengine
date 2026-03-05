package entity

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// --- mocks ---

type mockRepo struct {
	entities   map[uuid.UUID]*types.Entity
	components map[string]*types.Component // key: entityID:compType
}

func newMockRepo() *mockRepo {
	return &mockRepo{
		entities:   make(map[uuid.UUID]*types.Entity),
		components: make(map[string]*types.Component),
	}
}

func (m *mockRepo) Create(_ context.Context, e *types.Entity) error {
	cp := *e
	m.entities[e.ID] = &cp
	return nil
}

func (m *mockRepo) CreateTx(_ context.Context, _ pgx.Tx, e *types.Entity) error {
	cp := *e
	m.entities[e.ID] = &cp
	return nil
}

func (m *mockRepo) GetByID(_ context.Context, orgID, id uuid.UUID) (*types.Entity, error) {
	e, ok := m.entities[id]
	if !ok || e.OrganizationID != orgID {
		return nil, errs.NewNotFound("entity not found")
	}
	cp := *e
	return &cp, nil
}

func (m *mockRepo) List(_ context.Context, orgID uuid.UUID, filter ListFilter) ([]types.Entity, int, error) {
	var result []types.Entity
	for _, e := range m.entities {
		if e.OrganizationID != orgID {
			continue
		}
		if filter.Kind != nil && e.Kind != *filter.Kind {
			continue
		}
		result = append(result, *e)
	}
	return result, len(result), nil
}

func (m *mockRepo) Update(_ context.Context, e *types.Entity) error {
	if _, ok := m.entities[e.ID]; !ok {
		return errs.NewNotFound("entity not found")
	}
	cp := *e
	m.entities[e.ID] = &cp
	return nil
}

func (m *mockRepo) SoftDelete(_ context.Context, orgID, id uuid.UUID) error {
	e, ok := m.entities[id]
	if !ok || e.OrganizationID != orgID {
		return errs.NewNotFound("entity not found")
	}
	delete(m.entities, id)
	return nil
}

func (m *mockRepo) SetComponent(_ context.Context, c *types.Component) error {
	key := c.EntityID.String() + ":" + c.Type
	cp := *c
	m.components[key] = &cp
	return nil
}

func (m *mockRepo) SetComponentTx(_ context.Context, _ pgx.Tx, c *types.Component) error {
	key := c.EntityID.String() + ":" + c.Type
	cp := *c
	m.components[key] = &cp
	return nil
}

func (m *mockRepo) GetComponent(_ context.Context, orgID, entityID uuid.UUID, compType string) (*types.Component, error) {
	key := entityID.String() + ":" + compType
	c, ok := m.components[key]
	if !ok {
		return nil, errs.NewNotFound("component not found")
	}
	cp := *c
	return &cp, nil
}

func (m *mockRepo) ListComponents(_ context.Context, orgID, entityID uuid.UUID) ([]types.Component, error) {
	var result []types.Component
	prefix := entityID.String() + ":"
	for k, c := range m.components {
		if len(k) > len(prefix) && k[:len(prefix)] == prefix {
			result = append(result, *c)
		}
	}
	return result, nil
}

func (m *mockRepo) DeleteComponent(_ context.Context, orgID, entityID uuid.UUID, compType string) error {
	key := entityID.String() + ":" + compType
	if _, ok := m.components[key]; !ok {
		return errs.NewNotFound("component not found")
	}
	delete(m.components, key)
	return nil
}

func (m *mockRepo) WithTx(_ context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
}

type mockEventStore struct {
	events []types.Event
}

func (m *mockEventStore) Append(_ context.Context, ev types.Event) error {
	m.events = append(m.events, ev)
	return nil
}

func (m *mockEventStore) AppendTx(_ context.Context, _ pgx.Tx, ev types.Event) error {
	m.events = append(m.events, ev)
	return nil
}

func (m *mockEventStore) GetByEntity(_ context.Context, _, _ uuid.UUID, _ *time.Time, _ int) ([]types.Event, error) {
	return nil, nil
}

func (m *mockEventStore) GetByOrganization(_ context.Context, _ uuid.UUID, _, _ int) ([]types.Event, int, error) {
	return nil, 0, nil
}

func (m *mockEventStore) GetByType(_ context.Context, _ uuid.UUID, _ string, _ *time.Time, _ int) ([]types.Event, error) {
	return nil, nil
}

type mockBus struct {
	published []types.Event
}

func (m *mockBus) Publish(_ context.Context, ev types.Event) error {
	m.published = append(m.published, ev)
	return nil
}
func (m *mockBus) Subscribe(_ string, _ event.Subscriber)        {}
func (m *mockBus) SubscribePattern(_ string, _ event.Subscriber) {}
func (m *mockBus) SubscribeAll(_ event.Subscriber)               {}

// --- helpers ---

func newTestService() (*Service, *mockRepo, *mockEventStore, *mockBus) {
	repo := newMockRepo()
	store := &mockEventStore{}
	bus := &mockBus{}
	svc := NewService(repo, store, bus)
	return svc, repo, store, bus
}

func ptr[T any](v T) *T { return &v }

// --- tests ---

func TestCreateEntity(t *testing.T) {
	svc, _, store, bus := newTestService()
	orgID := uuid.New()
	actorID := uuid.New()

	e, err := svc.Create(context.Background(), orgID, CreateEntityInput{
		Kind: "product",
		Name: "Widget",
	}, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "product", e.Kind)
	assert.Equal(t, "Widget", e.Name)
	assert.Equal(t, orgID, e.OrganizationID)

	require.Len(t, store.events, 1)
	assert.Equal(t, "entity.created", store.events[0].Type)

	require.Len(t, bus.published, 1)
	assert.Equal(t, "entity.created", bus.published[0].Type)
}

func TestCreateEntityEmptyName(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.Create(context.Background(), uuid.New(), CreateEntityInput{Kind: "product"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestCreateEntityEmptyKind(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.Create(context.Background(), uuid.New(), CreateEntityInput{Name: "Widget"}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kind is required")
}

func TestGetEntity(t *testing.T) {
	svc, repo, _, _ := newTestService()
	orgID := uuid.New()
	id := uuid.New()
	repo.entities[id] = &types.Entity{ID: id, OrganizationID: orgID, Kind: "product", Name: "W"}

	e, err := svc.Get(context.Background(), orgID, id, false)
	require.NoError(t, err)
	assert.Equal(t, "product", e.Kind)
}

func TestGetEntityNotFound(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.Get(context.Background(), uuid.New(), uuid.New(), false)
	require.Error(t, err)
	assert.Equal(t, errs.CodeNotFound, errs.GetCode(err))
}

func TestUpdateEntity(t *testing.T) {
	svc, repo, store, bus := newTestService()
	orgID := uuid.New()
	id := uuid.New()
	actorID := uuid.New()
	repo.entities[id] = &types.Entity{ID: id, OrganizationID: orgID, Kind: "product", Name: "Old"}

	e, err := svc.Update(context.Background(), orgID, id, UpdateEntityInput{Name: ptr("New")}, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "New", e.Name)

	require.Len(t, store.events, 1)
	assert.Equal(t, "entity.updated", store.events[0].Type)
	require.Len(t, bus.published, 1)
}

func TestUpdateEntityNotFound(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.Update(context.Background(), uuid.New(), uuid.New(), UpdateEntityInput{Name: ptr("X")}, nil)
	require.Error(t, err)
}

func TestUpdateEntityNoChanges(t *testing.T) {
	svc, repo, store, _ := newTestService()
	orgID := uuid.New()
	id := uuid.New()
	repo.entities[id] = &types.Entity{ID: id, OrganizationID: orgID, Kind: "product", Name: "Same"}

	e, err := svc.Update(context.Background(), orgID, id, UpdateEntityInput{Name: ptr("Same")}, nil)
	require.NoError(t, err)
	assert.Equal(t, "Same", e.Name)
	assert.Len(t, store.events, 0)
}

func TestDeleteEntity(t *testing.T) {
	svc, repo, store, bus := newTestService()
	orgID := uuid.New()
	id := uuid.New()
	actorID := uuid.New()
	repo.entities[id] = &types.Entity{ID: id, OrganizationID: orgID, Kind: "product", Name: "W"}

	err := svc.Delete(context.Background(), orgID, id, &actorID)
	require.NoError(t, err)

	require.Len(t, store.events, 1)
	assert.Equal(t, "entity.deleted", store.events[0].Type)
	require.Len(t, bus.published, 1)
}

func TestDeleteEntityNotFound(t *testing.T) {
	svc, _, _, _ := newTestService()
	err := svc.Delete(context.Background(), uuid.New(), uuid.New(), nil)
	require.Error(t, err)
}

func TestSetComponent(t *testing.T) {
	svc, repo, store, bus := newTestService()
	orgID := uuid.New()
	entityID := uuid.New()
	actorID := uuid.New()
	repo.entities[entityID] = &types.Entity{ID: entityID, OrganizationID: orgID, Kind: "product", Name: "W"}

	data := json.RawMessage(`{"color":"red"}`)
	c, err := svc.SetComponent(context.Background(), orgID, entityID, "appearance", data, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "appearance", c.Type)
	assert.Equal(t, entityID, c.EntityID)

	require.Len(t, store.events, 1)
	assert.Equal(t, "component.set", store.events[0].Type)
	require.Len(t, bus.published, 1)
}

func TestSetComponentEntityNotFound(t *testing.T) {
	svc, _, _, _ := newTestService()
	_, err := svc.SetComponent(context.Background(), uuid.New(), uuid.New(), "x", json.RawMessage(`{}`), nil)
	require.Error(t, err)
}

func TestDeleteComponent(t *testing.T) {
	svc, repo, _, bus := newTestService()
	orgID := uuid.New()
	entityID := uuid.New()
	repo.entities[entityID] = &types.Entity{ID: entityID, OrganizationID: orgID, Kind: "product", Name: "W"}
	repo.components[entityID.String()+":geo"] = &types.Component{ID: uuid.New(), EntityID: entityID, OrganizationID: orgID, Type: "geo"}

	err := svc.DeleteComponent(context.Background(), orgID, entityID, "geo", nil)
	require.NoError(t, err)

	require.Len(t, bus.published, 1)
	assert.Equal(t, "component.removed", bus.published[0].Type)
}

func TestListEntities(t *testing.T) {
	svc, repo, _, _ := newTestService()
	orgID := uuid.New()
	for i := 0; i < 3; i++ {
		id := uuid.New()
		repo.entities[id] = &types.Entity{ID: id, OrganizationID: orgID, Kind: "product", Name: "P"}
	}
	otherId := uuid.New()
	repo.entities[otherId] = &types.Entity{ID: otherId, OrganizationID: uuid.New(), Kind: "product", Name: "Other WS"}

	result, err := svc.List(context.Background(), orgID, ListFilter{}, false)
	require.NoError(t, err)
	assert.Len(t, result.Items, 3)
	assert.Equal(t, 3, result.Total)
}

func TestGetWithComponents(t *testing.T) {
	svc, repo, _, _ := newTestService()
	orgID := uuid.New()
	entityID := uuid.New()
	repo.entities[entityID] = &types.Entity{ID: entityID, OrganizationID: orgID, Kind: "vehicle", Name: "V"}
	repo.components[entityID.String()+":geo"] = &types.Component{ID: uuid.New(), EntityID: entityID, OrganizationID: orgID, Type: "geo", Data: json.RawMessage(`{"lat":55.75}`)}

	e, err := svc.Get(context.Background(), orgID, entityID, true)
	require.NoError(t, err)
	assert.Len(t, e.Components, 1)
	assert.Equal(t, "geo", e.Components[0].Type)
}
