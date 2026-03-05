package catalog

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/core/entity"
	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// Product represents a product entity with its components.
type Product struct {
	types.Entity
	Price        json.RawMessage `json:"price,omitempty"`
	Barcode      json.RawMessage `json:"barcode,omitempty"`
	Media        json.RawMessage `json:"media,omitempty"`
	Attributes   json.RawMessage `json:"attributes,omitempty"`
	PricingRules json.RawMessage `json:"pricing_rules,omitempty"`
}

// Category represents a category entity.
type Category struct {
	types.Entity
}

// CreateProductInput is the input for creating a product.
type CreateProductInput struct {
	Name         string          `json:"name"`
	SKU          string          `json:"sku"`
	CategoryID   *uuid.UUID      `json:"category_id,omitempty"`
	Price        json.RawMessage `json:"price"`
	Barcode      json.RawMessage `json:"barcode,omitempty"`
	Attributes   json.RawMessage `json:"attributes,omitempty"`
	Media        json.RawMessage `json:"media,omitempty"`
	PricingRules json.RawMessage `json:"pricing_rules,omitempty"`
}

// UpdateProductInput is the input for updating a product.
type UpdateProductInput struct {
	Name         *string         `json:"name,omitempty"`
	CategoryID   *uuid.UUID      `json:"category_id,omitempty"`
	Price        json.RawMessage `json:"price,omitempty"`
	Barcode      json.RawMessage `json:"barcode,omitempty"`
	Attributes   json.RawMessage `json:"attributes,omitempty"`
	Media        json.RawMessage `json:"media,omitempty"`
	PricingRules json.RawMessage `json:"pricing_rules,omitempty"`
}

// ProductFilter defines product listing parameters.
type ProductFilter struct {
	CategoryID *uuid.UUID
	Search     *string
	InStock    *bool
	MinPrice   *int64
	MaxPrice   *int64
	Status     *string
	Page       types.PageRequest
}

// CreateCategoryInput is the input for creating a category.
type CreateCategoryInput struct {
	Name     string     `json:"name"`
	ParentID *uuid.UUID `json:"parent_id,omitempty"`
}

// Service provides catalog business logic.
type Service struct {
	entitySvc *entity.Service
	repo      Repository
	eventBus  event.Bus
}

// NewService creates a new catalog service.
func NewService(entitySvc *entity.Service, repo Repository, eventBus event.Bus) *Service {
	return &Service{
		entitySvc: entitySvc,
		repo:      repo,
		eventBus:  eventBus,
	}
}

// CreateProduct creates a product entity with its components.
func (s *Service) CreateProduct(ctx context.Context, orgID uuid.UUID, input CreateProductInput, actorID *uuid.UUID) (*Product, error) {
	if input.Name == "" {
		return nil, errs.NewBadRequest("name is required")
	}
	if input.Price == nil {
		return nil, errs.NewBadRequest("price is required")
	}

	// Check SKU uniqueness
	if input.SKU != "" {
		existing, err := s.repo.FindBySKU(ctx, orgID, input.SKU)
		if err == nil && existing != nil {
			return nil, errs.NewConflict("SKU already exists: " + input.SKU)
		}
	}

	// Build components map
	components := map[string]json.RawMessage{
		"price": input.Price,
	}
	if input.Barcode == nil && input.SKU != "" {
		input.Barcode, _ = json.Marshal(map[string]string{"internal": input.SKU})
	}
	if input.Barcode != nil {
		components["barcode"] = input.Barcode
	}
	if input.Attributes != nil {
		components["attributes"] = input.Attributes
	}
	if input.Media != nil {
		components["media"] = input.Media
	}
	if input.PricingRules != nil {
		components["pricing_rules"] = input.PricingRules
	}

	// Create entity + components in a single transaction
	e, err := s.entitySvc.CreateWithComponents(ctx, orgID, entity.CreateEntityInput{
		Kind:     "product",
		Name:     input.Name,
		ParentID: input.CategoryID,
	}, components, actorID)
	if err != nil {
		return nil, err
	}

	// Publish catalog-specific event
	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EntityID:       &e.ID,
		Type:           "catalog.product.created",
		ActorID:        actorID,
		Timestamp:      time.Now(),
		Version:        1,
	}
	ev.Data, _ = json.Marshal(map[string]any{
		"id":          e.ID,
		"name":        input.Name,
		"sku":         input.SKU,
		"category_id": input.CategoryID,
	})
	s.eventBus.Publish(ctx, ev)

	return entityToProduct(e), nil
}

