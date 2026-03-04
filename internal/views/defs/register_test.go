package defs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/views"
)

func TestRegisterAll(t *testing.T) {
	r := views.NewRegistry()
	RegisterAll(r)

	all := r.All()
	assert.Len(t, all, 20, "expected 20 view definitions registered")

	// Verify all expected views are present.
	expected := []string{
		"warehouse_stock_list", "warehouse_stock_detail", "warehouse_low_stock",
		"orders_list", "order_detail", "orders_dashboard",
		"catalog_products_list", "catalog_product_detail", "catalog_categories_tree",
		"hr_employees_list", "hr_employee_detail", "hr_shifts_schedule", "hr_timesheets_list",
		"finance_trial_balance", "finance_transactions_list", "finance_account_balance",
		"logistics_routes_list", "logistics_route_detail", "logistics_vehicles_map",
		"dashboard_summary",
	}
	for _, key := range expected {
		def, ok := r.Get(key)
		require.True(t, ok, "view %q not registered", key)
		assert.NotNil(t, def.Factory, "view %q has nil factory", key)
		assert.NotEmpty(t, def.Tables, "view %q has no table deps", key)
	}
}

func TestRegisterAll_InvertedIndex(t *testing.T) {
	r := views.NewRegistry()
	RegisterAll(r)

	// "orders" table should be referenced by orders_list, order_detail, orders_dashboard, dashboard_summary.
	orderViews := r.GetByTable("orders")
	assert.GreaterOrEqual(t, len(orderViews), 4, "orders table should have >= 4 views depending on it")

	// "entities" table should be referenced by many views.
	entityViews := r.GetByTable("entities")
	assert.GreaterOrEqual(t, len(entityViews), 8, "entities table should have >= 8 views depending on it")

	// "stock_levels" should appear in warehouse + catalog + dashboard views.
	stockViews := r.GetByTable("stock_levels")
	assert.GreaterOrEqual(t, len(stockViews), 4)
}

func TestViewDef_ParamSchemas(t *testing.T) {
	tests := []struct {
		name   string
		def    *views.ViewDef
		params map[string]string
	}{
		{"warehouse_stock_list", WarehouseStockList, map[string]string{"warehouse_id": "uuid"}},
		{"order_detail", OrderDetail, map[string]string{"order_id": "uuid"}},
		{"catalog_products_list", CatalogProductsList, map[string]string{"category_id": "uuid", "search": "string"}},
		{"orders_list", OrdersList, map[string]string{"status": "string"}},
		{"hr_employees_list", HREmployeesList, map[string]string{"search": "string", "status": "string"}},
		{"finance_account_balance", FinanceAccountBalance, map[string]string{"account_id": "uuid"}},
		{"logistics_route_detail", LogisticsRouteDetail, map[string]string{"route_id": "uuid"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.params, tt.def.ParamSchema)
		})
	}
}

func TestViewDef_NoParamSchema(t *testing.T) {
	noParams := []*views.ViewDef{
		OrdersDashboard,
		CatalogCategoriesTree,
		HRShiftsSchedule,
		FinanceTrialBalance,
		LogisticsVehiclesMap,
		DashboardSummary,
	}
	for _, def := range noParams {
		assert.Empty(t, def.ParamSchema, "view %q should have no params", def.Key)
	}
}

func TestIntParam(t *testing.T) {
	assert.Equal(t, 10, intParam(map[string]any{"limit": float64(10)}, "limit", 50))
	assert.Equal(t, 42, intParam(map[string]any{"limit": 42}, "limit", 50))
	assert.Equal(t, 50, intParam(map[string]any{}, "limit", 50))
	assert.Equal(t, 50, intParam(map[string]any{"limit": "invalid"}, "limit", 50))
	assert.Equal(t, 7, intParam(map[string]any{"limit": int64(7)}, "limit", 50))
}
