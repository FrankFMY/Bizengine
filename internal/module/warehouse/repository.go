// Package warehouse provides the stock management module.
package warehouse

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/bizengine/engine/pkg/types"
)

// Repository defines warehouse-specific storage operations.
type Repository interface {
	// GetStockLevel returns a stock level for a product in a warehouse.
	GetStockLevel(ctx context.Context, orgID, productID, warehouseID uuid.UUID) (*StockLevel, error)

	// ListStock returns stock levels for a warehouse with optional filters.
	ListStock(ctx context.Context, orgID, warehouseID uuid.UUID, filter StockFilter) ([]StockLevel, int, error)

	// GetLowStock returns stock levels where available <= min_quantity.
	GetLowStock(ctx context.Context, orgID uuid.UUID) ([]StockLevel, error)

	// UpsertStockLevel creates or updates a stock level.
	UpsertStockLevel(ctx context.Context, tx pgx.Tx, sl *StockLevel) error

	// InsertMovement records a stock movement.
	InsertMovement(ctx context.Context, tx pgx.Tx, m *StockMovement) error

	// ListMovements returns stock movements with optional filters.
	ListMovements(ctx context.Context, orgID uuid.UUID, filter MovementFilter) ([]StockMovement, int, error)

	// Inventory
	CreateInventory(ctx context.Context, tx pgx.Tx, inv *Inventory) error
	GetInventory(ctx context.Context, orgID, invID uuid.UUID) (*Inventory, error)
	ListInventories(ctx context.Context, orgID uuid.UUID, warehouseID *uuid.UUID, page types.PageRequest) ([]Inventory, int, error)
	UpdateInventory(ctx context.Context, tx pgx.Tx, inv *Inventory) error
	CreateInventoryItems(ctx context.Context, tx pgx.Tx, items []InventoryItem) error
	GetInventoryItems(ctx context.Context, inventoryID uuid.UUID) ([]InventoryItem, error)
	UpdateInventoryItem(ctx context.Context, tx pgx.Tx, item *InventoryItem) error
	ListStockForWarehouse(ctx context.Context, orgID, warehouseID uuid.UUID) ([]StockLevel, error)

	// WithTx executes fn within a transaction.
	WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// StockLevel represents current stock for a product in a warehouse.
type StockLevel struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	ProductID      uuid.UUID `json:"product_id"`
	WarehouseID    uuid.UUID `json:"warehouse_id"`
	Quantity       float64   `json:"quantity"`
	Reserved       float64   `json:"reserved"`
	Available      float64   `json:"available"`
	Unit           string    `json:"unit"`
	CostPerUnit    *int64    `json:"cost_per_unit,omitempty"`
	MinQuantity    float64   `json:"min_quantity"`
	MaxQuantity    *float64  `json:"max_quantity,omitempty"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// StockMovement represents a stock movement record.
type StockMovement struct {
	ID              uuid.UUID  `json:"id"`
	OrganizationID  uuid.UUID  `json:"organization_id"`
	ProductID       uuid.UUID  `json:"product_id"`
	WarehouseID     uuid.UUID  `json:"warehouse_id"`
	Type            string     `json:"type"`
	Quantity        float64    `json:"quantity"`
	Unit            string     `json:"unit"`
	CostPerUnit     *int64     `json:"cost_per_unit,omitempty"`
	Reason          string     `json:"reason"`
	ReferenceType   *string    `json:"reference_type,omitempty"`
	ReferenceID     *uuid.UUID `json:"reference_id,omitempty"`
	DestWarehouseID *uuid.UUID `json:"dest_warehouse_id,omitempty"`
	ActorID         *uuid.UUID `json:"actor_id,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// StockFilter defines stock listing parameters.
type StockFilter struct {
	Search   *string
	LowStock *bool
	Page     types.PageRequest
}

// MovementFilter defines movement listing parameters.
type MovementFilter struct {
	ProductID   *uuid.UUID
	WarehouseID *uuid.UUID
	Type        *string
	Since       *time.Time
	Page        types.PageRequest
}

// MovementResult is returned by stock operations.
type MovementResult struct {
	MovementID  uuid.UUID `json:"movement_id"`
	NewQuantity float64   `json:"new_quantity"`
	OldQuantity float64   `json:"old_quantity,omitempty"`
}

// Inventory represents a stocktaking session.
type Inventory struct {
	ID             uuid.UUID       `json:"id"`
	OrganizationID uuid.UUID       `json:"organization_id"`
	WarehouseID    uuid.UUID       `json:"warehouse_id"`
	Status         string          `json:"status"` // draft, in_progress, applied, cancelled
	Notes          string          `json:"notes"`
	ActorID        *uuid.UUID      `json:"actor_id,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	AppliedAt      *time.Time      `json:"applied_at,omitempty"`
	Items          []InventoryItem `json:"items,omitempty"`
}

// InventoryItem represents a single product count in an inventory session.
type InventoryItem struct {
	ID             uuid.UUID  `json:"id"`
	InventoryID    uuid.UUID  `json:"inventory_id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	ProductID      uuid.UUID  `json:"product_id"`
	Expected       float64    `json:"expected"`
	Actual         *float64   `json:"actual"`
	Discrepancy    float64    `json:"discrepancy"`
	CountedAt      *time.Time `json:"counted_at,omitempty"`
}

// StartInventoryInput is the input for starting a stocktaking session.
type StartInventoryInput struct {
	WarehouseID uuid.UUID `json:"warehouse_id"`
	Notes       string    `json:"notes"`
}
