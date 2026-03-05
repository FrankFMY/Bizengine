package warehouse

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// ReceiveInput is the input for receiving stock.
type ReceiveInput struct {
	ProductID   uuid.UUID `json:"product_id"`
	WarehouseID uuid.UUID `json:"warehouse_id"`
	Quantity    float64   `json:"quantity"`
	Unit        string    `json:"unit"`
	CostPerUnit *int64    `json:"cost_per_unit,omitempty"`
	Reason      string    `json:"reason"`
	ActorID     *uuid.UUID
}

// ShipInput is the input for shipping stock.
type ShipInput struct {
	ProductID     uuid.UUID  `json:"product_id"`
	WarehouseID   uuid.UUID  `json:"warehouse_id"`
	Quantity      float64    `json:"quantity"`
	Unit          string     `json:"unit"`
	ReferenceType *string    `json:"reference_type,omitempty"`
	ReferenceID   *uuid.UUID `json:"reference_id,omitempty"`
	ActorID       *uuid.UUID
}

// TransferInput is the input for transferring stock between warehouses.
type TransferInput struct {
	ProductID       uuid.UUID `json:"product_id"`
	FromWarehouseID uuid.UUID `json:"from_warehouse_id"`
	ToWarehouseID   uuid.UUID `json:"to_warehouse_id"`
	Quantity        float64   `json:"quantity"`
	Unit            string    `json:"unit"`
	ActorID         *uuid.UUID
}

// AdjustInput is the input for stock adjustment (inventory).
type AdjustInput struct {
	ProductID   uuid.UUID `json:"product_id"`
	WarehouseID uuid.UUID `json:"warehouse_id"`
	NewQuantity float64   `json:"new_quantity"`
	Reason      string    `json:"reason"`
	ActorID     *uuid.UUID
}

// ReserveInput is the input for reserving stock.
type ReserveInput struct {
	ProductID   uuid.UUID  `json:"product_id"`
	WarehouseID uuid.UUID  `json:"warehouse_id"`
	Quantity    float64    `json:"quantity"`
	ReferenceID *uuid.UUID `json:"reference_id,omitempty"`
}

// UnreserveInput is the input for unreserving stock.
type UnreserveInput struct {
	ProductID   uuid.UUID  `json:"product_id"`
	WarehouseID uuid.UUID  `json:"warehouse_id"`
	Quantity    float64    `json:"quantity"`
	ReferenceID *uuid.UUID `json:"reference_id,omitempty"`
}

// CheckItem represents an item to check availability for.
type CheckItem struct {
	ProductID   uuid.UUID `json:"product_id"`
	WarehouseID uuid.UUID `json:"warehouse_id"`
	Quantity    float64   `json:"quantity"`
}

// Service provides warehouse business logic.
type Service struct {
	repo     Repository
	eventBus event.Bus
}

// NewService creates a new warehouse service.
func NewService(repo Repository, eventBus event.Bus) *Service {
	return &Service{repo: repo, eventBus: eventBus}
}

