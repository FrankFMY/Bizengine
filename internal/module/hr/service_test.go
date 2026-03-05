package hr

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/entity"
	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/types"
)

// --- mocks ---

type mockHRRepo struct {
	shifts     map[uuid.UUID]*Shift
	timesheets map[uuid.UUID]*Timesheet
	overlap    bool
}

func newMockHRRepo() *mockHRRepo {
	return &mockHRRepo{
		shifts:     make(map[uuid.UUID]*Shift),
		timesheets: make(map[uuid.UUID]*Timesheet),
	}
}

func (m *mockHRRepo) CreateShift(_ context.Context, s *Shift) error {
	cp := *s
	m.shifts[s.ID] = &cp
	return nil
}

func (m *mockHRRepo) GetShift(_ context.Context, orgID, shiftID uuid.UUID) (*Shift, error) {
	s, ok := m.shifts[shiftID]
	if !ok || s.OrganizationID != orgID {
		return nil, assert.AnError
	}
	cp := *s
	return &cp, nil
}

func (m *mockHRRepo) ListShifts(_ context.Context, orgID uuid.UUID, _ ShiftFilter) ([]Shift, int, error) {
	var result []Shift
	for _, s := range m.shifts {
		if s.OrganizationID == orgID {
			result = append(result, *s)
		}
	}
	return result, len(result), nil
}

func (m *mockHRRepo) UpdateShift(_ context.Context, s *Shift) error {
	cp := *s
	m.shifts[s.ID] = &cp
	return nil
}

func (m *mockHRRepo) DeleteShift(_ context.Context, orgID, shiftID uuid.UUID) error {
	if s, ok := m.shifts[shiftID]; ok && s.OrganizationID == orgID {
		delete(m.shifts, shiftID)
		return nil
	}
	return assert.AnError
}

func (m *mockHRRepo) HasOverlappingShift(_ context.Context, _, _ uuid.UUID, _, _ time.Time, _ *uuid.UUID) (bool, error) {
	return m.overlap, nil
}

func (m *mockHRRepo) CreateTimesheet(_ context.Context, ts *Timesheet) error {
	cp := *ts
	m.timesheets[ts.ID] = &cp
	return nil
}

func (m *mockHRRepo) GetTimesheet(_ context.Context, orgID, tsID uuid.UUID) (*Timesheet, error) {
	ts, ok := m.timesheets[tsID]
	if !ok || ts.OrganizationID != orgID {
		return nil, assert.AnError
	}
	cp := *ts
	return &cp, nil
}

func (m *mockHRRepo) ListTimesheets(_ context.Context, orgID uuid.UUID, _ TimesheetFilter) ([]Timesheet, int, error) {
	var result []Timesheet
	for _, ts := range m.timesheets {
		if ts.OrganizationID == orgID {
			result = append(result, *ts)
		}
	}
	return result, len(result), nil
}

func (m *mockHRRepo) UpdateTimesheet(_ context.Context, ts *Timesheet) error {
	cp := *ts
	m.timesheets[ts.ID] = &cp
	return nil
}

func (m *mockHRRepo) GetOpenTimesheet(_ context.Context, orgID, employeeID uuid.UUID) (*Timesheet, error) {
	for _, ts := range m.timesheets {
		if ts.OrganizationID == orgID && ts.EmployeeID == employeeID && ts.Status == "open" {
			cp := *ts
			return &cp, nil
		}
	}
	return nil, nil
}

type mockEntityRepo struct {
	entities   map[uuid.UUID]*types.Entity
	components map[uuid.UUID][]types.Component
}

func newMockEntityRepo() *mockEntityRepo {
	return &mockEntityRepo{
		entities:   make(map[uuid.UUID]*types.Entity),
		components: make(map[uuid.UUID][]types.Component),
	}
}

