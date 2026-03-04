package defs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/views"
)

// OrdersList returns a paginated list of orders.
var OrdersList = &views.ViewDef{
	Key: "orders_list",
	Tables: []views.TableDep{
		{Table: "orders", Columns: []string{"status", "total", "updated_at"}},
		{Table: "entities", Columns: []string{"name"}, Filter: "kind=customer"},
	},
	ParamSchema: map[string]string{
		"status": "string",
	},
	Factory: ordersListFactory,
}

func ordersListFactory(ctx context.Context, pool *pgxpool.Pool, wsID uuid.UUID, params map[string]any) (*views.ViewResult, error) {
	limit := intParam(params, "limit", 50)
	offset := intParam(params, "offset", 0)
	status, _ := params["status"].(string)

	query := `
		SELECT o.id, o.number, COALESCE(c.name,'') AS customer_name,
		       o.status, o.total, o.created_at, COUNT(*) OVER() AS total_count
		FROM orders o
		LEFT JOIN entities c ON c.id = o.customer_id AND c.workspace_id = $1
		WHERE o.workspace_id = $1
	`
	args := []any{wsID}
	argIdx := 2

	if status != "" {
		query += fmt.Sprintf(` AND o.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY o.created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]views.DataRef, 0)
	tables := map[string]map[string]any{"orders": {}}
	var total int

	for rows.Next() {
		var id, number, customerName, st string
		var totalAmt int64
		var createdAt any
		var cnt int

		if err := rows.Scan(&id, &number, &customerName, &st, &totalAmt, &createdAt, &cnt); err != nil {
			return nil, err
		}
		total = cnt

		refs = append(refs, views.DataRef{Table: "orders", ID: id, Fields: []string{"number", "customer_name", "status", "total", "created_at"}})
		tables["orders"][id] = map[string]any{
			"id":            id,
			"number":        number,
			"customer_name": customerName,
			"status":        st,
			"total":         totalAmt,
			"created_at":    createdAt,
		}
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: int64(total)}, nil
}

// OrderDetail returns full details for a single order.
var OrderDetail = &views.ViewDef{
	Key: "order_detail",
	Tables: []views.TableDep{
		{Table: "orders", Columns: []string{"status", "total", "updated_at"}},
		{Table: "order_items", Columns: []string{"quantity", "unit_price", "total"}},
		{Table: "entities", Columns: []string{"name"}},
	},
	ParamSchema: map[string]string{
		"order_id": "uuid",
	},
	Factory: orderDetailFactory,
}

func orderDetailFactory(ctx context.Context, pool *pgxpool.Pool, wsID uuid.UUID, params map[string]any) (*views.ViewResult, error) {
	orderID, _ := params["order_id"].(string)

	tables := map[string]map[string]any{"orders": {}, "order_items": {}}
	refs := make([]views.DataRef, 0, 1)

	var id, number, status string
	var customerName *string
	var subtotal, discount, tax, total int64
	var paidAt, shippedAt, deliveredAt, createdAt any

	err := pool.QueryRow(ctx, `
		SELECT o.id, o.number, c.name, o.status, o.subtotal, o.discount, o.tax, o.total,
		       o.paid_at, o.shipped_at, o.delivered_at, o.created_at
		FROM orders o
		LEFT JOIN entities c ON c.id = o.customer_id AND c.workspace_id = $1
		WHERE o.workspace_id = $1 AND o.id = $2
	`, wsID, orderID).Scan(&id, &number, &customerName, &status, &subtotal, &discount, &tax, &total,
		&paidAt, &shippedAt, &deliveredAt, &createdAt)
	if err != nil {
		return nil, err
	}

	refs = append(refs, views.DataRef{Table: "orders", ID: id, Fields: []string{"number", "customer_name", "status", "subtotal", "discount", "tax", "total"}})
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
	tables["orders"][id] = orderRow

	// Order items
	itemRows, err := pool.Query(ctx, `
		SELECT oi.id, oi.name, oi.sku, oi.quantity, oi.unit_price, oi.discount, oi.tax, oi.total
		FROM order_items oi
		WHERE oi.workspace_id = $1 AND oi.order_id = $2
		ORDER BY oi.sort_order
	`, wsID, orderID)
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

		refs = append(refs, views.DataRef{Table: "order_items", ID: itemID, Fields: []string{"name", "quantity", "unit_price", "total"}})
		tables["order_items"][itemID] = map[string]any{
			"id":         itemID,
			"name":       itemName,
			"sku":        sku,
			"quantity":   qty,
			"unit_price": unitPrice,
			"discount":   itemDiscount,
			"tax":        itemTax,
			"total":      itemTotal,
		}
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}

// OrdersDashboard returns order summary stats.
var OrdersDashboard = &views.ViewDef{
	Key: "orders_dashboard",
	Tables: []views.TableDep{
		{Table: "orders", Columns: []string{"status", "total", "created_at"}},
	},
	Factory: ordersDashboardFactory,
}

func ordersDashboardFactory(ctx context.Context, pool *pgxpool.Pool, wsID uuid.UUID, _ map[string]any) (*views.ViewResult, error) {
	tables := map[string]map[string]any{"orders": {}}

	var totalToday, totalWeek int64
	var byStatus map[string]int

	row := pool.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN created_at >= CURRENT_DATE THEN total ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN created_at >= CURRENT_DATE - INTERVAL '7 days' THEN total ELSE 0 END), 0)
		FROM orders
		WHERE workspace_id = $1 AND cancelled_at IS NULL
	`, wsID)
	if err := row.Scan(&totalToday, &totalWeek); err != nil {
		return nil, err
	}

	byStatus = make(map[string]int)
	rows, err := pool.Query(ctx, `
		SELECT status, COUNT(*)
		FROM orders
		WHERE workspace_id = $1 AND cancelled_at IS NULL
		GROUP BY status
	`, wsID)
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

	refs := []views.DataRef{{Table: "orders", ID: "dashboard", Fields: []string{"total_today", "total_week", "by_status"}}}
	tables["orders"]["dashboard"] = map[string]any{
		"total_today": totalToday,
		"total_week":  totalWeek,
		"by_status":   byStatus,
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}
