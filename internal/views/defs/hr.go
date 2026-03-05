package defs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/views"
)

// HREmployeesList returns a paginated list of employees.
var HREmployeesList = &views.ViewDef{
	Key: "hr_employees_list",
	Tables: []views.TableDep{
		{Table: "entities", Columns: []string{"name", "status"}, Filter: "kind=employee"},
		{Table: "components", Columns: []string{"data"}, Filter: "type=hr"},
	},
	ParamSchema: map[string]string{
		"search": "string",
		"status": "string",
	},
	Factory: hrEmployeesListFactory,
}

func hrEmployeesListFactory(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, params map[string]any) (*views.ViewResult, error) {
	limit := intParam(params, "limit", 50)
	offset := intParam(params, "offset", 0)
	search, _ := params["search"].(string)
	status, _ := params["status"].(string)

	query := `
		SELECT e.id, e.name, e.status,
		       COALESCE(hr.data->>'position','') AS position,
		       COALESCE(hr.data->>'department','') AS department,
		       COALESCE(hr.data->>'phone','') AS phone,
		       COUNT(*) OVER() AS total_count
		FROM entities e
		LEFT JOIN components hr ON hr.entity_id = e.id AND hr.type = 'hr' AND hr.organization_id = $1
		WHERE e.organization_id = $1 AND e.kind = 'employee' AND e.deleted_at IS NULL
	`
	args := []any{orgID}
	argIdx := 2

	if search != "" {
		query += fmt.Sprintf(` AND e.name ILIKE $%d`, argIdx)
		args = append(args, "%"+search+"%")
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(` AND e.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY e.name LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]views.DataRef, 0)
	tables := map[string]map[string]any{"employees": {}}
	var total int

	for rows.Next() {
		var id, name, st, position, department, phone string
		var cnt int

		if err := rows.Scan(&id, &name, &st, &position, &department, &phone, &cnt); err != nil {
			return nil, err
		}
		total = cnt

		refs = append(refs, views.DataRef{Table: "employees", ID: id, Fields: []string{"name", "status", "position", "department"}})
		tables["employees"][id] = map[string]any{
			"id":         id,
			"name":       name,
			"status":     st,
			"position":   position,
			"department": department,
			"phone":      phone,
		}
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: int64(total)}, nil
}

// HREmployeeDetail returns full employee details.
var HREmployeeDetail = &views.ViewDef{
	Key: "hr_employee_detail",
	Tables: []views.TableDep{
		{Table: "entities", Columns: []string{"name", "status"}, Filter: "kind=employee"},
		{Table: "components", Columns: []string{"data"}},
		{Table: "shifts", Columns: []string{"start_time", "end_time", "status"}},
	},
	ParamSchema: map[string]string{
		"employee_id": "uuid",
	},
	Factory: hrEmployeeDetailFactory,
}

func hrEmployeeDetailFactory(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, params map[string]any) (*views.ViewResult, error) {
	employeeID, _ := params["employee_id"].(string)

	tables := map[string]map[string]any{"employees": {}}
	refs := make([]views.DataRef, 0, 1)

	var id, name, status string
	err := pool.QueryRow(ctx, `
		SELECT id, name, status FROM entities
		WHERE organization_id = $1 AND id = $2 AND kind = 'employee' AND deleted_at IS NULL
	`, orgID, employeeID).Scan(&id, &name, &status)
	if err != nil {
		return nil, err
	}

	emp := map[string]any{"id": id, "name": name, "status": status}

	compRows, err := pool.Query(ctx, `
		SELECT type, data FROM components WHERE organization_id = $1 AND entity_id = $2
	`, orgID, employeeID)
	if err != nil {
		return nil, err
	}
	defer compRows.Close()

	for compRows.Next() {
		var compType string
		var compData any
		if err := compRows.Scan(&compType, &compData); err != nil {
			return nil, err
		}
		emp[compType] = compData
	}

	refs = append(refs, views.DataRef{Table: "employees", ID: id, Fields: []string{"name", "status", "hr"}})
	tables["employees"][id] = emp

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}

// HRShiftsSchedule returns shifts for the current week.
var HRShiftsSchedule = &views.ViewDef{
	Key: "hr_shifts_schedule",
	Tables: []views.TableDep{
		{Table: "shifts", Columns: []string{"start_time", "end_time", "status"}},
		{Table: "entities", Columns: []string{"name"}, Filter: "kind=employee"},
	},
	Factory: hrShiftsScheduleFactory,
}

func hrShiftsScheduleFactory(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, _ map[string]any) (*views.ViewResult, error) {
	rows, err := pool.Query(ctx, `
		SELECT s.id, e.name AS employee_name, s.start_time, s.end_time,
		       s.break_minutes, s.status
		FROM shifts s
		JOIN entities e ON e.id = s.employee_id AND e.organization_id = $1
		WHERE s.organization_id = $1
		  AND s.start_time >= date_trunc('week', CURRENT_DATE)
		  AND s.start_time < date_trunc('week', CURRENT_DATE) + INTERVAL '7 days'
		ORDER BY s.start_time
	`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]views.DataRef, 0)
	tables := map[string]map[string]any{"shifts": {}}

	for rows.Next() {
		var id, empName, status string
		var startTime, endTime any
		var breakMin int

		if err := rows.Scan(&id, &empName, &startTime, &endTime, &breakMin, &status); err != nil {
			return nil, err
		}

		refs = append(refs, views.DataRef{Table: "shifts", ID: id, Fields: []string{"employee_name", "start_time", "end_time", "status"}})
		tables["shifts"][id] = map[string]any{
			"id":              id,
			"employee_name":   empName,
			"start_time":      startTime,
			"end_time":        endTime,
			"break_minutes":   breakMin,
			"status":          status,
		}
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}

// HRTimesheetsList returns timesheets.
var HRTimesheetsList = &views.ViewDef{
	Key: "hr_timesheets_list",
	Tables: []views.TableDep{
		{Table: "timesheets", Columns: []string{"clock_in", "clock_out", "status"}},
		{Table: "entities", Columns: []string{"name"}, Filter: "kind=employee"},
	},
	ParamSchema: map[string]string{
		"employee_id": "uuid",
		"status":      "string",
	},
	Factory: hrTimesheetsListFactory,
}

func hrTimesheetsListFactory(ctx context.Context, pool *pgxpool.Pool, orgID uuid.UUID, params map[string]any) (*views.ViewResult, error) {
	limit := intParam(params, "limit", 50)
	offset := intParam(params, "offset", 0)
	employeeID, _ := params["employee_id"].(string)
	status, _ := params["status"].(string)

	query := `
		SELECT t.id, e.name AS employee_name, t.clock_in, t.clock_out,
		       t.hours_worked, t.status, COUNT(*) OVER() AS total_count
		FROM timesheets t
		JOIN entities e ON e.id = t.employee_id AND e.organization_id = $1
		WHERE t.organization_id = $1
	`
	args := []any{orgID}
	argIdx := 2

	if employeeID != "" {
		query += fmt.Sprintf(` AND t.employee_id = $%d`, argIdx)
		args = append(args, employeeID)
		argIdx++
	}
	if status != "" {
		query += fmt.Sprintf(` AND t.status = $%d`, argIdx)
		args = append(args, status)
		argIdx++
	}

	query += fmt.Sprintf(` ORDER BY t.clock_in DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]views.DataRef, 0)
	tables := map[string]map[string]any{"timesheets": {}}

	for rows.Next() {
		var id, empName, st string
		var clockIn, clockOut any
		var hoursWorked *float64
		var cnt int

		if err := rows.Scan(&id, &empName, &clockIn, &clockOut, &hoursWorked, &st, &cnt); err != nil {
			return nil, err
		}

		refs = append(refs, views.DataRef{Table: "timesheets", ID: id, Fields: []string{"employee_name", "clock_in", "clock_out", "hours_worked", "status"}})
		row := map[string]any{
			"id":            id,
			"employee_name": empName,
			"clock_in":      clockIn,
			"clock_out":     clockOut,
			"status":        st,
		}
		if hoursWorked != nil {
			row["hours_worked"] = *hoursWorked
		}
		tables["timesheets"][id] = row
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}
