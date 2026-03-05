package rest

import (
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/module/hr"
)

// HRHandler handles HR endpoints.
type HRHandler struct {
	hrSvc *hr.Service
}

// NewHRHandler creates a new HRHandler.
func NewHRHandler(hrSvc *hr.Service) *HRHandler {
	return &HRHandler{hrSvc: hrSvc}
}

// HireEmployee handles POST /api/v1/organizations/{orgID}/hr/employees.
func (h *HRHandler) HireEmployee(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input hr.HireInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	emp, err := h.hrSvc.HireEmployee(r.Context(), orgID, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, emp)
}

// ListEmployees handles GET /api/v1/organizations/{orgID}/hr/employees.
func (h *HRHandler) ListEmployees(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	filter := hr.EmployeeFilter{
		DepartmentID: queryUUID(r, "department_id"),
		Status:       queryString(r, "status"),
		Page:         parsePage(r),
	}

	employees, total, err := h.hrSvc.ListEmployees(r.Context(), orgID, filter)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]any{
		"items":  employees,
		"total":  total,
		"limit":  filter.Page.Limit,
		"offset": filter.Page.Offset,
	})
}

// GetEmployee handles GET /api/v1/organizations/{orgID}/hr/employees/{id}.
func (h *HRHandler) GetEmployee(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}

	emp, err := h.hrSvc.GetEmployee(r.Context(), orgID, id)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, emp)
}

// UpdateEmployee handles PUT /api/v1/organizations/{orgID}/hr/employees/{id}.
func (h *HRHandler) UpdateEmployee(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input hr.UpdateEmployeeInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	emp, err := h.hrSvc.UpdateEmployee(r.Context(), orgID, id, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, emp)
}

// TerminateEmployee handles POST /api/v1/organizations/{orgID}/hr/employees/{id}/terminate.
func (h *HRHandler) TerminateEmployee(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	if err := h.hrSvc.TerminateEmployee(r.Context(), orgID, id, input.Reason, &userID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, nil)
}

// CreateShift handles POST /api/v1/organizations/{orgID}/hr/shifts.
func (h *HRHandler) CreateShift(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input hr.CreateShiftInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	shift, err := h.hrSvc.CreateShift(r.Context(), orgID, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, shift)
}

// ListShifts handles GET /api/v1/organizations/{orgID}/hr/shifts.
func (h *HRHandler) ListShifts(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	filter := hr.ShiftFilter{
		EmployeeID: queryUUID(r, "employee_id"),
		LocationID: queryUUID(r, "location_id"),
		Page:       parsePage(r),
	}

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			filter.From = &t
		} else if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			filter.From = &t
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			filter.To = &t
		} else if t, err := time.Parse("2006-01-02", toStr); err == nil {
			filter.To = &t
		}
	}

	shifts, total, err := h.hrSvc.ListShifts(r.Context(), orgID, filter)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]any{
		"items":  shifts,
		"total":  total,
		"limit":  filter.Page.Limit,
		"offset": filter.Page.Offset,
	})
}

// UpdateShift handles PUT /api/v1/organizations/{orgID}/hr/shifts/{id}.
func (h *HRHandler) UpdateShift(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input hr.UpdateShiftInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	shift, err := h.hrSvc.UpdateShift(r.Context(), orgID, id, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, shift)
}

// DeleteShift handles DELETE /api/v1/organizations/{orgID}/hr/shifts/{id}.
func (h *HRHandler) DeleteShift(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}

	if err := h.hrSvc.DeleteShift(r.Context(), orgID, id); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, nil)
}