// Receive adds stock to a warehouse (incoming goods).
func (s *Service) Receive(ctx context.Context, orgID uuid.UUID, input ReceiveInput) (*MovementResult, error) {
	if input.Quantity <= 0 {
		return nil, errs.NewBadRequest("quantity must be positive")
	}
	if input.Unit == "" {
		input.Unit = "шт"
	}

	var result MovementResult
	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		sl, err := s.getOrCreateStockLevel(ctx, tx, orgID, input.ProductID, input.WarehouseID, input.Unit)
		if err != nil {
			return err
		}

		// Recalculate weighted average cost
		if input.CostPerUnit != nil {
			oldCost := int64(0)
			if sl.CostPerUnit != nil {
				oldCost = *sl.CostPerUnit
			}
			if sl.Quantity == 0 {
				sl.CostPerUnit = input.CostPerUnit
			} else {
				totalCost := int64(math.Round(float64(oldCost)*sl.Quantity + float64(*input.CostPerUnit)*input.Quantity))
				newQty := sl.Quantity + input.Quantity
				avg := int64(math.Round(float64(totalCost) / newQty))
				sl.CostPerUnit = &avg
			}
		}

		sl.Quantity += input.Quantity
		sl.UpdatedAt = time.Now()
		if err := s.repo.UpsertStockLevel(ctx, tx, sl); err != nil {
			return err
		}

		m := &StockMovement{
			ID:          uuid.New(),
			OrganizationID: orgID,
			ProductID:   input.ProductID,
			WarehouseID: input.WarehouseID,
			Type:        "receive",
			Quantity:    input.Quantity,
			Unit:        input.Unit,
			CostPerUnit: input.CostPerUnit,
			Reason:      input.Reason,
			ActorID:     input.ActorID,
			CreatedAt:   time.Now(),
		}
		if err := s.repo.InsertMovement(ctx, tx, m); err != nil {
			return err
		}

		result = MovementResult{
			MovementID:  m.ID,
			NewQuantity: sl.Quantity,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	evData := map[string]any{
		"product_id":   input.ProductID,
		"warehouse_id": input.WarehouseID,
		"quantity":     input.Quantity,
		"new_quantity": result.NewQuantity,
	}
	if input.CostPerUnit != nil {
		evData["total_cost"] = float64(*input.CostPerUnit) * input.Quantity
	}
	s.publishEvent(ctx, orgID, input.ProductID, "warehouse.stock.received", input.ActorID, evData)

	return &result, nil
}

// Ship removes stock from a warehouse (outgoing goods).
func (s *Service) Ship(ctx context.Context, orgID uuid.UUID, input ShipInput) (*MovementResult, error) {
	if input.Quantity <= 0 {
		return nil, errs.NewBadRequest("quantity must be positive")
	}
	if input.Unit == "" {
		input.Unit = "шт"
	}

	var result MovementResult
	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		sl, err := s.repo.GetStockLevel(ctx, orgID, input.ProductID, input.WarehouseID)
		if err != nil {
			return errs.NewNotFound("stock level not found")
		}

		available := sl.Quantity - sl.Reserved
		if available < input.Quantity {
			return errs.NewConflict("insufficient available stock")
		}

		sl.Quantity -= input.Quantity
		if sl.Reserved >= input.Quantity {
			sl.Reserved -= input.Quantity
		}
		sl.UpdatedAt = time.Now()
		if err := s.repo.UpsertStockLevel(ctx, tx, sl); err != nil {
			return err
		}

		m := &StockMovement{
			ID:            uuid.New(),
			OrganizationID:   orgID,
			ProductID:     input.ProductID,
			WarehouseID:   input.WarehouseID,
			Type:          "ship",
			Quantity:      input.Quantity,
			Unit:          input.Unit,
			ReferenceType: input.ReferenceType,
			ReferenceID:   input.ReferenceID,
			ActorID:       input.ActorID,
			CreatedAt:     time.Now(),
		}
		if err := s.repo.InsertMovement(ctx, tx, m); err != nil {
			return err
		}

		result = MovementResult{
			MovementID:  m.ID,
			NewQuantity: sl.Quantity,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, input.ProductID, "warehouse.stock.shipped", input.ActorID, map[string]any{
		"product_id":   input.ProductID,
		"warehouse_id": input.WarehouseID,
		"quantity":     input.Quantity,
		"new_quantity": result.NewQuantity,
	})

	// Check low stock
	s.checkLowStock(ctx, orgID, input.ProductID, input.WarehouseID, input.ActorID)

	return &result, nil
}

// Transfer moves stock between warehouses atomically.
func (s *Service) Transfer(ctx context.Context, orgID uuid.UUID, input TransferInput) (*MovementResult, error) {
	if input.Quantity <= 0 {
		return nil, errs.NewBadRequest("quantity must be positive")
	}
	if input.FromWarehouseID == input.ToWarehouseID {
		return nil, errs.NewBadRequest("source and destination warehouses must differ")
	}
	if input.Unit == "" {
		input.Unit = "шт"
	}

	var result MovementResult
	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		// Source
		src, err := s.repo.GetStockLevel(ctx, orgID, input.ProductID, input.FromWarehouseID)
		if err != nil {
			return errs.NewNotFound("source stock level not found")
		}
		available := src.Quantity - src.Reserved
		if available < input.Quantity {
			return errs.NewConflict("insufficient available stock in source warehouse")
		}

		src.Quantity -= input.Quantity
		src.UpdatedAt = time.Now()
		if err := s.repo.UpsertStockLevel(ctx, tx, src); err != nil {
			return err
		}

		// Destination
		dst, err := s.getOrCreateStockLevel(ctx, tx, orgID, input.ProductID, input.ToWarehouseID, input.Unit)
		if err != nil {
			return err
		}
		dst.Quantity += input.Quantity
		dst.UpdatedAt = time.Now()
		if err := s.repo.UpsertStockLevel(ctx, tx, dst); err != nil {
			return err
		}

		m := &StockMovement{
			ID:              uuid.New(),
			OrganizationID:     orgID,
			ProductID:       input.ProductID,
			WarehouseID:     input.FromWarehouseID,
			Type:            "transfer",
			Quantity:        input.Quantity,
			Unit:            input.Unit,
			DestWarehouseID: &input.ToWarehouseID,
			ActorID:         input.ActorID,
			CreatedAt:       time.Now(),
		}
		if err := s.repo.InsertMovement(ctx, tx, m); err != nil {
			return err
		}

		result = MovementResult{MovementID: m.ID}
		return nil
	}); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, input.ProductID, "warehouse.stock.transferred", input.ActorID, map[string]any{
		"product_id":       input.ProductID,
		"from_warehouse":   input.FromWarehouseID,
		"to_warehouse":     input.ToWarehouseID,
		"quantity":         input.Quantity,
	})

	return &result, nil
}

