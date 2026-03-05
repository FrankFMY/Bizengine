package logistics

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/types"
)

// --- mocks ---

type mockLogisticsRepo struct {
	routes map[uuid.UUID]*Route
	stops  map[uuid.UUID]*RouteStop
	geo    map[string][]GeoPoint // key: entityID
}

func newMockRepo() *mockLogisticsRepo {
	return &mockLogisticsRepo{
		routes: make(map[uuid.UUID]*Route),
		stops:  make(map[uuid.UUID]*RouteStop),
		geo:    make(map[string][]GeoPoint),
	}
}

func (m *mockLogisticsRepo) CreateRoute(_ context.Context, r *Route) error {
	cp := *r
	m.routes[r.ID] = &cp
	return nil
}

func (m *mockLogisticsRepo) GetRoute(_ context.Context, orgID, routeID uuid.UUID) (*Route, error) {
	r, ok := m.routes[routeID]
	if !ok || r.OrganizationID != orgID {
		return nil, assert.AnError
	}
	cp := *r
	return &cp, nil
}

func (m *mockLogisticsRepo) ListRoutes(_ context.Context, orgID uuid.UUID, filter RouteFilter) ([]Route, int, error) {
	var result []Route
	for _, r := range m.routes {
		if r.OrganizationID != orgID {
			continue
		}
		if filter.Status != nil && r.Status != *filter.Status {
			continue
		}
		if filter.DriverID != nil && (r.DriverID == nil || *r.DriverID != *filter.DriverID) {
			continue
		}
		result = append(result, *r)
	}
	return result, len(result), nil
}

func (m *mockLogisticsRepo) UpdateRoute(_ context.Context, r *Route) error {
	if _, ok := m.routes[r.ID]; !ok {
		return assert.AnError
	}
	cp := *r
	m.routes[r.ID] = &cp
	return nil
}

func (m *mockLogisticsRepo) CreateStops(_ context.Context, stops []RouteStop) error {
	for _, s := range stops {
		cp := s
		m.stops[s.ID] = &cp
	}
	return nil
}

func (m *mockLogisticsRepo) GetStop(_ context.Context, orgID, stopID uuid.UUID) (*RouteStop, error) {
	s, ok := m.stops[stopID]
	if !ok || s.OrganizationID != orgID {
		return nil, assert.AnError
	}
	cp := *s
	return &cp, nil
}

func (m *mockLogisticsRepo) ListStops(_ context.Context, routeID uuid.UUID) ([]RouteStop, error) {
	var result []RouteStop
	for _, s := range m.stops {
		if s.RouteID == routeID {
			result = append(result, *s)
		}
	}
	return result, nil
}

func (m *mockLogisticsRepo) UpdateStop(_ context.Context, s *RouteStop) error {
	if _, ok := m.stops[s.ID]; !ok {
		return assert.AnError
	}
	cp := *s
	m.stops[s.ID] = &cp
	return nil
}

func (m *mockLogisticsRepo) InsertGeoPoint(_ context.Context, _ uuid.UUID, entityID uuid.UUID, point GeoPoint) error {
	key := entityID.String()
	m.geo[key] = append(m.geo[key], point)
	return nil
}

func (m *mockLogisticsRepo) GetTrack(_ context.Context, _ uuid.UUID, entityID uuid.UUID, from, to time.Time) ([]GeoPoint, error) {
	key := entityID.String()
	var result []GeoPoint
	for _, p := range m.geo[key] {
		if !p.RecordedAt.Before(from) && !p.RecordedAt.After(to) {
			result = append(result, p)
		}
	}
	return result, nil
}

type mockBus struct {
	published []types.Event
}

func (m *mockBus) Publish(_ context.Context, ev types.Event) error {
	m.published = append(m.published, ev)
	return nil
}
func (m *mockBus) Subscribe(_ string, _ event.Subscriber)        {}
func (m *mockBus) SubscribePattern(_ string, _ event.Subscriber) {}
func (m *mockBus) SubscribeAll(_ event.Subscriber)               {}

// --- helpers ---

func newTestService() (*Service, *mockLogisticsRepo, *mockBus) {
	repo := newMockRepo()
	bus := &mockBus{}
	svc := NewService(repo, bus)
	return svc, repo, bus
}

func ptr[T any](v T) *T { return &v }

// --- tests ---

