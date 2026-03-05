package logistics

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// Service provides logistics operations.
type Service struct {
	repo Repository
	bus  event.Bus
}

// NewService creates a new logistics service.
func NewService(repo Repository, bus event.Bus) *Service {
	return &Service{repo: repo, bus: bus}
}

// CreateRoute creates a new delivery route with stops.
func (s *Service) CreateRoute(ctx context.Context, orgID uuid.UUID, input CreateRouteInput, actorID *uuid.UUID) (*Route, error) {
	if input.Name == "" {
		return nil, errs.NewBadRequest("name is required")
	}

	route := &Route{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Name:           input.Name,
		VehicleID:      input.VehicleID,
		DriverID:       input.DriverID,
		Status:         "planned",
		PlannedStart:   input.PlannedStart,
		PlannedEnd:     input.PlannedEnd,
		CreatedAt:      time.Now(),
	}

	if err := s.repo.CreateRoute(ctx, route); err != nil {
		return nil, err
	}

	if len(input.Stops) > 0 {
		stops := make([]RouteStop, len(input.Stops))
		for i, stopInput := range input.Stops {
			stops[i] = RouteStop{
				ID:             uuid.New(),
				RouteID:        route.ID,
				OrganizationID: orgID,
				LocationID:     stopInput.LocationID,
				Address:        stopInput.Address,
				Latitude:       stopInput.Latitude,
				Longitude:      stopInput.Longitude,
				SortOrder:      i,
				PlannedArrival: stopInput.PlannedArrival,
				Status:         "pending",
				DeliveryIDs:    stopInput.DeliveryIDs,
				Notes:          stopInput.Notes,
			}
		}
		if err := s.repo.CreateStops(ctx, stops); err != nil {
			return nil, err
		}
		route.Stops = stops
	}

	s.publishEvent(ctx, orgID, "logistics.route.created", map[string]any{
		"route_id": route.ID.String(),
		"name":     route.Name,
	}, actorID)

	return route, nil
}

// GetRoute returns a route with its stops.
func (s *Service) GetRoute(ctx context.Context, orgID, routeID uuid.UUID) (*Route, error) {
	route, err := s.repo.GetRoute(ctx, orgID, routeID)
	if err != nil {
		return nil, err
	}
	stops, err := s.repo.ListStops(ctx, routeID)
	if err != nil {
		return nil, err
	}
	route.Stops = stops
	return route, nil
}

// ListRoutes returns routes matching the filter.
func (s *Service) ListRoutes(ctx context.Context, orgID uuid.UUID, filter RouteFilter) ([]Route, int, error) {
	filter.Page.Normalize()
	return s.repo.ListRoutes(ctx, orgID, filter)
}

// StartRoute transitions a route from planned to in_progress.
func (s *Service) StartRoute(ctx context.Context, orgID, routeID uuid.UUID, actorID *uuid.UUID) (*Route, error) {
	route, err := s.repo.GetRoute(ctx, orgID, routeID)
	if err != nil {
		return nil, err
	}
	if route.Status != "planned" {
		return nil, errs.NewConflict("route can only be started from planned status")
	}

	now := time.Now()
	route.Status = "in_progress"
	route.ActualStart = &now

	if err := s.repo.UpdateRoute(ctx, route); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, "logistics.route.started", map[string]any{
		"route_id": routeID.String(),
	}, actorID)

	return route, nil
}

// CompleteRoute transitions a route to completed.
func (s *Service) CompleteRoute(ctx context.Context, orgID, routeID uuid.UUID, actorID *uuid.UUID) (*Route, error) {
	route, err := s.repo.GetRoute(ctx, orgID, routeID)
	if err != nil {
		return nil, err
	}
	if route.Status != "in_progress" {
		return nil, errs.NewConflict("route can only be completed from in_progress status")
	}

	now := time.Now()
	route.Status = "completed"
	route.ActualEnd = &now

	if err := s.repo.UpdateRoute(ctx, route); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, "logistics.route.completed", map[string]any{
		"route_id": routeID.String(),
	}, actorID)

	return route, nil
}

// ArriveAtStop marks arrival at a route stop.
func (s *Service) ArriveAtStop(ctx context.Context, orgID, routeID, stopID uuid.UUID, actorID *uuid.UUID) (*RouteStop, error) {
	stop, err := s.repo.GetStop(ctx, orgID, stopID)
	if err != nil {
		return nil, err
	}
	if stop.RouteID != routeID {
		return nil, errs.NewNotFound("stop not found on this route")
	}
	if stop.Status != "pending" {
		return nil, errs.NewConflict("stop has already been visited")
	}

	// Enforce sequential order: all previous stops must be completed
	stops, err := s.repo.ListStops(ctx, routeID)
	if err != nil {
		return nil, err
	}
	for _, prev := range stops {
		if prev.SortOrder < stop.SortOrder && prev.Status != "completed" {
			return nil, errs.NewConflict("previous stops must be completed first")
		}
	}

	now := time.Now()
	stop.Status = "arrived"
	stop.ActualArrival = &now

	if err := s.repo.UpdateStop(ctx, stop); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, "logistics.stop.arrived", map[string]any{
		"route_id": routeID.String(),
		"stop_id":  stopID.String(),
	}, actorID)

	return stop, nil
}

// CompleteStop marks a route stop as completed.
func (s *Service) CompleteStop(ctx context.Context, orgID, routeID, stopID uuid.UUID, actorID *uuid.UUID) (*RouteStop, error) {
	stop, err := s.repo.GetStop(ctx, orgID, stopID)
	if err != nil {
		return nil, err
	}
	if stop.RouteID != routeID {
		return nil, errs.NewNotFound("stop not found on this route")
	}
	if stop.Status != "arrived" {
		return nil, errs.NewConflict("stop must be in arrived status to complete")
	}

	stop.Status = "completed"
	if err := s.repo.UpdateStop(ctx, stop); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, "logistics.stop.completed", map[string]any{
		"route_id":     routeID.String(),
		"stop_id":      stopID.String(),
		"delivery_ids": stop.DeliveryIDs,
	}, actorID)

	return stop, nil
}

// UpdateGeo records a GPS data point and publishes an event.
func (s *Service) UpdateGeo(ctx context.Context, orgID, entityID uuid.UUID, point GeoPoint, actorID *uuid.UUID) error {
	if point.RecordedAt.IsZero() {
		point.RecordedAt = time.Now()
	}

	if err := s.repo.InsertGeoPoint(ctx, orgID, entityID, point); err != nil {
		return err
	}

	s.publishEvent(ctx, orgID, "logistics.geo.updated", map[string]any{
		"entity_id": entityID.String(),
		"latitude":  point.Latitude,
		"longitude": point.Longitude,
		"speed":     point.Speed,
		"heading":   point.Heading,
	}, actorID)

	return nil
}

// GetTrack returns GPS track for an entity in a time range.
func (s *Service) GetTrack(ctx context.Context, orgID, entityID uuid.UUID, from, to time.Time) ([]GeoPoint, error) {
	return s.repo.GetTrack(ctx, orgID, entityID, from, to)
}

func (s *Service) publishEvent(ctx context.Context, orgID uuid.UUID, eventType string, data map[string]any, actorID *uuid.UUID) {
	payload, _ := json.Marshal(data)
	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Type:           eventType,
		Data:           payload,
		Timestamp:      time.Now(),
	}
	if actorID != nil {
		ev.ActorID = actorID
	}
	s.bus.Publish(ctx, ev)
}
