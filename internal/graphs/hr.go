package graphs

import (
	"context"
	"fmt"

	"github.com/FrankFMY/arcana"
)

var hrEmployeesList = arcana.GraphDef{
	Key: "hr_employees_list",
	Deps: []arcana.TableDep{
		{Table: "entities", Columns: []string{"name", "status"}},
		{Table: "components", Columns: []string{"data"}},
	},
	Params: arcana.ParamSchema{
		"search": arcana.ParamString().Build(),
		"status": arcana.ParamString().Build(),
		"limit":  arcana.ParamInt().Default(50),
		"offset": arcana.ParamInt().Default(0),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		limit := p.Int("limit")
		offset := p.Int("offset")
		search := p.String("search")
		status := p.String("status")

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

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()

		for rows.Next() {
			var id, name, st, position, department, phone string
			var cnt int

			if err := rows.Scan(&id, &name, &st, &position, &department, &phone, &cnt); err != nil {
				return nil, err
			}

			result.AddRef(arcana.Ref{Table: "employees", ID: id, Fields: []string{"name", "status", "position", "department"}})
			result.AddRow("employees", id, map[string]any{
				"id":         id,
				"name":       name,
				"status":     st,
				"position":   position,
				"department": department,
				"phone":      phone,
			})
		}

		return result, rows.Err()
	},
}

var hrEmployeeDetail = arcana.GraphDef{
	Key: "hr_employee_detail",
	Deps: []arcana.TableDep{
		{Table: "entities", Columns: []string{"name", "status"}},
		{Table: "components", Columns: []string{"data"}},
		{Table: "shifts", Columns: []string{"start_time", "end_time", "status"}},
	},
	Params: arcana.ParamSchema{
		"employee_id": arcana.ParamUUID().Required(),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		employeeID := p.UUID("employee_id")

		result := arcana.NewResult()

		var id, name, status string
		err := q.QueryRow(ctx, `
			SELECT id, name, status FROM entities
			WHERE organization_id = $1 AND id = $2 AND kind = 'employee' AND deleted_at IS NULL
		`, orgID, employeeID).Scan(&id, &name, &status)
		if err != nil {
			return nil, err
		}

		emp := map[string]any{"id": id, "name": name, "status": status}

		compRows, err := q.Query(ctx, `
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

		result.AddRef(arcana.Ref{Table: "employees", ID: id, Fields: []string{"name", "status", "hr"}})
		result.AddRow("employees", id, emp)

		return result, nil
	},
}

var hrShiftsSchedule = arcana.GraphDef{
	Key: "hr_shifts_schedule",
	Deps: []arcana.TableDep{
		{Table: "shifts", Columns: []string{"start_time", "end_time", "status"}},
		{Table: "entities", Columns: []string{"name"}},
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)

		rows, err := q.Query(ctx, `
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

		result := arcana.NewResult()

		for rows.Next() {
			var id, empName, status string
			var startTime, endTime any
			var breakMin int

			if err := rows.Scan(&id, &empName, &startTime, &endTime, &breakMin, &status); err != nil {
				return nil, err
			}

			result.AddRef(arcana.Ref{Table: "shifts", ID: id, Fields: []string{"employee_name", "start_time", "end_time", "status"}})
			result.AddRow("shifts", id, map[string]any{
				"id":            id,
				"employee_name": empName,
				"start_time":    startTime,
				"end_time":      endTime,
				"break_minutes": breakMin,
				"status":        status,
			})
		}

		return result, rows.Err()
	},
}

var hrTimesheetsList = arcana.GraphDef{
	Key: "hr_timesheets_list",
	Deps: []arcana.TableDep{
		{Table: "timesheets", Columns: []string{"clock_in", "clock_out", "status"}},
		{Table: "entities", Columns: []string{"name"}},
	},
	Params: arcana.ParamSchema{
		"employee_id": arcana.ParamUUID().Build(),
		"status":      arcana.ParamString().Build(),
		"limit":       arcana.ParamInt().Default(50),
		"offset":      arcana.ParamInt().Default(0),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		limit := p.Int("limit")
		offset := p.Int("offset")
		employeeID := p.UUID("employee_id")
		status := p.String("status")

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

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()

		for rows.Next() {
			var id, empName, st string
			var clockIn, clockOut any
			var hoursWorked *float64
			var cnt int

			if err := rows.Scan(&id, &empName, &clockIn, &clockOut, &hoursWorked, &st, &cnt); err != nil {
				return nil, err
			}

			result.AddRef(arcana.Ref{Table: "timesheets", ID: id, Fields: []string{"employee_name", "clock_in", "clock_out", "hours_worked", "status"}})
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
			result.AddRow("timesheets", id, row)
		}

		return result, rows.Err()
	},
}
