package hr

import (
	"context"
	"encoding/json"
	"math"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/core/entity"
	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// Service provides HR operations.
type Service struct {
	repo      Repository
	entitySvc *entity.Service
	bus       event.Bus
}

// NewService creates a new HR service.
func NewService(repo Repository, entitySvc *entity.Service, bus event.Bus) *Service {
	return &Service{repo: repo, entitySvc: entitySvc, bus: bus}
}

// HireEmployee creates a new employee entity with components.
func (s *Service) HireEmployee(ctx context.Context, orgID uuid.UUID, input HireInput, actorID *uuid.UUID) (*Employee, error) {
	if input.Name == "" {
		return nil, errs.NewBadRequest("name is required")
	}
	if input.Position == "" {
		return nil, errs.NewBadRequest("position is required")
	}

	e, err := s.entitySvc.Create(ctx, orgID, entity.CreateEntityInput{
		Kind: "employee",
		Name: input.Name,
	}, actorID)
	if err != nil {
		return nil, err
	}

	employment := input.Employment
	if employment == nil {
		employment = make(map[string]any)
	}
	employment["position"] = input.Position
	if input.DepartmentID != nil {
		employment["department_id"] = input.DepartmentID.String()
	}
	if _, ok := employment["hire_date"]; !ok {
		employment["hire_date"] = time.Now().Format("2006-01-02")
	}

	empData, _ := json.Marshal(employment)
	if _, err := s.entitySvc.SetComponent(ctx, orgID, e.ID, "employment", empData, actorID); err != nil {
		return nil, err
	}

	if input.Salary != nil {
		salData, _ := json.Marshal(input.Salary)
		if _, err := s.entitySvc.SetComponent(ctx, orgID, e.ID, "salary", salData, actorID); err != nil {
			return nil, err
		}
	}

	if input.Contact != nil {
		conData, _ := json.Marshal(input.Contact)
		if _, err := s.entitySvc.SetComponent(ctx, orgID, e.ID, "contact", conData, actorID); err != nil {
			return nil, err
		}
	}

	s.publishEvent(ctx, orgID, e.ID, "hr.employee.hired", map[string]any{
		"employee_id": e.ID.String(),
		"name":        input.Name,
		"position":    input.Position,
	}, actorID)

	return &Employee{
		Entity:     *e,
		Employment: employment,
		Salary:     input.Salary,
		Contact:    input.Contact,
	}, nil
}

// GetEmployee returns an employee with components.
func (s *Service) GetEmployee(ctx context.Context, orgID, employeeID uuid.UUID) (*Employee, error) {
	e, err := s.entitySvc.Get(ctx, orgID, employeeID, true)
	if err != nil {
		return nil, err
	}
	if e.Kind != "employee" {
		return nil, errs.NewNotFound("employee not found")
	}

	emp := &Employee{Entity: *e}
	for _, c := range e.Components {
		var data map[string]any
		json.Unmarshal(c.Data, &data)
		switch c.Type {
		case "employment":
			emp.Employment = data
		case "salary":
			emp.Salary = data
		case "contact":
			emp.Contact = data
		}
	}
	return emp, nil
}

// ListEmployees returns employees matching the filter.
func (s *Service) ListEmployees(ctx context.Context, orgID uuid.UUID, filter EmployeeFilter) ([]Employee, int, error) {
	filter.Page.Normalize()
	status := "active"
	if filter.Status != nil {
		status = *filter.Status
	}

	page, err := s.entitySvc.List(ctx, orgID, entity.ListFilter{
		Kind:   strPtr("employee"),
		Status: &status,
		Page:   filter.Page,
	}, false)
	if err != nil {
		return nil, 0, err
	}

	employees := make([]Employee, len(page.Items))
	for i, e := range page.Items {
		employees[i] = Employee{Entity: e}
	}
	return employees, page.Total, nil
}

// UpdateEmployee updates employee entity and components.
func (s *Service) UpdateEmployee(ctx context.Context, orgID, employeeID uuid.UUID, input UpdateEmployeeInput, actorID *uuid.UUID) (*Employee, error) {
	e, err := s.entitySvc.Get(ctx, orgID, employeeID, false)
	if err != nil {
		return nil, err
	}
	if e.Kind != "employee" {
		return nil, errs.NewNotFound("employee not found")
	}

	if input.Name != nil {
		if _, err := s.entitySvc.Update(ctx, orgID, e.ID, entity.UpdateEntityInput{Name: input.Name}, actorID); err != nil {
			return nil, err
		}
	}

	if input.Employment != nil {
		data, _ := json.Marshal(input.Employment)
		if _, err := s.entitySvc.SetComponent(ctx, orgID, e.ID, "employment", data, actorID); err != nil {
			return nil, err
		}
	}
	if input.Salary != nil {
		data, _ := json.Marshal(input.Salary)
		if _, err := s.entitySvc.SetComponent(ctx, orgID, e.ID, "salary", data, actorID); err != nil {
			return nil, err
		}
	}
	if input.Contact != nil {
		data, _ := json.Marshal(input.Contact)
		if _, err := s.entitySvc.SetComponent(ctx, orgID, e.ID, "contact", data, actorID); err != nil {
			return nil, err
		}
	}

	return s.GetEmployee(ctx, orgID, employeeID)
}

// TerminateEmployee soft-deletes the employee entity.
func (s *Service) TerminateEmployee(ctx context.Context, orgID, employeeID uuid.UUID, reason string, actorID *uuid.UUID) error {
	e, err := s.entitySvc.Get(ctx, orgID, employeeID, false)
	if err != nil {
		return err
	}
	if e.Kind != "employee" {
		return errs.NewNotFound("employee not found")
	}
	if e.Status == "terminated" {
		return errs.NewConflict("employee already terminated")
	}

	terminated := "terminated"
	if _, err := s.entitySvc.Update(ctx, orgID, employeeID, entity.UpdateEntityInput{Status: &terminated}, actorID); err != nil {
		return err
	}

	s.publishEvent(ctx, orgID, employeeID, "hr.employee.terminated", map[string]any{
		"employee_id": employeeID.String(),
		"reason":      reason,
	}, actorID)
	return nil
}

// CreateShift creates a new planned shift.
func (s *Service) CreateShift(ctx context.Context, orgID uuid.UUID, input CreateShiftInput, actorID *uuid.UUID) (*Shift, error) {
	if input.EmployeeID == uuid.Nil {
		return nil, errs.NewBadRequest("employee_id is required")
	}
	if input.StartTime.IsZero() || input.EndTime.IsZero() {
		return nil, errs.NewBadRequest("start_time and end_time are required")
	}
	if !input.EndTime.After(input.StartTime) {
		return nil, errs.NewBadRequest("end_time must be after start_time")
	}

	overlap, err := s.repo.HasOverlappingShift(ctx, orgID, input.EmployeeID, input.StartTime, input.EndTime, nil)
	if err != nil {
		return nil, err
	}
	if overlap {
		return nil, errs.NewConflict("shift overlaps with existing shift for this employee")
	}

	shift := &Shift{
		ID:           uuid.New(),
		OrganizationID:  orgID,
		EmployeeID:   input.EmployeeID,
		LocationID:   input.LocationID,
		StartTime:    input.StartTime,
		EndTime:      input.EndTime,
		BreakMinutes: input.BreakMinutes,
		Status:       "scheduled",
		Notes:        input.Notes,
		CreatedAt:    time.Now(),
	}
	if err := s.repo.CreateShift(ctx, shift); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, input.EmployeeID, "hr.shift.created", map[string]any{
		"shift_id":    shift.ID.String(),
		"employee_id": input.EmployeeID.String(),
		"start_time":  input.StartTime.Format(time.RFC3339),
		"end_time":    input.EndTime.Format(time.RFC3339),
	}, actorID)

	return shift, nil
}

