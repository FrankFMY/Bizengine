package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/module/hr"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// HRRepo implements hr.Repository using PostgreSQL.
type HRRepo struct {
	pool *pgxpool.Pool
}

// NewHRRepo creates a new HRRepo.
func NewHRRepo(pool *pgxpool.Pool) *HRRepo {
	return &HRRepo{pool: pool}
}

// CreateShift inserts a shift record.
func (r *HRRepo) CreateShift(ctx context.Context, s *hr.Shift) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO shifts (id, organization_id, employee_id, location_id, start_time, end_time, break_minutes, status, notes, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		s.ID, s.OrganizationID, s.EmployeeID, s.LocationID, s.StartTime, s.EndTime, s.BreakMinutes, s.Status, s.Notes, s.CreatedAt,
	)
	return err
}

// GetShift returns a shift by ID.
func (r *HRRepo) GetShift(ctx context.Context, orgID, shiftID uuid.UUID) (*hr.Shift, error) {
	var s hr.Shift
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, employee_id, location_id, start_time, end_time, break_minutes, status, notes, created_at
		 FROM shifts WHERE organization_id = $1 AND id = $2`,
		orgID, shiftID,
	).Scan(&s.ID, &s.OrganizationID, &s.EmployeeID, &s.LocationID, &s.StartTime, &s.EndTime, &s.BreakMinutes, &s.Status, &s.Notes, &s.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("shift not found")
		}
		return nil, err
	}
	return &s, nil
}

// ListShifts returns shifts matching the filter.
func (r *HRRepo) ListShifts(ctx context.Context, orgID uuid.UUID, filter hr.ShiftFilter) ([]hr.Shift, int, error) {
	filter.Page.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("organization_id = $%d", argIdx))
	args = append(args, orgID)
	argIdx++

	if filter.EmployeeID != nil {
		conditions = append(conditions, fmt.Sprintf("employee_id = $%d", argIdx))
		args = append(args, *filter.EmployeeID)
		argIdx++
	}
	if filter.LocationID != nil {
		conditions = append(conditions, fmt.Sprintf("location_id = $%d", argIdx))
		args = append(args, *filter.LocationID)
		argIdx++
	}
	if filter.From != nil {
		conditions = append(conditions, fmt.Sprintf("start_time >= $%d", argIdx))
		args = append(args, *filter.From)
		argIdx++
	}
	if filter.To != nil {
		conditions = append(conditions, fmt.Sprintf("end_time <= $%d", argIdx))
		args = append(args, *filter.To)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM shifts WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT id, organization_id, employee_id, location_id, start_time, end_time, break_minutes, status, notes, created_at
		 FROM shifts WHERE %s ORDER BY start_time DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, filter.Page.Limit, filter.Page.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var shifts []hr.Shift
	for rows.Next() {
		var s hr.Shift
		if err := rows.Scan(&s.ID, &s.OrganizationID, &s.EmployeeID, &s.LocationID, &s.StartTime, &s.EndTime, &s.BreakMinutes, &s.Status, &s.Notes, &s.CreatedAt); err != nil {
			return nil, 0, err
		}
		shifts = append(shifts, s)
	}
	return shifts, total, rows.Err()
}

// UpdateShift updates a shift record.
func (r *HRRepo) UpdateShift(ctx context.Context, s *hr.Shift) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE shifts SET start_time = $3, end_time = $4, break_minutes = $5, status = $6, notes = $7
		 WHERE organization_id = $1 AND id = $2`,
		s.OrganizationID, s.ID, s.StartTime, s.EndTime, s.BreakMinutes, s.Status, s.Notes,
	)
	return err
}

// DeleteShift deletes a shift by ID.
func (r *HRRepo) DeleteShift(ctx context.Context, orgID, shiftID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM shifts WHERE organization_id = $1 AND id = $2`,
		orgID, shiftID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NewNotFound("shift not found")
	}
	return nil
}

// HasOverlappingShift checks if there's an overlapping shift for the employee.
func (r *HRRepo) HasOverlappingShift(ctx context.Context, orgID, employeeID uuid.UUID, start, end time.Time, excludeID *uuid.UUID) (bool, error) {
	query := `SELECT EXISTS(
		SELECT 1 FROM shifts
		WHERE organization_id = $1 AND employee_id = $2
		  AND start_time < $4 AND end_time > $3`
	args := []any{orgID, employeeID, start, end}
	if excludeID != nil {
		query += " AND id != $5"
		args = append(args, *excludeID)
	}
	query += ")"

	var exists bool
	err := r.pool.QueryRow(ctx, query, args...).Scan(&exists)
	return exists, err
}

// CreateTimesheet inserts a timesheet record.
func (r *HRRepo) CreateTimesheet(ctx context.Context, ts *hr.Timesheet) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO timesheets (id, organization_id, employee_id, shift_id, clock_in, clock_out, hours_worked, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		ts.ID, ts.OrganizationID, ts.EmployeeID, ts.ShiftID, ts.ClockIn, ts.ClockOut, ts.HoursWorked, ts.Status, ts.CreatedAt,
	)
	return err
}

// GetTimesheet returns a timesheet by ID.
func (r *HRRepo) GetTimesheet(ctx context.Context, orgID, tsID uuid.UUID) (*hr.Timesheet, error) {
	var ts hr.Timesheet
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, employee_id, shift_id, clock_in, clock_out, hours_worked, status, created_at
		 FROM timesheets WHERE organization_id = $1 AND id = $2`,
		orgID, tsID,
	).Scan(&ts.ID, &ts.OrganizationID, &ts.EmployeeID, &ts.ShiftID, &ts.ClockIn, &ts.ClockOut, &ts.HoursWorked, &ts.Status, &ts.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("timesheet not found")
		}
		return nil, err
	}
	return &ts, nil
}

