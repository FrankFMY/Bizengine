package views

import (
	"encoding/json"
	"strings"

	"github.com/bizengine/engine/pkg/types"
)

// EventToChanges maps a domain event to the set of data changes it implies.
func EventToChanges(ev types.Event) []ChangeEvent {
	parts := strings.SplitN(ev.Type, ".", 3)
	if len(parts) < 2 {
		return nil
	}

	base := ChangeEvent{WorkspaceID: ev.WorkspaceID}

	switch parts[0] {
	case "entity":
		return entityChanges(ev, base)
	case "component":
		return componentChanges(ev, base)
	case "order":
		return orderChanges(ev, base)
	case "catalog":
		return catalogChanges(ev, base, parts)
	case "warehouse":
		return warehouseChanges(ev, base, parts)
	case "hr":
		return hrChanges(ev, base, parts)
	case "finance":
		return financeChanges(ev, base, parts)
	case "logistics":
		return logisticsChanges(ev, base, parts)
	default:
		return nil
	}
}

func entityChanges(ev types.Event, base ChangeEvent) []ChangeEvent {
	if ev.EntityID == nil {
		return nil
	}
	base.Table = "entities"
	base.RowID = ev.EntityID.String()
	base.ChangedColumns = []string{"name", "status", "meta", "updated_at"}
	return []ChangeEvent{base}
}

func componentChanges(ev types.Event, base ChangeEvent) []ChangeEvent {
	if ev.EntityID == nil {
		return nil
	}
	var changes []ChangeEvent

	// Entity itself may be affected.
	ec := base
	ec.Table = "entities"
	ec.RowID = ev.EntityID.String()
	ec.ChangedColumns = []string{"updated_at"}
	changes = append(changes, ec)

	// Extract component type from event data if available.
	var data struct {
		Type string `json:"type"`
	}
	if json.Unmarshal(ev.Data, &data) == nil && data.Type != "" {
		cc := base
		cc.Table = "components"
		cc.RowID = ev.EntityID.String()
		cc.ChangedColumns = []string{"data", "version", "updated_at"}
		changes = append(changes, cc)
	}

	return changes
}

func orderChanges(ev types.Event, base ChangeEvent) []ChangeEvent {
	if ev.EntityID == nil {
		return nil
	}
	base.Table = "orders"
	base.RowID = ev.EntityID.String()
	base.ChangedColumns = []string{"status", "total", "updated_at"}
	return []ChangeEvent{base}
}

func catalogChanges(ev types.Event, base ChangeEvent, parts []string) []ChangeEvent {
	if ev.EntityID == nil {
		return nil
	}
	if len(parts) >= 2 && parts[1] == "product" {
		base.Table = "catalog_products"
		base.RowID = ev.EntityID.String()
		base.ChangedColumns = []string{"name", "sku", "price", "status", "updated_at"}
		return []ChangeEvent{base}
	}
	if len(parts) >= 2 && parts[1] == "category" {
		base.Table = "catalog_categories"
		base.RowID = ev.EntityID.String()
		base.ChangedColumns = []string{"name", "updated_at"}
		return []ChangeEvent{base}
	}
	return nil
}

func warehouseChanges(ev types.Event, base ChangeEvent, parts []string) []ChangeEvent {
	if len(parts) >= 2 && parts[1] == "stock" {
		rowID := extractDataField(ev.Data, "product_id")
		if rowID == "" && ev.EntityID != nil {
			rowID = ev.EntityID.String()
		}
		if rowID == "" {
			return nil
		}
		base.Table = "stock_levels"
		base.RowID = rowID
		base.ChangedColumns = []string{"quantity", "reserved", "updated_at"}
		return []ChangeEvent{base}
	}
	return nil
}

func hrChanges(ev types.Event, base ChangeEvent, parts []string) []ChangeEvent {
	if ev.EntityID == nil {
		return nil
	}
	switch {
	case len(parts) >= 2 && parts[1] == "employee":
		base.Table = "employees"
	case len(parts) >= 2 && parts[1] == "shift":
		base.Table = "shifts"
	case len(parts) >= 2 && parts[1] == "timesheet":
		base.Table = "timesheets"
	default:
		base.Table = "employees"
	}
	base.RowID = ev.EntityID.String()
	base.ChangedColumns = []string{"status", "updated_at"}
	return []ChangeEvent{base}
}

func financeChanges(ev types.Event, base ChangeEvent, parts []string) []ChangeEvent {
	if ev.EntityID == nil {
		return nil
	}
	switch {
	case len(parts) >= 2 && parts[1] == "transaction":
		base.Table = "transactions"
	case len(parts) >= 2 && parts[1] == "invoice":
		base.Table = "invoices"
	case len(parts) >= 2 && parts[1] == "account":
		base.Table = "accounts"
	default:
		base.Table = "transactions"
	}
	base.RowID = ev.EntityID.String()
	base.ChangedColumns = []string{"status", "amount", "updated_at"}
	return []ChangeEvent{base}
}

func logisticsChanges(ev types.Event, base ChangeEvent, parts []string) []ChangeEvent {
	if ev.EntityID == nil {
		return nil
	}
	switch {
	case len(parts) >= 2 && parts[1] == "route":
		base.Table = "routes"
	case len(parts) >= 2 && parts[1] == "geo":
		base.Table = "geo_points"
	default:
		base.Table = "routes"
	}
	base.RowID = ev.EntityID.String()
	base.ChangedColumns = []string{"status", "updated_at"}
	return []ChangeEvent{base}
}

func extractDataField(data json.RawMessage, field string) string {
	var m map[string]any
	if json.Unmarshal(data, &m) != nil {
		return ""
	}
	v, ok := m[field]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}
