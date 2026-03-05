package order

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

type mockOrderRepo struct {
	orders map[uuid.UUID]*Order
	items  map[uuid.UUID][]OrderItem
	seq    int64
}

func newMockOrderRepo() *mockOrderRepo {
	return &mockOrderRepo{
		orders: make(map[uuid.UUID]*Order),
		items:  make(map[uuid.UUID][]OrderItem),
	}
}

func (m *mockOrderRepo) CreateOrder(_ context.Context, _ pgx.Tx, o *Order) error {
	cp := *o
	m.orders[o.ID] = &cp
	return nil
}

func (m *mockOrderRepo) CreateOrderItems(_ context.Context, _ pgx.Tx, items []OrderItem) error {
	if len(items) == 0 {
		return nil
	}
	orderID := items[0].OrderID
	m.items[orderID] = append(m.items[orderID], items...)
	return nil
}

func (m *mockOrderRepo) GetOrder(_ context.Context, orgID, orderID uuid.UUID) (*Order, error) {
	o, ok := m.orders[orderID]
	if !ok || o.OrganizationID != orgID {
		return nil, pgx.ErrNoRows
	}
	cp := *o
	return &cp, nil
}

func (m *mockOrderRepo) GetOrderItems(_ context.Context, orderID uuid.UUID) ([]OrderItem, error) {
	return m.items[orderID], nil
}

func (m *mockOrderRepo) ListOrders(_ context.Context, orgID uuid.UUID, _ OrderFilter) ([]Order, int, error) {
	var result []Order
	for _, o := range m.orders {
		if o.OrganizationID == orgID {
			result = append(result, *o)
		}
	}
	return result, len(result), nil
}

func (m *mockOrderRepo) UpdateOrder(_ context.Context, _ pgx.Tx, o *Order) error {
	cp := *o
	m.orders[o.ID] = &cp
	return nil
}

func (m *mockOrderRepo) UpdateOrderItems(_ context.Context, _ pgx.Tx, orderID uuid.UUID, items []OrderItem) error {
	m.items[orderID] = items
	return nil
}

func (m *mockOrderRepo) NextOrderNumber(_ context.Context, _ pgx.Tx, _ uuid.UUID) (int64, error) {
	m.seq++
	return m.seq, nil
}