// ListTimesheets returns timesheets matching the filter.
func (r *HRRepo) ListTimesheets(ctx context.Context, orgID uuid.UUID, filter hr.TimesheetFilter) ([]hr.Timesheet, int, error) {
	filter.Page.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("organization_id = $%d", argIdx))
	args = append(args, orgID)
	argIdx++

	if filter.EmployeeID != nil {
		conditions = append(conditions, fmt.Sprintf("employee_id = $%d", argIdx))
		args = append(args, *filter.EmployeeID)
		argIdx++
	}
	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}
	if filter.From != nil {
		conditions = append(conditions, fmt.Sprintf("clock_in >= $%d", argIdx))
		args = append(args, *filter.From)
		argIdx++
	}
	if filter.To != nil {
		conditions = append(conditions, fmt.Sprintf("clock_in <= $%d", argIdx))
		args = append(args, *filter.To)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM timesheets WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT id, organization_id, employee_id, shift_id, clock_in, clock_out, hours_worked, status, created_at
		 FROM timesheets WHERE %s ORDER BY clock_in DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, filter.Page.Limit, filter.Page.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var timesheets []hr.Timesheet
	for rows.Next() {
		var ts hr.Timesheet
		if err := rows.Scan(&ts.ID, &ts.OrganizationID, &ts.EmployeeID, &ts.ShiftID, &ts.ClockIn, &ts.ClockOut, &ts.HoursWorked, &ts.Status, &ts.CreatedAt); err != nil {
			return nil, 0, err
		}
		timesheets = append(timesheets, ts)
	}
	return timesheets, total, rows.Err()
}

// UpdateTimesheet updates a timesheet record.
func (r *HRRepo) UpdateTimesheet(ctx context.Context, ts *hr.Timesheet) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE timesheets SET clock_out = $3, hours_worked = $4, status = $5
		 WHERE organization_id = $1 AND id = $2`,
		ts.OrganizationID, ts.ID, ts.ClockOut, ts.HoursWorked, ts.Status,
	)
	return err
}

// GetOpenTimesheet returns an open timesheet for an employee, or nil.
func (r *HRRepo) GetOpenTimesheet(ctx context.Context, orgID, employeeID uuid.UUID) (*hr.Timesheet, error) {
	var ts hr.Timesheet
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, employee_id, shift_id, clock_in, clock_out, hours_worked, status, created_at
		 FROM timesheets WHERE organization_id = $1 AND employee_id = $2 AND status = 'open' LIMIT 1`,
		orgID, employeeID,
	).Scan(&ts.ID, &ts.OrganizationID, &ts.EmployeeID, &ts.ShiftID, &ts.ClockIn, &ts.ClockOut, &ts.HoursWorked, &ts.Status, &ts.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &ts, nil
}

// CreatePayroll inserts a payroll record.
func (r *HRRepo) CreatePayroll(ctx context.Context, p *hr.Payroll) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO payrolls (id, organization_id, employee_id, year, month, gross_salary, ndfl, deductions, net_salary, status, approved_at, approved_by, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		p.ID, p.OrganizationID, p.EmployeeID, p.Year, p.Month, p.GrossSalary, p.NDFL, p.Deductions, p.NetSalary, p.Status, p.ApprovedAt, p.ApprovedBy, p.CreatedAt,
	)
	return err
}

