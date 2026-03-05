package graphs

import (
	"context"
	"fmt"

	"github.com/FrankFMY/arcana"
)

var catalogProductsList = arcana.GraphDef{
	Key: "catalog_products_list",
	Deps: []arcana.TableDep{
		{Table: "entities", Columns: []string{"name", "status"}},
		{Table: "components", Columns: []string{"data"}},
	},
	Params: arcana.ParamSchema{
		"category_id": arcana.ParamUUID().Build(),
		"search":      arcana.ParamString().Build(),
		"limit":       arcana.ParamInt().Default(50),
		"offset":      arcana.ParamInt().Default(0),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		limit := p.Int("limit")
		offset := p.Int("offset")
		search := p.String("search")
		categoryID := p.UUID("category_id")

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

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()

		for rows.Next() {
			var id, name, sku, categoryName, status string
			var sellingPrice int64
			var cnt int

			if err := rows.Scan(&id, &name, &sku, &sellingPrice, &categoryName, &status, &cnt); err != nil {
				return nil, err
			}

			result.AddRef(arcana.Ref{Table: "catalog_products", ID: id, Fields: []string{"name", "sku", "selling_price", "category_name", "status"}})
			result.AddRow("catalog_products", id, map[string]any{
				"id":            id,
				"name":          name,
				"sku":           sku,
				"selling_price": sellingPrice,
				"category_name": categoryName,
				"status":        status,
			})
		}

		return result, rows.Err()
	},
}

var catalogProductDetail = arcana.GraphDef{
	Key: "catalog_product_detail",
	Deps: []arcana.TableDep{
		{Table: "entities", Columns: []string{"name", "status"}},
		{Table: "components", Columns: []string{"data"}},
		{Table: "stock_levels", Columns: []string{"quantity", "reserved"}},
	},
	Params: arcana.ParamSchema{
		"product_id": arcana.ParamUUID().Required(),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		productID := p.UUID("product_id")

		result := arcana.NewResult()

		var id, name, status string
		err := q.QueryRow(ctx, `
			SELECT id, name, status FROM entities
			WHERE organization_id = $1 AND id = $2 AND kind = 'product' AND deleted_at IS NULL
		`, orgID, productID).Scan(&id, &name, &status)
		if err != nil {
			return nil, err
		}

		product := map[string]any{"id": id, "name": name, "status": status}

		compRows, err := q.Query(ctx, `
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

		result.AddRef(arcana.Ref{Table: "catalog_products", ID: id, Fields: []string{"name", "status", "price", "barcode", "attributes"}})
		result.AddRow("catalog_products", id, product)

		stockRows, err := q.Query(ctx, `
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
			result.AddRef(arcana.Ref{Table: "stock_levels", ID: stockKey, Fields: []string{"warehouse_name", "quantity", "available"}})
			result.AddRow("stock_levels", stockKey, map[string]any{
				"warehouse_id":   whID,
				"warehouse_name": whName,
				"quantity":       qty,
				"available":      avail,
			})
		}

		return result, nil
	},
}

var catalogCategoriesTree = arcana.GraphDef{
	Key: "catalog_categories_tree",
	Deps: []arcana.TableDep{
		{Table: "entities", Columns: []string{"name", "parent_id", "sort_order"}},
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)

		rows, err := q.Query(ctx, `
			SELECT id, name, parent_id, sort_order
			FROM entities
			WHERE organization_id = $1 AND kind = 'category' AND deleted_at IS NULL
			ORDER BY sort_order, name
		`, orgID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()

		for rows.Next() {
			var id, name string
			var parentID *string
			var sortOrder int

			if err := rows.Scan(&id, &name, &parentID, &sortOrder); err != nil {
				return nil, err
			}

			result.AddRef(arcana.Ref{Table: "catalog_categories", ID: id, Fields: []string{"name", "parent_id", "sort_order"}})
			cat := map[string]any{
				"id":         id,
				"name":       name,
				"sort_order": sortOrder,
			}
			if parentID != nil {
				cat["parent_id"] = *parentID
			}
			result.AddRow("catalog_categories", id, cat)
		}

		return result, rows.Err()
	},
}
