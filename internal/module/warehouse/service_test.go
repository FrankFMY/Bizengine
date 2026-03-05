package warehouse

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/types"
)

// --- mocks ---

type mockWarehouseRepo struct {
	stockLevels    map[string]*StockLevel // key: "productID:warehouseID"
	movements      []StockMovement
	inventories    map[uuid.UUID]*Inventory
	inventoryItems map[uuid.UUID][]InventoryItem
}

func newMockRepo() *mockWarehouseRepo {
	return &mockWarehouseRepo{
		stockLevels:    make(map[string]*StockLevel),
		inventories:    make(map[uuid.UUID]*Inventory),
		inventoryItems: make(map[uuid.UUID][]InventoryItem),
	}
}

func stockKey(productID, warehouseID uuid.UUID) string {
	return productID.String() + ":" + warehouseID.String()
}

func (m *mockWarehouseRepo) GetStockLevel(_ context.Context, orgID, productID, warehouseID uuid.UUID) (*StockLevel, error) {
	key := stockKey(productID, warehouseID)
	sl, ok := m.stockLevels[key]
	if !ok || sl.OrganizationID != orgID {
		return nil, pgx.ErrNoRows
	}
	cp := *sl
	return &cp, nil
}

func (m *mockWarehouseRepo) ListStock(_ context.Context, orgID, warehouseID uuid.UUID, _ StockFilter) ([]StockLevel, int, error) {
	var result []StockLevel
	for _, sl := range m.stockLevels {
		if sl.OrganizationID == orgID && sl.WarehouseID == warehouseID {
			result = append(result, *sl)
		}
	}
	return result, len(result), nil
}

func (m *mockWarehouseRepo) GetLowStock(_ context.Context, orgID uuid.UUID) ([]StockLevel, error) {
	var result []StockLevel
	for _, sl := range m.stockLevels {
		if sl.OrganizationID == orgID && sl.MinQuantity > 0 && sl.Quantity-sl.Reserved <= sl.MinQuantity {
			result = append(result, *sl)
		}
	}
	return result, nil
}

func (m *mockWarehouseRepo) UpsertStockLevel(_ context.Context, _ pgx.Tx, sl *StockLevel) error {
	key := stockKey(sl.ProductID, sl.WarehouseID)
	cp := *sl
	m.stockLevels[key] = &cp
	return nil
}

func (m *mockWarehouseRepo) InsertMovement(_ context.Context, _ pgx.Tx, mv *StockMovement) error {
	m.movements = append(m.movements, *mv)
	return nil
}

func (m *mockWarehouseRepo) ListMovements(_ context.Context, orgID uuid.UUID, _ MovementFilter) ([]StockMovement, int, error) {
	var result []StockMovement
	for _, mv := range m.movements {
		if mv.OrganizationID == orgID {
			result = append(result, mv)
		}
	}
	return result, len(result), nil
}

func (m *mockWarehouseRepo) WithTx(_ context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
}

func (m *mockWarehouseRepo) CreateInventory(_ context.Context, _ pgx.Tx, inv *Inventory) error {
	cp := *inv
	m.inventories[inv.ID] = &cp
	return nil
}

func (m *mockWarehouseRepo) GetInventory(_ context.Context, orgID, invID uuid.UUID) (*Inventory, error) {
	inv, ok := m.inventories[invID]
	if !ok || inv.OrganizationID != orgID {
		return nil, pgx.ErrNoRows
	}
	cp := *inv
	return &cp, nil
}

func (m *mockWarehouseRepo) ListInventories(_ context.Context, orgID uuid.UUID, warehouseID *uuid.UUID, _ types.PageRequest) ([]Inventory, int, error) {
	var result []Inventory
	for _, inv := range m.inventories {
		if inv.OrganizationID != orgID {
			continue
		}
		if warehouseID != nil && inv.WarehouseID != *warehouseID {
			continue
		}
		result = append(result, *inv)
	}
	return result, len(result), nil
}

func (m *mockWarehouseRepo) UpdateInventory(_ context.Context, _ pgx.Tx, inv *Inventory) error {
	cp := *inv
	m.inventories[inv.ID] = &cp
	return nil
}

func (m *mockWarehouseRepo) CreateInventoryItems(_ context.Context, _ pgx.Tx, items []InventoryItem) error {
	if len(items) == 0 {
		return nil
	}
	invID := items[0].InventoryID
	m.inventoryItems[invID] = append(m.inventoryItems[invID], items...)
	return nil
}

func (m *mockWarehouseRepo) GetInventoryItems(_ context.Context, inventoryID uuid.UUID) ([]InventoryItem, error) {
	return m.inventoryItems[inventoryID], nil
}

func (m *mockWarehouseRepo) UpdateInventoryItem(_ context.Context, _ pgx.Tx, item *InventoryItem) error {
	items := m.inventoryItems[item.InventoryID]
	for i := range items {
		if items[i].ID == item.ID {
			items[i] = *item
			break
		}
	}
	m.inventoryItems[item.InventoryID] = items
	return nil
}

