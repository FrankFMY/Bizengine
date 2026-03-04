package catalog

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

func (m *mockEntityRepo) GetByID(_ context.Context, wsID, id uuid.UUID) (*types.Entity, error) {
	e, ok := m.entities[id]
	if !ok || e.WorkspaceID != wsID {
		return nil, pgx.ErrNoRows
	}
	return e, nil
}

func (m *mockEntityRepo) List(_ context.Context, wsID uuid.UUID, filter entity.ListFilter) ([]types.Entity, int, error) {
	var result []types.Entity
	for _, e := range m.entities {
		if e.WorkspaceID != wsID || e.DeletedAt != nil {
			continue
		}
		if filter.Kind != nil && e.Kind != *filter.Kind {
			continue
		}
		if filter.ParentID != nil && (e.ParentID == nil || *e.ParentID != *filter.ParentID) {
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

func (m *mockEntityRepo) SoftDelete(_ context.Context, _, id uuid.UUID) error {
	delete(m.entities, id)
	return nil
}

func (m *mockEntityRepo) SetComponent(_ context.Context, c *types.Component) error {
	comps := m.components[c.EntityID]
	for i, existing := range comps {
		if existing.Type == c.Type {
			comps[i] = *c
			m.components[c.EntityID] = comps
			return nil
		}
	}
	m.components[c.EntityID] = append(comps, *c)
	return nil
}

func (m *mockEntityRepo) GetComponent(_ context.Context, _, entityID uuid.UUID, compType string) (*types.Component, error) {
	for _, c := range m.components[entityID] {
		if c.Type == compType {
			return &c, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (m *mockEntityRepo) ListComponents(_ context.Context, _, entityID uuid.UUID) ([]types.Component, error) {
	return m.components[entityID], nil
}

func (m *mockEntityRepo) DeleteComponent(_ context.Context, _, entityID uuid.UUID, compType string) error {
	comps := m.components[entityID]
	for i, c := range comps {
		if c.Type == compType {
			m.components[entityID] = append(comps[:i], comps[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockEntityRepo) WithTx(_ context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
}

func (m *mockEntityRepo) CreateTx(ctx context.Context, _ pgx.Tx, e *types.Entity) error {
	return m.Create(ctx, e)
}

func (m *mockEntityRepo) SetComponentTx(ctx context.Context, _ pgx.Tx, c *types.Component) error {
	return m.SetComponent(ctx, c)
}

type mockEventStore struct{}

func (m *mockEventStore) Append(context.Context, types.Event) error              { return nil }
func (m *mockEventStore) AppendTx(context.Context, pgx.Tx, types.Event) error   { return nil }
func (m *mockEventStore) GetByEntity(_ context.Context, _, _ uuid.UUID, _ *time.Time, _ int) ([]types.Event, error) {
	return nil, nil
}
func (m *mockEventStore) GetByWorkspace(_ context.Context, _ uuid.UUID, _, _ int) ([]types.Event, int, error) {
	return nil, 0, nil
}
func (m *mockEventStore) GetByType(_ context.Context, _ uuid.UUID, _ string, _ *time.Time, _ int) ([]types.Event, error) {
	return nil, nil
}

type mockEventBus struct {
	published []types.Event
}

func (m *mockEventBus) Publish(_ context.Context, ev types.Event) error {
	m.published = append(m.published, ev)
	return nil
}
func (m *mockEventBus) Subscribe(string, event.Subscriber)        {}
func (m *mockEventBus) SubscribePattern(string, event.Subscriber) {}
func (m *mockEventBus) SubscribeAll(event.Subscriber)             {}

type mockCatalogRepo struct {
	findBySKUResult *types.Entity
	findBySKUErr    error
	listResult      []Product
	listTotal       int
	listErr         error
}

func (m *mockCatalogRepo) FindBySKU(context.Context, uuid.UUID, string) (*types.Entity, error) {
	return m.findBySKUResult, m.findBySKUErr
}
func (m *mockCatalogRepo) ListProductsWithComponents(_ context.Context, _ uuid.UUID, _ ProductFilter) ([]Product, int, error) {
	return m.listResult, m.listTotal, m.listErr
}

// --- helpers ---

func setupService(catalogRepo *mockCatalogRepo) (*Service, *mockEntityRepo, *mockEventBus) {
	entityRepo := newMockEntityRepo()
	eventBus := &mockEventBus{}
	eventStore := &mockEventStore{}
	entitySvc := entity.NewService(entityRepo, eventStore, eventBus)
	svc := NewService(entitySvc, catalogRepo, eventBus)
	return svc, entityRepo, eventBus
}

// --- tests ---

func TestCreateProduct(t *testing.T) {
	wsID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		repo := &mockCatalogRepo{findBySKUErr: pgx.ErrNoRows}
		svc, _, eventBus := setupService(repo)

		p, err := svc.CreateProduct(ctx, wsID, CreateProductInput{
			Name:  "Test Product",
			SKU:   "SKU-001",
			Price: json.RawMessage(`{"amount":10000,"currency":"RUB"}`),
		}, &actorID)

		require.NoError(t, err)
		assert.Equal(t, "Test Product", p.Name)
		assert.Equal(t, "product", p.Kind)
		assert.NotNil(t, p.Price)
		assert.NotNil(t, p.Barcode)
		assert.True(t, len(eventBus.published) > 0)
	})

	t.Run("missing name", func(t *testing.T) {
		repo := &mockCatalogRepo{}
		svc, _, _ := setupService(repo)

		_, err := svc.CreateProduct(ctx, wsID, CreateProductInput{
			Price: json.RawMessage(`{"amount":10000}`),
		}, &actorID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("missing price", func(t *testing.T) {
		repo := &mockCatalogRepo{}
		svc, _, _ := setupService(repo)

		_, err := svc.CreateProduct(ctx, wsID, CreateProductInput{
			Name: "No Price",
		}, &actorID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "price is required")
	})

	t.Run("duplicate SKU", func(t *testing.T) {
		existing := &types.Entity{ID: uuid.New()}
		repo := &mockCatalogRepo{findBySKUResult: existing}
		svc, _, _ := setupService(repo)

		_, err := svc.CreateProduct(ctx, wsID, CreateProductInput{
			Name:  "Dup",
			SKU:   "SKU-001",
			Price: json.RawMessage(`{"amount":5000}`),
		}, &actorID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "SKU already exists")
	})
}

func TestGetProduct(t *testing.T) {
	wsID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	repo := &mockCatalogRepo{findBySKUErr: pgx.ErrNoRows}
	svc, _, _ := setupService(repo)

	p, err := svc.CreateProduct(ctx, wsID, CreateProductInput{
		Name:  "Get Me",
		SKU:   "SKU-GET",
		Price: json.RawMessage(`{"amount":1000}`),
	}, &actorID)
	require.NoError(t, err)

	got, err := svc.GetProduct(ctx, wsID, p.ID)
	require.NoError(t, err)
	assert.Equal(t, "Get Me", got.Name)
	assert.NotNil(t, got.Price)
}

func TestListProducts(t *testing.T) {
	wsID := uuid.New()
	ctx := context.Background()

	products := []Product{
		{Entity: types.Entity{ID: uuid.New(), Name: "P1"}},
		{Entity: types.Entity{ID: uuid.New(), Name: "P2"}},
	}
	repo := &mockCatalogRepo{listResult: products, listTotal: 2}
	svc, _, _ := setupService(repo)

	result, err := svc.ListProducts(ctx, wsID, ProductFilter{})
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)
	assert.Len(t, result.Items, 2)
}

func TestUpdateProduct(t *testing.T) {
	wsID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	repo := &mockCatalogRepo{findBySKUErr: pgx.ErrNoRows}
	svc, _, _ := setupService(repo)

	p, err := svc.CreateProduct(ctx, wsID, CreateProductInput{
		Name:  "Before",
		SKU:   "SKU-UPD",
		Price: json.RawMessage(`{"amount":5000}`),
	}, &actorID)
	require.NoError(t, err)

	newName := "After"
	updated, err := svc.UpdateProduct(ctx, wsID, p.ID, UpdateProductInput{
		Name:  &newName,
		Price: json.RawMessage(`{"amount":7000}`),
	}, &actorID)

	require.NoError(t, err)
	assert.Equal(t, "After", updated.Name)
}

func TestArchiveProduct(t *testing.T) {
	wsID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	repo := &mockCatalogRepo{findBySKUErr: pgx.ErrNoRows}
	svc, entityRepo, _ := setupService(repo)

	p, err := svc.CreateProduct(ctx, wsID, CreateProductInput{
		Name:  "To Archive",
		SKU:   "SKU-ARC",
		Price: json.RawMessage(`{"amount":1000}`),
	}, &actorID)
	require.NoError(t, err)

	err = svc.ArchiveProduct(ctx, wsID, p.ID, &actorID)
	require.NoError(t, err)

	archived := entityRepo.entities[p.ID]
	assert.Equal(t, "archived", archived.Status)
}

func TestCreateCategory(t *testing.T) {
	wsID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	repo := &mockCatalogRepo{}
	svc, _, _ := setupService(repo)

	t.Run("success", func(t *testing.T) {
		c, err := svc.CreateCategory(ctx, wsID, CreateCategoryInput{Name: "Electronics"}, &actorID)
		require.NoError(t, err)
		assert.Equal(t, "Electronics", c.Name)
		assert.Equal(t, "category", c.Kind)
	})

	t.Run("empty name", func(t *testing.T) {
		_, err := svc.CreateCategory(ctx, wsID, CreateCategoryInput{}, &actorID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})
}

func TestDeleteCategory(t *testing.T) {
	wsID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	t.Run("empty category", func(t *testing.T) {
		repo := &mockCatalogRepo{}
		svc, _, _ := setupService(repo)

		c, err := svc.CreateCategory(ctx, wsID, CreateCategoryInput{Name: "Empty"}, &actorID)
		require.NoError(t, err)

		err = svc.DeleteCategory(ctx, wsID, c.ID, &actorID)
		require.NoError(t, err)
	})

	t.Run("category with products", func(t *testing.T) {
		repo := &mockCatalogRepo{findBySKUErr: pgx.ErrNoRows}
		svc, _, _ := setupService(repo)

		cat, err := svc.CreateCategory(ctx, wsID, CreateCategoryInput{Name: "Has Products"}, &actorID)
		require.NoError(t, err)

		_, err = svc.CreateProduct(ctx, wsID, CreateProductInput{
			Name:       "In Category",
			CategoryID: &cat.ID,
			Price:      json.RawMessage(`{"amount":1000}`),
		}, &actorID)
		require.NoError(t, err)

		err = svc.DeleteCategory(ctx, wsID, cat.ID, &actorID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "category has products")
	})
}
