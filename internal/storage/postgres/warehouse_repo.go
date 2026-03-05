package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/module/warehouse"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// WarehouseRepo implements warehouse.Repository using PostgreSQL.
type WarehouseRepo struct {
	pool *pgxpool.Pool
}

// NewWarehouseRepo creates a new WarehouseRepo.
func NewWarehouseRepo(pool *pgxpool.Pool) *WarehouseRepo {
	return &WarehouseRepo{pool: pool}
}

// GetStockLevel returns a stock level for a product in a warehouse.
func (r *WarehouseRepo) GetStockLevel(ctx context.Context, orgID, productID, warehouseID uuid.UUID) (*warehouse.StockLevel, error) {
	var sl warehouse.StockLevel
	err := r.pool.QueryRow(ctx,
		`SELECT organization_id, product_id, warehouse_id, quantity, reserved, unit, min_quantity, max_quantity, cost_per_unit, updated_at
		 FROM stock_levels
		 WHERE organization_id = $1 AND product_id = $2 AND warehouse_id = $3`,
		orgID, productID, warehouseID,
	).Scan(&sl.OrganizationID, &sl.ProductID, &sl.WarehouseID, &sl.Quantity, &sl.Reserved,
		&sl.Unit, &sl.MinQuantity, &sl.MaxQuantity, &sl.CostPerUnit, &sl.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("stock level not found")
		}
		return nil, err
	}
	sl.Available = sl.Quantity - sl.Reserved
	return &sl, nil
}

// ListStock returns stock levels for a warehouse.
func (r *WarehouseRepo) ListStock(ctx context.Context, orgID, warehouseID uuid.UUID, filter warehouse.StockFilter) ([]warehouse.StockLevel, int, error) {
	filter.Page.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("sl.organization_id = $%d", argIdx))
	args = append(args, orgID)
	argIdx++

	conditions = append(conditions, fmt.Sprintf("sl.warehouse_id = $%d", argIdx))
	args = append(args, warehouseID)
	argIdx++

	if filter.LowStock != nil && *filter.LowStock {
		conditions = append(conditions, "sl.quantity - sl.reserved <= sl.min_quantity AND sl.min_quantity > 0")
	}

	if filter.Search != nil && *filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf(
			"EXISTS (SELECT 1 FROM entities e WHERE e.id = sl.product_id AND e.name ILIKE '%%' || $%d || '%%')", argIdx))
		args = append(args, *filter.Search)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_levels sl WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT sl.organization_id, sl.product_id, sl.warehouse_id, sl.quantity, sl.reserved, sl.unit, sl.min_quantity, sl.max_quantity, sl.updated_at
		 FROM stock_levels sl WHERE %s ORDER BY sl.updated_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, filter.Page.Limit, filter.Page.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []warehouse.StockLevel
	for rows.Next() {
		var sl warehouse.StockLevel
		if err := rows.Scan(&sl.OrganizationID, &sl.ProductID, &sl.WarehouseID, &sl.Quantity, &sl.Reserved,
			&sl.Unit, &sl.MinQuantity, &sl.MaxQuantity, &sl.UpdatedAt); err != nil {
			return nil, 0, err
		}
		sl.Available = sl.Quantity - sl.Reserved
		items = append(items, sl)
	}
	return items, total, rows.Err()
}

// GetLowStock returns stock levels where available <= min_quantity.
func (r *WarehouseRepo) GetLowStock(ctx context.Context, orgID uuid.UUID) ([]warehouse.StockLevel, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT organization_id, product_id, warehouse_id, quantity, reserved, unit, min_quantity, max_quantity, updated_at
		 FROM stock_levels
		 WHERE organization_id = $1 AND quantity - reserved <= min_quantity AND min_quantity > 0`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []warehouse.StockLevel
	for rows.Next() {
		var sl warehouse.StockLevel
		if err := rows.Scan(&sl.OrganizationID, &sl.ProductID, &sl.WarehouseID, &sl.Quantity, &sl.Reserved,
			&sl.Unit, &sl.MinQuantity, &sl.MaxQuantity, &sl.UpdatedAt); err != nil {
			return nil, err
		}
		sl.Available = sl.Quantity - sl.Reserved
		items = append(items, sl)
	}
	return items, rows.Err()
}

// UpsertStockLevel creates or updates a stock level.
func (r *WarehouseRepo) UpsertStockLevel(ctx context.Context, tx pgx.Tx, sl *warehouse.StockLevel) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO stock_levels (organization_id, product_id, warehouse_id, quantity, reserved, unit, min_quantity, max_quantity, cost_per_unit, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		 ON CONFLICT (organization_id, product_id, warehouse_id) DO UPDATE SET
		   quantity = $4, reserved = $5, unit = $6, min_quantity = $7, max_quantity = $8, cost_per_unit = $9, updated_at = $10`,
		sl.OrganizationID, sl.ProductID, sl.WarehouseID, sl.Quantity, sl.Reserved,
		sl.Unit, sl.MinQuantity, sl.MaxQuantity, sl.CostPerUnit, sl.UpdatedAt,
	)
	return err
}

