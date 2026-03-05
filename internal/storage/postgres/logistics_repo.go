package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/module/logistics"
	"github.com/bizengine/engine/pkg/errs"
)

// LogisticsRepo implements logistics.Repository using PostgreSQL.
type LogisticsRepo struct {
	pool *pgxpool.Pool
}

// NewLogisticsRepo creates a new LogisticsRepo.
func NewLogisticsRepo(pool *pgxpool.Pool) *LogisticsRepo {
	return &LogisticsRepo{pool: pool}
}

// CreateRoute inserts a route record.
func (r *LogisticsRepo) CreateRoute(ctx context.Context, route *logistics.Route) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO routes (id, organization_id, name, vehicle_id, driver_id, status, planned_start, planned_end, actual_start, actual_end, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		route.ID, route.OrganizationID, route.Name, route.VehicleID, route.DriverID,
		route.Status, route.PlannedStart, route.PlannedEnd, route.ActualStart, route.ActualEnd, route.CreatedAt,
	)
	return err
}

// GetRoute returns a route by ID.
func (r *LogisticsRepo) GetRoute(ctx context.Context, orgID, routeID uuid.UUID) (*logistics.Route, error) {
	var route logistics.Route
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, name, vehicle_id, driver_id, status, planned_start, planned_end, actual_start, actual_end, created_at
		 FROM routes WHERE organization_id = $1 AND id = $2`,
		orgID, routeID,
	).Scan(&route.ID, &route.OrganizationID, &route.Name, &route.VehicleID, &route.DriverID,
		&route.Status, &route.PlannedStart, &route.PlannedEnd, &route.ActualStart, &route.ActualEnd, &route.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("route not found")
		}
		return nil, err
	}
	return &route, nil
}

// ListRoutes returns routes matching the filter.
func (r *LogisticsRepo) ListRoutes(ctx context.Context, orgID uuid.UUID, filter logistics.RouteFilter) ([]logistics.Route, int, error) {
	filter.Page.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("organization_id = $%d", argIdx))
	args = append(args, orgID)
	argIdx++

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.DriverID != nil {
		conditions = append(conditions, fmt.Sprintf("driver_id = $%d", argIdx))
		args = append(args, *filter.DriverID)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM routes WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT id, organization_id, name, vehicle_id, driver_id, status, planned_start, planned_end, actual_start, actual_end, created_at
		 FROM routes WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, filter.Page.Limit, filter.Page.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var routes []logistics.Route
	for rows.Next() {
		var route logistics.Route
		if err := rows.Scan(&route.ID, &route.OrganizationID, &route.Name, &route.VehicleID, &route.DriverID,
			&route.Status, &route.PlannedStart, &route.PlannedEnd, &route.ActualStart, &route.ActualEnd, &route.CreatedAt); err != nil {
			return nil, 0, err
		}
		routes = append(routes, route)
	}
	return routes, total, rows.Err()
}

// UpdateRoute updates a route record.
func (r *LogisticsRepo) UpdateRoute(ctx context.Context, route *logistics.Route) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE routes SET status = $3, actual_start = $4, actual_end = $5
		 WHERE organization_id = $1 AND id = $2`,
		route.OrganizationID, route.ID, route.Status, route.ActualStart, route.ActualEnd,
	)
	return err
}

// CreateStops inserts route stop records.
func (r *LogisticsRepo) CreateStops(ctx context.Context, stops []logistics.RouteStop) error {
	for _, s := range stops {
		if _, err := r.pool.Exec(ctx,
			`INSERT INTO route_stops (id, route_id, organization_id, location_id, address, latitude, longitude, sort_order, planned_arrival, actual_arrival, status, delivery_ids, notes)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
			s.ID, s.RouteID, s.OrganizationID, s.LocationID, s.Address, s.Latitude, s.Longitude,
			s.SortOrder, s.PlannedArrival, s.ActualArrival, s.Status, s.DeliveryIDs, s.Notes,
		); err != nil {
			return err
		}
	}
	return nil
}

// GetStop returns a route stop by ID.
func (r *LogisticsRepo) GetStop(ctx context.Context, orgID, stopID uuid.UUID) (*logistics.RouteStop, error) {
	var s logistics.RouteStop
	err := r.pool.QueryRow(ctx,
		`SELECT id, route_id, organization_id, location_id, address, latitude, longitude, sort_order, planned_arrival, actual_arrival, status, delivery_ids, notes
		 FROM route_stops WHERE organization_id = $1 AND id = $2`,
		orgID, stopID,
	).Scan(&s.ID, &s.RouteID, &s.OrganizationID, &s.LocationID, &s.Address, &s.Latitude, &s.Longitude,
		&s.SortOrder, &s.PlannedArrival, &s.ActualArrival, &s.Status, &s.DeliveryIDs, &s.Notes)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("stop not found")
		}
		return nil, err
	}
	return &s, nil
}

// ListStops returns stops for a route ordered by sort_order.
func (r *LogisticsRepo) ListStops(ctx context.Context, routeID uuid.UUID) ([]logistics.RouteStop, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, route_id, organization_id, location_id, address, latitude, longitude, sort_order, planned_arrival, actual_arrival, status, delivery_ids, notes
		 FROM route_stops WHERE route_id = $1 ORDER BY sort_order`,
		routeID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stops []logistics.RouteStop
	for rows.Next() {
		var s logistics.RouteStop
		if err := rows.Scan(&s.ID, &s.RouteID, &s.OrganizationID, &s.LocationID, &s.Address, &s.Latitude, &s.Longitude,
			&s.SortOrder, &s.PlannedArrival, &s.ActualArrival, &s.Status, &s.DeliveryIDs, &s.Notes); err != nil {
			return nil, err
		}
		stops = append(stops, s)
	}
	return stops, rows.Err()
}

// UpdateStop updates a route stop record.
func (r *LogisticsRepo) UpdateStop(ctx context.Context, s *logistics.RouteStop) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE route_stops SET status = $3, actual_arrival = $4
		 WHERE organization_id = $1 AND id = $2`,
		s.OrganizationID, s.ID, s.Status, s.ActualArrival,
	)
	return err
}

// InsertGeoPoint inserts a GPS data point.
func (r *LogisticsRepo) InsertGeoPoint(ctx context.Context, orgID, entityID uuid.UUID, point logistics.GeoPoint) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO geo_tracks (organization_id, entity_id, recorded_at, latitude, longitude, speed, heading)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)
		 ON CONFLICT (entity_id, recorded_at) DO UPDATE SET latitude = $4, longitude = $5, speed = $6, heading = $7`,
		orgID, entityID, point.RecordedAt, point.Latitude, point.Longitude, point.Speed, point.Heading,
	)
	return err
}

// GetTrack returns GPS track for an entity in a time range.
func (r *LogisticsRepo) GetTrack(ctx context.Context, orgID, entityID uuid.UUID, from, to time.Time) ([]logistics.GeoPoint, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT latitude, longitude, speed, heading, recorded_at
		 FROM geo_tracks WHERE organization_id = $1 AND entity_id = $2 AND recorded_at >= $3 AND recorded_at <= $4
		 ORDER BY recorded_at`,
		orgID, entityID, from, to,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []logistics.GeoPoint
	for rows.Next() {
		var p logistics.GeoPoint
		if err := rows.Scan(&p.Latitude, &p.Longitude, &p.Speed, &p.Heading, &p.RecordedAt); err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, rows.Err()
}
