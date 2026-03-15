package graphs

import (
	"encoding/json"
	"testing"

	"github.com/FrankFMY/arcana"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func allGraphDefs() []arcana.GraphDef {
	return []arcana.GraphDef{
		catalogProductsList,
		catalogProductDetail,
		catalogCategoriesTree,
		warehouseStockList,
		warehouseStockDetail,
		warehouseInventoryDetail,
		warehouseLowStock,
		ordersList,
		orderDetail,
		orderRefundsList,
		ordersDashboard,
		hrEmployeesList,
		hrEmployeeDetail,
		hrShiftsSchedule,
		hrTimesheetsList,
		hrPayrollList,
		hrAbsencesList,
		financeTrialBalance,
		financeTransactionsList,
		financeAccountBalance,
		financePnL,
		financePeriodsList,
		financeCashOperations,
		crmCustomersList,
		crmCustomerDetail,
		crmSuppliersList,
		organizationSettings,
		logisticsRoutesList,
		logisticsRouteDetail,
		logisticsVehiclesMap,
		notificationsUnread,
		notificationsList,
		dashboardSummary,
		bankReconciliationList,
		bankReconciliationDetail,
		chatConversationsList,
		chatMessages,
		chatUnreadTotal,
	}
}

func TestRegisterAll(t *testing.T) {
	engine := arcana.New(arcana.Config{})

	RegisterAll(engine)

	defs := allGraphDefs()
	for _, def := range defs {
		t.Run(def.Key, func(t *testing.T) {
			assert.NotEmpty(t, def.Key)
			assert.NotNil(t, def.Factory)
		})
	}

	reg := engine.Registry()
	require.NotNil(t, reg)
	assert.Equal(t, len(defs), reg.GraphCount())

	for _, def := range defs {
		got, ok := reg.Get(def.Key)
		require.True(t, ok, "graph %q should be registered", def.Key)
		assert.Equal(t, def.Key, got.Key)
	}
}

func TestRegisterAllNoDuplicateKeys(t *testing.T) {
	defs := allGraphDefs()
	seen := make(map[string]bool, len(defs))

	for _, def := range defs {
		assert.False(t, seen[def.Key], "duplicate graph key: %s", def.Key)
		seen[def.Key] = true
	}
}

func TestRegisterAllGraphsHaveDeps(t *testing.T) {
	for _, def := range allGraphDefs() {
		t.Run(def.Key, func(t *testing.T) {
			assert.NotEmpty(t, def.Deps, "graph %q should declare at least one table dependency", def.Key)
			for _, dep := range def.Deps {
				assert.NotEmpty(t, dep.Table)
				assert.NotEmpty(t, dep.Columns)
			}
		})
	}
}

func TestEventToChangesEntity(t *testing.T) {
	changes := EventToChanges("entity.created", map[string]any{"entity_id": "abc-123"})

	require.Len(t, changes, 1)
	assert.Equal(t, "entities", changes[0].Table)
	assert.Equal(t, "abc-123", changes[0].RowID)
	assert.Contains(t, changes[0].Columns, "name")
	assert.Contains(t, changes[0].Columns, "status")
}

func TestEventToChangesEntityMissingID(t *testing.T) {
	changes := EventToChanges("entity.updated", map[string]any{})
	assert.Nil(t, changes)
}

func TestEventToChangesComponent(t *testing.T) {
	changes := EventToChanges("component.updated", map[string]any{"entity_id": "ent-1"})

	require.Len(t, changes, 2)
	assert.Equal(t, "entities", changes[0].Table)
	assert.Equal(t, "ent-1", changes[0].RowID)
	assert.Equal(t, "components", changes[1].Table)
	assert.Equal(t, "ent-1", changes[1].RowID)
}

func TestEventToChangesOrder(t *testing.T) {
	changes := EventToChanges("order.created", map[string]any{"order_id": "ord-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "orders", changes[0].Table)
	assert.Equal(t, "ord-1", changes[0].RowID)
	assert.Contains(t, changes[0].Columns, "status")
	assert.Contains(t, changes[0].Columns, "total")
}

func TestEventToChangesOrderItem(t *testing.T) {
	changes := EventToChanges("order.item.added", map[string]any{
		"order_id": "ord-1",
		"item_id":  "item-1",
	})

	require.Len(t, changes, 2)
	assert.Equal(t, "orders", changes[0].Table)
	assert.Equal(t, "ord-1", changes[0].RowID)
	assert.Equal(t, "order_items", changes[1].Table)
	assert.Equal(t, "item-1", changes[1].RowID)
}

func TestEventToChangesOrderFallbackEntityID(t *testing.T) {
	changes := EventToChanges("order.paid", map[string]any{"entity_id": "ent-ord-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "orders", changes[0].Table)
	assert.Equal(t, "ent-ord-1", changes[0].RowID)
}

func TestEventToChangesOrderNoID(t *testing.T) {
	changes := EventToChanges("order.created", map[string]any{})
	assert.Nil(t, changes)
}

func TestEventToChangesCatalogProduct(t *testing.T) {
	changes := EventToChanges("catalog.product.updated", map[string]any{"entity_id": "prod-1"})

	require.Len(t, changes, 2)
	assert.Equal(t, "entities", changes[0].Table)
	assert.Equal(t, "components", changes[1].Table)
}

func TestEventToChangesCatalogCategory(t *testing.T) {
	changes := EventToChanges("catalog.category.updated", map[string]any{"entity_id": "cat-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "entities", changes[0].Table)
	assert.Contains(t, changes[0].Columns, "parent_id")
	assert.Contains(t, changes[0].Columns, "sort_order")
}

func TestEventToChangesWarehouseStock(t *testing.T) {
	changes := EventToChanges("warehouse.stock.updated", map[string]any{"product_id": "prod-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "stock_levels", changes[0].Table)
	assert.Equal(t, "prod-1", changes[0].RowID)
}

func TestEventToChangesWarehouseStockReceived(t *testing.T) {
	changes := EventToChanges("warehouse.stock.received", map[string]any{"product_id": "prod-1"})

	require.Len(t, changes, 2)
	assert.Equal(t, "stock_levels", changes[0].Table)
	assert.Equal(t, "stock_movements", changes[1].Table)
}

func TestEventToChangesWarehouseStockShipped(t *testing.T) {
	changes := EventToChanges("warehouse.stock.shipped", map[string]any{"entity_id": "prod-2"})

	require.Len(t, changes, 2)
	assert.Equal(t, "stock_levels", changes[0].Table)
	assert.Equal(t, "prod-2", changes[0].RowID)
	assert.Equal(t, "stock_movements", changes[1].Table)
}

func TestEventToChangesWarehouseStockAdjusted(t *testing.T) {
	changes := EventToChanges("warehouse.stock.adjusted", map[string]any{"product_id": "prod-3"})

	require.Len(t, changes, 2)
	assert.Equal(t, "stock_movements", changes[1].Table)
}

func TestEventToChangesWarehouseNonStock(t *testing.T) {
	changes := EventToChanges("warehouse.other", map[string]any{"entity_id": "x"})
	assert.Nil(t, changes)
}

func TestEventToChangesWarehouseStockNoID(t *testing.T) {
	changes := EventToChanges("warehouse.stock.updated", map[string]any{})
	assert.Nil(t, changes)
}

func TestEventToChangesHREmployee(t *testing.T) {
	changes := EventToChanges("hr.employee.updated", map[string]any{"entity_id": "emp-1"})

	require.Len(t, changes, 2)
	assert.Equal(t, "entities", changes[0].Table)
	assert.Equal(t, "components", changes[1].Table)
}

func TestEventToChangesHRShift(t *testing.T) {
	changes := EventToChanges("hr.shift.created", map[string]any{"entity_id": "shift-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "shifts", changes[0].Table)
	assert.Equal(t, "shift-1", changes[0].RowID)
}

func TestEventToChangesHRTimesheet(t *testing.T) {
	changes := EventToChanges("hr.timesheet.updated", map[string]any{"entity_id": "ts-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "timesheets", changes[0].Table)
}

func TestEventToChangesHRPayroll(t *testing.T) {
	changes := EventToChanges("hr.payroll.calculated", map[string]any{
		"entity_id":  "emp-1",
		"payroll_id": "pay-1",
	})

	require.Len(t, changes, 1)
	assert.Equal(t, "payrolls", changes[0].Table)
	assert.Equal(t, "pay-1", changes[0].RowID)
}

func TestEventToChangesHRPayrollFallback(t *testing.T) {
	changes := EventToChanges("hr.payroll.calculated", map[string]any{"entity_id": "emp-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "payrolls", changes[0].Table)
	assert.Equal(t, "emp-1", changes[0].RowID)
}

func TestEventToChangesHRAbsence(t *testing.T) {
	changes := EventToChanges("hr.absence.approved", map[string]any{
		"entity_id":  "emp-1",
		"absence_id": "abs-1",
	})

	require.Len(t, changes, 1)
	assert.Equal(t, "absences", changes[0].Table)
	assert.Equal(t, "abs-1", changes[0].RowID)
}

func TestEventToChangesHRNoEntityID(t *testing.T) {
	changes := EventToChanges("hr.shift.created", map[string]any{})
	assert.Nil(t, changes)
}

func TestEventToChangesFinanceTransaction(t *testing.T) {
	changes := EventToChanges("finance.transaction.posted", map[string]any{"entity_id": "txn-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "transactions", changes[0].Table)
	assert.Equal(t, "txn-1", changes[0].RowID)
}

func TestEventToChangesFinanceAccount(t *testing.T) {
	changes := EventToChanges("finance.account.created", map[string]any{"entity_id": "acc-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "accounts", changes[0].Table)
}

func TestEventToChangesFinancePeriod(t *testing.T) {
	changes := EventToChanges("finance.period.closed", map[string]any{
		"entity_id": "e-1",
		"period_id": "per-1",
	})

	require.Len(t, changes, 1)
	assert.Equal(t, "finance_periods", changes[0].Table)
	assert.Equal(t, "per-1", changes[0].RowID)
}

func TestEventToChangesFinanceCash(t *testing.T) {
	changes := EventToChanges("finance.cash.created", map[string]any{
		"entity_id":    "e-1",
		"operation_id": "op-1",
	})

	require.Len(t, changes, 1)
	assert.Equal(t, "cash_operations", changes[0].Table)
	assert.Equal(t, "op-1", changes[0].RowID)
}

func TestEventToChangesFinanceDefault(t *testing.T) {
	changes := EventToChanges("finance.something", map[string]any{"entity_id": "e-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "transactions", changes[0].Table)
}

func TestEventToChangesFinanceNoEntityID(t *testing.T) {
	changes := EventToChanges("finance.transaction.posted", map[string]any{})
	assert.Nil(t, changes)
}

func TestEventToChangesLogisticsRoute(t *testing.T) {
	changes := EventToChanges("logistics.route.started", map[string]any{"entity_id": "route-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "routes", changes[0].Table)
	assert.Equal(t, "route-1", changes[0].RowID)
}

func TestEventToChangesLogisticsRouteStop(t *testing.T) {
	changes := EventToChanges("logistics.route.stop", map[string]any{
		"entity_id": "route-1",
		"stop_id":   "stop-1",
	})

	require.Len(t, changes, 2)
	assert.Equal(t, "routes", changes[0].Table)
	assert.Equal(t, "route_stops", changes[1].Table)
	assert.Equal(t, "stop-1", changes[1].RowID)
}

func TestEventToChangesLogisticsGeo(t *testing.T) {
	changes := EventToChanges("logistics.geo.updated", map[string]any{"entity_id": "v-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "geo_tracks", changes[0].Table)
}

func TestEventToChangesLogisticsDefault(t *testing.T) {
	changes := EventToChanges("logistics.unknown", map[string]any{"entity_id": "r-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "routes", changes[0].Table)
}

func TestEventToChangesLogisticsNoEntityID(t *testing.T) {
	changes := EventToChanges("logistics.route.started", map[string]any{})
	assert.Nil(t, changes)
}

func TestEventToChangesCRM(t *testing.T) {
	changes := EventToChanges("crm.customer.created", map[string]any{"entity_id": "cust-1"})

	require.Len(t, changes, 2)
	assert.Equal(t, "entities", changes[0].Table)
	assert.Equal(t, "components", changes[1].Table)
}

func TestEventToChangesCRMNoEntityID(t *testing.T) {
	changes := EventToChanges("crm.customer.created", map[string]any{})
	assert.Nil(t, changes)
}

func TestEventToChangesSettings(t *testing.T) {
	changes := EventToChanges("settings.updated", map[string]any{"organization_id": "org-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "organization_settings", changes[0].Table)
	assert.Equal(t, "org-1", changes[0].RowID)
}

func TestEventToChangesSettingsNoOrgID(t *testing.T) {
	changes := EventToChanges("settings.updated", map[string]any{})
	assert.Nil(t, changes)
}

func TestEventToChangesNotification(t *testing.T) {
	changes := EventToChanges("notification.created", map[string]any{})

	require.Len(t, changes, 1)
	assert.Equal(t, "notifications", changes[0].Table)
	assert.Empty(t, changes[0].RowID)
}

func TestEventToChangesBankReconciliation(t *testing.T) {
	changes := EventToChanges("bank.reconciliation.completed", map[string]any{"reconciliation_id": "rec-1"})

	require.Len(t, changes, 2)
	assert.Equal(t, "bank_reconciliations", changes[0].Table)
	assert.Equal(t, "rec-1", changes[0].RowID)
	assert.Equal(t, "bank_reconciliation_entries", changes[1].Table)
}

func TestEventToChangesBankReconciliationNoID(t *testing.T) {
	changes := EventToChanges("bank.reconciliation.completed", map[string]any{})
	assert.Nil(t, changes)
}

func TestEventToChangesBankPayment(t *testing.T) {
	changes := EventToChanges("bank.payment.processed", map[string]any{})

	require.Len(t, changes, 1)
	assert.Equal(t, "orders", changes[0].Table)
}

func TestEventToChangesBankPayroll(t *testing.T) {
	changes := EventToChanges("bank.payroll.processed", map[string]any{})

	require.Len(t, changes, 1)
	assert.Equal(t, "payrolls", changes[0].Table)
}

func TestEventToChangesBankDefault(t *testing.T) {
	changes := EventToChanges("bank.unknown", map[string]any{})
	assert.Nil(t, changes)
}

func TestEventToChangesMessengerMessage(t *testing.T) {
	changes := EventToChanges("messenger.message.sent", map[string]any{"conversation_id": "conv-1"})

	require.Len(t, changes, 3)
	assert.Equal(t, "messages", changes[0].Table)
	assert.Equal(t, "conv-1", changes[0].RowID)
	assert.Equal(t, "conversations", changes[1].Table)
	assert.Equal(t, "conv-1", changes[1].RowID)
	assert.Equal(t, "conversation_members", changes[2].Table)
}

func TestEventToChangesMessengerMessageNoConvID(t *testing.T) {
	changes := EventToChanges("messenger.message.sent", map[string]any{})
	assert.Nil(t, changes)
}

func TestEventToChangesMessengerReaction(t *testing.T) {
	changes := EventToChanges("messenger.reaction.added", map[string]any{"message_id": "msg-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "message_reactions", changes[0].Table)
	assert.Equal(t, "msg-1", changes[0].RowID)
}

func TestEventToChangesMessengerReactionNoMsgID(t *testing.T) {
	changes := EventToChanges("messenger.reaction.added", map[string]any{})
	assert.Nil(t, changes)
}

func TestEventToChangesMessengerConversation(t *testing.T) {
	changes := EventToChanges("messenger.conversation.created", map[string]any{})

	require.Len(t, changes, 1)
	assert.Equal(t, "conversations", changes[0].Table)
}

func TestEventToChangesMessengerMember(t *testing.T) {
	changes := EventToChanges("messenger.member.added", map[string]any{"conversation_id": "conv-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "conversation_members", changes[0].Table)
	assert.Equal(t, "conv-1", changes[0].RowID)
}

func TestEventToChangesMessengerTyping(t *testing.T) {
	changes := EventToChanges("messenger.typing.started", map[string]any{"conversation_id": "conv-1"})

	require.Len(t, changes, 1)
	assert.Equal(t, "conversation_members", changes[0].Table)
}

func TestEventToChangesMessengerMemberNoConvID(t *testing.T) {
	changes := EventToChanges("messenger.member.added", map[string]any{})
	assert.Nil(t, changes)
}

func TestEventToChangesMessengerDefault(t *testing.T) {
	changes := EventToChanges("messenger.unknown", map[string]any{})
	assert.Nil(t, changes)
}

func TestEventToChangesUnknown(t *testing.T) {
	assert.Nil(t, EventToChanges("unknown.event", map[string]any{"entity_id": "x"}))
	assert.Nil(t, EventToChanges("foobar.baz.qux", map[string]any{}))
	assert.Nil(t, EventToChanges("x.y", nil))
}

func TestEventToChangesSinglePart(t *testing.T) {
	assert.Nil(t, EventToChanges("singlepart", map[string]any{}))
}

func TestEventToChangesEmptyString(t *testing.T) {
	assert.Nil(t, EventToChanges("", map[string]any{}))
}

func TestEventDataFromJSON(t *testing.T) {
	t.Run("valid json", func(t *testing.T) {
		raw := json.RawMessage(`{"entity_id":"abc-123","order_id":"ord-456"}`)
		m := EventDataFromJSON(raw)

		require.NotNil(t, m)
		assert.Equal(t, "abc-123", m["entity_id"])
		assert.Equal(t, "ord-456", m["order_id"])
	})

	t.Run("empty raw", func(t *testing.T) {
		m := EventDataFromJSON(json.RawMessage{})
		assert.Nil(t, m)
	})

	t.Run("nil raw", func(t *testing.T) {
		m := EventDataFromJSON(nil)
		assert.Nil(t, m)
	})

	t.Run("invalid json", func(t *testing.T) {
		m := EventDataFromJSON(json.RawMessage(`not json`))
		assert.Nil(t, m)
	})

	t.Run("nested json", func(t *testing.T) {
		raw := json.RawMessage(`{"entity_id":"e-1","meta":{"key":"val"}}`)
		m := EventDataFromJSON(raw)

		require.NotNil(t, m)
		assert.Equal(t, "e-1", m["entity_id"])
		meta, ok := m["meta"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "val", meta["key"])
	})

	t.Run("empty object", func(t *testing.T) {
		m := EventDataFromJSON(json.RawMessage(`{}`))
		require.NotNil(t, m)
		assert.Empty(t, m)
	})
}

func TestExtractID(t *testing.T) {
	tests := []struct {
		name  string
		data  map[string]any
		field string
		want  string
	}{
		{"string value", map[string]any{"id": "abc"}, "id", "abc"},
		{"missing field", map[string]any{}, "id", ""},
		{"non-string value", map[string]any{"id": 123}, "id", ""},
		{"nil data", nil, "id", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, extractID(tt.data, tt.field))
		})
	}
}
