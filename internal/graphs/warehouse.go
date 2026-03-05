package graphs

import (
	"context"
	"fmt"

	"github.com/FrankFMY/arcana"
)

var warehouseStockList = arcana.GraphDef{
	Key: "warehouse_stock_list",
	Deps: []arcana.TableDep{
		{Table: "stock_levels", Columns: []string{"quantity", "reserved", "updated_at"}},
		{Table: "entities", Columns: []string{"name", "status"}},
	},
	Params: arcana.ParamSchema{
		"warehouse_id": arcana.ParamUUID().Required(),
		"search":       arcana.ParamString().Build(),
		"limit":        arcana.ParamInt().Default(50),
		"offset":       arcana.ParamInt().Default(0),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		warehouseID := p.UUID("warehouse_id")
		limit := p.Int("limit")
		offset := p.Int("offset")
		search := p.String("search")

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

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()

		for rows.Next() {
			var productID, name, sku string
			var quantity, reserved, available, minQty float64
			var maxQty *float64
			var cnt int

			if err := rows.Scan(&productID, &name, &sku, &quantity, &reserved, &available, &minQty, &maxQty, &cnt); err != nil {
				return nil, err
			}

			result.AddRef(arcana.Ref{
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
			result.AddRow("stock_levels", productID, row)
		}

		return result, rows.Err()
	},
}

var warehouseStockDetail = arcana.GraphDef{
	Key: "warehouse_stock_detail",
	Deps: []arcana.TableDep{
		{Table: "stock_levels", Columns: []string{"quantity", "reserved", "updated_at"}},
		{Table: "stock_movements", Columns: []string{"quantity", "type", "created_at"}},
		{Table: "entities", Columns: []string{"name"}},
	},
	Params: arcana.ParamSchema{
		"warehouse_id": arcana.ParamUUID().Required(),
		"product_id":   arcana.ParamUUID().Required(),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		warehouseID := p.UUID("warehouse_id")
		productID := p.UUID("product_id")

		result := arcana.NewResult()

		var name string
		var quantity, reserved float64
		err := q.QueryRow(ctx, `
			SELECT e.name, sl.quantity, sl.reserved
			FROM stock_levels sl
			JOIN entities e ON e.id = sl.product_id AND e.organization_id = $1
			WHERE sl.organization_id = $1 AND sl.warehouse_id = $2 AND sl.product_id = $3
		`, orgID, warehouseID, productID).Scan(&name, &quantity, &reserved)
		if err != nil {
			return nil, err
		}

		result.AddRef(arcana.Ref{
			Table:  "stock_levels",
			ID:     productID,
			Fields: []string{"product_name", "quantity", "reserved", "available"},
		})
		result.AddRow("stock_levels", productID, map[string]any{
			"product_id":   productID,
			"product_name": name,
			"quantity":     quantity,
			"reserved":     reserved,
			"available":    quantity - reserved,
		})

		movRows, err := q.Query(ctx, `
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

			result.AddRef(arcana.Ref{
				Table:  "stock_movements",
				ID:     movID,
				Fields: []string{"type", "quantity", "reason", "created_at"},
			})
			result.AddRow("stock_movements", movID, map[string]any{
				"id":         movID,
				"type":       movType,
				"quantity":   movQty,
				"reason":     reason,
				"created_at": createdAt,
			})
		}

		return result, nil
	},
}

var warehouseLowStock = arcana.GraphDef{
	Key: "warehouse_low_stock",
	Deps: []arcana.TableDep{
		{Table: "stock_levels", Columns: []string{"quantity", "reserved", "min_quantity"}},
		{Table: "entities", Columns: []string{"name"}},
	},
	Params: arcana.ParamSchema{
		"warehouse_id": arcana.ParamUUID().Required(),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		warehouseID := p.UUID("warehouse_id")

		rows, err := q.Query(ctx, `
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

		result := arcana.NewResult()

		for rows.Next() {
			var productID, name string
			var quantity, reserved, available, minQty float64

			if err := rows.Scan(&productID, &name, &quantity, &reserved, &available, &minQty); err != nil {
				return nil, err
			}

			result.AddRef(arcana.Ref{Table: "stock_levels", ID: productID, Fields: []string{"product_name", "quantity", "available", "min_quantity"}})
			result.AddRow("stock_levels", productID, map[string]any{
				"product_id":   productID,
				"product_name": name,
				"quantity":     quantity,
				"reserved":     reserved,
				"available":    available,
				"min_quantity": minQty,
			})
		}

		return result, rows.Err()
	},
}
