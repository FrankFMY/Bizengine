package hr

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/pkg/types"
)

// Repository defines data access for the HR module.
type Repository interface {
	// Shifts
	CreateShift(ctx context.Context, s *Shift) error
	GetShift(ctx context.Context, orgID, shiftID uuid.UUID) (*Shift, error)
	ListShifts(ctx context.Context, orgID uuid.UUID, filter ShiftFilter) ([]Shift, int, error)
	UpdateShift(ctx context.Context, s *Shift) error
	DeleteShift(ctx context.Context, orgID, shiftID uuid.UUID) error
	HasOverlappingShift(ctx context.Context, orgID, employeeID uuid.UUID, start, end time.Time, excludeID *uuid.UUID) (bool, error)

	// Timesheets
	CreateTimesheet(ctx context.Context, ts *Timesheet) error
	GetTimesheet(ctx context.Context, orgID, tsID uuid.UUID) (*Timesheet, error)
	ListTimesheets(ctx context.Context, orgID uuid.UUID, filter TimesheetFilter) ([]Timesheet, int, error)
	UpdateTimesheet(ctx context.Context, ts *Timesheet) error
	GetOpenTimesheet(ctx context.Context, orgID, employeeID uuid.UUID) (*Timesheet, error)
}

// Shift represents a planned work shift.
type Shift struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	EmployeeID     uuid.UUID  `json:"employee_id"`
	LocationID     *uuid.UUID `json:"location_id,omitempty"`
	StartTime      time.Time  `json:"start_time"`
	EndTime        time.Time  `json:"end_time"`
	BreakMinutes   int        `json:"break_minutes"`
	Status         string     `json:"status"`
	Notes          string     `json:"notes"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Timesheet represents an employee time entry.
type Timesheet struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	EmployeeID     uuid.UUID  `json:"employee_id"`
	ShiftID        *uuid.UUID `json:"shift_id,omitempty"`
	ClockIn        time.Time  `json:"clock_in"`
	ClockOut       *time.Time `json:"clock_out,omitempty"`
	HoursWorked    *float64   `json:"hours_worked,omitempty"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
}

// Employee wraps an entity with HR-specific components.
type Employee struct {
	types.Entity
	Employment map[string]any `json:"employment,omitempty"`
	Salary     map[string]any `json:"salary,omitempty"`
	Contact    map[string]any `json:"contact,omitempty"`
}

// ShiftFilter holds query params for listing shifts.
type ShiftFilter struct {
	EmployeeID *uuid.UUID
	LocationID *uuid.UUID
	From       *time.Time
	To         *time.Time
	Page       types.PageRequest
}

// TimesheetFilter holds query params for listing timesheets.
type TimesheetFilter struct {
	EmployeeID *uuid.UUID
	Status     *string
	From       *time.Time
	To         *time.Time
	Page       types.PageRequest
}

// CreateShiftInput is the input for creating a shift.
type CreateShiftInput struct {
	EmployeeID   uuid.UUID  `json:"employee_id"`
	LocationID   *uuid.UUID `json:"location_id"`
	StartTime    time.Time  `json:"start_time"`
	EndTime      time.Time  `json:"end_time"`
	BreakMinutes int        `json:"break_minutes"`
	Notes        string     `json:"notes"`
}

// UpdateShiftInput is the input for updating a shift.
type UpdateShiftInput struct {
	StartTime    *time.Time `json:"start_time"`
	EndTime      *time.Time `json:"end_time"`
	BreakMinutes *int       `json:"break_minutes"`
	Status       *string    `json:"status"`
	Notes        *string    `json:"notes"`
}

// HireInput is the input for hiring an employee.
type HireInput struct {
	Name         string         `json:"name"`
	Position     string         `json:"position"`
	DepartmentID *uuid.UUID     `json:"department_id"`
	Salary       map[string]any `json:"salary"`
	Contact      map[string]any `json:"contact"`
	Employment   map[string]any `json:"employment"`
}

// UpdateEmployeeInput is the input for updating employee info.
type UpdateEmployeeInput struct {
	Name       *string        `json:"name"`
	Salary     map[string]any `json:"salary"`
	Contact    map[string]any `json:"contact"`
	Employment map[string]any `json:"employment"`
}

// EmployeeFilter holds query params for listing employees.
type EmployeeFilter struct {
	DepartmentID *uuid.UUID
	Status       *string
	Page         types.PageRequest
}
