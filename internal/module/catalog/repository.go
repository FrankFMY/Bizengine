// Package catalog provides the product catalog module.
package catalog

import (
	"context"

	"github.com/google/uuid"

	"github.com/bizengine/engine/pkg/types"
)

// Repository defines catalog-specific storage operations beyond core entity CRUD.
type Repository interface {
	// FindBySKU returns a product entity by its internal barcode/SKU within an organization.
	FindBySKU(ctx context.Context, orgID uuid.UUID, sku string) (*types.Entity, error)

	// ListProductsWithComponents returns products with their components, supporting catalog-specific filters.
	ListProductsWithComponents(ctx context.Context, orgID uuid.UUID, filter ProductFilter) ([]Product, int, error)
}
