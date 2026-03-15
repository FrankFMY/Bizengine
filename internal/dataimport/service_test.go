package dataimport

import (
	"context"
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
	m.components[c.EntityID] = append(m.components[c.EntityID], *c)
	return nil
}
func (m *mockEntityRepo) GetComponent(context.Context, uuid.UUID, uuid.UUID, string) (*types.Component, error) {
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

func setupImportService() (*Service, *mockEntityRepo) {
	entityRepo := newMockEntityRepo()
	bus := &mockBus{}
	eventStore := &mockEventStore{}
	entitySvc := entity.NewService(entityRepo, eventStore, bus)
	svc := NewService(entitySvc, bus)
	return svc, entityRepo
}

// --- tests ---

func TestImportProductsCSV(t *testing.T) {
	svc, entityRepo := setupImportService()
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	csv := "name,sku,category,selling_price,purchase_price,unit,barcode\n" +
		"Кофе Латте,LATTE-01,Напитки,250.00,100.00,шт,4600000000001\n" +
		"Круассан,CROIS-01,Выпечка,150,60,шт,4600000000002\n" +
		",EMPTY,,100,50,шт,\n"

	result, err := svc.ImportProducts(ctx, orgID, []byte(csv), "products.csv", &actorID)
	require.NoError(t, err)
	assert.Equal(t, 3, result.Total)
	assert.Equal(t, 2, result.Imported)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, 4, result.Errors[0].Row)
	assert.Equal(t, "REQUIRED", result.Errors[0].Error)

	assert.Len(t, entityRepo.entities, 2)
}

func TestImportProductsInvalidPrice(t *testing.T) {
	svc, _ := setupImportService()
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	csv := "name,sku,selling_price\nGood Item,SKU1,100\nBad Item,SKU2,abc\n"

	result, err := svc.ImportProducts(ctx, orgID, []byte(csv), "test.csv", &actorID)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Imported)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "INVALID", result.Errors[0].Error)
}

func TestImportCustomersCSV(t *testing.T) {
	svc, _ := setupImportService()
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	csv := "name,phone,email,tags\nИван Иванов,+79001234567,ivan@test.ru,vip;regular\nМария Петрова,+79007654321,,\n"

	result, err := svc.ImportCustomers(ctx, orgID, []byte(csv), "customers.csv", &actorID)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Total)
	assert.Equal(t, 2, result.Imported)
	assert.Len(t, result.Errors, 0)
}

func TestImportSuppliersCSV(t *testing.T) {
	svc, _ := setupImportService()
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	csv := "name,inn,phone,email,contact_person\nООО Поставщик,7700000000,+79001111111,sup@test.ru,Иван\n"

	result, err := svc.ImportSuppliers(ctx, orgID, []byte(csv), "suppliers.csv", &actorID)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, 1, result.Imported)
}

func TestImportEmptyFile(t *testing.T) {
	svc, _ := setupImportService()
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	csv := "name,sku\n"

	_, err := svc.ImportProducts(ctx, orgID, []byte(csv), "empty.csv", &actorID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no data rows")
}

func TestParsePriceFormats(t *testing.T) {
	assert.Equal(t, int64(25000), parsePrice("250.00"))
	assert.Equal(t, int64(25000), parsePrice("250,00"))
	assert.Equal(t, int64(100), parsePrice("100"))
	assert.Equal(t, int64(0), parsePrice(""))
	assert.Equal(t, int64(-1), parsePrice("abc"))
	assert.Equal(t, int64(15050), parsePrice("150.50"))
	assert.Equal(t, int64(999), parsePrice("9.99"))
}
