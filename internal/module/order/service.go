package order

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/bizengine/engine/internal/core/entity"
	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// WarehouseChecker checks stock availability (provided by warehouse module).
type WarehouseChecker interface {
	CheckAvailability(ctx context.Context, orgID uuid.UUID, items []CheckItem) error
}

// CheckItem mirrors warehouse.CheckItem to avoid import.
type CheckItem struct {
	ProductID   uuid.UUID
	WarehouseID uuid.UUID
	Quantity    float64
}

// Service provides order business logic.
type Service struct {
	repo       Repository
	entitySvc  *entity.Service
	eventBus   event.Bus
	warehouse  WarehouseChecker
}

// NewService creates a new order service.
func NewService(repo Repository, entitySvc *entity.Service, eventBus event.Bus, warehouse WarehouseChecker) *Service {
	return &Service{
		repo:      repo,
		entitySvc: entitySvc,
		eventBus:  eventBus,
		warehouse: warehouse,
	}
}

// Create creates a new order with items.
func (s *Service) Create(ctx context.Context, orgID uuid.UUID, input CreateOrderInput, actorID *uuid.UUID) (*Order, error) {
	if len(input.Items) == 0 {
		return nil, errs.NewBadRequest("items are required")
	}

	var order *Order
	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		// Create entity
		e, err := s.entitySvc.Create(ctx, orgID, entity.CreateEntityInput{
			Kind: "order",
			Name: "Order",
		}, actorID)
		if err != nil {
			return err
		}

		// Generate number
		seq, err := s.repo.NextOrderNumber(ctx, tx, orgID)
		if err != nil {
			return err
		}
		number := fmt.Sprintf("ORD-%05d", seq)

		// Build items and calculate totals
		now := time.Now()
		items := make([]OrderItem, len(input.Items))
		var subtotal, totalDiscount, totalTax int64

		for i, item := range input.Items {
			itemTotal := int64(math.Round(float64(item.UnitPrice)*item.Quantity)) - item.Discount + item.Tax
			items[i] = OrderItem{
				ID:          uuid.New(),
				OrderID:     uuid.Nil, // will be set after order creation
				OrganizationID: orgID,
				ProductID:   item.ProductID,
				Name:        "Product", // snapshot — in real scenario, look up product name
				Quantity:    item.Quantity,
				Unit:        "шт",
				UnitPrice:   item.UnitPrice,
				Discount:    item.Discount,
				Tax:         item.Tax,
				Total:       itemTotal,
				SortOrder:   i,
				CreatedAt:   now,
			}
			subtotal += int64(math.Round(float64(item.UnitPrice) * item.Quantity))
			totalDiscount += item.Discount
			totalTax += item.Tax
		}

		order = &Order{
			ID:          uuid.New(),
			OrganizationID: orgID,
			EntityID:    e.ID,
			Number:      number,
			CustomerID:  input.CustomerID,
			Status:      "new",
			Subtotal:    subtotal,
			Discount:    totalDiscount,
			Tax:         totalTax,
			Total:       subtotal - totalDiscount + totalTax,
			Currency:    "RUB",
			Notes:       input.Notes,
			Source:      "manual",
			WarehouseID: input.WarehouseID,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		if err := s.repo.CreateOrder(ctx, tx, order); err != nil {
			return err
		}

		for i := range items {
			items[i].OrderID = order.ID
		}
		if err := s.repo.CreateOrderItems(ctx, tx, items); err != nil {
			return err
		}

		order.Items = items
		return nil
	}); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, order.EntityID, "order.created", actorID, map[string]any{
		"order_id": order.ID,
		"number":   order.Number,
		"total":    order.Total,
	})

	return order, nil
}

// Get returns an order with optional items.
func (s *Service) Get(ctx context.Context, orgID, orderID uuid.UUID, includeItems bool) (*Order, error) {
	o, err := s.repo.GetOrder(ctx, orgID, orderID)
	if err != nil {
		return nil, err
	}
	if includeItems {
		items, err := s.repo.GetOrderItems(ctx, orderID)
		if err != nil {
			return nil, err
		}
		o.Items = items
	}
	return o, nil
}

// List returns orders matching the filter.
func (s *Service) List(ctx context.Context, orgID uuid.UUID, filter OrderFilter) (*types.PageResponse[Order], error) {
	filter.Page.Normalize()

	orders, total, err := s.repo.ListOrders(ctx, orgID, filter)
	if err != nil {
		return nil, err
	}
	if orders == nil {
		orders = []Order{}
	}

	return &types.PageResponse[Order]{
		Items:  orders,
		Total:  total,
		Limit:  filter.Page.Limit,
		Offset: filter.Page.Offset,
	}, nil
}

