package defs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/views"
)

// CatalogProductsList returns a paginated list of catalog products.
var CatalogProductsList = &views.ViewDef{
	Key: "catalog_products_list",
	Tables: []views.TableDep{
		{Table: "entities", Columns: []string{"name", "status"}, Filter: "kind=product"},
		{Table: "components", Columns: []string{"data"}, Filter: "type=price"},
	},
	ParamSchema: map[string]string{
		"category_id": "uuid",
		"search":      "string",
	},
	Factory: catalogProductsListFactory,
}

func catalogProductsListFactory(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, params map[string]any) (*views.ViewResult, error) {
	limit := intParam(params, "limit", 50)
	offset := intParam(params, "offset", 0)
	search, _ := params["search"].(string)
	categoryID, _ := params["category_id"].(string)

	query := `
		SELECT e.id, e.name, COALESCE(bc.data->>'sku','') AS sku,
		       COALESCE((pc.data->>'selling_price')::bigint, 0) AS selling_price,
		       COALESCE(cat.name, '') AS category_name,
		       e.status, COUNT(*) OVER() AS total_count
		FROM entities e
		LEFT JOIN components pc ON pc.entity_id = e.id AND pc.type = 'price' AND pc.organization_id = $1
		LEFT JOIN components bc ON bc.entity_id = e.id AND bc.type = 'barcode' AND bc.organization_id = $1
		LEFT JOIN entities cat ON cat.id = e.parent_id AND cat.organization_id = $1 AND cat.kind = 'category'
		WHERE e.organization_id = $1 AND e.kind = 'product' AND e.deleted_at IS NULL
	`
	args := []any{orgID}
	argIdx := 2

	if search != "" {
		query += fmt.Sprintf(` AND e.name ILIKE $%d`, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if categoryID != "" {
		query += fmt.Sprintf(` AND e.parent_id = $%d`, argIdx)
		args = append(args, categoryID)
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY e.name LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]views.DataRef, 0)
	tables := map[string]map[string]any{"catalog_products": {}}
	var total int

	for rows.Next() {
		var id, name, sku, categoryName, status string
		var sellingPrice int64
		var cnt int

		if err := rows.Scan(&id, &name, &sku, &sellingPrice, &categoryName, &status, &cnt); err != nil {
			return nil, err
		}
		total = cnt

		refs = append(refs, views.DataRef{Table: "catalog_products", ID: id, Fields: []string{"name", "sku", "selling_price", "category_name", "status"}})
		tables["catalog_products"][id] = map[string]any{
			"id":            id,
			"name":          name,
			"sku":           sku,
			"selling_price": sellingPrice,
			"category_name": categoryName,
			"status":        status,
		}
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: int64(total)}, nil
}

// CatalogProductDetail returns full product card with components and stock.
var CatalogProductDetail = &views.ViewDef{
	Key: "catalog_product_detail",
	Tables: []views.TableDep{
		{Table: "entities", Columns: []string{"name", "status"}, Filter: "kind=product"},
		{Table: "components", Columns: []string{"data"}},
		{Table: "stock_levels", Columns: []string{"quantity", "reserved"}},
	},
	ParamSchema: map[string]string{
		"product_id": "uuid",
	},
	Factory: catalogProductDetailFactory,
}

func catalogProductDetailFactory(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, params map[string]any) (*views.ViewResult, error) {
	productID, _ := params["product_id"].(string)

	tables := map[string]map[string]any{"catalog_products": {}, "stock_levels": {}}
	refs := make([]views.DataRef, 0, 1)

	// Product entity + components
	var id, name, status string
	err := pool.QueryRow(ctx, `
		SELECT id, name, status FROM entities
		WHERE organization_id = $1 AND id = $2 AND kind = 'product' AND deleted_at IS NULL
	`, orgID, productID).Scan(&id, &name, &status)
	if err != nil {
		return nil, err
	}

	product := map[string]any{"id": id, "name": name, "status": status}

	// Load all components for this product
	compRows, err := pool.Query(ctx, `
		SELECT type, data FROM components WHERE organization_id = $1 AND entity_id = $2
	`, orgID, productID)
	if err != nil {
		return nil, err
	}
	defer compRows.Close()

	for compRows.Next() {
		var compType string
		var compData any
		if err := compRows.Scan(&compType, &compData); err != nil {
			return nil, err
		}
		product[compType] = compData
	}

	refs = append(refs, views.DataRef{Table: "catalog_products", ID: id, Fields: []string{"name", "status", "price", "barcode", "attributes"}})
	tables["catalog_products"][id] = product

	// Stock across warehouses
	stockRows, err := pool.Query(ctx, `
		SELECT sl.warehouse_id, e.name, sl.quantity, sl.quantity - sl.reserved AS available
		FROM stock_levels sl
		JOIN entities e ON e.id = sl.warehouse_id AND e.organization_id = $1
		WHERE sl.organization_id = $1 AND sl.product_id = $2
	`, orgID, productID)
	if err != nil {
		return nil, err
	}
	defer stockRows.Close()

	for stockRows.Next() {
		var whID, whName string
		var qty, avail float64
		if err := stockRows.Scan(&whID, &whName, &qty, &avail); err != nil {
			return nil, err
		}

		stockKey := productID + ":" + whID
		refs = append(refs, views.DataRef{Table: "stock_levels", ID: stockKey, Fields: []string{"warehouse_name", "quantity", "available"}})
		tables["stock_levels"][stockKey] = map[string]any{
			"warehouse_id":   whID,
			"warehouse_name": whName,
			"quantity":       qty,
			"available":      avail,
		}
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}

// CatalogCategoriesTree returns the full category tree.
var CatalogCategoriesTree = &views.ViewDef{
	Key: "catalog_categories_tree",
	Tables: []views.TableDep{
		{Table: "entities", Columns: []string{"name", "parent_id", "sort_order"}, Filter: "kind=category"},
	},
	Factory: catalogCategoriesTreeFactory,
}

func catalogCategoriesTreeFactory(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, _ map[string]any) (*views.ViewResult, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, name, parent_id, sort_order
		FROM entities
		WHERE organization_id = $1 AND kind = 'category' AND deleted_at IS NULL
		ORDER BY sort_order, name
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]views.DataRef, 0)
	tables := map[string]map[string]any{"catalog_categories": {}}

	for rows.Next() {
		var id, name string
		var parentID *string
		var sortOrder int

		if err := rows.Scan(&id, &name, &parentID, &sortOrder); err != nil {
			return nil, err
		}

		refs = append(refs, views.DataRef{Table: "catalog_categories", ID: id, Fields: []string{"name", "parent_id", "sort_order"}})
		cat := map[string]any{
			"id":         id,
			"name":       name,
			"sort_order": sortOrder,
		}
		if parentID != nil {
			cat["parent_id"] = *parentID
		}
		tables["catalog_categories"][id] = cat
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}