// GetProduct returns a product with all its components.
func (s *Service) GetProduct(ctx context.Context, orgID uuid.UUID, productID uuid.UUID) (*Product, error) {
	e, err := s.entitySvc.Get(ctx, orgID, productID, true)
	if err != nil {
		return nil, err
	}
	if e.Kind != "product" {
		return nil, errs.NewNotFound("product not found")
	}
	return entityToProduct(e), nil
}

// ListProducts returns products matching the filter.
func (s *Service) ListProducts(ctx context.Context, orgID uuid.UUID, filter ProductFilter) (*types.PageResponse[Product], error) {
	filter.Page.Normalize()

	products, total, err := s.repo.ListProductsWithComponents(ctx, orgID, filter)
	if err != nil {
		return nil, err
	}

	if products == nil {
		products = []Product{}
	}

	return &types.PageResponse[Product]{
		Items:  products,
		Total:  total,
		Limit:  filter.Page.Limit,
		Offset: filter.Page.Offset,
	}, nil
}

// UpdateProduct updates a product and its components.
func (s *Service) UpdateProduct(ctx context.Context, orgID uuid.UUID, productID uuid.UUID, input UpdateProductInput, actorID *uuid.UUID) (*Product, error) {
	e, err := s.entitySvc.Get(ctx, orgID, productID, false)
	if err != nil {
		return nil, err
	}
	if e.Kind != "product" {
		return nil, errs.NewNotFound("product not found")
	}

	// Update entity fields
	updateInput := entity.UpdateEntityInput{}
	if input.Name != nil {
		updateInput.Name = input.Name
	}
	if input.CategoryID != nil {
		updateInput.ParentID = input.CategoryID
	}
	if input.Name != nil || input.CategoryID != nil {
		if _, err := s.entitySvc.Update(ctx, orgID, productID, updateInput, actorID); err != nil {
			return nil, err
		}
	}

	// Update components
	if input.Price != nil {
		if _, err := s.entitySvc.SetComponent(ctx, orgID, productID, "price", input.Price, actorID); err != nil {
			return nil, err
		}
	}
	if input.Barcode != nil {
		if _, err := s.entitySvc.SetComponent(ctx, orgID, productID, "barcode", input.Barcode, actorID); err != nil {
			return nil, err
		}
	}
	if input.Attributes != nil {
		if _, err := s.entitySvc.SetComponent(ctx, orgID, productID, "attributes", input.Attributes, actorID); err != nil {
			return nil, err
		}
	}
	if input.Media != nil {
		if _, err := s.entitySvc.SetComponent(ctx, orgID, productID, "media", input.Media, actorID); err != nil {
			return nil, err
		}
	}
	if input.PricingRules != nil {
		if _, err := s.entitySvc.SetComponent(ctx, orgID, productID, "pricing_rules", input.PricingRules, actorID); err != nil {
			return nil, err
		}
	}

	// Publish event
	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EntityID:       &productID,
		Type:           "catalog.product.updated",
		ActorID:        actorID,
		Timestamp:      time.Now(),
		Version:        1,
	}
	ev.Data, _ = json.Marshal(map[string]any{"id": productID})
	s.eventBus.Publish(ctx, ev)

	return s.GetProduct(ctx, orgID, productID)
}

// ArchiveProduct sets a product status to archived.
func (s *Service) ArchiveProduct(ctx context.Context, orgID uuid.UUID, productID uuid.UUID, actorID *uuid.UUID) error {
	status := "archived"
	if _, err := s.entitySvc.Update(ctx, orgID, productID, entity.UpdateEntityInput{Status: &status}, actorID); err != nil {
		return err
	}

	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EntityID:       &productID,
		Type:           "catalog.product.archived",
		ActorID:        actorID,
		Timestamp:      time.Now(),
		Version:        1,
	}
	ev.Data, _ = json.Marshal(map[string]any{"id": productID})
	s.eventBus.Publish(ctx, ev)
	return nil
}

