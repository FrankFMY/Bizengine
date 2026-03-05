package defs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/views"
)

// LogisticsRoutesList returns a list of routes.
var LogisticsRoutesList = &views.ViewDef{
	Key: "logistics_routes_list",
	Tables: []views.TableDep{
		{Table: "routes", Columns: []string{"status", "planned_start", "actual_start"}},
		{Table: "entities", Columns: []string{"name"}, Filter: "kind=vehicle"},
	},
	ParamSchema: map[string]string{
		"status": "string",
	},
	Factory: logisticsRoutesListFactory,
}

func logisticsRoutesListFactory(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, params map[string]any) (*views.ViewResult, error) {
	limit := intParam(params, "limit", 50)
	offset := intParam(params, "offset", 0)
	status, _ := params["status"].(string)

	query := `
		SELECT r.id, r.name, r.status,
		       COALESCE(v.name, '') AS vehicle_name,
		       COALESCE(d.name, '') AS driver_name,
		       r.planned_start, r.actual_start,
		       (SELECT COUNT(*) FROM route_stops rs WHERE rs.route_id = r.id) AS stop_count,
		       COUNT(*) OVER() AS total_count
		FROM routes r
		LEFT JOIN entities v ON v.id = r.vehicle_id AND v.organization_id = $1
		LEFT JOIN entities d ON d.id = r.driver_id AND d.organization_id = $1
		WHERE r.organization_id = $1
	`
	args := []any{orgID}
	argIdx := 2

	if status != "" {
		query += fmt.Sprintf(` AND r.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY r.planned_start DESC NULLS LAST LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]views.DataRef, 0)
	tables := map[string]map[string]any{"routes": {}}

	for rows.Next() {
		var id, name, st, vehicleName, driverName string
		var plannedStart, actualStart any
		var stopCount, cnt int

		if err := rows.Scan(&id, &name, &st, &vehicleName, &driverName, &plannedStart, &actualStart, &stopCount, &cnt); err != nil {
			return nil, err
		}

		refs = append(refs, views.DataRef{Table: "routes", ID: id, Fields: []string{"name", "status", "vehicle_name", "driver_name", "stop_count"}})
		tables["routes"][id] = map[string]any{
			"id":            id,
			"name":          name,
			"status":        st,
			"vehicle_name":  vehicleName,
			"driver_name":   driverName,
			"planned_start": plannedStart,
			"actual_start":  actualStart,
			"stop_count":    stopCount,
		}
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}

// LogisticsRouteDetail returns a route with its stops.
var LogisticsRouteDetail = &views.ViewDef{
	Key: "logistics_route_detail",
	Tables: []views.TableDep{
		{Table: "routes", Columns: []string{"status", "planned_start", "actual_start", "actual_end"}},
		{Table: "route_stops", Columns: []string{"status", "actual_arrival"}},
	},
	ParamSchema: map[string]string{
		"route_id": "uuid",
	},
	Factory: logisticsRouteDetailFactory,
}

func logisticsRouteDetailFactory(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, params map[string]any) (*views.ViewResult, error) {
	routeID, _ := params["route_id"].(string)

	tables := map[string]map[string]any{"routes": {}, "route_stops": {}}
	refs := make([]views.DataRef, 0, 1)

	var id, name, status string
	var vehicleName, driverName *string
	var plannedStart, plannedEnd, actualStart, actualEnd any

	err := pool.QueryRow(ctx, `
		SELECT r.id, r.name, r.status,
		       v.name, d.name,
		       r.planned_start, r.planned_end, r.actual_start, r.actual_end
		FROM routes r
		LEFT JOIN entities v ON v.id = r.vehicle_id AND v.organization_id = $1
		LEFT JOIN entities d ON d.id = r.driver_id AND d.organization_id = $1
		WHERE r.organization_id = $1 AND r.id = $2
	`, orgID, routeID).Scan(&id, &name, &status, &vehicleName, &driverName,
		&plannedStart, &plannedEnd, &actualStart, &actualEnd)
	if err != nil {
		return nil, err
	}

	refs = append(refs, views.DataRef{Table: "routes", ID: id, Fields: []string{"name", "status", "vehicle_name", "driver_name"}})
	route := map[string]any{
		"id":            id,
		"name":          name,
		"status":        status,
		"planned_start": plannedStart,
		"planned_end":   plannedEnd,
		"actual_start":  actualStart,
		"actual_end":    actualEnd,
	}
	if vehicleName != nil {
		route["vehicle_name"] = *vehicleName
	}
	if driverName != nil {
		route["driver_name"] = *driverName
	}
	tables["routes"][id] = route

	// Stops
	stopRows, err := pool.Query(ctx, `
		SELECT id, address, latitude, longitude, sort_order,
		       planned_arrival, actual_arrival, status, notes
		FROM route_stops
		WHERE organization_id = $1 AND route_id = $2
		ORDER BY sort_order
	`, orgID, routeID)
	if err != nil {
		return nil, err
	}
	defer stopRows.Close()

	for stopRows.Next() {
		var stopID, addr, stopStatus, notes string
		var lat, lon *float64
		var sortOrder int
		var plannedArr, actualArr any

		if err := stopRows.Scan(&stopID, &addr, &lat, &lon, &sortOrder, &plannedArr, &actualArr, &stopStatus, &notes); err != nil {
			return nil, err
		}

		refs = append(refs, views.DataRef{Table: "route_stops", ID: stopID, Fields: []string{"address", "status", "sort_order"}})
		stop := map[string]any{
			"id":              stopID,
			"address":         addr,
			"sort_order":      sortOrder,
			"planned_arrival": plannedArr,
			"actual_arrival":  actualArr,
			"status":          stopStatus,
			"notes":           notes,
		}
		if lat != nil {
			stop["latitude"] = *lat
		}
		if lon != nil {
			stop["longitude"] = *lon
		}
		tables["route_stops"][stopID] = stop
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}

// LogisticsVehiclesMap returns current positions of vehicles.
var LogisticsVehiclesMap = &views.ViewDef{
	Key: "logistics_vehicles_map",
	Tables: []views.TableDep{
		{Table: "geo_points", Columns: []string{"latitude", "longitude"}},
		{Table: "entities", Columns: []string{"name"}, Filter: "kind=vehicle"},
	},
	Factory: logisticsVehiclesMapFactory,
}

func logisticsVehiclesMapFactory(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, _ map[string]any) (*views.ViewResult, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT ON (gt.entity_id)
		       gt.entity_id, e.name, gt.latitude, gt.longitude, gt.speed, gt.recorded_at
		FROM geo_tracks gt
		JOIN entities e ON e.id = gt.entity_id AND e.organization_id = $1 AND e.kind = 'vehicle'
		WHERE gt.organization_id = $1
		ORDER BY gt.entity_id, gt.recorded_at DESC
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]views.DataRef, 0)
	tables := map[string]map[string]any{"geo_points": {}}

	for rows.Next() {
		var entityID, name string
		var lat, lon, speed float64
		var recordedAt any

		if err := rows.Scan(&entityID, &name, &lat, &lon, &speed, &recordedAt); err != nil {
			return nil, err
		}

		refs = append(refs, views.DataRef{Table: "geo_points", ID: entityID, Fields: []string{"name", "latitude", "longitude", "speed"}})
		tables["geo_points"][entityID] = map[string]any{
			"entity_id":   entityID,
			"name":        name,
			"latitude":    lat,
			"longitude":   lon,
			"speed":       speed,
			"recorded_at": recordedAt,
		}
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}