// ClockIn handles POST /api/v1/organizations/{orgID}/hr/timesheets/clock-in.
func (h *HRHandler) ClockIn(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input struct {
		EmployeeID string  `json:"employee_id"`
		ShiftID    *string `json:"shift_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	empID, err := parseUUIDString(input.EmployeeID)
	if err != nil {
		respondError(w, err)
		return
	}

	var shiftID *uuid.UUID
	if input.ShiftID != nil && *input.ShiftID != "" {
		id, err := parseUUIDString(*input.ShiftID)
		if err != nil {
			respondError(w, err)
			return
		}
		shiftID = &id
	}

	ts, err := h.hrSvc.ClockIn(r.Context(), orgID, empID, shiftID, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, ts)
}

// ClockOut handles POST /api/v1/organizations/{orgID}/hr/timesheets/{id}/clock-out.
func (h *HRHandler) ClockOut(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	ts, err := h.hrSvc.ClockOut(r.Context(), orgID, id, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, ts)
}

// ListTimesheets handles GET /api/v1/organizations/{orgID}/hr/timesheets.
func (h *HRHandler) ListTimesheets(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	filter := hr.TimesheetFilter{
		EmployeeID: queryUUID(r, "employee_id"),
		Status:     queryString(r, "status"),
		Page:       parsePage(r),
	}

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			filter.From = &t
		} else if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			filter.From = &t
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			filter.To = &t
		} else if t, err := time.Parse("2006-01-02", toStr); err == nil {
			filter.To = &t
		}
	}

	timesheets, total, err := h.hrSvc.ListTimesheets(r.Context(), orgID, filter)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]any{
		"items":  timesheets,
		"total":  total,
		"limit":  filter.Page.Limit,
		"offset": filter.Page.Offset,
	})
}

// ApproveTimesheet handles POST /api/v1/organizations/{orgID}/hr/timesheets/{id}/approve.
func (h *HRHandler) ApproveTimesheet(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	if err := h.hrSvc.ApproveTimesheet(r.Context(), orgID, id, &userID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, nil)
}

// CalculatePayroll handles POST /api/v1/organizations/{orgID}/hr/payroll.
func (h *HRHandler) CalculatePayroll(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input hr.CreatePayrollInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	p, err := h.hrSvc.CalculatePayroll(r.Context(), orgID, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, p)
}

// ApprovePayroll handles POST /api/v1/organizations/{orgID}/hr/payroll/{id}/approve.
func (h *HRHandler) ApprovePayroll(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	p, err := h.hrSvc.ApprovePayroll(r.Context(), orgID, id, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, p)
}

// ListPayrolls handles GET /api/v1/organizations/{orgID}/hr/payroll.
func (h *HRHandler) ListPayrolls(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	employeeID := queryUUID(r, "employee_id")
	var year, month *int
	if v := r.URL.Query().Get("year"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			year = &n
		}
	}
	if v := r.URL.Query().Get("month"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			month = &n
		}
	}

	resp, err := h.hrSvc.ListPayrolls(r.Context(), orgID, employeeID, year, month, parsePage(r))
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, resp)
}

// RequestAbsence handles POST /api/v1/organizations/{orgID}/hr/absences.
func (h *HRHandler) RequestAbsence(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input hr.CreateAbsenceInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	a, err := h.hrSvc.RequestAbsence(r.Context(), orgID, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, a)
}

// ApproveAbsence handles POST /api/v1/organizations/{orgID}/hr/absences/{id}/approve.
func (h *HRHandler) ApproveAbsence(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	a, err := h.hrSvc.ApproveAbsence(r.Context(), orgID, id, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, a)
}

// RejectAbsence handles POST /api/v1/organizations/{orgID}/hr/absences/{id}/reject.
func (h *HRHandler) RejectAbsence(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	a, err := h.hrSvc.RejectAbsence(r.Context(), orgID, id, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, a)
}

// ListAbsences handles GET /api/v1/organizations/{orgID}/hr/absences.
func (h *HRHandler) ListAbsences(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	filter := hr.AbsenceFilter{
		EmployeeID: queryUUID(r, "employee_id"),
		Type:       queryString(r, "type"),
		Status:     queryString(r, "status"),
		Page:       parsePage(r),
	}

	resp, err := h.hrSvc.ListAbsences(r.Context(), orgID, filter)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, resp)
}