// CreateCategory creates a category entity.
func (s *Service) CreateCategory(ctx context.Context, orgID uuid.UUID, input CreateCategoryInput, actorID *uuid.UUID) (*Category, error) {
	if input.Name == "" {
		return nil, errs.NewBadRequest("name is required")
	}

	e, err := s.entitySvc.Create(ctx, orgID, entity.CreateEntityInput{
		Kind:     "category",
		Name:     input.Name,
		ParentID: input.ParentID,
	}, actorID)
	if err != nil {
		return nil, err
	}

	return &Category{Entity: *e}, nil
}

// ListCategories returns categories, optionally filtered by parent.
func (s *Service) ListCategories(ctx context.Context, orgID uuid.UUID, parentID *uuid.UUID) ([]Category, error) {
	kind := "category"
	filter := entity.ListFilter{
		Kind:     &kind,
		ParentID: parentID,
		Page:     types.PageRequest{Limit: 200, Sort: "sort_order", Order: "asc"},
	}

	result, err := s.entitySvc.List(ctx, orgID, filter, false)
	if err != nil {
		return nil, err
	}

	categories := make([]Category, len(result.Items))
	for i, e := range result.Items {
		categories[i] = Category{Entity: e}
	}
	return categories, nil
}

// UpdateCategory updates a category.
func (s *Service) UpdateCategory(ctx context.Context, orgID uuid.UUID, categoryID uuid.UUID, name *string, parentID *uuid.UUID, actorID *uuid.UUID) (*Category, error) {
	input := entity.UpdateEntityInput{Name: name, ParentID: parentID}
	e, err := s.entitySvc.Update(ctx, orgID, categoryID, input, actorID)
	if err != nil {
		return nil, err
	}
	return &Category{Entity: *e}, nil
}

// DeleteCategory deletes a category (only if no products in it).
func (s *Service) DeleteCategory(ctx context.Context, orgID uuid.UUID, categoryID uuid.UUID, actorID *uuid.UUID) error {
	kind := "product"
	result, err := s.entitySvc.List(ctx, orgID, entity.ListFilter{
		Kind:     &kind,
		ParentID: &categoryID,
		Page:     types.PageRequest{Limit: 1},
	}, false)
	if err != nil {
		return err
	}
	if result.Total > 0 {
		return errs.NewConflict("category has products, cannot delete")
	}

	return s.entitySvc.Delete(ctx, orgID, categoryID, actorID)
}

// PricingRule represents a single pricing rule for a product.
type PricingRule struct {
	ID            string         `json:"id"`
	Name          string         `json:"name"`
	Type          string         `json:"type"` // schedule_discount, quantity_bonus, customer_discount
	DiscountType  string         `json:"discount_type,omitempty"`  // percent, fixed
	DiscountValue int64          `json:"discount_value,omitempty"` // percent (whole number) or fixed (kopecks)
	BuyQuantity   float64        `json:"buy_quantity,omitempty"`
	BonusQuantity float64        `json:"bonus_quantity,omitempty"`
	Conditions    RuleConditions `json:"conditions,omitempty"`
	Active        bool           `json:"active"`
}

// RuleConditions holds optional conditions for a pricing rule.
type RuleConditions struct {
	DaysOfWeek   []int    `json:"days_of_week,omitempty"`
	TimeFrom     string   `json:"time_from,omitempty"`
	TimeTo       string   `json:"time_to,omitempty"`
	ValidFrom    string   `json:"valid_from,omitempty"`
	ValidTo      string   `json:"valid_to,omitempty"`
	CustomerTags []string `json:"customer_tags,omitempty"`
}

// PriceInput holds context for calculating the effective price.
type PriceInput struct {
	Quantity     float64
	CustomerTags []string
	Now          time.Time
}

// PriceResult holds the calculated price details.
type PriceResult struct {
	BasePrice      int64   `json:"base_price"`
	EffectivePrice int64   `json:"effective_price"`
	TotalQuantity  float64 `json:"total_quantity"` // includes bonus items
	AppliedRules   []string `json:"applied_rules,omitempty"`
}

