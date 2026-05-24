package graphs

import (
	"context"
	"fmt"

	"github.com/FrankFMY/arcana"
)

var crmCustomersList = arcana.GraphDef{
	Key: "crm_customers_list",
	Deps: []arcana.TableDep{
		{Table: "entities", Columns: []string{"name", "status"}},
		{Table: "components", Columns: []string{"data"}},
	},
	Params: arcana.ParamSchema{
		"search":   arcana.ParamString().Build(),
		"tag":      arcana.ParamString().Build(),
		"category": arcana.ParamString().Build(),
		"limit":    arcana.ParamInt().Default(50),
		"offset":   arcana.ParamInt().Default(0),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		limit := p.Int("limit")
		offset := p.Int("offset")
		search := p.String("search")
		tag := p.String("tag")
		category := p.String("category")

		query := `
			SELECT e.id, e.name, e.status,
			       COALESCE(cp.data->>'category','') AS category,
			       COALESCE(cp.data->'tags', '[]') AS tags,
			       COALESCE(cb.data->>'balance','0') AS balance,
			       COALESCE(ct.data->>'phone','') AS phone,
			       COALESCE(ct.data->>'email','') AS email,
			       COUNT(*) OVER() AS total_count
			FROM entities e
			LEFT JOIN components cp ON cp.entity_id = e.id AND cp.type = 'crm_profile' AND cp.organization_id = $1
			LEFT JOIN components cb ON cb.entity_id = e.id AND cb.type = 'crm_balance' AND cb.organization_id = $1
			LEFT JOIN components ct ON ct.entity_id = e.id AND ct.type = 'contact' AND ct.organization_id = $1
			WHERE e.organization_id = $1 AND e.kind = 'customer' AND e.deleted_at IS NULL
		`
		args := []any{orgID}
		argIdx := 2

		if search != "" {
			query += fmt.Sprintf(` AND e.name ILIKE $%d`, argIdx)
			args = append(args, "%"+search+"%")
			argIdx++
		}
		if tag != "" {
			query += fmt.Sprintf(` AND COALESCE(cp.data->'tags', '[]'::jsonb) ? $%d`, argIdx)
			args = append(args, tag)
			argIdx++
		}
		if category != "" {
			query += fmt.Sprintf(` AND cp.data->>'category' = $%d`, argIdx)
			args = append(args, category)
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
			var id, name, status, category, tags, balance, phone, email string
			var total int
			if err := rows.Scan(&id, &name, &status, &category, &tags, &balance, &phone, &email, &total); err != nil {
				return nil, err
			}
			result.SetTotal(total)
			result.AddRef(arcana.Ref{Table: "entities", ID: id, Fields: []string{"name", "status", "category", "tags", "balance", "phone", "email"}})
			result.AddRow("entities", id, map[string]any{
				"id": id, "name": name, "status": status, "category": category,
				"tags": tags, "balance": balance, "phone": phone, "email": email,
			})
		}
		return result, rows.Err()
	},
}

var crmCustomerDetail = arcana.GraphDef{
	Key: "crm_customer_detail",
	Deps: []arcana.TableDep{
		{Table: "entities", Columns: []string{"name", "status"}},
		{Table: "components", Columns: []string{"data"}},
		{Table: "orders", Columns: []string{"status", "total"}},
	},
	Params: arcana.ParamSchema{
		"id": arcana.ParamString().Required(),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		id := p.String("id")

		result := arcana.NewResult()

		var name, status string
		err := q.QueryRow(ctx, `SELECT name, status FROM entities WHERE id = $1 AND organization_id = $2`, id, orgID).Scan(&name, &status)
		if err != nil {
			return nil, err
		}
		result.AddRef(arcana.Ref{Table: "entities", ID: id, Fields: []string{"name", "status"}})
		result.AddRow("entities", id, map[string]any{"id": id, "name": name, "status": status})

		compRows, _ := q.Query(ctx, `SELECT type, data FROM components WHERE entity_id = $1 AND organization_id = $2`, id, orgID)
		if compRows != nil {
			defer compRows.Close()
			for compRows.Next() {
				var cType string
				var cData []byte
				if compRows.Scan(&cType, &cData) == nil {
					result.AddRow("components", id+"_"+cType, map[string]any{"type": cType, "data": string(cData)})
				}
			}
		}

		orderRows, _ := q.Query(ctx, `SELECT id, status, total, iat FROM orders WHERE customer_id = $1 AND organization_id = $2 ORDER BY iat DESC LIMIT 10`, id, orgID)
		if orderRows != nil {
			defer orderRows.Close()
			for orderRows.Next() {
				var oid, ostatus string
				var ototal int64
				var oiat string
				if orderRows.Scan(&oid, &ostatus, &ototal, &oiat) == nil {
					result.AddRef(arcana.Ref{Table: "orders", ID: oid, Fields: []string{"status", "total", "created_at"}})
					result.AddRow("orders", oid, map[string]any{"id": oid, "status": ostatus, "total": ototal, "created_at": oiat})
				}
			}
		}

		return result, nil
	},
}

var crmSuppliersList = arcana.GraphDef{
	Key: "crm_suppliers_list",
	Deps: []arcana.TableDep{
		{Table: "entities", Columns: []string{"name", "status"}},
		{Table: "components", Columns: []string{"data"}},
	},
	Params: arcana.ParamSchema{
		"search": arcana.ParamString().Build(),
		"limit":  arcana.ParamInt().Default(50),
		"offset": arcana.ParamInt().Default(0),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		limit := p.Int("limit")
		offset := p.Int("offset")
		search := p.String("search")

		query := `
			SELECT e.id, e.name, e.status,
			       COALESCE(ct.data->>'phone','') AS phone,
			       COALESCE(ct.data->>'inn','') AS inn,
			       COUNT(*) OVER() AS total_count
			FROM entities e
			LEFT JOIN components ct ON ct.entity_id = e.id AND ct.type = 'contact' AND ct.organization_id = $1
			WHERE e.organization_id = $1 AND e.kind = 'supplier' AND e.deleted_at IS NULL
		`
		args := []any{orgID}
		argIdx := 2

		if search != "" {
			query += fmt.Sprintf(` AND e.name ILIKE $%d`, argIdx)
			args = append(args, "%"+search+"%")
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
			var id, name, status, phone, inn string
			var total int
			if err := rows.Scan(&id, &name, &status, &phone, &inn, &total); err != nil {
				return nil, err
			}
			result.SetTotal(total)
			result.AddRef(arcana.Ref{Table: "entities", ID: id, Fields: []string{"name", "status", "phone", "inn"}})
			result.AddRow("entities", id, map[string]any{
				"id": id, "name": name, "status": status, "phone": phone, "inn": inn,
			})
		}
		return result, rows.Err()
	},
}
