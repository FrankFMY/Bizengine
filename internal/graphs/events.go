package graphs

import (
	"encoding/json"
	"strings"

	"github.com/FrankFMY/arcana"
)

// EventToChanges maps a BizEngine event type and its data to Arcana change notifications.
func EventToChanges(eventType string, data map[string]any) []arcana.Change {
	parts := strings.SplitN(eventType, ".", 3)
	if len(parts) < 2 {
		return nil
	}

	entityID, _ := data["entity_id"].(string)

	switch parts[0] {
	case "entity":
		if entityID == "" {
			return nil
		}
		return []arcana.Change{
			{Table: "entities", RowID: entityID, Columns: []string{"name", "status", "meta", "updated_at"}},
		}

	case "component":
		if entityID == "" {
			return nil
		}
		return []arcana.Change{
			{Table: "entities", RowID: entityID, Columns: []string{"updated_at"}},
			{Table: "components", RowID: entityID, Columns: []string{"data", "version", "updated_at"}},
		}

	case "order":
		rowID := extractID(data, "order_id")
		if rowID == "" {
			rowID = entityID
		}
		if rowID == "" {
			return nil
		}
		changes := []arcana.Change{
			{Table: "orders", RowID: rowID, Columns: []string{"status", "total", "updated_at"}},
		}
		if len(parts) >= 2 && parts[1] == "item" {
			itemID := extractID(data, "item_id")
			if itemID != "" {
				changes = append(changes, arcana.Change{
					Table: "order_items", RowID: itemID, Columns: []string{"quantity", "unit_price", "total"},
				})
			}
		}
		return changes

	case "catalog":
		if entityID == "" {
			return nil
		}
		if len(parts) >= 2 && parts[1] == "category" {
			return []arcana.Change{
				{Table: "entities", RowID: entityID, Columns: []string{"name", "parent_id", "sort_order"}},
			}
		}
		return []arcana.Change{
			{Table: "entities", RowID: entityID, Columns: []string{"name", "status"}},
			{Table: "components", RowID: entityID, Columns: []string{"data"}},
		}

	case "warehouse":
		if len(parts) >= 2 && parts[1] == "stock" {
			productID := extractID(data, "product_id")
			if productID == "" {
				productID = entityID
			}
			if productID == "" {
				return nil
			}
			changes := []arcana.Change{
				{Table: "stock_levels", RowID: productID, Columns: []string{"quantity", "reserved", "updated_at"}},
			}
			if len(parts) >= 3 && (parts[2] == "received" || parts[2] == "shipped" || parts[2] == "adjusted") {
				changes = append(changes, arcana.Change{
					Table: "stock_movements", RowID: productID, Columns: []string{"quantity", "type", "created_at"},
				})
			}
			return changes
		}
		return nil

	case "hr":
		if entityID == "" {
			return nil
		}
		switch {
		case len(parts) >= 2 && parts[1] == "shift":
			return []arcana.Change{
				{Table: "shifts", RowID: entityID, Columns: []string{"start_time", "end_time", "status"}},
			}
		case len(parts) >= 2 && parts[1] == "timesheet":
			return []arcana.Change{
				{Table: "timesheets", RowID: entityID, Columns: []string{"clock_in", "clock_out", "status"}},
			}
		default:
			return []arcana.Change{
				{Table: "entities", RowID: entityID, Columns: []string{"name", "status"}},
				{Table: "components", RowID: entityID, Columns: []string{"data"}},
			}
		}

	case "finance":
		if entityID == "" {
			return nil
		}
		switch {
		case len(parts) >= 2 && parts[1] == "transaction":
			return []arcana.Change{
				{Table: "transactions", RowID: entityID, Columns: []string{"date", "description", "is_posted"}},
			}
		case len(parts) >= 2 && parts[1] == "account":
			return []arcana.Change{
				{Table: "accounts", RowID: entityID, Columns: []string{"code", "name", "type"}},
			}
		default:
			return []arcana.Change{
				{Table: "transactions", RowID: entityID, Columns: []string{"date", "description", "is_posted"}},
			}
		}

	case "logistics":
		if entityID == "" {
			return nil
		}
		switch {
		case len(parts) >= 2 && parts[1] == "route":
			changes := []arcana.Change{
				{Table: "routes", RowID: entityID, Columns: []string{"status", "planned_start", "actual_start"}},
			}
			if len(parts) >= 3 && parts[2] == "stop" {
				stopID := extractID(data, "stop_id")
				if stopID != "" {
					changes = append(changes, arcana.Change{
						Table: "route_stops", RowID: stopID, Columns: []string{"status", "actual_arrival"},
					})
				}
			}
			return changes
		case len(parts) >= 2 && parts[1] == "geo":
			return []arcana.Change{
				{Table: "geo_tracks", RowID: entityID, Columns: []string{"latitude", "longitude"}},
			}
		default:
			return []arcana.Change{
				{Table: "routes", RowID: entityID, Columns: []string{"status", "updated_at"}},
			}
		}

	case "notification":
		return []arcana.Change{
			{Table: "notifications", Columns: []string{"read", "iat"}},
		}

	default:
		return nil
	}
}

func extractID(data map[string]any, field string) string {
	v, ok := data[field]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

// EventDataFromJSON converts json.RawMessage to a map for EventToChanges.
func EventDataFromJSON(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return m
}
