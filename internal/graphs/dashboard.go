package graphs

import (
	"context"

	"github.com/FrankFMY/arcana"
)

var dashboardSummary = arcana.GraphDef{
	Key: "dashboard_summary",
	Deps: []arcana.TableDep{
		{Table: "orders", Columns: []string{"status", "total", "created_at"}},
		{Table: "stock_levels", Columns: []string{"quantity", "min_quantity"}},
		{Table: "entities", Columns: []string{"status"}},
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		result := arcana.NewResult()

		var ordersToday, revenueToday int64
		err := q.QueryRow(ctx, `
			SELECT COUNT(*), COALESCE(SUM(total), 0)
			FROM orders
			WHERE organization_id = $1 AND created_at >= CURRENT_DATE AND cancelled_at IS NULL
		`, orgID).Scan(&ordersToday, &revenueToday)
		if err != nil {
			return nil, err
		}

		var lowStockCount int64
		err = q.QueryRow(ctx, `
			SELECT COUNT(*)
			FROM stock_levels
			WHERE organization_id = $1 AND min_quantity > 0 AND quantity - reserved <= min_quantity
		`, orgID).Scan(&lowStockCount)
		if err != nil {
			return nil, err
		}

		var employeesOnShift int64
		err = q.QueryRow(ctx, `
			SELECT COUNT(DISTINCT t.employee_id)
			FROM timesheets t
			WHERE t.organization_id = $1 AND t.status = 'open' AND t.clock_out IS NULL
		`, orgID).Scan(&employeesOnShift)
		if err != nil {
			return nil, err
		}

		result.AddRef(arcana.Ref{Table: "dashboard", ID: "summary", Fields: []string{"orders_today", "revenue_today", "low_stock_count", "employees_on_shift"}})
		result.AddRow("dashboard", "summary", map[string]any{
			"orders_today":       ordersToday,
			"revenue_today":      revenueToday,
			"low_stock_count":    lowStockCount,
			"employees_on_shift": employeesOnShift,
		})

		return result, nil
	},
}
