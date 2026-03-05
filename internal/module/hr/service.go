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
		ID:             uuid.New(),
		OrganizationID: orgID,
		EmployeeID:     input.EmployeeID,
		LocationID:     input.LocationID,
		StartTime:      input.StartTime,
		EndTime:        input.EndTime,
		BreakMinutes:   input.BreakMinutes,
		Status:         "scheduled",
		Notes:          input.Notes,
		CreatedAt:      time.Now(),
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
		ID:             uuid.New(),
		OrganizationID: orgID,
		EmployeeID:     employeeID,
		ShiftID:        shiftID,
		ClockIn:        time.Now(),
		Status:         "open",
		CreatedAt:      time.Now(),
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

// CalculatePayroll calculates and creates a payroll record for an employee.
func (s *Service) CalculatePayroll(ctx context.Context, orgID uuid.UUID, input CreatePayrollInput, actorID *uuid.UUID) (*Payroll, error) {
	if input.Month < 1 || input.Month > 12 {
		return nil, errs.NewBadRequest("month must be between 1 and 12")
	}

	emp, err := s.GetEmployee(ctx, orgID, input.EmployeeID)
	if err != nil {
		return nil, errs.NewBadRequest("employee not found")
	}

	var grossSalary int64
	if emp.Salary != nil {
		if base, ok := emp.Salary["base_salary"].(float64); ok {
			grossSalary = int64(base)
		}
	}
	if grossSalary == 0 {
		return nil, errs.NewBadRequest("employee has no base_salary set")
	}

	// NDFL 13%
	ndfl := int64(math.Round(float64(grossSalary) * 0.13))
	netSalary := grossSalary - ndfl - input.Deductions
	if netSalary < 0 {
		netSalary = 0
	}

	p := &Payroll{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EmployeeID:     input.EmployeeID,
		Year:           input.Year,
		Month:          input.Month,
		GrossSalary:    grossSalary,
		NDFL:           ndfl,
		Deductions:     input.Deductions,
		NetSalary:      netSalary,
		Status:         "draft",
		CreatedAt:      time.Now(),
	}

	if err := s.repo.CreatePayroll(ctx, p); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, input.EmployeeID, "hr.payroll.calculated", map[string]any{
		"payroll_id":   p.ID,
		"employee_id":  input.EmployeeID,
		"gross_salary": grossSalary,
		"ndfl":         ndfl,
		"net_salary":   netSalary,
		"year":         input.Year,
		"month":        input.Month,
	}, actorID)

	return p, nil
}

// ApprovePayroll approves a draft payroll.
func (s *Service) ApprovePayroll(ctx context.Context, orgID, payrollID uuid.UUID, actorID *uuid.UUID) (*Payroll, error) {
	p, err := s.repo.GetPayroll(ctx, orgID, payrollID)
	if err != nil {
		return nil, err
	}
	if p.Status != "draft" {
		return nil, errs.NewConflict("only draft payrolls can be approved")
	}

	now := time.Now()
	p.Status = "approved"
	p.ApprovedAt = &now
	p.ApprovedBy = actorID

	if err := s.repo.UpdatePayroll(ctx, p); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, p.EmployeeID, "hr.payroll.approved", map[string]any{
		"payroll_id":  p.ID,
		"employee_id": p.EmployeeID,
		"net_salary":  p.NetSalary,
	}, actorID)

	return p, nil
}

// ListPayrolls returns payrolls matching filters.
func (s *Service) ListPayrolls(ctx context.Context, orgID uuid.UUID, employeeID *uuid.UUID, year, month *int, page types.PageRequest) (*types.PageResponse[Payroll], error) {
	page.Normalize()
	payrolls, total, err := s.repo.ListPayrolls(ctx, orgID, employeeID, year, month, page)
	if err != nil {
		return nil, err
	}
	if payrolls == nil {
		payrolls = []Payroll{}
	}
	return &types.PageResponse[Payroll]{
		Items:  payrolls,
		Total:  total,
		Limit:  page.Limit,
		Offset: page.Offset,
	}, nil
}