func (m *mockEntityRepo) Create(_ context.Context, e *types.Entity) error {
	m.entities[e.ID] = e
	return nil
}
func (m *mockEntityRepo) GetByID(_ context.Context, _, id uuid.UUID) (*types.Entity, error) {
	if e, ok := m.entities[id]; ok {
		cp := *e
		return &cp, nil
	}
	return nil, pgx.ErrNoRows
}
func (m *mockEntityRepo) List(_ context.Context, orgID uuid.UUID, filter entity.ListFilter) ([]types.Entity, int, error) {
	var result []types.Entity
	for _, e := range m.entities {
		if e.OrganizationID != orgID {
			continue
		}
		if filter.Kind != nil && e.Kind != *filter.Kind {
			continue
		}
		if filter.Status != nil && *filter.Status != "" && e.Status != "" && e.Status != *filter.Status {
			continue
		}
		result = append(result, *e)
	}
	return result, len(result), nil
}
func (m *mockEntityRepo) Update(_ context.Context, e *types.Entity) error {
	m.entities[e.ID] = e
	return nil
}
func (m *mockEntityRepo) SoftDelete(context.Context, uuid.UUID, uuid.UUID) error { return nil }
func (m *mockEntityRepo) SetComponent(_ context.Context, c *types.Component) error {
	comps := m.components[c.EntityID]
	for i, existing := range comps {
		if existing.Type == c.Type {
			comps[i] = *c
			m.components[c.EntityID] = comps
			return nil
		}
	}
	m.components[c.EntityID] = append(m.components[c.EntityID], *c)
	return nil
}
func (m *mockEntityRepo) GetComponent(context.Context, uuid.UUID, uuid.UUID, string) (*types.Component, error) {
	return nil, nil
}
func (m *mockEntityRepo) ListComponents(_ context.Context, _, entityID uuid.UUID) ([]types.Component, error) {
	return m.components[entityID], nil
}
func (m *mockEntityRepo) DeleteComponent(context.Context, uuid.UUID, uuid.UUID, string) error {
	return nil
}
func (m *mockEntityRepo) WithTx(_ context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
}
func (m *mockEntityRepo) CreateTx(_ context.Context, _ pgx.Tx, e *types.Entity) error {
	return m.Create(context.Background(), e)
}
func (m *mockEntityRepo) SetComponentTx(_ context.Context, _ pgx.Tx, c *types.Component) error {
	return m.SetComponent(context.Background(), c)
}

type mockEventStore struct{}

func (m *mockEventStore) Append(context.Context, types.Event) error           { return nil }
func (m *mockEventStore) AppendTx(context.Context, pgx.Tx, types.Event) error { return nil }
func (m *mockEventStore) GetByEntity(_ context.Context, _, _ uuid.UUID, _ *time.Time, _ int) ([]types.Event, error) {
	return nil, nil
}
func (m *mockEventStore) GetByOrganization(_ context.Context, _ uuid.UUID, _, _ int) ([]types.Event, int, error) {
	return nil, 0, nil
}
func (m *mockEventStore) GetByType(_ context.Context, _ uuid.UUID, _ string, _ *time.Time, _ int) ([]types.Event, error) {
	return nil, nil
}

type mockBus struct {
	published []types.Event
}

func (b *mockBus) Publish(_ context.Context, ev types.Event) error {
	b.published = append(b.published, ev)
	return nil
}
func (b *mockBus) Subscribe(string, event.Subscriber)        {}
func (b *mockBus) SubscribePattern(string, event.Subscriber) {}
func (b *mockBus) SubscribeAll(event.Subscriber)             {}

// --- helpers ---

func setupHRService() (*Service, *mockHRRepo, *mockBus) {
	hrRepo := newMockHRRepo()
	entityRepo := newMockEntityRepo()
	bus := &mockBus{}
	eventStore := &mockEventStore{}
	entitySvc := entity.NewService(entityRepo, eventStore, bus)
	svc := NewService(hrRepo, entitySvc, bus)
	return svc, hrRepo, bus
}

// --- tests ---

func TestHireEmployee(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, _, bus := setupHRService()

		emp, err := svc.HireEmployee(ctx, orgID, HireInput{
			Name:     "Ivan Petrov",
			Position: "Developer",
			Salary:   map[string]any{"base_salary": 8000000, "currency": "RUB"},
			Contact:  map[string]any{"phone": "+79001234567"},
		}, &actorID)

		require.NoError(t, err)
		assert.Equal(t, "Ivan Petrov", emp.Name)
		assert.Equal(t, "employee", emp.Kind)
		assert.Equal(t, "Developer", emp.Employment["position"])

		hasEvent := false
		for _, ev := range bus.published {
			if ev.Type == "hr.employee.hired" {
				hasEvent = true
			}
		}
		assert.True(t, hasEvent)
	})

	t.Run("missing name", func(t *testing.T) {
		svc, _, _ := setupHRService()
		_, err := svc.HireEmployee(ctx, orgID, HireInput{Position: "Dev"}, &actorID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "name is required")
	})

	t.Run("missing position", func(t *testing.T) {
		svc, _, _ := setupHRService()
		_, err := svc.HireEmployee(ctx, orgID, HireInput{Name: "Test"}, &actorID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "position is required")
	})
}

func TestGetEmployee(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupHRService()
	emp, _ := svc.HireEmployee(ctx, orgID, HireInput{
		Name:     "Test Employee",
		Position: "QA",
	}, &actorID)

	got, err := svc.GetEmployee(ctx, orgID, emp.ID)
	require.NoError(t, err)
	assert.Equal(t, "Test Employee", got.Name)
	assert.Equal(t, "QA", got.Employment["position"])
}

func TestTerminateEmployee(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _, bus := setupHRService()

	emp, _ := svc.HireEmployee(ctx, orgID, HireInput{
		Name:     "To Terminate",
		Position: "Temp",
	}, &actorID)

	err := svc.TerminateEmployee(ctx, orgID, emp.ID, "contract ended", &actorID)
	require.NoError(t, err)

	hasEvent := false
	for _, ev := range bus.published {
		if ev.Type == "hr.employee.terminated" {
			hasEvent = true
		}
	}
	assert.True(t, hasEvent)
}

