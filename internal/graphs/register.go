package graphs

import "github.com/FrankFMY/arcana"

// RegisterAll registers all Arcana graph definitions into the engine.
func RegisterAll(engine *arcana.Engine) {
	defs := []arcana.GraphDef{
		// Catalog
		catalogProductsList,
		catalogProductDetail,
		catalogCategoriesTree,

		// Warehouse
		warehouseStockList,
		warehouseStockDetail,
		warehouseLowStock,

		// Orders
		ordersList,
		orderDetail,
		ordersDashboard,

		// HR
		hrEmployeesList,
		hrEmployeeDetail,
		hrShiftsSchedule,
		hrTimesheetsList,

		// Finance
		financeTrialBalance,
		financeTransactionsList,
		financeAccountBalance,

		// Logistics
		logisticsRoutesList,
		logisticsRouteDetail,
		logisticsVehiclesMap,

		// Notifications
		notificationsUnread,

		// Dashboard
		dashboardSummary,
	}
	for _, def := range defs {
		engine.Register(def)
	}
}