// ListShifts returns shifts matching the filter.
func (s *Service) ListShifts(ctx context.Context, orgID uuid.UUID, filter ShiftFilter) ([]Shift, int, error) {
	filter.Page.Normalize()
	return s.repo.ListShifts(ctx, orgID, filter)
}

// UpdateShift updates a shift.
func (s *Service) UpdateShift(ctx context.Context, orgID, shiftID uuid.UUID, input UpdateShiftInput, actorID *uuid.UUID) (*Shift, error) {
	shift, err := s.repo.GetShift(ctx, orgID, shiftID)
	if err != nil {
		return nil, err
	}

	if input.StartTime != nil {
		shift.StartTime = *input.StartTime
	}
	if input.EndTime != nil {
		shift.EndTime = *input.EndTime
	}
	if input.BreakMinutes != nil {
		shift.BreakMinutes = *input.BreakMinutes
	}
	if input.Status != nil {
		shift.Status = *input.Status
	}
	if input.Notes != nil {
		shift.Notes = *input.Notes
	}

	if !shift.EndTime.After(shift.StartTime) {
		return nil, errs.NewBadRequest("end_time must be after start_time")
	}

	overlap, err := s.repo.HasOverlappingShift(ctx, orgID, shift.EmployeeID, shift.StartTime, shift.EndTime, &shiftID)
	if err != nil {
		return nil, err
	}
	if overlap {
		return nil, errs.NewConflict("shift overlaps with existing shift for this employee")
	}

	if err := s.repo.UpdateShift(ctx, shift); err != nil {
		return nil, err
	}
	return shift, nil
}