func TestCreateShift(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()
	employeeID := uuid.New()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := setupHRService()

		start := time.Now().Add(24 * time.Hour)
		end := start.Add(8 * time.Hour)

		shift, err := svc.CreateShift(ctx, orgID, CreateShiftInput{
			EmployeeID:   employeeID,
			StartTime:    start,
			EndTime:      end,
			BreakMinutes: 60,
			Notes:        "morning shift",
		}, &actorID)

		require.NoError(t, err)
		assert.Equal(t, "scheduled", shift.Status)
		assert.Equal(t, 60, shift.BreakMinutes)
		assert.Len(t, repo.shifts, 1)
	})

	t.Run("end before start", func(t *testing.T) {
		svc, _, _ := setupHRService()

		start := time.Now().Add(24 * time.Hour)
		end := start.Add(-1 * time.Hour)

		_, err := svc.CreateShift(ctx, orgID, CreateShiftInput{
			EmployeeID: employeeID,
			StartTime:  start,
			EndTime:    end,
		}, &actorID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "end_time must be after start_time")
	})

	t.Run("overlap rejected", func(t *testing.T) {
		svc, repo, _ := setupHRService()
		repo.overlap = true

		start := time.Now().Add(24 * time.Hour)
		end := start.Add(8 * time.Hour)

		_, err := svc.CreateShift(ctx, orgID, CreateShiftInput{
			EmployeeID: employeeID,
			StartTime:  start,
			EndTime:    end,
		}, &actorID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "overlaps")
	})
}

func TestClockInClockOut(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()
	employeeID := uuid.New()

	t.Run("clock in and out", func(t *testing.T) {
		svc, _, bus := setupHRService()

		ts, err := svc.ClockIn(ctx, orgID, employeeID, nil, &actorID)
		require.NoError(t, err)
		assert.Equal(t, "open", ts.Status)
		assert.Nil(t, ts.ClockOut)

		ts, err = svc.ClockOut(ctx, orgID, ts.ID, &actorID)
		require.NoError(t, err)
		assert.Equal(t, "closed", ts.Status)
		assert.NotNil(t, ts.ClockOut)
		assert.NotNil(t, ts.HoursWorked)

		hasStarted := false
		hasCompleted := false
		for _, ev := range bus.published {
			if ev.Type == "hr.shift.started" {
				hasStarted = true
			}
			if ev.Type == "hr.shift.completed" {
				hasCompleted = true
			}
		}
		assert.True(t, hasStarted)
		assert.True(t, hasCompleted)
	})

	t.Run("double clock in rejected", func(t *testing.T) {
		svc, _, _ := setupHRService()

		_, err := svc.ClockIn(ctx, orgID, employeeID, nil, &actorID)
		require.NoError(t, err)

		_, err = svc.ClockIn(ctx, orgID, employeeID, nil, &actorID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "already clocked in")
	})
}

func TestApproveTimesheet(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()
	employeeID := uuid.New()

	svc, _, bus := setupHRService()

	ts, _ := svc.ClockIn(ctx, orgID, employeeID, nil, &actorID)
	ts, _ = svc.ClockOut(ctx, orgID, ts.ID, &actorID)

	err := svc.ApproveTimesheet(ctx, orgID, ts.ID, &actorID)
	require.NoError(t, err)

	hasEvent := false
	for _, ev := range bus.published {
		if ev.Type == "hr.timesheet.approved" {
			hasEvent = true
		}
	}
	assert.True(t, hasEvent)
}

func TestApproveOpenTimesheetFails(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()
	employeeID := uuid.New()

	svc, _, _ := setupHRService()
	ts, _ := svc.ClockIn(ctx, orgID, employeeID, nil, &actorID)

	err := svc.ApproveTimesheet(ctx, orgID, ts.ID, &actorID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only closed timesheets")
}

func TestListEmployees(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupHRService()
	svc.HireEmployee(ctx, orgID, HireInput{Name: "Emp1", Position: "Dev"}, &actorID)
	svc.HireEmployee(ctx, orgID, HireInput{Name: "Emp2", Position: "QA"}, &actorID)

	employees, total, err := svc.ListEmployees(ctx, orgID, EmployeeFilter{})
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	assert.Len(t, employees, 2)
}

func TestEmploymentHasAutoFields(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupHRService()

	emp, err := svc.HireEmployee(ctx, orgID, HireInput{
		Name:     "Auto Fields",
		Position: "Manager",
	}, &actorID)
	require.NoError(t, err)

	assert.Equal(t, "Manager", emp.Employment["position"])
	_, hasHireDate := emp.Employment["hire_date"]
	assert.True(t, hasHireDate, "should auto-fill hire_date")
}