// Confirm transitions order to confirmed status.
func (s *Service) Confirm(ctx context.Context, orgID, orderID uuid.UUID, actorID *uuid.UUID) (*Order, error) {
	o, err := s.repo.GetOrder(ctx, orgID, orderID)
	if err != nil {
		return nil, err
	}
	if o.Status != "new" {
		return nil, errs.NewConflict("order must be in 'new' status to confirm")
	}

	items, err := s.repo.GetOrderItems(ctx, orderID)
	if err != nil {
		return nil, err
	}

	// Check availability if warehouse is set
	if o.WarehouseID != nil && s.warehouse != nil {
		checkItems := make([]CheckItem, len(items))
		for i, item := range items {
			checkItems[i] = CheckItem{
				ProductID:   item.ProductID,
				WarehouseID: *o.WarehouseID,
				Quantity:    item.Quantity,
			}
		}
		if err := s.warehouse.CheckAvailability(ctx, orgID, checkItems); err != nil {
			return nil, err
		}
	}

	o.Status = "confirmed"
	o.UpdatedAt = time.Now()
	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		return s.repo.UpdateOrder(ctx, tx, o)
	}); err != nil {
		return nil, err
	}

	// Publish enriched event with items for warehouse reservation
	eventItems := make([]map[string]any, len(items))
	for i, item := range items {
		eventItems[i] = map[string]any{
			"product_id": item.ProductID,
			"quantity":   item.Quantity,
			"unit_price": item.UnitPrice,
		}
	}
	eventData := map[string]any{
		"order_id": o.ID,
		"number":   o.Number,
		"items":    eventItems,
		"total":    o.Total,
	}
	if o.WarehouseID != nil {
		eventData["warehouse_id"] = *o.WarehouseID
	}
	s.publishEvent(ctx, orgID, o.EntityID, "order.confirmed", actorID, eventData)

	return o, nil
}

// Pay records payment on an order.
func (s *Service) Pay(ctx context.Context, orgID, orderID uuid.UUID, input PayInput, actorID *uuid.UUID) (*Order, error) {
	o, err := s.repo.GetOrder(ctx, orgID, orderID)
	if err != nil {
		return nil, err
	}
	if o.Status != "confirmed" {
		return nil, errs.NewConflict("order must be in 'confirmed' status to pay")
	}

	now := time.Now()
	o.PaidAt = &now
	o.Status = "paid"
	o.UpdatedAt = now

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		return s.repo.UpdateOrder(ctx, tx, o)
	}); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, o.EntityID, "order.paid", actorID, map[string]any{
		"order_id": o.ID,
		"amount":   input.Amount,
		"method":   input.Method,
	})

	return o, nil
}

// Ship records shipment on an order.
func (s *Service) Ship(ctx context.Context, orgID, orderID uuid.UUID, input ShipInput, actorID *uuid.UUID) (*Order, error) {
	o, err := s.repo.GetOrder(ctx, orgID, orderID)
	if err != nil {
		return nil, err
	}
	if o.Status != "paid" && o.Status != "assembling" {
		return nil, errs.NewConflict("order must be in 'paid' or 'assembling' status to ship")
	}

	now := time.Now()
	o.ShippedAt = &now
	o.Status = "shipped"
	o.UpdatedAt = now

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		return s.repo.UpdateOrder(ctx, tx, o)
	}); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, o.EntityID, "order.shipped", actorID, map[string]any{
		"order_id": o.ID,
		"tracking": input.Tracking,
	})

	return o, nil
}

// Deliver marks order as delivered.
func (s *Service) Deliver(ctx context.Context, orgID, orderID uuid.UUID, actorID *uuid.UUID) (*Order, error) {
	o, err := s.repo.GetOrder(ctx, orgID, orderID)
	if err != nil {
		return nil, err
	}
	if o.Status != "shipped" {
		return nil, errs.NewConflict("order must be in 'shipped' status to deliver")
	}

	now := time.Now()
	o.DeliveredAt = &now
	o.Status = "delivered"
	o.UpdatedAt = now

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		return s.repo.UpdateOrder(ctx, tx, o)
	}); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, o.EntityID, "order.delivered", actorID, map[string]any{
		"order_id": o.ID,
	})

	return o, nil
}

