// Package order provides the order management module.
package order

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/bizengine/engine/pkg/types"
)

// Repository defines order-specific storage operations.
type Repository interface {
	// CreateOrder inserts an order and its items in a transaction.
	CreateOrder(ctx context.Context, tx pgx.Tx, o *Order) error
	CreateOrderItems(ctx context.Context, tx pgx.Tx, items []OrderItem) error

	// GetOrder returns an order by ID within an organization.
	GetOrder(ctx context.Context, orgID, orderID uuid.UUID) (*Order, error)

	// GetOrderItems returns items for an order.
	GetOrderItems(ctx context.Context, orderID uuid.UUID) ([]OrderItem, error)

	// ListOrders returns orders matching the filter.
	ListOrders(ctx context.Context, orgID uuid.UUID, filter OrderFilter) ([]Order, int, error)

	// UpdateOrder updates order fields.
	UpdateOrder(ctx context.Context, tx pgx.Tx, o *Order) error

	// UpdateOrderItems replaces order items (for draft orders).
	UpdateOrderItems(ctx context.Context, tx pgx.Tx, orderID uuid.UUID, items []OrderItem) error

	// NextOrderNumber atomically generates the next order number for an organization.
	NextOrderNumber(ctx context.Context, tx pgx.Tx, orgID uuid.UUID) (int64, error)

	// Refunds
	CreateRefund(ctx context.Context, tx pgx.Tx, r *Refund) error
	CreateRefundItems(ctx context.Context, tx pgx.Tx, items []RefundItem) error
	GetRefund(ctx context.Context, orgID, refundID uuid.UUID) (*Refund, error)
	GetRefundItems(ctx context.Context, refundID uuid.UUID) ([]RefundItem, error)
	ListRefunds(ctx context.Context, orgID uuid.UUID, orderID *uuid.UUID, page types.PageRequest) ([]Refund, int, error)

	// WithTx executes fn within a transaction.
	WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Order represents an order record.
type Order struct {
	ID             uuid.UUID   `json:"id"`
	OrganizationID uuid.UUID   `json:"organization_id"`
	EntityID       uuid.UUID   `json:"entity_id"`
	Number         string      `json:"number"`
	CustomerID     *uuid.UUID  `json:"customer_id,omitempty"`
	Status         string      `json:"status"`
	Subtotal       int64       `json:"subtotal"`
	Discount       int64       `json:"discount"`
	Tax            int64       `json:"tax"`
	Total          int64       `json:"total"`
	Currency       string      `json:"currency"`
	Notes          string      `json:"notes"`
	Source         string      `json:"source"`
	WarehouseID    *uuid.UUID  `json:"warehouse_id,omitempty"`
	AssignedTo     *uuid.UUID  `json:"assigned_to,omitempty"`
	PaidAt         *time.Time  `json:"paid_at,omitempty"`
	ShippedAt      *time.Time  `json:"shipped_at,omitempty"`
	DeliveredAt    *time.Time  `json:"delivered_at,omitempty"`
	CancelledAt    *time.Time  `json:"cancelled_at,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
	Items          []OrderItem `json:"items,omitempty"`
}

// OrderItem represents a line item in an order.
type OrderItem struct {
	ID             uuid.UUID `json:"id"`
	OrderID        uuid.UUID `json:"order_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	ProductID      uuid.UUID `json:"product_id"`
	Name           string    `json:"name"`
	SKU            string    `json:"sku"`
	Quantity       float64   `json:"quantity"`
	Unit           string    `json:"unit"`
	UnitPrice      int64     `json:"unit_price"`
	Discount       int64     `json:"discount"`
	Tax            int64     `json:"tax"`
	Total          int64     `json:"total"`
	SortOrder      int       `json:"sort_order"`
	CreatedAt      time.Time `json:"created_at"`
}

// OrderFilter defines order listing parameters.
type OrderFilter struct {
	Status     *string
	CustomerID *uuid.UUID
	Search     *string
	Since      *time.Time
	Until      *time.Time
	Page       types.PageRequest
}

// CreateOrderInput is the input for creating an order.
type CreateOrderInput struct {
	CustomerID  *uuid.UUID        `json:"customer_id,omitempty"`
	WarehouseID *uuid.UUID        `json:"warehouse_id,omitempty"`
	Items       []CreateItemInput `json:"items"`
	Notes       string            `json:"notes"`
}

// CreateItemInput is a line item in a create order request.
type CreateItemInput struct {
	ProductID uuid.UUID `json:"product_id"`
	Quantity  float64   `json:"quantity"`
	UnitPrice int64     `json:"unit_price"`
	Discount  int64     `json:"discount"`
	Tax       int64     `json:"tax"`
}

// UpdateOrderInput is the input for updating a draft order.
type UpdateOrderInput struct {
	Items []CreateItemInput `json:"items,omitempty"`
	Notes *string           `json:"notes,omitempty"`
}

// PayInput is the input for recording payment.
type PayInput struct {
	Amount int64  `json:"amount"`
	Method string `json:"method"`
}

// ShipInput is the input for recording shipment.
type ShipInput struct {
	Tracking string `json:"tracking"`
}

// Refund represents a refund record.
type Refund struct {
	ID             uuid.UUID    `json:"id"`
	OrganizationID uuid.UUID    `json:"organization_id"`
	OrderID        uuid.UUID    `json:"order_id"`
	Status         string       `json:"status"`
	Total          int64        `json:"total"`
	Reason         string       `json:"reason"`
	RefundMethod   string       `json:"refund_method"`
	Ver            int          `json:"ver"`
	Upd            time.Time    `json:"upd"`
	Iat            time.Time    `json:"iat"`
	Items          []RefundItem `json:"items,omitempty"`
}

// RefundItem represents a line item in a refund.
type RefundItem struct {
	ID          uuid.UUID `json:"id"`
	RefundID    uuid.UUID `json:"refund_id"`
	OrderItemID uuid.UUID `json:"order_item_id"`
	Quantity    int       `json:"quantity"`
	UnitPrice   int64     `json:"unit_price"`
	Total       int64     `json:"total"`
	Reason      string    `json:"reason"`
}

// CreateRefundInput is the input for creating a refund.
type CreateRefundInput struct {
	Items        []RefundItemInput `json:"items"`
	RefundMethod string            `json:"refund_method"`
}

// RefundItemInput is a line item in a create refund request.
type RefundItemInput struct {
	OrderItemID uuid.UUID `json:"order_item_id"`
	Quantity    int       `json:"quantity"`
	Reason      string    `json:"reason"`
}