// CalculatePrice applies all active pricing rules to determine the effective price.
func (s *Service) CalculatePrice(ctx context.Context, orgID uuid.UUID, productID uuid.UUID, input PriceInput) (*PriceResult, error) {
	e, err := s.entitySvc.Get(ctx, orgID, productID, true)
	if err != nil {
		return nil, err
	}
	if e.Kind != "product" {
		return nil, errs.NewNotFound("product not found")
	}

	// Extract selling_price from price component
	var basePrice int64
	var rules []PricingRule
	for _, c := range e.Components {
		switch c.Type {
		case "price":
			var priceData map[string]any
			if json.Unmarshal(c.Data, &priceData) == nil {
				if sp, ok := priceData["selling_price"].(float64); ok {
					basePrice = int64(sp)
				}
			}
		case "pricing_rules":
			var rulesData struct {
				Rules []PricingRule `json:"rules"`
			}
			json.Unmarshal(c.Data, &rulesData)
			rules = rulesData.Rules
		}
	}

	if basePrice == 0 {
		return nil, errs.NewBadRequest("product has no selling_price")
	}

	result := &PriceResult{
		BasePrice:      basePrice,
		EffectivePrice: basePrice,
		TotalQuantity:  input.Quantity,
	}

	for _, rule := range rules {
		if !rule.Active {
			continue
		}
		switch rule.Type {
		case "schedule_discount":
			if matchesSchedule(rule.Conditions, input.Now) {
				result.EffectivePrice = applyDiscount(result.EffectivePrice, rule.DiscountType, rule.DiscountValue)
				result.AppliedRules = append(result.AppliedRules, rule.Name)
			}
		case "quantity_bonus":
			if rule.BuyQuantity > 0 && input.Quantity >= rule.BuyQuantity {
				sets := int(input.Quantity / rule.BuyQuantity)
				result.TotalQuantity = input.Quantity + float64(sets)*rule.BonusQuantity
				result.AppliedRules = append(result.AppliedRules, rule.Name)
			}
		case "customer_discount":
			if matchesCustomerTags(rule.Conditions.CustomerTags, input.CustomerTags) {
				result.EffectivePrice = applyDiscount(result.EffectivePrice, rule.DiscountType, rule.DiscountValue)
				result.AppliedRules = append(result.AppliedRules, rule.Name)
			}
		}
	}

	return result, nil
}

func matchesSchedule(cond RuleConditions, now time.Time) bool {
	if len(cond.DaysOfWeek) > 0 {
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday = 7 (ISO)
		}
		found := false
		for _, d := range cond.DaysOfWeek {
			if d == weekday {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if cond.ValidFrom != "" {
		if vf, err := time.Parse("2006-01-02", cond.ValidFrom); err == nil {
			if now.Before(vf) {
				return false
			}
		}
	}
	if cond.ValidTo != "" {
		if vt, err := time.Parse("2006-01-02", cond.ValidTo); err == nil {
			if now.After(vt.Add(24 * time.Hour)) {
				return false
			}
		}
	}

	if cond.TimeFrom != "" && cond.TimeTo != "" {
		nowTime := now.Format("15:04")
		if nowTime < cond.TimeFrom || nowTime > cond.TimeTo {
			return false
		}
	}

	return true
}

func matchesCustomerTags(requiredTags, customerTags []string) bool {
	if len(requiredTags) == 0 {
		return false
	}
	tagSet := make(map[string]bool, len(customerTags))
	for _, t := range customerTags {
		tagSet[t] = true
	}
	for _, req := range requiredTags {
		if tagSet[req] {
			return true
		}
	}
	return false
}

func applyDiscount(price int64, discountType string, value int64) int64 {
	switch discountType {
	case "percent":
		discount := price * value / 100
		return price - discount
	case "fixed":
		result := price - value
		if result < 0 {
			return 0
		}
		return result
	}
	return price
}

func entityToProduct(e *types.Entity) *Product {
	p := &Product{Entity: *e}
	for _, c := range e.Components {
		switch c.Type {
		case "price":
			p.Price = c.Data
		case "barcode":
			p.Barcode = c.Data
		case "media":
			p.Media = c.Data
		case "attributes":
			p.Attributes = c.Data
		case "pricing_rules":
			p.PricingRules = c.Data
		}
	}
	return p
}