// Adjust sets stock to an absolute value (inventory count).
func (s *Service) Adjust(ctx context.Context, orgID uuid.UUID, input AdjustInput) (*MovementResult, error) {
	if input.NewQuantity < 0 {
		return nil, errs.NewBadRequest("new_quantity cannot be negative")
	}
	if input.Reason == "" {
		return nil, errs.NewBadRequest("reason is required for adjustment")
	}

	var result MovementResult
	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		sl, err := s.getOrCreateStockLevel(ctx, tx, orgID, input.ProductID, input.WarehouseID, "шт")
		if err != nil {
			return err
		}

		oldQty := sl.Quantity
		diff := input.NewQuantity - oldQty

		sl.Quantity = input.NewQuantity
		sl.UpdatedAt = time.Now()
		if err := s.repo.UpsertStockLevel(ctx, tx, sl); err != nil {
			return err
		}

		m := &StockMovement{
			ID:          uuid.New(),
			OrganizationID: orgID,
			ProductID:   input.ProductID,
			WarehouseID: input.WarehouseID,
			Type:        "adjust",
			Quantity:    diff,
			Unit:        sl.Unit,
			Reason:      input.Reason,
			ActorID:     input.ActorID,
			CreatedAt:   time.Now(),
		}
		if err := s.repo.InsertMovement(ctx, tx, m); err != nil {
			return err
		}

		result = MovementResult{
			MovementID:  m.ID,
			OldQuantity: oldQty,
			NewQuantity: input.NewQuantity,
		}
		return nil
	}); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, input.ProductID, "warehouse.stock.adjusted", input.ActorID, map[string]any{
		"product_id":   input.ProductID,
		"warehouse_id": input.WarehouseID,
		"old_quantity": result.OldQuantity,
		"new_quantity": result.NewQuantity,
	})

	return &result, nil
}

// Reserve increases reserved stock for a product.
func (s *Service) Reserve(ctx context.Context, orgID uuid.UUID, input ReserveInput) error {
	if input.Quantity <= 0 {
		return errs.NewBadRequest("quantity must be positive")
	}

	return s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		sl, err := s.repo.GetStockLevel(ctx, orgID, input.ProductID, input.WarehouseID)
		if err != nil {
			return errs.NewNotFound("stock level not found")
		}

		available := sl.Quantity - sl.Reserved
		if available < input.Quantity {
			return errs.NewConflict("insufficient stock to reserve")
		}

		sl.Reserved += input.Quantity
		sl.UpdatedAt = time.Now()
		if err := s.repo.UpsertStockLevel(ctx, tx, sl); err != nil {
			return err
		}

		s.publishEvent(ctx, orgID, input.ProductID, "warehouse.stock.reserved", nil, map[string]any{
			"product_id":   input.ProductID,
			"warehouse_id": input.WarehouseID,
			"quantity":     input.Quantity,
			"reference_id": input.ReferenceID,
		})

		return nil
	})
}

// Unreserve decreases reserved stock for a product.
func (s *Service) Unreserve(ctx context.Context, orgID uuid.UUID, input UnreserveInput) error {
	if input.Quantity <= 0 {
		return errs.NewBadRequest("quantity must be positive")
	}

	return s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		sl, err := s.repo.GetStockLevel(ctx, orgID, input.ProductID, input.WarehouseID)
		if err != nil {
			return errs.NewNotFound("stock level not found")
		}

		if sl.Reserved < input.Quantity {
			sl.Reserved = 0
		} else {
			sl.Reserved -= input.Quantity
		}
		sl.UpdatedAt = time.Now()
		if err := s.repo.UpsertStockLevel(ctx, tx, sl); err != nil {
			return err
		}

		s.publishEvent(ctx, orgID, input.ProductID, "warehouse.stock.unreserved", nil, map[string]any{
			"product_id":   input.ProductID,
			"warehouse_id": input.WarehouseID,
			"quantity":     input.Quantity,
			"reference_id": input.ReferenceID,
		})

		return nil
	})
}

