package views

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/pkg/types"
)

func ptrUUID(id uuid.UUID) *uuid.UUID { return &id }

func TestEventToChanges_Entity(t *testing.T) {
	eid := uuid.New()
	orgID := uuid.New()
	ev := types.Event{
		Type:        "entity.created",
		EntityID:    ptrUUID(eid),
		OrganizationID: orgID,
	}

	changes := EventToChanges(ev)
	require.Len(t, changes, 1)
	assert.Equal(t, "entities", changes[0].Table)
	assert.Equal(t, eid.String(), changes[0].RowID)
	assert.Equal(t, orgID, changes[0].OrganizationID)
}

func TestEventToChanges_Order(t *testing.T) {
	eid := uuid.New()
	ev := types.Event{
		Type:        "order.status_changed",
		EntityID:    ptrUUID(eid),
		OrganizationID: uuid.New(),
	}

	changes := EventToChanges(ev)
	require.Len(t, changes, 1)
	assert.Equal(t, "orders", changes[0].Table)
	assert.Equal(t, eid.String(), changes[0].RowID)
}

func TestEventToChanges_CatalogProduct(t *testing.T) {
	eid := uuid.New()
	ev := types.Event{
		Type:        "catalog.product.created",
		EntityID:    ptrUUID(eid),
		OrganizationID: uuid.New(),
	}

	changes := EventToChanges(ev)
	require.Len(t, changes, 1)
	assert.Equal(t, "catalog_products", changes[0].Table)
}

func TestEventToChanges_WarehouseStock(t *testing.T) {
	pid := uuid.New()
	ev := types.Event{
		Type:        "warehouse.stock.received",
		OrganizationID: uuid.New(),
		Data:        json.RawMessage(`{"product_id":"` + pid.String() + `"}`),
	}

	changes := EventToChanges(ev)
	require.Len(t, changes, 1)
	assert.Equal(t, "stock_levels", changes[0].Table)
	assert.Equal(t, pid.String(), changes[0].RowID)
}

func TestEventToChanges_Component(t *testing.T) {
	eid := uuid.New()
	ev := types.Event{
		Type:        "component.updated",
		EntityID:    ptrUUID(eid),
		OrganizationID: uuid.New(),
		Data:        json.RawMessage(`{"type":"pricing"}`),
	}

	changes := EventToChanges(ev)
	require.Len(t, changes, 2)
	assert.Equal(t, "entities", changes[0].Table)
	assert.Equal(t, "components", changes[1].Table)
}

func TestEventToChanges_HR(t *testing.T) {
	eid := uuid.New()
	ev := types.Event{
		Type:        "hr.employee.hired",
		EntityID:    ptrUUID(eid),
		OrganizationID: uuid.New(),
	}

	changes := EventToChanges(ev)
	require.Len(t, changes, 1)
	assert.Equal(t, "employees", changes[0].Table)
}

func TestEventToChanges_Finance(t *testing.T) {
	eid := uuid.New()
	ev := types.Event{
		Type:        "finance.transaction.posted",
		EntityID:    ptrUUID(eid),
		OrganizationID: uuid.New(),
	}

	changes := EventToChanges(ev)
	require.Len(t, changes, 1)
	assert.Equal(t, "transactions", changes[0].Table)
}

func TestEventToChanges_Logistics(t *testing.T) {
	eid := uuid.New()
	ev := types.Event{
		Type:        "logistics.route.started",
		EntityID:    ptrUUID(eid),
		OrganizationID: uuid.New(),
	}

	changes := EventToChanges(ev)
	require.Len(t, changes, 1)
	assert.Equal(t, "routes", changes[0].Table)
}

func TestEventToChanges_Unknown(t *testing.T) {
	ev := types.Event{Type: "unknown.event"}
	assert.Empty(t, EventToChanges(ev))
}

func TestEventToChanges_SinglePart(t *testing.T) {
	ev := types.Event{Type: "ping"}
	assert.Nil(t, EventToChanges(ev))
}
