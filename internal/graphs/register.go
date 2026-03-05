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
		warehouseInventoryDetail,
		warehouseLowStock,

		// Orders
		ordersList,
		orderDetail,
		orderRefundsList,
		ordersDashboard,

		// HR
		hrEmployeesList,
		hrEmployeeDetail,
		hrShiftsSchedule,
		hrTimesheetsList,
		hrPayrollList,
		hrAbsencesList,

		// Finance
		financeTrialBalance,
		financeTransactionsList,
		financeAccountBalance,
		financePnL,
		financePeriodsList,
		financeCashOperations,

		// CRM
		crmCustomersList,
		crmCustomerDetail,
		crmSuppliersList,

		// Settings
		organizationSettings,

		// Logistics
		logisticsRoutesList,
		logisticsRouteDetail,
		logisticsVehiclesMap,

		// Notifications
		notificationsUnread,
		notificationsList,

		// Dashboard
		dashboardSummary,
	}
	for _, def := range defs {
		engine.Register(def)
	}
}