func TestCreateRoute(t *testing.T) {
	svc, _, bus := newTestService()
	orgID := uuid.New()
	actorID := uuid.New()

	input := CreateRouteInput{
		Name:      "Route A",
		VehicleID: ptr(uuid.New()),
		DriverID:  ptr(uuid.New()),
		Stops: []CreateStopInput{
			{Address: "Stop 1", Latitude: ptr(55.75), Longitude: ptr(37.62)},
			{Address: "Stop 2", Latitude: ptr(55.80), Longitude: ptr(37.65)},
		},
	}

	route, err := svc.CreateRoute(context.Background(), orgID, input, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "Route A", route.Name)
	assert.Equal(t, "planned", route.Status)
	assert.Equal(t, orgID, route.OrganizationID)
	assert.Len(t, route.Stops, 2)
	assert.Equal(t, 0, route.Stops[0].SortOrder)
	assert.Equal(t, 1, route.Stops[1].SortOrder)
	assert.Equal(t, "pending", route.Stops[0].Status)

	require.Len(t, bus.published, 1)
	assert.Equal(t, "logistics.route.created", bus.published[0].Type)
}

func TestCreateRouteNameRequired(t *testing.T) {
	svc, _, _ := newTestService()
	_, err := svc.CreateRoute(context.Background(), uuid.New(), CreateRouteInput{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestGetRouteWithStops(t *testing.T) {
	svc, repo, _ := newTestService()
	orgID := uuid.New()

	route := &Route{ID: uuid.New(), OrganizationID: orgID, Name: "R1", Status: "planned"}
	repo.routes[route.ID] = route

	stop := &RouteStop{ID: uuid.New(), RouteID: route.ID, OrganizationID: orgID, Address: "Addr", Status: "pending"}
	repo.stops[stop.ID] = stop

	result, err := svc.GetRoute(context.Background(), orgID, route.ID)
	require.NoError(t, err)
	assert.Equal(t, "R1", result.Name)
	assert.Len(t, result.Stops, 1)
}

func TestStartRoute(t *testing.T) {
	svc, repo, bus := newTestService()
	orgID := uuid.New()
	actorID := uuid.New()

	route := &Route{ID: uuid.New(), OrganizationID: orgID, Name: "R1", Status: "planned"}
	repo.routes[route.ID] = route

	result, err := svc.StartRoute(context.Background(), orgID, route.ID, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "in_progress", result.Status)
	assert.NotNil(t, result.ActualStart)

	require.Len(t, bus.published, 1)
	assert.Equal(t, "logistics.route.started", bus.published[0].Type)
}

func TestStartRouteWrongStatus(t *testing.T) {
	svc, repo, _ := newTestService()
	orgID := uuid.New()

	route := &Route{ID: uuid.New(), OrganizationID: orgID, Name: "R1", Status: "in_progress"}
	repo.routes[route.ID] = route

	_, err := svc.StartRoute(context.Background(), orgID, route.ID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "planned")
}

func TestCompleteRoute(t *testing.T) {
	svc, repo, bus := newTestService()
	orgID := uuid.New()
	actorID := uuid.New()

	now := time.Now()
	route := &Route{ID: uuid.New(), OrganizationID: orgID, Name: "R1", Status: "in_progress", ActualStart: &now}
	repo.routes[route.ID] = route

	result, err := svc.CompleteRoute(context.Background(), orgID, route.ID, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.NotNil(t, result.ActualEnd)

	require.Len(t, bus.published, 1)
	assert.Equal(t, "logistics.route.completed", bus.published[0].Type)
}

func TestCompleteRouteWrongStatus(t *testing.T) {
	svc, repo, _ := newTestService()
	orgID := uuid.New()

	route := &Route{ID: uuid.New(), OrganizationID: orgID, Name: "R1", Status: "planned"}
	repo.routes[route.ID] = route

	_, err := svc.CompleteRoute(context.Background(), orgID, route.ID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "in_progress")
}

func TestArriveAtStop(t *testing.T) {
	svc, repo, bus := newTestService()
	orgID := uuid.New()
	routeID := uuid.New()
	actorID := uuid.New()

	stop := &RouteStop{ID: uuid.New(), RouteID: routeID, OrganizationID: orgID, Status: "pending"}
	repo.stops[stop.ID] = stop

	result, err := svc.ArriveAtStop(context.Background(), orgID, routeID, stop.ID, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "arrived", result.Status)
	assert.NotNil(t, result.ActualArrival)

	require.Len(t, bus.published, 1)
	assert.Equal(t, "logistics.stop.arrived", bus.published[0].Type)
}

func TestArriveAtStopWrongRoute(t *testing.T) {
	svc, repo, _ := newTestService()
	orgID := uuid.New()

	stop := &RouteStop{ID: uuid.New(), RouteID: uuid.New(), OrganizationID: orgID, Status: "pending"}
	repo.stops[stop.ID] = stop

	_, err := svc.ArriveAtStop(context.Background(), orgID, uuid.New(), stop.ID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestArriveAtStopAlreadyVisited(t *testing.T) {
	svc, repo, _ := newTestService()
	orgID := uuid.New()
	routeID := uuid.New()

	stop := &RouteStop{ID: uuid.New(), RouteID: routeID, OrganizationID: orgID, Status: "arrived"}
	repo.stops[stop.ID] = stop

	_, err := svc.ArriveAtStop(context.Background(), orgID, routeID, stop.ID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already been visited")
}

func TestCompleteStop(t *testing.T) {
	svc, repo, bus := newTestService()
	orgID := uuid.New()
	routeID := uuid.New()
	actorID := uuid.New()
	deliveryIDs := []uuid.UUID{uuid.New(), uuid.New()}

	stop := &RouteStop{ID: uuid.New(), RouteID: routeID, OrganizationID: orgID, Status: "arrived", DeliveryIDs: deliveryIDs}
	repo.stops[stop.ID] = stop

	result, err := svc.CompleteStop(context.Background(), orgID, routeID, stop.ID, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)

	require.Len(t, bus.published, 1)
	assert.Equal(t, "logistics.stop.completed", bus.published[0].Type)
}

func TestCompleteStopMustBeArrived(t *testing.T) {
	svc, repo, _ := newTestService()
	orgID := uuid.New()
	routeID := uuid.New()

	stop := &RouteStop{ID: uuid.New(), RouteID: routeID, OrganizationID: orgID, Status: "pending"}
	repo.stops[stop.ID] = stop

	_, err := svc.CompleteStop(context.Background(), orgID, routeID, stop.ID, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "arrived")
}

func TestUpdateGeo(t *testing.T) {
	svc, repo, bus := newTestService()
	orgID := uuid.New()
	entityID := uuid.New()
	actorID := uuid.New()

	point := GeoPoint{
		Latitude:  55.7558,
		Longitude: 37.6173,
		Speed:     60,
		Heading:   180,
	}

	err := svc.UpdateGeo(context.Background(), orgID, entityID, point, &actorID)
	require.NoError(t, err)

	key := entityID.String()
	require.Len(t, repo.geo[key], 1)
	assert.NotZero(t, repo.geo[key][0].RecordedAt)

	require.Len(t, bus.published, 1)
	assert.Equal(t, "logistics.geo.updated", bus.published[0].Type)
}

func TestGetTrack(t *testing.T) {
	svc, repo, _ := newTestService()
	orgID := uuid.New()
	entityID := uuid.New()

	now := time.Now()
	key := entityID.String()
	repo.geo[key] = []GeoPoint{
		{Latitude: 55.75, Longitude: 37.61, RecordedAt: now.Add(-2 * time.Hour)},
		{Latitude: 55.76, Longitude: 37.62, RecordedAt: now.Add(-1 * time.Hour)},
		{Latitude: 55.77, Longitude: 37.63, RecordedAt: now.Add(1 * time.Hour)},
	}

	points, err := svc.GetTrack(context.Background(), orgID, entityID, now.Add(-3*time.Hour), now)
	require.NoError(t, err)
	assert.Len(t, points, 2)
}

func TestFullRouteLifecycle(t *testing.T) {
	svc, _, bus := newTestService()
	orgID := uuid.New()
	actorID := uuid.New()

	// Create route with stops
	route, err := svc.CreateRoute(context.Background(), orgID, CreateRouteInput{
		Name: "Delivery Run",
		Stops: []CreateStopInput{
			{Address: "Warehouse", Latitude: ptr(55.75), Longitude: ptr(37.62)},
			{Address: "Customer", Latitude: ptr(55.80), Longitude: ptr(37.65)},
		},
	}, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "planned", route.Status)
	stopIDs := make([]uuid.UUID, len(route.Stops))
	for i, s := range route.Stops {
		stopIDs[i] = s.ID
	}

	// Start
	route, err = svc.StartRoute(context.Background(), orgID, route.ID, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "in_progress", route.Status)

	// Arrive at first stop
	stop1, err := svc.ArriveAtStop(context.Background(), orgID, route.ID, stopIDs[0], &actorID)
	require.NoError(t, err)
	assert.Equal(t, "arrived", stop1.Status)

	// Complete first stop
	stop1, err = svc.CompleteStop(context.Background(), orgID, route.ID, stopIDs[0], &actorID)
	require.NoError(t, err)
	assert.Equal(t, "completed", stop1.Status)

	// Complete route
	route, err = svc.CompleteRoute(context.Background(), orgID, route.ID, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "completed", route.Status)
	assert.NotNil(t, route.ActualEnd)

	// Events: created, started, stop.arrived, stop.completed, route.completed
	assert.Len(t, bus.published, 5)
}
