// Package defs contains reactive view definitions for all business modules.
package defs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/views"
)

// WarehouseStockList returns a paginated list of stock levels for a warehouse.
var WarehouseStockList = &views.ViewDef{
	Key: "warehouse_stock_list",
	Tables: []views.TableDep{
		{Table: "stock_levels", Columns: []string{"quantity", "reserved", "updated_at"}},
		{Table: "entities", Columns: []string{"name", "status"}, Filter: "kind=product"},
	},
	ParamSchema: map[string]string{
		"warehouse_id": "uuid",
	},
	Factory: warehouseStockListFactory,
}

func warehouseStockListFactory(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, params map[string]any) (*views.ViewResult, error) {
	warehouseID, _ := params["warehouse_id"].(string)
	limit := intParam(params, "limit", 50)
	offset := intParam(params, "offset", 0)
	search, _ := params["search"].(string)

	query := `
		SELECT sl.product_id, e.name, COALESCE(cp.data->>'sku','') AS sku,
		       sl.quantity, sl.reserved, sl.quantity - sl.reserved AS available,
		       sl.min_quantity, sl.max_quantity, COUNT(*) OVER() AS total_count
		FROM stock_levels sl
		JOIN entities e ON e.id = sl.product_id AND e.organization_id = $1
		LEFT JOIN components cp ON cp.entity_id = sl.product_id AND cp.type = 'barcode' AND cp.organization_id = $1
		WHERE sl.organization_id = $1 AND sl.warehouse_id = $2
	`
	args := []any{orgID, warehouseID}
	argIdx := 3

	if search != "" {
		query += fmt.Sprintf(` AND e.name ILIKE $%d`, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY e.name LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]views.DataRef, 0)
	tables := map[string]map[string]any{"stock_levels": {}}
	var total int

	for rows.Next() {
		var productID, name, sku string
		var quantity, reserved, available, minQty float64
		var maxQty *float64
		var cnt int

		if err := rows.Scan(&productID, &name, &sku, &quantity, &reserved, &available, &minQty, &maxQty, &cnt); err != nil {
			return nil, err
		}
		total = cnt

		refs = append(refs, views.DataRef{
			Table:  "stock_levels",
			ID:     productID,
			Fields: []string{"product_name", "sku", "quantity", "reserved", "available", "min_quantity", "max_quantity"},
		})

		row := map[string]any{
			"product_id":   productID,
			"product_name": name,
			"sku":          sku,
			"quantity":     quantity,
			"reserved":     reserved,
			"available":    available,
			"min_quantity": minQty,
		}
		if maxQty != nil {
			row["max_quantity"] = *maxQty
		}
		tables["stock_levels"][productID] = row
	}

	return &views.ViewResult{
		Refs:    refs,
		Tables:  tables,
		Version: int64(total),
	}, nil
}

// WarehouseStockDetail returns detailed stock info for a single product in a warehouse.
var WarehouseStockDetail = &views.ViewDef{
	Key: "warehouse_stock_detail",
	Tables: []views.TableDep{
		{Table: "stock_levels", Columns: []string{"quantity", "reserved", "updated_at"}},
		{Table: "stock_movements", Columns: []string{"quantity", "type", "created_at"}},
		{Table: "entities", Columns: []string{"name"}, Filter: "kind=product"},
	},
	ParamSchema: map[string]string{
		"warehouse_id": "uuid",
		"product_id":   "uuid",
	},
	Factory: warehouseStockDetailFactory,
}

