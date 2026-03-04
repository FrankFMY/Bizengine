package logistics

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/pkg/types"
)

// Repository defines data access for the logistics module.
type Repository interface {
	// Routes
	CreateRoute(ctx context.Context, r *Route) error
	GetRoute(ctx context.Context, wsID, routeID uuid.UUID) (*Route, error)
	ListRoutes(ctx context.Context, wsID uuid.UUID, filter RouteFilter) ([]Route, int, error)
	UpdateRoute(ctx context.Context, r *Route) error

	// Stops
	CreateStops(ctx context.Context, stops []RouteStop) error
	GetStop(ctx context.Context, wsID, stopID uuid.UUID) (*RouteStop, error)
	ListStops(ctx context.Context, routeID uuid.UUID) ([]RouteStop, error)
	UpdateStop(ctx context.Context, s *RouteStop) error

	// Geo
	InsertGeoPoint(ctx context.Context, wsID, entityID uuid.UUID, point GeoPoint) error
	GetTrack(ctx context.Context, wsID, entityID uuid.UUID, from, to time.Time) ([]GeoPoint, error)
}

// Route represents a delivery route.
type Route struct {
	ID           uuid.UUID   `json:"id"`
	WorkspaceID  uuid.UUID   `json:"workspace_id"`
	Name         string      `json:"name"`
	VehicleID    *uuid.UUID  `json:"vehicle_id,omitempty"`
	DriverID     *uuid.UUID  `json:"driver_id,omitempty"`
	Status       string      `json:"status"`
	PlannedStart *time.Time  `json:"planned_start,omitempty"`
	PlannedEnd   *time.Time  `json:"planned_end,omitempty"`
	ActualStart  *time.Time  `json:"actual_start,omitempty"`
	ActualEnd    *time.Time  `json:"actual_end,omitempty"`
	CreatedAt    time.Time   `json:"created_at"`
	Stops        []RouteStop `json:"stops,omitempty"`
}

// RouteStop represents a stop on a route.
type RouteStop struct {
	ID             uuid.UUID   `json:"id"`
	RouteID        uuid.UUID   `json:"route_id"`
	WorkspaceID    uuid.UUID   `json:"workspace_id"`
	LocationID     *uuid.UUID  `json:"location_id,omitempty"`
	Address        string      `json:"address"`
	Latitude       *float64    `json:"latitude,omitempty"`
	Longitude      *float64    `json:"longitude,omitempty"`
	SortOrder      int         `json:"sort_order"`
	PlannedArrival *time.Time  `json:"planned_arrival,omitempty"`
	ActualArrival  *time.Time  `json:"actual_arrival,omitempty"`
	Status         string      `json:"status"`
	DeliveryIDs    []uuid.UUID `json:"delivery_ids,omitempty"`
	Notes          string      `json:"notes"`
}

// GeoPoint represents a GPS data point.
type GeoPoint struct {
	Latitude   float64   `json:"latitude"`
	Longitude  float64   `json:"longitude"`
	Speed      float64   `json:"speed"`
	Heading    float64   `json:"heading"`
	RecordedAt time.Time `json:"recorded_at"`
}

// RouteFilter holds query params for listing routes.
type RouteFilter struct {
	Status   *string
	DriverID *uuid.UUID
	Page     types.PageRequest
}

// CreateRouteInput is the input for creating a route.
type CreateRouteInput struct {
	Name         string           `json:"name"`
	VehicleID    *uuid.UUID       `json:"vehicle_id"`
	DriverID     *uuid.UUID       `json:"driver_id"`
	PlannedStart *time.Time       `json:"planned_start"`
	PlannedEnd   *time.Time       `json:"planned_end"`
	Stops        []CreateStopInput `json:"stops"`
}

// CreateStopInput is the input for a route stop.
type CreateStopInput struct {
	LocationID     *uuid.UUID  `json:"location_id"`
	Address        string      `json:"address"`
	Latitude       *float64    `json:"latitude"`
	Longitude      *float64    `json:"longitude"`
	PlannedArrival *time.Time  `json:"planned_arrival"`
	DeliveryIDs    []uuid.UUID `json:"delivery_ids"`
	Notes          string      `json:"notes"`
}
