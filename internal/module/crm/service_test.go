package crm

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/entity"
	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/types"
)

// --- mocks ---

type mockCRMRepo struct{}

func (m *mockCRMRepo) GetOrdersByCustomer(_ context.Context, _, _ uuid.UUID, _ types.PageRequest) ([]json.RawMessage, int, error) {
	return []json.RawMessage{}, 0, nil
}

func (m *mockCRMRepo) GetTransactionsByCounterparty(_ context.Context, _, _ uuid.UUID, _ types.PageRequest) ([]json.RawMessage, int, error) {
	return []json.RawMessage{}, 0, nil
}

func (m *mockCRMRepo) GetDeliveriesBySupplier(_ context.Context, _, _ uuid.UUID, _ types.PageRequest) ([]json.RawMessage, int, error) {
	return []json.RawMessage{}, 0, nil
}

type mockEntityRepo struct {
	entities   map[uuid.UUID]*types.Entity
	components map[uuid.UUID][]types.Component
}

func newMockEntityRepo() *mockEntityRepo {
	return &mockEntityRepo{
		entities:   make(map[uuid.UUID]*types.Entity),
		components: make(map[uuid.UUID][]types.Component),
	}
}

func (m *mockEntityRepo) Create(_ context.Context, e *types.Entity) error {
	m.entities[e.ID] = e
	return nil
}
func (m *mockEntityRepo) GetByID(_ context.Context, _, id uuid.UUID) (*types.Entity, error) {
	if e, ok := m.entities[id]; ok {
		cp := *e
		return &cp, nil
	}
	return nil, pgx.ErrNoRows
}
func (m *mockEntityRepo) List(_ context.Context, orgID uuid.UUID, filter entity.ListFilter) ([]types.Entity, int, error) {
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
func (m *mockEntityRepo) Update(_ context.Context, e *types.Entity) error {
	m.entities[e.ID] = e
	return nil
}
func (m *mockEntityRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (m *mockEntityRepo) SetComponent(_ context.Context, c *types.Component) error {
	comps := m.components[c.EntityID]
	for i, existing := range comps {
		if existing.Type == c.Type {
			comps[i] = *c
			m.components[c.EntityID] = comps
			return nil
		}
	}
	m.components[c.EntityID] = append(m.components[c.EntityID], *c)
	return nil
}
func (m *mockEntityRepo) GetComponent(_ context.Context, _, entityID uuid.UUID, compType string) (*types.Component, error) {
	for _, c := range m.components[entityID] {
		if c.Type == compType {
			return &c, nil
		}
	}
	return nil, nil
}
func (m *mockEntityRepo) ListComponents(_ context.Context, _, entityID uuid.UUID) ([]types.Component, error) {
	return m.components[entityID], nil
}
func (m *mockEntityRepo) DeleteComponent(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (m *mockEntityRepo) WithTx(_ context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
}
func (m *mockEntityRepo) CreateTx(_ context.Context, _ pgx.Tx, e *types.Entity) error {
	return m.Create(context.Background(), e)
}
func (m *mockEntityRepo) SetComponentTx(_ context.Context, _ pgx.Tx, c *types.Component) error {
	return m.SetComponent(context.Background(), c)
}

type mockEventStore struct{}

func (m *mockEventStore) Append(context.Context, types.Event) error           { return nil }
func (m *mockEventStore) AppendTx(context.Context, pgx.Tx, types.Event) error { return nil }
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

func (b *mockBus) Publish(_ context.Context, ev types.Event) error {
	b.published = append(b.published, ev)
	return nil
}
func (b *mockBus) Subscribe(string, event.Subscriber)        {}
func (b *mockBus) SubscribePattern(string, event.Subscriber) {}
func (b *mockBus) SubscribeAll(event.Subscriber)             {}

// --- helpers ---

func setupCRMService() (*Service, *mockBus) {
	crmRepo := &mockCRMRepo{}
	entityRepo := newMockEntityRepo()
	bus := &mockBus{}
	eventStore := &mockEventStore{}
	entitySvc := entity.NewService(entityRepo, eventStore, bus)
	svc := NewService(crmRepo, entitySvc, bus)
	return svc, bus
}

// --- tests ---

func TestCreateCustomer(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, bus := setupCRMService()

		cp, err := svc.Create(ctx, orgID, CreateInput{
			Kind: "customer",
			Name: "Test Customer",
			Contact: map[string]any{
				"phone": "+79001234567",
				"email": "test@example.com",
			},
			Profile: map[string]any{
				"tags":     []string{"vip"},
				"category": "retail",
			},
		}, &actorID)

		require.NoError(t, err)
		assert.Equal(t, "Test Customer", cp.Name)
		assert.Equal(t, "customer", cp.Kind)
		assert.NotNil(t, cp.Profile)
		assert.NotNil(t, cp.Balance)
		assert.NotNil(t, cp.Contact)

		hasEvent := false
		for _, ev := range bus.published {
			if ev.Type == "crm.customer.created" {
				hasEvent = true
			}
		}
		assert.True(t, hasEvent)
	})

	t.Run("missing name", func(t *testing.T) {
		svc, _ := setupCRMService()
		_, err := svc.Create(ctx, orgID, CreateInput{Kind: "customer"}, &actorID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("invalid kind", func(t *testing.T) {
		svc, _ := setupCRMService()
		_, err := svc.Create(ctx, orgID, CreateInput{Kind: "unknown", Name: "Test"}, &actorID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "kind must be customer or supplier")
	})
}

func TestCreateSupplier(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, bus := setupCRMService()

	cp, err := svc.Create(ctx, orgID, CreateInput{
		Kind: "supplier",
		Name: "Test Supplier",
	}, &actorID)

	require.NoError(t, err)
	assert.Equal(t, "Test Supplier", cp.Name)
	assert.Equal(t, "supplier", cp.Kind)

	hasEvent := false
	for _, ev := range bus.published {
		if ev.Type == "crm.supplier.created" {
			hasEvent = true
		}
	}
	assert.True(t, hasEvent)
}

func TestGetCounterparty(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _ := setupCRMService()
	cp, _ := svc.Create(ctx, orgID, CreateInput{
		Kind: "customer",
		Name: "Retrieve Me",
	}, &actorID)

	got, err := svc.Get(ctx, orgID, cp.ID)
	require.NoError(t, err)
	assert.Equal(t, "Retrieve Me", got.Name)
}

func TestUpdateCounterparty(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _ := setupCRMService()
	cp, _ := svc.Create(ctx, orgID, CreateInput{
		Kind: "customer",
		Name: "Old Name",
	}, &actorID)

	newName := "New Name"
	updated, err := svc.Update(ctx, orgID, cp.ID, UpdateInput{
		Name: &newName,
		Contact: map[string]any{
			"phone": "+79009876543",
		},
	}, &actorID)

	require.NoError(t, err)
	assert.Equal(t, "New Name", updated.Name)
	assert.Equal(t, "+79009876543", updated.Contact["phone"])
}

func TestUpdateTags(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _ := setupCRMService()
	cp, _ := svc.Create(ctx, orgID, CreateInput{
		Kind: "customer",
		Name: "Tagged Customer",
		Profile: map[string]any{
			"tags": []string{"regular"},
		},
	}, &actorID)

	updated, err := svc.UpdateTags(ctx, orgID, cp.ID, TagsInput{
		Add:    []string{"vip"},
		Remove: []string{"regular"},
	}, &actorID)

	require.NoError(t, err)
	tags, _ := updated.Profile["tags"].([]any)
	assert.Contains(t, tags, "vip")
	for _, t2 := range tags {
		assert.NotEqual(t, "regular", t2)
	}
}

func TestListCounterparties(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _ := setupCRMService()
	svc.Create(ctx, orgID, CreateInput{Kind: "customer", Name: "C1"}, &actorID)
	svc.Create(ctx, orgID, CreateInput{Kind: "customer", Name: "C2"}, &actorID)
	svc.Create(ctx, orgID, CreateInput{Kind: "supplier", Name: "S1"}, &actorID)

	customers, err := svc.List(ctx, orgID, "customer", CounterpartyFilter{})
	require.NoError(t, err)
	assert.Equal(t, 2, customers.Total)

	suppliers, err := svc.List(ctx, orgID, "supplier", CounterpartyFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, suppliers.Total)
}

func TestDefaultBalance(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _ := setupCRMService()
	cp, err := svc.Create(ctx, orgID, CreateInput{
		Kind: "customer",
		Name: "No Balance Input",
	}, &actorID)

	require.NoError(t, err)
	assert.NotNil(t, cp.Balance)
	assert.Equal(t, float64(0), cp.Balance["balance"])
	assert.Equal(t, float64(0), cp.Balance["credit_limit"])
}
