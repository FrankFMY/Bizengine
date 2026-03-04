package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/module/order"
	"github.com/bizengine/engine/pkg/errs"
)

// OrderRepo implements order.Repository using PostgreSQL.
type OrderRepo struct {
	pool *pgxpool.Pool
}

// NewOrderRepo creates a new OrderRepo.
func NewOrderRepo(pool *pgxpool.Pool) *OrderRepo {
	return &OrderRepo{pool: pool}
}

// CreateOrder inserts an order record.
func (r *OrderRepo) CreateOrder(ctx context.Context, tx pgx.Tx, o *order.Order) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO orders (id, workspace_id, entity_id, number, customer_id, status, subtotal, discount, tax, total, currency, notes, source, warehouse_id, assigned_to, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)`,
		o.ID, o.WorkspaceID, o.EntityID, o.Number, o.CustomerID, o.Status,
		o.Subtotal, o.Discount, o.Tax, o.Total, o.Currency, o.Notes, o.Source,
		o.WarehouseID, o.AssignedTo, o.CreatedAt, o.UpdatedAt,
	)
	return err
}

// CreateOrderItems inserts order item records.
func (r *OrderRepo) CreateOrderItems(ctx context.Context, tx pgx.Tx, items []order.OrderItem) error {
	for _, item := range items {
		if _, err := tx.Exec(ctx,
			`INSERT INTO order_items (id, order_id, workspace_id, product_id, name, sku, quantity, unit, unit_price, discount, tax, total, sort_order, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
			item.ID, item.OrderID, item.WorkspaceID, item.ProductID, item.Name, item.SKU,
			item.Quantity, item.Unit, item.UnitPrice, item.Discount, item.Tax, item.Total,
			item.SortOrder, item.CreatedAt,
		); err != nil {
			return err
		}
	}
	return nil
}

// GetOrder returns an order by ID.
func (r *OrderRepo) GetOrder(ctx context.Context, wsID, orderID uuid.UUID) (*order.Order, error) {
	var o order.Order
	err := r.pool.QueryRow(ctx,
		`SELECT id, workspace_id, entity_id, number, customer_id, status, subtotal, discount, tax, total, currency, notes, source,
		        warehouse_id, assigned_to, paid_at, shipped_at, delivered_at, cancelled_at, created_at, updated_at
		 FROM orders WHERE workspace_id = $1 AND id = $2`,
		wsID, orderID,
	).Scan(&o.ID, &o.WorkspaceID, &o.EntityID, &o.Number, &o.CustomerID, &o.Status,
		&o.Subtotal, &o.Discount, &o.Tax, &o.Total, &o.Currency, &o.Notes, &o.Source,
		&o.WarehouseID, &o.AssignedTo, &o.PaidAt, &o.ShippedAt, &o.DeliveredAt, &o.CancelledAt,
		&o.CreatedAt, &o.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("order not found")
		}
		return nil, err
	}
	return &o, nil
}

// GetOrderItems returns items for an order.
func (r *OrderRepo) GetOrderItems(ctx context.Context, orderID uuid.UUID) ([]order.OrderItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, order_id, workspace_id, product_id, name, sku, quantity, unit, unit_price, discount, tax, total, sort_order, created_at
		 FROM order_items WHERE order_id = $1 ORDER BY sort_order`,
		orderID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []order.OrderItem
	for rows.Next() {
		var item order.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.WorkspaceID, &item.ProductID, &item.Name, &item.SKU,
			&item.Quantity, &item.Unit, &item.UnitPrice, &item.Discount, &item.Tax, &item.Total,
			&item.SortOrder, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

// ListOrders returns orders matching the filter.
func (r *OrderRepo) ListOrders(ctx context.Context, wsID uuid.UUID, filter order.OrderFilter) ([]order.Order, int, error) {
	filter.Page.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("workspace_id = $%d", argIdx))
	args = append(args, wsID)
	argIdx++

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.CustomerID != nil {
		conditions = append(conditions, fmt.Sprintf("customer_id = $%d", argIdx))
		args = append(args, *filter.CustomerID)
		argIdx++
	}
	if filter.Search != nil && *filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("number ILIKE '%%' || $%d || '%%'", argIdx))
		args = append(args, *filter.Search)
		argIdx++
	}
	if filter.Since != nil {
		conditions = append(conditions, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *filter.Since)
		argIdx++
	}
	if filter.Until != nil {
		conditions = append(conditions, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *filter.Until)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM orders WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT id, workspace_id, entity_id, number, customer_id, status, subtotal, discount, tax, total, currency, notes, source,
		        warehouse_id, assigned_to, paid_at, shipped_at, delivered_at, cancelled_at, created_at, updated_at
		 FROM orders WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, filter.Page.Limit, filter.Page.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var orders []order.Order
	for rows.Next() {
		var o order.Order
		if err := rows.Scan(&o.ID, &o.WorkspaceID, &o.EntityID, &o.Number, &o.CustomerID, &o.Status,
			&o.Subtotal, &o.Discount, &o.Tax, &o.Total, &o.Currency, &o.Notes, &o.Source,
			&o.WarehouseID, &o.AssignedTo, &o.PaidAt, &o.ShippedAt, &o.DeliveredAt, &o.CancelledAt,
			&o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, 0, err
		}
		orders = append(orders, o)
	}
	return orders, total, rows.Err()
}

// UpdateOrder updates order fields.
func (r *OrderRepo) UpdateOrder(ctx context.Context, tx pgx.Tx, o *order.Order) error {
	_, err := tx.Exec(ctx,
		`UPDATE orders SET status = $2, subtotal = $3, discount = $4, tax = $5, total = $6, notes = $7,
		        paid_at = $8, shipped_at = $9, delivered_at = $10, cancelled_at = $11, updated_at = $12
		 WHERE id = $1`,
		o.ID, o.Status, o.Subtotal, o.Discount, o.Tax, o.Total, o.Notes,
		o.PaidAt, o.ShippedAt, o.DeliveredAt, o.CancelledAt, o.UpdatedAt,
	)
	return err
}

// UpdateOrderItems replaces order items.
func (r *OrderRepo) UpdateOrderItems(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, items []order.OrderItem) error {
	if _, err := tx.Exec(ctx, "DELETE FROM order_items WHERE order_id = $1", orderID); err != nil {
		return err
	}
	return r.CreateOrderItems(ctx, tx, items)
}

// NextOrderNumber atomically generates the next order number.
func (r *OrderRepo) NextOrderNumber(ctx context.Context, tx pgx.Tx, wsID uuid.UUID) (int64, error) {
	// Ensure sequence row exists
	_, err := tx.Exec(ctx,
		`INSERT INTO order_number_sequences (workspace_id, last_number) VALUES ($1, 0)
		 ON CONFLICT (workspace_id) DO NOTHING`,
		wsID,
	)
	if err != nil {
		return 0, err
	}

	var num int64
	err = tx.QueryRow(ctx,
		`UPDATE order_number_sequences SET last_number = last_number + 1
		 WHERE workspace_id = $1 RETURNING last_number`,
		wsID,
	).Scan(&num)
	return num, err
}

// WithTx executes fn within a transaction.
func (r *OrderRepo) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
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
