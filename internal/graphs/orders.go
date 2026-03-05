package graphs

import (
	"context"
	"fmt"

	"github.com/FrankFMY/arcana"
)

var ordersList = arcana.GraphDef{
	Key: "orders_list",
	Deps: []arcana.TableDep{
		{Table: "orders", Columns: []string{"status", "total", "updated_at"}},
		{Table: "entities", Columns: []string{"name"}},
	},
	Params: arcana.ParamSchema{
		"status": arcana.ParamString().Build(),
		"limit":  arcana.ParamInt().Default(50),
		"offset": arcana.ParamInt().Default(0),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		limit := p.Int("limit")
		offset := p.Int("offset")
		status := p.String("status")

		query := `
			SELECT o.id, o.number, COALESCE(c.name,'') AS customer_name,
			       o.status, o.total, o.created_at, COUNT(*) OVER() AS total_count
			FROM orders o
			LEFT JOIN entities c ON c.id = o.customer_id AND c.organization_id = $1
			WHERE o.organization_id = $1
		`
		args := []any{orgID}
		argIdx := 2

		if status != "" {
			query += fmt.Sprintf(` AND o.status = $%d`, argIdx)
			args = append(args, status)
			argIdx++
		}

		query += fmt.Sprintf(` ORDER BY o.created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
		args = append(args, limit, offset)

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()

		for rows.Next() {
			var id, number, customerName, st string
			var totalAmt int64
			var createdAt any
			var cnt int

			if err := rows.Scan(&id, &number, &customerName, &st, &totalAmt, &createdAt, &cnt); err != nil {
				return nil, err
			}

			result.AddRef(arcana.Ref{Table: "orders", ID: id, Fields: []string{"number", "customer_name", "status", "total", "created_at"}})
			result.AddRow("orders", id, map[string]any{
				"id":            id,
				"number":        number,
				"customer_name": customerName,
				"status":        st,
				"total":         totalAmt,
				"created_at":    createdAt,
			})
		}

		return result, rows.Err()
	},
}

var orderDetail = arcana.GraphDef{
	Key: "order_detail",
	Deps: []arcana.TableDep{
		{Table: "orders", Columns: []string{"status", "total", "updated_at"}},
		{Table: "order_items", Columns: []string{"quantity", "unit_price", "total"}},
		{Table: "entities", Columns: []string{"name"}},
	},
	Params: arcana.ParamSchema{
		"order_id": arcana.ParamUUID().Required(),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		orderID := p.UUID("order_id")

		result := arcana.NewResult()

		var id, number, status string
		var customerName *string
		var subtotal, discount, tax, total int64
		var paidAt, shippedAt, deliveredAt, createdAt any

		err := q.QueryRow(ctx, `
			SELECT o.id, o.number, c.name, o.status, o.subtotal, o.discount, o.tax, o.total,
			       o.paid_at, o.shipped_at, o.delivered_at, o.created_at
			FROM orders o
			LEFT JOIN entities c ON c.id = o.customer_id AND c.organization_id = $1
			WHERE o.organization_id = $1 AND o.id = $2
		`, orgID, orderID).Scan(&id, &number, &customerName, &status, &subtotal, &discount, &tax, &total,
			&paidAt, &shippedAt, &deliveredAt, &createdAt)
		if err != nil {
			return nil, err
		}

		result.AddRef(arcana.Ref{Table: "orders", ID: id, Fields: []string{"number", "customer_name", "status", "subtotal", "discount", "tax", "total"}})
		orderRow := map[string]any{
			"id":           id,
			"number":       number,
			"status":       status,
			"subtotal":     subtotal,
			"discount":     discount,
			"tax":          tax,
			"total":        total,
			"paid_at":      paidAt,
			"shipped_at":   shippedAt,
			"delivered_at": deliveredAt,
			"created_at":   createdAt,
		}
		if customerName != nil {
			orderRow["customer_name"] = *customerName
		}
		result.AddRow("orders", id, orderRow)

		itemRows, err := q.Query(ctx, `
			SELECT oi.id, oi.name, oi.sku, oi.quantity, oi.unit_price, oi.discount, oi.tax, oi.total
			FROM order_items oi
			WHERE oi.organization_id = $1 AND oi.order_id = $2
			ORDER BY oi.sort_order
		`, orgID, orderID)
		if err != nil {
			return nil, err
		}
		defer itemRows.Close()

		for itemRows.Next() {
			var itemID, itemName, sku string
			var qty float64
			var unitPrice, itemDiscount, itemTax, itemTotal int64

			if err := itemRows.Scan(&itemID, &itemName, &sku, &qty, &unitPrice, &itemDiscount, &itemTax, &itemTotal); err != nil {
				return nil, err
			}

			result.AddRef(arcana.Ref{Table: "order_items", ID: itemID, Fields: []string{"name", "quantity", "unit_price", "total"}})
			result.AddRow("order_items", itemID, map[string]any{
				"id":         itemID,
				"name":       itemName,
				"sku":        sku,
				"quantity":   qty,
				"unit_price": unitPrice,
				"discount":   itemDiscount,
				"tax":        itemTax,
				"total":      itemTotal,
			})
		}

		return result, nil
	},
}

var ordersDashboard = arcana.GraphDef{
	Key: "orders_dashboard",
	Deps: []arcana.TableDep{
		{Table: "orders", Columns: []string{"status", "total", "created_at"}},
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		result := arcana.NewResult()

		var totalToday, totalWeek int64
		err := q.QueryRow(ctx, `
			SELECT
				COALESCE(SUM(CASE WHEN created_at >= CURRENT_DATE THEN total ELSE 0 END), 0),
				COALESCE(SUM(CASE WHEN created_at >= CURRENT_DATE - INTERVAL '7 days' THEN total ELSE 0 END), 0)
			FROM orders
			WHERE organization_id = $1 AND cancelled_at IS NULL
		`, orgID).Scan(&totalToday, &totalWeek)
		if err != nil {
			return nil, err
		}

		byStatus := make(map[string]int)
		rows, err := q.Query(ctx, `
			SELECT status, COUNT(*)
			FROM orders
			WHERE organization_id = $1 AND cancelled_at IS NULL
			GROUP BY status
		`, orgID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var st string
			var cnt int
			if err := rows.Scan(&st, &cnt); err != nil {
				return nil, err
			}
			byStatus[st] = cnt
		}

		result.AddRef(arcana.Ref{Table: "orders", ID: "dashboard", Fields: []string{"total_today", "total_week", "by_status"}})
		result.AddRow("orders", "dashboard", map[string]any{
			"total_today": totalToday,
			"total_week":  totalWeek,
			"by_status":   byStatus,
		})

		return result, nil
	},
}