// GetPayroll returns a payroll by ID.
func (r *HRRepo) GetPayroll(ctx context.Context, orgID, payrollID uuid.UUID) (*hr.Payroll, error) {
	var p hr.Payroll
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, employee_id, year, month, gross_salary, ndfl, deductions, net_salary, status, approved_at, approved_by, created_at
		 FROM payrolls WHERE organization_id = $1 AND id = $2`,
		orgID, payrollID,
	).Scan(&p.ID, &p.OrganizationID, &p.EmployeeID, &p.Year, &p.Month, &p.GrossSalary, &p.NDFL, &p.Deductions, &p.NetSalary, &p.Status, &p.ApprovedAt, &p.ApprovedBy, &p.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("payroll not found")
		}
		return nil, err
	}
	return &p, nil
}

// ListPayrolls returns payrolls matching filters.
func (r *HRRepo) ListPayrolls(ctx context.Context, orgID uuid.UUID, employeeID *uuid.UUID, year, month *int, page types.PageRequest) ([]hr.Payroll, int, error) {
	page.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("organization_id = $%d", argIdx))
	args = append(args, orgID)
	argIdx++

	if employeeID != nil {
		conditions = append(conditions, fmt.Sprintf("employee_id = $%d", argIdx))
		args = append(args, *employeeID)
		argIdx++
	}
	if year != nil {
		conditions = append(conditions, fmt.Sprintf("year = $%d", argIdx))
		args = append(args, *year)
		argIdx++
	}
	if month != nil {
		conditions = append(conditions, fmt.Sprintf("month = $%d", argIdx))
		args = append(args, *month)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM payrolls WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT id, organization_id, employee_id, year, month, gross_salary, ndfl, deductions, net_salary, status, approved_at, approved_by, created_at
		 FROM payrolls WHERE %s ORDER BY year DESC, month DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, page.Limit, page.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var payrolls []hr.Payroll
	for rows.Next() {
		var p hr.Payroll
		if err := rows.Scan(&p.ID, &p.OrganizationID, &p.EmployeeID, &p.Year, &p.Month, &p.GrossSalary, &p.NDFL, &p.Deductions, &p.NetSalary, &p.Status, &p.ApprovedAt, &p.ApprovedBy, &p.CreatedAt); err != nil {
			return nil, 0, err
		}
		payrolls = append(payrolls, p)
	}
	return payrolls, total, rows.Err()
}

// UpdatePayroll updates a payroll record.
func (r *HRRepo) UpdatePayroll(ctx context.Context, p *hr.Payroll) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE payrolls SET status = $3, approved_at = $4, approved_by = $5 WHERE organization_id = $1 AND id = $2`,
		p.OrganizationID, p.ID, p.Status, p.ApprovedAt, p.ApprovedBy,
	)
	return err
}

// CreateAbsence inserts an absence record.
func (r *HRRepo) CreateAbsence(ctx context.Context, a *hr.Absence) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO absences (id, organization_id, employee_id, type, start_date, end_date, days, status, notes, approved_by, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		a.ID, a.OrganizationID, a.EmployeeID, a.Type, a.StartDate, a.EndDate, a.Days, a.Status, a.Notes, a.ApprovedBy, a.CreatedAt,
	)
	return err
}

// GetAbsence returns an absence by ID.
func (r *HRRepo) GetAbsence(ctx context.Context, orgID, absenceID uuid.UUID) (*hr.Absence, error) {
	var a hr.Absence
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, employee_id, type, start_date, end_date, days, status, notes, approved_by, created_at
		 FROM absences WHERE organization_id = $1 AND id = $2`,
		orgID, absenceID,
	).Scan(&a.ID, &a.OrganizationID, &a.EmployeeID, &a.Type, &a.StartDate, &a.EndDate, &a.Days, &a.Status, &a.Notes, &a.ApprovedBy, &a.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("absence not found")
		}
		return nil, err
	}
	return &a, nil
}

// ListAbsences returns absences matching the filter.
func (r *HRRepo) ListAbsences(ctx context.Context, orgID uuid.UUID, filter hr.AbsenceFilter) ([]hr.Absence, int, error) {
	filter.Page.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("organization_id = $%d", argIdx))
	args = append(args, orgID)
	argIdx++

	if filter.EmployeeID != nil {
		conditions = append(conditions, fmt.Sprintf("employee_id = $%d", argIdx))
		args = append(args, *filter.EmployeeID)
		argIdx++
	}
	if filter.Type != nil {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *filter.Type)
		argIdx++
	}
	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM absences WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT id, organization_id, employee_id, type, start_date, end_date, days, status, notes, approved_by, created_at
		 FROM absences WHERE %s ORDER BY start_date DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, filter.Page.Limit, filter.Page.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var absences []hr.Absence
	for rows.Next() {
		var a hr.Absence
		if err := rows.Scan(&a.ID, &a.OrganizationID, &a.EmployeeID, &a.Type, &a.StartDate, &a.EndDate, &a.Days, &a.Status, &a.Notes, &a.ApprovedBy, &a.CreatedAt); err != nil {
			return nil, 0, err
		}
		absences = append(absences, a)
	}
	return absences, total, rows.Err()
}

// UpdateAbsence updates an absence record.
func (r *HRRepo) UpdateAbsence(ctx context.Context, a *hr.Absence) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE absences SET status = $3, approved_by = $4 WHERE organization_id = $1 AND id = $2`,
		a.OrganizationID, a.ID, a.Status, a.ApprovedBy,
	)
	return err
}