// RequestAbsence creates a new absence request.
func (s *Service) RequestAbsence(ctx context.Context, orgID uuid.UUID, input CreateAbsenceInput, actorID *uuid.UUID) (*Absence, error) {
	validTypes := map[string]bool{"vacation": true, "sick_leave": true, "personal": true, "unpaid": true}
	if !validTypes[input.Type] {
		return nil, errs.NewBadRequest("type must be vacation, sick_leave, personal, or unpaid")
	}

	startDate, err := time.Parse("2006-01-02", input.StartDate)
	if err != nil {
		return nil, errs.NewBadRequest("invalid start_date format")
	}
	endDate, err := time.Parse("2006-01-02", input.EndDate)
	if err != nil {
		return nil, errs.NewBadRequest("invalid end_date format")
	}
	if !endDate.After(startDate) && endDate != startDate {
		return nil, errs.NewBadRequest("end_date must be >= start_date")
	}

	days := int(endDate.Sub(startDate).Hours()/24) + 1

	a := &Absence{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EmployeeID:     input.EmployeeID,
		Type:           input.Type,
		StartDate:      startDate,
		EndDate:        endDate,
		Days:           days,
		Status:         "pending",
		Notes:          input.Notes,
		CreatedAt:      time.Now(),
	}

	if err := s.repo.CreateAbsence(ctx, a); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, input.EmployeeID, "hr.absence.requested", map[string]any{
		"absence_id":  a.ID,
		"employee_id": input.EmployeeID,
		"type":        input.Type,
		"start_date":  input.StartDate,
		"end_date":    input.EndDate,
		"days":        days,
	}, actorID)

	return a, nil
}

// ApproveAbsence approves a pending absence.
func (s *Service) ApproveAbsence(ctx context.Context, orgID, absenceID uuid.UUID, actorID *uuid.UUID) (*Absence, error) {
	a, err := s.repo.GetAbsence(ctx, orgID, absenceID)
	if err != nil {
		return nil, err
	}
	if a.Status != "pending" {
		return nil, errs.NewConflict("only pending absences can be approved")
	}

	a.Status = "approved"
	a.ApprovedBy = actorID
	if err := s.repo.UpdateAbsence(ctx, a); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, a.EmployeeID, "hr.absence.approved", map[string]any{
		"absence_id":  a.ID,
		"employee_id": a.EmployeeID,
		"type":        a.Type,
		"days":        a.Days,
	}, actorID)

	return a, nil
}

// RejectAbsence rejects a pending absence.
func (s *Service) RejectAbsence(ctx context.Context, orgID, absenceID uuid.UUID, actorID *uuid.UUID) (*Absence, error) {
	a, err := s.repo.GetAbsence(ctx, orgID, absenceID)
	if err != nil {
		return nil, err
	}
	if a.Status != "pending" {
		return nil, errs.NewConflict("only pending absences can be rejected")
	}

	a.Status = "rejected"
	if err := s.repo.UpdateAbsence(ctx, a); err != nil {
		return nil, err
	}

	return a, nil
}

// ListAbsences returns absences matching the filter.
func (s *Service) ListAbsences(ctx context.Context, orgID uuid.UUID, filter AbsenceFilter) (*types.PageResponse[Absence], error) {
	filter.Page.Normalize()
	absences, total, err := s.repo.ListAbsences(ctx, orgID, filter)
	if err != nil {
		return nil, err
	}
	if absences == nil {
		absences = []Absence{}
	}
	return &types.PageResponse[Absence]{
		Items:  absences,
		Total:  total,
		Limit:  filter.Page.Limit,
		Offset: filter.Page.Offset,
	}, nil
}

func (s *Service) publishEvent(ctx context.Context, orgID uuid.UUID, entityID uuid.UUID, eventType string, data map[string]any, actorID *uuid.UUID) {
	payload, _ := json.Marshal(data)
	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EntityID:       &entityID,
		Type:           eventType,
		Data:           payload,
		Timestamp:      time.Now(),
	}
	if actorID != nil {
		ev.ActorID = actorID
	}
	s.bus.Publish(ctx, ev)
}

func strPtr(s string) *string { return &s }