// InsertMovement records a stock movement.
func (r *WarehouseRepo) InsertMovement(ctx context.Context, tx pgx.Tx, m *warehouse.StockMovement) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO stock_movements (id, organization_id, product_id, warehouse_id, type, quantity, unit, cost_per_unit, reason, reference_type, reference_id, dest_warehouse_id, actor_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		m.ID, m.OrganizationID, m.ProductID, m.WarehouseID, m.Type, m.Quantity, m.Unit,
		m.CostPerUnit, m.Reason, m.ReferenceType, m.ReferenceID, m.DestWarehouseID, m.ActorID, m.CreatedAt,
	)
	return err
}

// ListMovements returns stock movements with optional filters.
func (r *WarehouseRepo) ListMovements(ctx context.Context, orgID uuid.UUID, filter warehouse.MovementFilter) ([]warehouse.StockMovement, int, error) {
	filter.Page.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("organization_id = $%d", argIdx))
	args = append(args, orgID)
	argIdx++

	if filter.ProductID != nil {
		conditions = append(conditions, fmt.Sprintf("product_id = $%d", argIdx))
		args = append(args, *filter.ProductID)
		argIdx++
	}
	if filter.WarehouseID != nil {
		conditions = append(conditions, fmt.Sprintf("warehouse_id = $%d", argIdx))
		args = append(args, *filter.WarehouseID)
		argIdx++
	}
	if filter.Type != nil {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *filter.Type)
		argIdx++
	}
	if filter.Since != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *filter.Since)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM stock_movements WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT id, organization_id, product_id, warehouse_id, type, quantity, unit, cost_per_unit, reason, reference_type, reference_id, dest_warehouse_id, actor_id, created_at
		 FROM stock_movements WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, filter.Page.Limit, filter.Page.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []warehouse.StockMovement
	for rows.Next() {
		var m warehouse.StockMovement
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.ProductID, &m.WarehouseID, &m.Type, &m.Quantity,
			&m.Unit, &m.CostPerUnit, &m.Reason, &m.ReferenceType, &m.ReferenceID, &m.DestWarehouseID,
			&m.ActorID, &m.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, m)
	}
	return items, total, rows.Err()
}

// CreateInventory inserts an inventory record.
func (r *WarehouseRepo) CreateInventory(ctx context.Context, tx pgx.Tx, inv *warehouse.Inventory) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO inventories (id, organization_id, warehouse_id, status, notes, actor_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		inv.ID, inv.OrganizationID, inv.WarehouseID, inv.Status, inv.Notes, inv.ActorID, inv.CreatedAt,
	)
	return err
}