// DeleteShift deletes a shift.
func (s *Service) DeleteShift(ctx context.Context, orgID, shiftID uuid.UUID) error {
	return s.repo.DeleteShift(ctx, orgID, shiftID)
}

// ClockIn creates a new timesheet entry.
func (s *Service) ClockIn(ctx context.Context, orgID, employeeID uuid.UUID, shiftID *uuid.UUID, actorID *uuid.UUID) (*Timesheet, error) {
	if employeeID == uuid.Nil {
		return nil, errs.NewBadRequest("employee_id is required")
	}

	existing, err := s.repo.GetOpenTimesheet(ctx, orgID, employeeID)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, errs.NewConflict("employee already clocked in")
	}

	ts := &Timesheet{
		ID:          uuid.New(),
		OrganizationID: orgID,
		EmployeeID:  employeeID,
		ShiftID:     shiftID,
		ClockIn:     time.Now(),
		Status:      "open",
		CreatedAt:   time.Now(),
	}
	if err := s.repo.CreateTimesheet(ctx, ts); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, employeeID, "hr.shift.started", map[string]any{
		"timesheet_id": ts.ID.String(),
		"employee_id":  employeeID.String(),
	}, actorID)

	return ts, nil
}

// ClockOut closes a timesheet and calculates hours worked.
func (s *Service) ClockOut(ctx context.Context, orgID, tsID uuid.UUID, actorID *uuid.UUID) (*Timesheet, error) {
	ts, err := s.repo.GetTimesheet(ctx, orgID, tsID)
	if err != nil {
		return nil, err
	}
	if ts.Status != "open" {
		return nil, errs.NewConflict("timesheet is not open")
	}

	now := time.Now()
	ts.ClockOut = &now
	ts.Status = "closed"

	breakDuration := time.Duration(0)
	if ts.ShiftID != nil {
		shift, err := s.repo.GetShift(ctx, orgID, *ts.ShiftID)
		if err == nil {
			breakDuration = time.Duration(shift.BreakMinutes) * time.Minute
		}
	}

	workedDuration := now.Sub(ts.ClockIn) - breakDuration
	if workedDuration < 0 {
		workedDuration = 0
	}
	hours := math.Round(workedDuration.Hours()*100) / 100
	ts.HoursWorked = &hours

	if err := s.repo.UpdateTimesheet(ctx, ts); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, ts.EmployeeID, "hr.shift.completed", map[string]any{
		"timesheet_id": ts.ID.String(),
		"employee_id":  ts.EmployeeID.String(),
		"hours_worked": hours,
	}, actorID)

	return ts, nil
}

// ListTimesheets returns timesheets matching the filter.
func (s *Service) ListTimesheets(ctx context.Context, orgID uuid.UUID, filter TimesheetFilter) ([]Timesheet, int, error) {
	filter.Page.Normalize()
	return s.repo.ListTimesheets(ctx, orgID, filter)
}

// ApproveTimesheet approves a closed timesheet.
func (s *Service) ApproveTimesheet(ctx context.Context, orgID, tsID uuid.UUID, actorID *uuid.UUID) error {
	ts, err := s.repo.GetTimesheet(ctx, orgID, tsID)
	if err != nil {
		return err
	}
	if ts.Status != "closed" {
		return errs.NewConflict("only closed timesheets can be approved")
	}

	ts.Status = "approved"
	if err := s.repo.UpdateTimesheet(ctx, ts); err != nil {
		return err
	}

	evData := map[string]any{
		"timesheet_id": ts.ID.String(),
		"employee_id":  ts.EmployeeID.String(),
		"hours_worked": ts.HoursWorked,
	}
	if emp, err := s.GetEmployee(ctx, orgID, ts.EmployeeID); err == nil && emp.Salary != nil {
		if baseSalary, ok := emp.Salary["base_salary"].(float64); ok && baseSalary > 0 {
			evData["hourly_rate"] = baseSalary / 100 / 176
		}
	}
	s.publishEvent(ctx, orgID, ts.EmployeeID, "hr.timesheet.approved", evData, actorID)

	return nil
}

func (s *Service) publishEvent(ctx context.Context, orgID uuid.UUID, entityID uuid.UUID, eventType string, data map[string]any, actorID *uuid.UUID) {
	payload, _ := json.Marshal(data)
	ev := types.Event{
		ID:          uuid.New(),
		OrganizationID: orgID,
		EntityID:    &entityID,
		Type:        eventType,
		Data:        payload,
		Timestamp:   time.Now(),
	}
	if actorID != nil {
		ev.ActorID = actorID
	}
	s.bus.Publish(ctx, ev)
}

func strPtr(s string) *string { return &s }
