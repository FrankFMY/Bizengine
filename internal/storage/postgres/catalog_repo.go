package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/module/catalog"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// CatalogRepo implements catalog.Repository using PostgreSQL.
type CatalogRepo struct {
	pool *pgxpool.Pool
}

// NewCatalogRepo creates a new CatalogRepo.
func NewCatalogRepo(pool *pgxpool.Pool) *CatalogRepo {
	return &CatalogRepo{pool: pool}
}

// FindBySKU returns a product entity by its internal barcode/SKU.
func (r *CatalogRepo) FindBySKU(ctx context.Context, orgID uuid.UUID, sku string) (*types.Entity, error) {
	var e types.Entity
	err := r.pool.QueryRow(ctx,
		`SELECT e.id, e.organization_id, e.kind, e.name, e.status, e.parent_id, e.meta, e.sort_order, e.created_at, e.updated_at, e.deleted_at
		 FROM entities e
		 JOIN components c ON c.entity_id = e.id AND c.type = 'barcode'
		 WHERE e.organization_id = $1 AND e.kind = 'product' AND e.deleted_at IS NULL
		   AND c.data->>'internal' = $2`,
		orgID, sku,
	).Scan(&e.ID, &e.OrganizationID, &e.Kind, &e.Name, &e.Status, &e.ParentID, &e.Meta, &e.SortOrder, &e.CreatedAt, &e.UpdatedAt, &e.DeletedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("product not found by SKU")
		}
		return nil, err
	}
	return &e, nil
}

// ListProductsWithComponents returns products with their components.
func (r *CatalogRepo) ListProductsWithComponents(ctx context.Context, orgID uuid.UUID, filter catalog.ProductFilter) ([]catalog.Product, int, error) {
	filter.Page.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("e.organization_id = $%d", argIdx))
	args = append(args, orgID)
	argIdx++

	conditions = append(conditions, "e.kind = 'product'")
	conditions = append(conditions, "e.deleted_at IS NULL")

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("e.status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	} else {
		conditions = append(conditions, "e.status != 'archived'")
	}

	if filter.CategoryID != nil {
		conditions = append(conditions, fmt.Sprintf("e.parent_id = $%d", argIdx))
		args = append(args, *filter.CategoryID)
		argIdx++
	}

	if filter.Search != nil && *filter.Search != "" {
		conditions = append(conditions, fmt.Sprintf("e.name ILIKE '%%' || $%d || '%%'", argIdx))
		args = append(args, *filter.Search)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	// Count
	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM entities e WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT e.id, e.organization_id, e.kind, e.name, e.status, e.parent_id, e.meta, e.sort_order, e.created_at, e.updated_at
		 FROM entities e WHERE %s ORDER BY e.created_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, filter.Page.Limit, filter.Page.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var products []catalog.Product
	for rows.Next() {
		var e types.Entity
		if err := rows.Scan(&e.ID, &e.OrganizationID, &e.Kind, &e.Name, &e.Status, &e.ParentID, &e.Meta, &e.SortOrder, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, 0, err
		}
		products = append(products, catalog.Product{Entity: e})
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	// Load components for all products
	if len(products) > 0 {
		ids := make([]uuid.UUID, len(products))
		for i, p := range products {
			ids[i] = p.ID
		}

		compRows, err := r.pool.Query(ctx,
			`SELECT entity_id, type, data FROM components WHERE organization_id = $1 AND entity_id = ANY($2)`,
			orgID, ids,
		)
		if err != nil {
			return nil, 0, err
		}
		defer compRows.Close()

		compMap := make(map[uuid.UUID]map[string]json.RawMessage)
		for compRows.Next() {
			var entityID uuid.UUID
			var compType string
			var data json.RawMessage
			if err := compRows.Scan(&entityID, &compType, &data); err != nil {
				return nil, 0, err
			}
			if compMap[entityID] == nil {
				compMap[entityID] = make(map[string]json.RawMessage)
			}
			compMap[entityID][compType] = data
		}

		for i := range products {
			if comps, ok := compMap[products[i].ID]; ok {
				products[i].Price = comps["price"]
				products[i].Barcode = comps["barcode"]
				products[i].Media = comps["media"]
				products[i].Attributes = comps["attributes"]
			}
		}
	}

	return products, total, nil
}
