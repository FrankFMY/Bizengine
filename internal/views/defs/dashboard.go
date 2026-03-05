package defs

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/views"
)

// DashboardSummary returns a high-level business overview.
var DashboardSummary = &views.ViewDef{
	Key: "dashboard_summary",
	Tables: []views.TableDep{
		{Table: "orders", Columns: []string{"status", "total", "created_at"}},
		{Table: "stock_levels", Columns: []string{"quantity", "min_quantity"}},
		{Table: "entities", Columns: []string{"status"}, Filter: "kind=employee"},
	},
	Factory: dashboardSummaryFactory,
}

func dashboardSummaryFactory(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, _ map[string]any) (*views.ViewResult, error) {
	tables := map[string]map[string]any{"dashboard": {}}

	var ordersToday, revenueToday int64
	err := pool.QueryRow(ctx, `
		SELECT COUNT(*), COALESCE(SUM(total), 0)
		FROM orders
		WHERE organization_id = $1 AND created_at >= CURRENT_DATE AND cancelled_at IS NULL
	`, orgID).Scan(&ordersToday, &revenueToday)
	if err != nil {
		return nil, err
	}

	var lowStockCount int64
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM stock_levels
		WHERE organization_id = $1 AND min_quantity > 0 AND quantity - reserved <= min_quantity
	`, orgID).Scan(&lowStockCount)
	if err != nil {
		return nil, err
	}

	var employeesOnShift int64
	err = pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT t.employee_id)
		FROM timesheets t
		WHERE t.organization_id = $1 AND t.status = 'open' AND t.clock_out IS NULL
	`, orgID).Scan(&employeesOnShift)
	if err != nil {
		return nil, err
	}

	refs := []views.DataRef{{Table: "dashboard", ID: "summary", Fields: []string{"orders_today", "revenue_today", "low_stock_count", "employees_on_shift"}}}
	tables["dashboard"]["summary"] = map[string]any{
		"orders_today":      ordersToday,
		"revenue_today":     revenueToday,
		"low_stock_count":   lowStockCount,
		"employees_on_shift": employeesOnShift,
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}