// GetInventory returns an inventory by ID.
func (r *WarehouseRepo) GetInventory(ctx context.Context, orgID, invID uuid.UUID) (*warehouse.Inventory, error) {
	var inv warehouse.Inventory
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, warehouse_id, status, notes, actor_id, created_at, applied_at
		 FROM inventories WHERE organization_id = $1 AND id = $2`,
		orgID, invID,
	).Scan(&inv.ID, &inv.OrganizationID, &inv.WarehouseID, &inv.Status, &inv.Notes, &inv.ActorID, &inv.CreatedAt, &inv.AppliedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("inventory not found")
		}
		return nil, err
	}
	return &inv, nil
}

// ListInventories returns inventories for an organization.
func (r *WarehouseRepo) ListInventories(ctx context.Context, orgID uuid.UUID, warehouseID *uuid.UUID, page types.PageRequest) ([]warehouse.Inventory, int, error) {
	page.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("organization_id = $%d", argIdx))
	args = append(args, orgID)
	argIdx++

	if warehouseID != nil {
		conditions = append(conditions, fmt.Sprintf("warehouse_id = $%d", argIdx))
		args = append(args, *warehouseID)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM inventories WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT id, organization_id, warehouse_id, status, notes, actor_id, created_at, applied_at
		 FROM inventories WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, page.Limit, page.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var invs []warehouse.Inventory
	for rows.Next() {
		var inv warehouse.Inventory
		if err := rows.Scan(&inv.ID, &inv.OrganizationID, &inv.WarehouseID, &inv.Status, &inv.Notes, &inv.ActorID, &inv.CreatedAt, &inv.AppliedAt); err != nil {
			return nil, 0, err
		}
		invs = append(invs, inv)
	}
	return invs, total, rows.Err()
}

// UpdateInventory updates an inventory record.
func (r *WarehouseRepo) UpdateInventory(ctx context.Context, tx pgx.Tx, inv *warehouse.Inventory) error {
	_, err := tx.Exec(ctx,
		`UPDATE inventories SET status = $2, applied_at = $3 WHERE id = $1`,
		inv.ID, inv.Status, inv.AppliedAt,
	)
	return err
}

// CreateInventoryItems inserts inventory item records.
func (r *WarehouseRepo) CreateInventoryItems(ctx context.Context, tx pgx.Tx, items []warehouse.InventoryItem) error {
	for _, item := range items {
		if _, err := tx.Exec(ctx,
			`INSERT INTO inventory_items (id, inventory_id, organization_id, product_id, expected, actual, discrepancy, counted_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			item.ID, item.InventoryID, item.OrganizationID, item.ProductID, item.Expected, item.Actual, item.Discrepancy, item.CountedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

// GetInventoryItems returns items for an inventory session.
func (r *WarehouseRepo) GetInventoryItems(ctx context.Context, inventoryID uuid.UUID) ([]warehouse.InventoryItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, inventory_id, organization_id, product_id, expected, actual, discrepancy, counted_at
		 FROM inventory_items WHERE inventory_id = $1 ORDER BY product_id`,
		inventoryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []warehouse.InventoryItem
	for rows.Next() {
		var item warehouse.InventoryItem
		if err := rows.Scan(&item.ID, &item.InventoryID, &item.OrganizationID, &item.ProductID, &item.Expected, &item.Actual, &item.Discrepancy, &item.CountedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// UpdateInventoryItem updates an inventory item (actual count).
func (r *WarehouseRepo) UpdateInventoryItem(ctx context.Context, tx pgx.Tx, item *warehouse.InventoryItem) error {
	_, err := tx.Exec(ctx,
		`UPDATE inventory_items SET actual = $2, discrepancy = $3, counted_at = $4 WHERE id = $1`,
		item.ID, item.Actual, item.Discrepancy, item.CountedAt,
	)
	return err
}

// ListStockForWarehouse returns all stock levels for a warehouse (no pagination).
func (r *WarehouseRepo) ListStockForWarehouse(ctx context.Context, orgID, warehouseID uuid.UUID) ([]warehouse.StockLevel, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT organization_id, product_id, warehouse_id, quantity, reserved, unit, min_quantity, max_quantity, cost_per_unit, updated_at
		 FROM stock_levels WHERE organization_id = $1 AND warehouse_id = $2`,
		orgID, warehouseID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []warehouse.StockLevel
	for rows.Next() {
		var sl warehouse.StockLevel
		if err := rows.Scan(&sl.OrganizationID, &sl.ProductID, &sl.WarehouseID, &sl.Quantity, &sl.Reserved,
			&sl.Unit, &sl.MinQuantity, &sl.MaxQuantity, &sl.CostPerUnit, &sl.UpdatedAt); err != nil {
			return nil, err
		}
		sl.Available = sl.Quantity - sl.Reserved
		items = append(items, sl)
	}
	return items, rows.Err()
}

// WithTx executes fn within a transaction.
func (r *WarehouseRepo) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