func (m *mockOrderRepo) WithTx(_ context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
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
		return e, nil
	}
	return nil, pgx.ErrNoRows
}
func (m *mockEntityRepo) List(context.Context, uuid.UUID, entity.ListFilter) ([]types.Entity, int, error) {
	return nil, 0, nil
}
func (m *mockEntityRepo) Update(_ context.Context, e *types.Entity) error {
	m.entities[e.ID] = e
	return nil
}
func (m *mockEntityRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (m *mockEntityRepo) SetComponent(_ context.Context, c *types.Component) error {
	return nil
}
func (m *mockEntityRepo) GetComponent(context.Context, uuid.UUID, uuid.UUID, string) (*types.Component, error) {
	return nil, nil
}
func (m *mockEntityRepo) ListComponents(context.Context, uuid.UUID, uuid.UUID) ([]types.Component, error) {
	return nil, nil
}
func (m *mockEntityRepo) DeleteComponent(context.Context, uuid.UUID, uuid.UUID, string) error {
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

type mockWarehouse struct {
	available bool
}

func (w *mockWarehouse) CheckAvailability(_ context.Context, _ uuid.UUID, _ []CheckItem) error {
	if !w.available {
		return assert.AnError
	}
	return nil
}

// --- helpers ---

func setupOrderService(wh WarehouseChecker) (*Service, *mockOrderRepo, *mockBus) {
	orderRepo := newMockOrderRepo()
	entityRepo := newMockEntityRepo()
	bus := &mockBus{}
	eventStore := &mockEventStore{}
	entitySvc := entity.NewService(entityRepo, eventStore, bus)
	svc := NewService(orderRepo, entitySvc, bus, wh)
	return svc, orderRepo, bus
}

// --- tests ---

func TestCreateOrder(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()
	productID := uuid.New()

	t.Run("success", func(t *testing.T) {
		svc, repo, bus := setupOrderService(nil)

		o, err := svc.Create(ctx, orgID, CreateOrderInput{
			Items: []CreateItemInput{
				{ProductID: productID, Quantity: 2, UnitPrice: 10000},
				{ProductID: uuid.New(), Quantity: 1, UnitPrice: 5000},
			},
			Notes: "test order",
		}, &actorID)

		require.NoError(t, err)
		assert.Equal(t, "draft", o.Status)
		assert.Equal(t, "ORD-00001", o.Number)
		assert.Equal(t, int64(25000), o.Total) // 2*10000 + 1*5000
		assert.Len(t, o.Items, 2)
		assert.Len(t, repo.orders, 1)

		hasCreatedEvent := false
		for _, ev := range bus.published {
			if ev.Type == "order.created" {
				hasCreatedEvent = true
			}
		}
		assert.True(t, hasCreatedEvent)
	})

	t.Run("no items rejected", func(t *testing.T) {
		svc, _, _ := setupOrderService(nil)

		_, err := svc.Create(ctx, orgID, CreateOrderInput{}, &actorID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "items are required")
	})

	t.Run("sequential numbers", func(t *testing.T) {
		svc, _, _ := setupOrderService(nil)

		o1, _ := svc.Create(ctx, orgID, CreateOrderInput{
			Items: []CreateItemInput{{ProductID: productID, Quantity: 1, UnitPrice: 1000}},
		}, &actorID)
		o2, _ := svc.Create(ctx, orgID, CreateOrderInput{
			Items: []CreateItemInput{{ProductID: productID, Quantity: 1, UnitPrice: 1000}},
		}, &actorID)

		assert.Equal(t, "ORD-00001", o1.Number)
		assert.Equal(t, "ORD-00002", o2.Number)
	})
}

func TestOrderLifecycle(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _, bus := setupOrderService(&mockWarehouse{available: true})

	o, err := svc.Create(ctx, orgID, CreateOrderInput{
		Items: []CreateItemInput{
			{ProductID: uuid.New(), Quantity: 1, UnitPrice: 50000},
		},
	}, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "draft", o.Status)

	// Submit
	o, err = svc.Submit(ctx, orgID, o.ID, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "new", o.Status)

	// Confirm
	o, err = svc.Confirm(ctx, orgID, o.ID, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "confirmed", o.Status)

	// Pay
	o, err = svc.Pay(ctx, orgID, o.ID, PayInput{Amount: 50000, Method: "card"}, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "paid", o.Status)
	assert.NotNil(t, o.PaidAt)

	// Ship
	o, err = svc.Ship(ctx, orgID, o.ID, ShipInput{Tracking: "TRACK-123"}, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "shipped", o.Status)
	assert.NotNil(t, o.ShippedAt)

	// Deliver
	o, err = svc.Deliver(ctx, orgID, o.ID, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "delivered", o.Status)
	assert.NotNil(t, o.DeliveredAt)

	// Check events
	eventTypes := make(map[string]bool)
	for _, ev := range bus.published {
		eventTypes[ev.Type] = true
	}
	assert.True(t, eventTypes["order.created"])
	assert.True(t, eventTypes["order.submitted"])
	assert.True(t, eventTypes["order.confirmed"])
	assert.True(t, eventTypes["order.paid"])
	assert.True(t, eventTypes["order.shipped"])
	assert.True(t, eventTypes["order.delivered"])
}

func TestCancelOrder(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	t.Run("cancel from new", func(t *testing.T) {
		svc, _, _ := setupOrderService(nil)

		o, _ := svc.Create(ctx, orgID, CreateOrderInput{
			Items: []CreateItemInput{{ProductID: uuid.New(), Quantity: 1, UnitPrice: 1000}},
		}, &actorID)

		o, err := svc.Cancel(ctx, orgID, o.ID, "changed mind", &actorID)
		require.NoError(t, err)
		assert.Equal(t, "cancelled", o.Status)
		assert.NotNil(t, o.CancelledAt)
	})

	t.Run("cannot cancel shipped", func(t *testing.T) {
		svc, _, _ := setupOrderService(&mockWarehouse{available: true})

		o, _ := svc.Create(ctx, orgID, CreateOrderInput{
			Items: []CreateItemInput{{ProductID: uuid.New(), Quantity: 1, UnitPrice: 1000}},
		}, &actorID)
		svc.Submit(ctx, orgID, o.ID, &actorID)
		svc.Confirm(ctx, orgID, o.ID, &actorID)
		svc.Pay(ctx, orgID, o.ID, PayInput{Amount: 1000, Method: "cash"}, &actorID)
		svc.Ship(ctx, orgID, o.ID, ShipInput{}, &actorID)

		_, err := svc.Cancel(ctx, orgID, o.ID, "test", &actorID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "can only be cancelled from draft, new, confirmed, or paid")
	})
}

func TestConfirmCheckAvailability(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()
	warehouseID := uuid.New()

	t.Run("insufficient stock blocks confirm", func(t *testing.T) {
		svc, _, _ := setupOrderService(&mockWarehouse{available: false})

		o, _ := svc.Create(ctx, orgID, CreateOrderInput{
			WarehouseID: &warehouseID,
			Items:       []CreateItemInput{{ProductID: uuid.New(), Quantity: 100, UnitPrice: 1000}},
		}, &actorID)
		svc.Submit(ctx, orgID, o.ID, &actorID)

		_, err := svc.Confirm(ctx, orgID, o.ID, &actorID)
		require.Error(t, err)
	})
}

func TestUpdateDraftOrder(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupOrderService(nil)

	o, _ := svc.Create(ctx, orgID, CreateOrderInput{
		Items: []CreateItemInput{{ProductID: uuid.New(), Quantity: 1, UnitPrice: 1000}},
		Notes: "original",
	}, &actorID)

	newNotes := "updated notes"
	o, err := svc.Update(ctx, orgID, o.ID, UpdateOrderInput{
		Notes: &newNotes,
		Items: []CreateItemInput{
			{ProductID: uuid.New(), Quantity: 3, UnitPrice: 2000},
		},
	}, &actorID)

	require.NoError(t, err)
	assert.Equal(t, "updated notes", o.Notes)
	assert.Equal(t, int64(6000), o.Total) // 3 * 2000
}

func TestCannotUpdateConfirmedOrder(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupOrderService(&mockWarehouse{available: true})

	o, _ := svc.Create(ctx, orgID, CreateOrderInput{
		Items: []CreateItemInput{{ProductID: uuid.New(), Quantity: 1, UnitPrice: 1000}},
	}, &actorID)
	svc.Submit(ctx, orgID, o.ID, &actorID)
	svc.Confirm(ctx, orgID, o.ID, &actorID)

	_, err := svc.Update(ctx, orgID, o.ID, UpdateOrderInput{
		Items: []CreateItemInput{{ProductID: uuid.New(), Quantity: 5, UnitPrice: 3000}},
	}, &actorID)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "draft or new")
}

func TestOrderItemTotalCalculation(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupOrderService(nil)

	o, err := svc.Create(ctx, orgID, CreateOrderInput{
		Items: []CreateItemInput{
			{ProductID: uuid.New(), Quantity: 2, UnitPrice: 10000, Discount: 500, Tax: 1800},
			{ProductID: uuid.New(), Quantity: 1, UnitPrice: 5000, Discount: 0, Tax: 900},
		},
	}, &actorID)

	require.NoError(t, err)
	// subtotal = 2*10000 + 1*5000 = 25000
	// discount = 500 + 0 = 500
	// tax = 1800 + 900 = 2700
	// total = 25000 - 500 + 2700 = 27200
	assert.Equal(t, int64(25000), o.Subtotal)
	assert.Equal(t, int64(500), o.Discount)
	assert.Equal(t, int64(2700), o.Tax)
	assert.Equal(t, int64(27200), o.Total)
}