func (m *mockWarehouseRepo) ListStockForWarehouse(_ context.Context, orgID, warehouseID uuid.UUID) ([]StockLevel, error) {
	var result []StockLevel
	for _, sl := range m.stockLevels {
		if sl.OrganizationID == orgID && sl.WarehouseID == warehouseID {
			result = append(result, *sl)
		}
	}
	return result, nil
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

func setupSvc() (*Service, *mockWarehouseRepo, *mockBus) {
	repo := newMockRepo()
	bus := &mockBus{}
	return NewService(repo, bus), repo, bus
}

// --- tests ---

func TestReceive(t *testing.T) {
	orgID := uuid.New()
	productID := uuid.New()
	warehouseID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	t.Run("new stock", func(t *testing.T) {
		svc, repo, bus := setupSvc()

		result, err := svc.Receive(ctx, orgID, ReceiveInput{
			ProductID:   productID,
			WarehouseID: warehouseID,
			Quantity:    100,
			Unit:        "шт",
			ActorID:     &actorID,
		})

		require.NoError(t, err)
		assert.Equal(t, float64(100), result.NewQuantity)
		assert.Len(t, repo.movements, 1)
		assert.Equal(t, "receive", repo.movements[0].Type)
		assert.True(t, len(bus.published) > 0)
	})

	t.Run("add to existing", func(t *testing.T) {
		svc, repo, _ := setupSvc()
		repo.stockLevels[stockKey(productID, warehouseID)] = &StockLevel{
			OrganizationID: orgID, ProductID: productID, WarehouseID: warehouseID,
			Quantity: 50, Unit: "шт", UpdatedAt: time.Now(),
		}

		result, err := svc.Receive(ctx, orgID, ReceiveInput{
			ProductID:   productID,
			WarehouseID: warehouseID,
			Quantity:    30,
		})

		require.NoError(t, err)
		assert.Equal(t, float64(80), result.NewQuantity)
	})

	t.Run("zero quantity rejected", func(t *testing.T) {
		svc, _, _ := setupSvc()

		_, err := svc.Receive(ctx, orgID, ReceiveInput{
			ProductID:   productID,
			WarehouseID: warehouseID,
			Quantity:    0,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "quantity must be positive")
	})
}

func TestShip(t *testing.T) {
	orgID := uuid.New()
	productID := uuid.New()
	warehouseID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := setupSvc()
		repo.stockLevels[stockKey(productID, warehouseID)] = &StockLevel{
			OrganizationID: orgID, ProductID: productID, WarehouseID: warehouseID,
			Quantity: 100, Reserved: 20, Unit: "шт", UpdatedAt: time.Now(),
		}

		result, err := svc.Ship(ctx, orgID, ShipInput{
			ProductID:   productID,
			WarehouseID: warehouseID,
			Quantity:    10,
			ActorID:     &actorID,
		})

		require.NoError(t, err)
		assert.Equal(t, float64(90), result.NewQuantity)

		sl := repo.stockLevels[stockKey(productID, warehouseID)]
		assert.Equal(t, float64(90), sl.Quantity)
		assert.Equal(t, float64(10), sl.Reserved)
	})

	t.Run("insufficient stock", func(t *testing.T) {
		svc, repo, _ := setupSvc()
		repo.stockLevels[stockKey(productID, warehouseID)] = &StockLevel{
			OrganizationID: orgID, ProductID: productID, WarehouseID: warehouseID,
			Quantity: 10, Reserved: 8, Unit: "шт", UpdatedAt: time.Now(),
		}

		_, err := svc.Ship(ctx, orgID, ShipInput{
			ProductID:   productID,
			WarehouseID: warehouseID,
			Quantity:    5,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient available stock")
	})
}

func TestTransfer(t *testing.T) {
	orgID := uuid.New()
	productID := uuid.New()
	fromWH := uuid.New()
	toWH := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := setupSvc()
		repo.stockLevels[stockKey(productID, fromWH)] = &StockLevel{
			OrganizationID: orgID, ProductID: productID, WarehouseID: fromWH,
			Quantity: 100, Unit: "шт", UpdatedAt: time.Now(),
		}

		result, err := svc.Transfer(ctx, orgID, TransferInput{
			ProductID:       productID,
			FromWarehouseID: fromWH,
			ToWarehouseID:   toWH,
			Quantity:        30,
			Unit:            "шт",
			ActorID:         &actorID,
		})

		require.NoError(t, err)
		assert.NotEqual(t, uuid.Nil, result.MovementID)

		srcSL := repo.stockLevels[stockKey(productID, fromWH)]
		dstSL := repo.stockLevels[stockKey(productID, toWH)]
		assert.Equal(t, float64(70), srcSL.Quantity)
		assert.Equal(t, float64(30), dstSL.Quantity)
	})

	t.Run("same warehouse rejected", func(t *testing.T) {
		svc, _, _ := setupSvc()

		_, err := svc.Transfer(ctx, orgID, TransferInput{
			ProductID:       productID,
			FromWarehouseID: fromWH,
			ToWarehouseID:   fromWH,
			Quantity:        10,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "source and destination")
	})
}

func TestAdjust(t *testing.T) {
	orgID := uuid.New()
	productID := uuid.New()
	warehouseID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	t.Run("adjust up", func(t *testing.T) {
		svc, repo, _ := setupSvc()
		repo.stockLevels[stockKey(productID, warehouseID)] = &StockLevel{
			OrganizationID: orgID, ProductID: productID, WarehouseID: warehouseID,
			Quantity: 95, Unit: "шт", UpdatedAt: time.Now(),
		}

		result, err := svc.Adjust(ctx, orgID, AdjustInput{
			ProductID:   productID,
			WarehouseID: warehouseID,
			NewQuantity: 100,
			Reason:      "Inventory count",
			ActorID:     &actorID,
		})

		require.NoError(t, err)
		assert.Equal(t, float64(95), result.OldQuantity)
		assert.Equal(t, float64(100), result.NewQuantity)

		sl := repo.stockLevels[stockKey(productID, warehouseID)]
		assert.Equal(t, float64(100), sl.Quantity)
	})

	t.Run("missing reason rejected", func(t *testing.T) {
		svc, _, _ := setupSvc()

		_, err := svc.Adjust(ctx, orgID, AdjustInput{
			ProductID:   productID,
			WarehouseID: warehouseID,
			NewQuantity: 100,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "reason is required")
	})
}

func TestReserveUnreserve(t *testing.T) {
	orgID := uuid.New()
	productID := uuid.New()
	warehouseID := uuid.New()
	ctx := context.Background()

	t.Run("reserve success", func(t *testing.T) {
		svc, repo, _ := setupSvc()
		repo.stockLevels[stockKey(productID, warehouseID)] = &StockLevel{
			OrganizationID: orgID, ProductID: productID, WarehouseID: warehouseID,
			Quantity: 100, Reserved: 0, Unit: "шт", UpdatedAt: time.Now(),
		}

		err := svc.Reserve(ctx, orgID, ReserveInput{
			ProductID:   productID,
			WarehouseID: warehouseID,
			Quantity:    10,
		})

		require.NoError(t, err)
		sl := repo.stockLevels[stockKey(productID, warehouseID)]
		assert.Equal(t, float64(10), sl.Reserved)
	})

	t.Run("reserve insufficient", func(t *testing.T) {
		svc, repo, _ := setupSvc()
		repo.stockLevels[stockKey(productID, warehouseID)] = &StockLevel{
			OrganizationID: orgID, ProductID: productID, WarehouseID: warehouseID,
			Quantity: 10, Reserved: 8, Unit: "шт", UpdatedAt: time.Now(),
		}

		err := svc.Reserve(ctx, orgID, ReserveInput{
			ProductID:   productID,
			WarehouseID: warehouseID,
			Quantity:    5,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient stock to reserve")
	})

	t.Run("unreserve success", func(t *testing.T) {
		svc, repo, _ := setupSvc()
		repo.stockLevels[stockKey(productID, warehouseID)] = &StockLevel{
			OrganizationID: orgID, ProductID: productID, WarehouseID: warehouseID,
			Quantity: 100, Reserved: 20, Unit: "шт", UpdatedAt: time.Now(),
		}

		err := svc.Unreserve(ctx, orgID, UnreserveInput{
			ProductID:   productID,
			WarehouseID: warehouseID,
			Quantity:    10,
		})

		require.NoError(t, err)
		sl := repo.stockLevels[stockKey(productID, warehouseID)]
		assert.Equal(t, float64(10), sl.Reserved)
	})
}

func TestCheckAvailability(t *testing.T) {
	orgID := uuid.New()
	productA := uuid.New()
	productB := uuid.New()
	warehouseID := uuid.New()
	ctx := context.Background()

	svc, repo, _ := setupSvc()
	repo.stockLevels[stockKey(productA, warehouseID)] = &StockLevel{
		OrganizationID: orgID, ProductID: productA, WarehouseID: warehouseID,
		Quantity: 100, Reserved: 0, Unit: "шт", UpdatedAt: time.Now(),
	}
	repo.stockLevels[stockKey(productB, warehouseID)] = &StockLevel{
		OrganizationID: orgID, ProductID: productB, WarehouseID: warehouseID,
		Quantity: 5, Reserved: 3, Unit: "шт", UpdatedAt: time.Now(),
	}

	t.Run("all available", func(t *testing.T) {
		err := svc.CheckAvailability(ctx, orgID, []CheckItem{
			{ProductID: productA, WarehouseID: warehouseID, Quantity: 50},
			{ProductID: productB, WarehouseID: warehouseID, Quantity: 2},
		})
		require.NoError(t, err)
	})

	t.Run("insufficient for one", func(t *testing.T) {
		err := svc.CheckAvailability(ctx, orgID, []CheckItem{
			{ProductID: productA, WarehouseID: warehouseID, Quantity: 50},
			{ProductID: productB, WarehouseID: warehouseID, Quantity: 5},
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "insufficient stock")
	})
}