// Cancel cancels an order.
func (s *Service) Cancel(ctx context.Context, orgID, orderID uuid.UUID, reason string, actorID *uuid.UUID) (*Order, error) {
	o, err := s.repo.GetOrder(ctx, orgID, orderID)
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{"new": true, "confirmed": true, "paid": true}
	if !allowed[o.Status] {
		return nil, errs.NewConflict("order can only be cancelled from new, confirmed, or paid status")
	}

	// Fetch items for unreserve event data
	items, err := s.repo.GetOrderItems(ctx, orderID)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	o.CancelledAt = &now
	o.Status = "cancelled"
	o.UpdatedAt = now

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		return s.repo.UpdateOrder(ctx, tx, o)
	}); err != nil {
		return nil, err
	}

	eventItems := make([]map[string]any, len(items))
	for i, item := range items {
		eventItems[i] = map[string]any{
			"product_id": item.ProductID,
			"quantity":   item.Quantity,
		}
	}
	eventData := map[string]any{
		"order_id": o.ID,
		"reason":   reason,
		"items":    eventItems,
	}
	if o.WarehouseID != nil {
		eventData["warehouse_id"] = *o.WarehouseID
	}
	s.publishEvent(ctx, orgID, o.EntityID, "order.cancelled", actorID, eventData)

	return o, nil
}

// Update modifies a draft/new order.
func (s *Service) Update(ctx context.Context, orgID, orderID uuid.UUID, input UpdateOrderInput, actorID *uuid.UUID) (*Order, error) {
	o, err := s.repo.GetOrder(ctx, orgID, orderID)
	if err != nil {
		return nil, err
	}
	if o.Status != "draft" && o.Status != "new" {
		return nil, errs.NewConflict("order can only be updated in draft or new status")
	}

	if input.Notes != nil {
		o.Notes = *input.Notes
	}

	if len(input.Items) > 0 {
		now := time.Now()
		items := make([]OrderItem, len(input.Items))
		var subtotal, totalDiscount, totalTax int64

		for i, item := range input.Items {
			itemTotal := int64(math.Round(float64(item.UnitPrice)*item.Quantity)) - item.Discount + item.Tax
			items[i] = OrderItem{
				ID:          uuid.New(),
				OrderID:     orderID,
				OrganizationID: orgID,
				ProductID:   item.ProductID,
				Name:        "Product",
				Quantity:    item.Quantity,
				Unit:        "шт",
				UnitPrice:   item.UnitPrice,
				Discount:    item.Discount,
				Tax:         item.Tax,
				Total:       itemTotal,
				SortOrder:   i,
				CreatedAt:   now,
			}
			subtotal += int64(math.Round(float64(item.UnitPrice) * item.Quantity))
			totalDiscount += item.Discount
			totalTax += item.Tax
		}

		o.Subtotal = subtotal
		o.Discount = totalDiscount
		o.Tax = totalTax
		o.Total = subtotal - totalDiscount + totalTax
		o.UpdatedAt = now

		if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
			if err := s.repo.UpdateOrderItems(ctx, tx, orderID, items); err != nil {
				return err
			}
			return s.repo.UpdateOrder(ctx, tx, o)
		}); err != nil {
			return nil, err
		}
		o.Items = items
	} else {
		o.UpdatedAt = time.Now()
		if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
			return s.repo.UpdateOrder(ctx, tx, o)
		}); err != nil {
			return nil, err
		}
	}

	return o, nil
}

func (s *Service) updateStatus(ctx context.Context, o *Order, status string, actorID *uuid.UUID) (*Order, error) {
	o.Status = status
	o.UpdatedAt = time.Now()

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		return s.repo.UpdateOrder(ctx, tx, o)
	}); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID(o), o.EntityID, "order."+status, actorID, map[string]any{
		"order_id": o.ID,
		"status":   status,
	})

	return o, nil
}

func orgID(o *Order) uuid.UUID { return o.OrganizationID }

func (s *Service) publishEvent(ctx context.Context, organizationID uuid.UUID, entityID uuid.UUID, eventType string, actorID *uuid.UUID, data map[string]any) {
	ev := types.Event{
		ID:          uuid.New(),
		OrganizationID: organizationID,
		EntityID:    &entityID,
		Type:        eventType,
		ActorID:     actorID,
		Timestamp:   time.Now(),
		Version:     1,
	}
	ev.Data, _ = json.Marshal(data)
	s.eventBus.Publish(ctx, ev)
}
