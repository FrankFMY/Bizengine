package graphs

import (
	"context"
	"fmt"

	"github.com/FrankFMY/arcana"
)

var logisticsRoutesList = arcana.GraphDef{
	Key: "logistics_routes_list",
	Deps: []arcana.TableDep{
		{Table: "routes", Columns: []string{"status", "planned_start", "actual_start"}},
		{Table: "entities", Columns: []string{"name"}},
	},
	Params: arcana.ParamSchema{
		"status": arcana.ParamString().Build(),
		"limit":  arcana.ParamInt().Default(50),
		"offset": arcana.ParamInt().Default(0),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		limit := p.Int("limit")
		offset := p.Int("offset")
		status := p.String("status")

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

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()

		for rows.Next() {
			var id, name, st, vehicleName, driverName string
			var plannedStart, actualStart any
			var stopCount, cnt int

			if err := rows.Scan(&id, &name, &st, &vehicleName, &driverName, &plannedStart, &actualStart, &stopCount, &cnt); err != nil {
				return nil, err
			}

			result.AddRef(arcana.Ref{Table: "routes", ID: id, Fields: []string{"name", "status", "vehicle_name", "driver_name", "stop_count"}})
			result.AddRow("routes", id, map[string]any{
				"id":            id,
				"name":          name,
				"status":        st,
				"vehicle_name":  vehicleName,
				"driver_name":   driverName,
				"planned_start": plannedStart,
				"actual_start":  actualStart,
				"stop_count":    stopCount,
			})
		}

		return result, rows.Err()
	},
}

var logisticsRouteDetail = arcana.GraphDef{
	Key: "logistics_route_detail",
	Deps: []arcana.TableDep{
		{Table: "routes", Columns: []string{"status", "planned_start", "actual_start", "actual_end"}},
		{Table: "route_stops", Columns: []string{"status", "actual_arrival"}},
	},
	Params: arcana.ParamSchema{
		"route_id": arcana.ParamUUID().Required(),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		routeID := p.UUID("route_id")

		result := arcana.NewResult()

		var id, name, status string
		var vehicleName, driverName *string
		var plannedStart, plannedEnd, actualStart, actualEnd any

		err := q.QueryRow(ctx, `
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

		result.AddRef(arcana.Ref{Table: "routes", ID: id, Fields: []string{"name", "status", "vehicle_name", "driver_name"}})
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
		result.AddRow("routes", id, route)

		stopRows, err := q.Query(ctx, `
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

			result.AddRef(arcana.Ref{Table: "route_stops", ID: stopID, Fields: []string{"address", "status", "sort_order"}})
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
			result.AddRow("route_stops", stopID, stop)
		}

		return result, nil
	},
}

var logisticsVehiclesMap = arcana.GraphDef{
	Key: "logistics_vehicles_map",
	Deps: []arcana.TableDep{
		{Table: "geo_tracks", Columns: []string{"latitude", "longitude"}},
		{Table: "entities", Columns: []string{"name"}},
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)

		rows, err := q.Query(ctx, `
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

		result := arcana.NewResult()

		for rows.Next() {
			var entityID, name string
			var lat, lon, speed float64
			var recordedAt any

			if err := rows.Scan(&entityID, &name, &lat, &lon, &speed, &recordedAt); err != nil {
				return nil, err
			}

			result.AddRef(arcana.Ref{Table: "geo_points", ID: entityID, Fields: []string{"name", "latitude", "longitude", "speed"}})
			result.AddRow("geo_points", entityID, map[string]any{
				"entity_id":   entityID,
				"name":        name,
				"latitude":    lat,
				"longitude":   lon,
				"speed":       speed,
				"recorded_at": recordedAt,
			})
		}

		return result, rows.Err()
	},
}