// GetStockLevel returns the stock level for a product in a warehouse.
func (s *Service) GetStockLevel(ctx context.Context, orgID, productID, warehouseID uuid.UUID) (*StockLevel, error) {
	sl, err := s.repo.GetStockLevel(ctx, orgID, productID, warehouseID)
	if err != nil {
		return nil, err
	}
	sl.Available = sl.Quantity - sl.Reserved
	return sl, nil
}

// ListStock returns stock levels for a warehouse.
func (s *Service) ListStock(ctx context.Context, orgID, warehouseID uuid.UUID, filter StockFilter) (*types.PageResponse[StockLevel], error) {
	filter.Page.Normalize()

	items, total, err := s.repo.ListStock(ctx, orgID, warehouseID, filter)
	if err != nil {
		return nil, err
	}

	for i := range items {
		items[i].Available = items[i].Quantity - items[i].Reserved
	}

	if items == nil {
		items = []StockLevel{}
	}

	return &types.PageResponse[StockLevel]{
		Items:  items,
		Total:  total,
		Limit:  filter.Page.Limit,
		Offset: filter.Page.Offset,
	}, nil
}

// GetLowStock returns stock levels where available <= min_quantity.
func (s *Service) GetLowStock(ctx context.Context, orgID uuid.UUID) ([]StockLevel, error) {
	items, err := s.repo.GetLowStock(ctx, orgID)
	if err != nil {
		return nil, err
	}
	for i := range items {
		items[i].Available = items[i].Quantity - items[i].Reserved
	}
	return items, nil
}

// GetMovements returns stock movements matching the filter.
func (s *Service) GetMovements(ctx context.Context, orgID uuid.UUID, filter MovementFilter) (*types.PageResponse[StockMovement], error) {
	filter.Page.Normalize()

	items, total, err := s.repo.ListMovements(ctx, orgID, filter)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []StockMovement{}
	}

	return &types.PageResponse[StockMovement]{
		Items:  items,
		Total:  total,
		Limit:  filter.Page.Limit,
		Offset: filter.Page.Offset,
	}, nil
}

// CheckAvailability checks if all items have sufficient stock.
func (s *Service) CheckAvailability(ctx context.Context, orgID uuid.UUID, items []CheckItem) error {
	for _, item := range items {
		sl, err := s.repo.GetStockLevel(ctx, orgID, item.ProductID, item.WarehouseID)
		if err != nil {
			return errs.NewConflict("stock not found for product " + item.ProductID.String())
		}
		available := sl.Quantity - sl.Reserved
		if available < item.Quantity {
			return errs.NewConflict("insufficient stock for product " + item.ProductID.String())
		}
	}
	return nil
}

// --- helpers ---

func (s *Service) getOrCreateStockLevel(ctx context.Context, tx pgx.Tx, orgID, productID, warehouseID uuid.UUID, unit string) (*StockLevel, error) {
	sl, err := s.repo.GetStockLevel(ctx, orgID, productID, warehouseID)
	if err != nil {
		sl = &StockLevel{
			OrganizationID: orgID,
			ProductID:   productID,
			WarehouseID: warehouseID,
			Unit:        unit,
			UpdatedAt:   time.Now(),
		}
	}
	return sl, nil
}

func (s *Service) publishEvent(ctx context.Context, orgID uuid.UUID, productID uuid.UUID, eventType string, actorID *uuid.UUID, data map[string]any) {
	ev := types.Event{
		ID:          uuid.New(),
		OrganizationID: orgID,
		EntityID:    &productID,
		Type:        eventType,
		ActorID:     actorID,
		Timestamp:   time.Now(),
		Version:     1,
	}
	ev.Data, _ = json.Marshal(data)
	s.eventBus.Publish(ctx, ev)
}

func (s *Service) checkLowStock(ctx context.Context, orgID uuid.UUID, productID, warehouseID uuid.UUID, actorID *uuid.UUID) {
	sl, err := s.repo.GetStockLevel(ctx, orgID, productID, warehouseID)
	if err != nil {
		return
	}
	available := sl.Quantity - sl.Reserved
	if sl.MinQuantity > 0 && available <= sl.MinQuantity {
		s.publishEvent(ctx, orgID, productID, "warehouse.stock.low", actorID, map[string]any{
			"product_id":   productID,
			"warehouse_id": warehouseID,
			"available":    available,
			"min_quantity": sl.MinQuantity,
		})
	}
}
