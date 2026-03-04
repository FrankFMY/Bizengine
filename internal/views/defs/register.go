package defs

import "github.com/bizengine/engine/internal/views"

// RegisterAll registers all view definitions into the given registry.
func RegisterAll(r *views.Registry) {
	// Warehouse
	r.Register(WarehouseStockList)
	r.Register(WarehouseStockDetail)
	r.Register(WarehouseLowStock)

	// Orders
	r.Register(OrdersList)
	r.Register(OrderDetail)
	r.Register(OrdersDashboard)

	// Catalog
	r.Register(CatalogProductsList)
	r.Register(CatalogProductDetail)
	r.Register(CatalogCategoriesTree)

	// HR
	r.Register(HREmployeesList)
	r.Register(HREmployeeDetail)
	r.Register(HRShiftsSchedule)
	r.Register(HRTimesheetsList)

	// Finance
	r.Register(FinanceTrialBalance)
	r.Register(FinanceTransactionsList)
	r.Register(FinanceAccountBalance)

	// Logistics
	r.Register(LogisticsRoutesList)
	r.Register(LogisticsRouteDetail)
	r.Register(LogisticsVehiclesMap)

	// Dashboard
	r.Register(DashboardSummary)
}