func warehouseStockDetailFactory(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, params map[string]any) (*views.ViewResult, error) {
	warehouseID, _ := params["warehouse_id"].(string)
	productID, _ := params["product_id"].(string)

	tables := map[string]map[string]any{"stock_levels": {}, "stock_movements": {}}
	refs := make([]views.DataRef, 0, 1)

	// Stock level
	var name string
	var quantity, reserved float64
	err := pool.QueryRow(ctx, `
		SELECT e.name, sl.quantity, sl.reserved
		FROM stock_levels sl
		JOIN entities e ON e.id = sl.product_id AND e.organization_id = $1
		WHERE sl.organization_id = $1 AND sl.warehouse_id = $2 AND sl.product_id = $3
	`, orgID, warehouseID, productID).Scan(&name, &quantity, &reserved)
	if err != nil {
		return nil, err
	}

	refs = append(refs, views.DataRef{
		Table:  "stock_levels",
		ID:     productID,
		Fields: []string{"product_name", "quantity", "reserved", "available"},
	})
	tables["stock_levels"][productID] = map[string]any{
		"product_id":   productID,
		"product_name": name,
		"quantity":     quantity,
		"reserved":     reserved,
		"available":    quantity - reserved,
	}

	// Recent movements (last 20)
	movRows, err := pool.Query(ctx, `
		SELECT id, type, quantity, reason, created_at
		FROM stock_movements
		WHERE organization_id = $1 AND warehouse_id = $2 AND product_id = $3
		ORDER BY created_at DESC LIMIT 20
	`, orgID, warehouseID, productID)
	if err != nil {
		return nil, err
	}
	defer movRows.Close()

	for movRows.Next() {
		var movID, movType, reason string
		var movQty float64
		var createdAt any

		if err := movRows.Scan(&movID, &movType, &movQty, &reason, &createdAt); err != nil {
			return nil, err
		}

		refs = append(refs, views.DataRef{
			Table:  "stock_movements",
			ID:     movID,
			Fields: []string{"type", "quantity", "reason", "created_at"},
		})
		tables["stock_movements"][movID] = map[string]any{
			"id":         movID,
			"type":       movType,
			"quantity":   movQty,
			"reason":     reason,
			"created_at": createdAt,
		}
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}

// WarehouseLowStock returns products with stock below min_quantity.
var WarehouseLowStock = &views.ViewDef{
	Key: "warehouse_low_stock",
	Tables: []views.TableDep{
		{Table: "stock_levels", Columns: []string{"quantity", "reserved", "min_quantity"}},
		{Table: "entities", Columns: []string{"name"}, Filter: "kind=product"},
	},
	ParamSchema: map[string]string{
		"warehouse_id": "uuid",
	},
	Factory: warehouseLowStockFactory,
}

func warehouseLowStockFactory(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, params map[string]any) (*views.ViewResult, error) {
	warehouseID, _ := params["warehouse_id"].(string)

	rows, err := pool.Query(ctx, `
		SELECT sl.product_id, e.name, sl.quantity, sl.reserved,
		       sl.quantity - sl.reserved AS available, sl.min_quantity
		FROM stock_levels sl
		JOIN entities e ON e.id = sl.product_id AND e.organization_id = $1
		WHERE sl.organization_id = $1 AND sl.warehouse_id = $2
		  AND sl.quantity - sl.reserved <= sl.min_quantity AND sl.min_quantity > 0
		ORDER BY (sl.quantity - sl.reserved) / NULLIF(sl.min_quantity, 0) ASC
	`, orgID, warehouseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]views.DataRef, 0)
	tables := map[string]map[string]any{"stock_levels": {}}

	for rows.Next() {
		var productID, name string
		var quantity, reserved, available, minQty float64

		if err := rows.Scan(&productID, &name, &quantity, &reserved, &available, &minQty); err != nil {
			return nil, err
		}

		refs = append(refs, views.DataRef{Table: "stock_levels", ID: productID, Fields: []string{"product_name", "quantity", "available", "min_quantity"}})
		tables["stock_levels"][productID] = map[string]any{
			"product_id":   productID,
			"product_name": name,
			"quantity":     quantity,
			"reserved":     reserved,
			"available":    available,
			"min_quantity": minQty,
		}
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}

func intParam(params map[string]any, key string, def int) int {
	v, ok := params[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return def
	}
}
