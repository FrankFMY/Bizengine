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
	Price      json.RawMessage `json:"price,omitempty"`
	Barcode    json.RawMessage `json:"barcode,omitempty"`
	Media      json.RawMessage `json:"media,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
}

// Category represents a category entity.
type Category struct {
	types.Entity
}

// CreateProductInput is the input for creating a product.
type CreateProductInput struct {
	Name       string          `json:"name"`
	SKU        string          `json:"sku"`
	CategoryID *uuid.UUID      `json:"category_id,omitempty"`
	Price      json.RawMessage `json:"price"`
	Barcode    json.RawMessage `json:"barcode,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
	Media      json.RawMessage `json:"media,omitempty"`
}

// UpdateProductInput is the input for updating a product.
type UpdateProductInput struct {
	Name       *string         `json:"name,omitempty"`
	CategoryID *uuid.UUID      `json:"category_id,omitempty"`
	Price      json.RawMessage `json:"price,omitempty"`
	Barcode    json.RawMessage `json:"barcode,omitempty"`
	Attributes json.RawMessage `json:"attributes,omitempty"`
	Media      json.RawMessage `json:"media,omitempty"`
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
		}
	}
	return p
}
