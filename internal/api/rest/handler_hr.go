package rest

import (
	"net/http"
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

// HireEmployee handles POST /api/v1/workspaces/{wsID}/hr/employees.
func (h *HRHandler) HireEmployee(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input hr.HireInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}

	emp, err := h.hrSvc.HireEmployee(r.Context(), wsID, input, &userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, emp)
}

// ListEmployees handles GET /api/v1/workspaces/{wsID}/hr/employees.
func (h *HRHandler) ListEmployees(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}

	filter := hr.EmployeeFilter{
		DepartmentID: queryUUID(r, "department_id"),
		Status:       queryString(r, "status"),
		Page:         parsePage(r),
	}

	employees, total, err := h.hrSvc.ListEmployees(r.Context(), wsID, filter)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  employees,
		"total":  total,
		"limit":  filter.Page.Limit,
		"offset": filter.Page.Offset,
	})
}

// GetEmployee handles GET /api/v1/workspaces/{wsID}/hr/employees/{id}.
func (h *HRHandler) GetEmployee(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}

	emp, err := h.hrSvc.GetEmployee(r.Context(), wsID, id)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, emp)
}

// UpdateEmployee handles PUT /api/v1/workspaces/{wsID}/hr/employees/{id}.
func (h *HRHandler) UpdateEmployee(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input hr.UpdateEmployeeInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}

	emp, err := h.hrSvc.UpdateEmployee(r.Context(), wsID, id, input, &userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, emp)
}

// TerminateEmployee handles POST /api/v1/workspaces/{wsID}/hr/employees/{id}/terminate.
func (h *HRHandler) TerminateEmployee(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}

	if err := h.hrSvc.TerminateEmployee(r.Context(), wsID, id, input.Reason, &userID); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// CreateShift handles POST /api/v1/workspaces/{wsID}/hr/shifts.
func (h *HRHandler) CreateShift(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input hr.CreateShiftInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}

	shift, err := h.hrSvc.CreateShift(r.Context(), wsID, input, &userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, shift)
}

// ListShifts handles GET /api/v1/workspaces/{wsID}/hr/shifts.
func (h *HRHandler) ListShifts(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
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

	shifts, total, err := h.hrSvc.ListShifts(r.Context(), wsID, filter)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  shifts,
		"total":  total,
		"limit":  filter.Page.Limit,
		"offset": filter.Page.Offset,
	})
}

// UpdateShift handles PUT /api/v1/workspaces/{wsID}/hr/shifts/{id}.
func (h *HRHandler) UpdateShift(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input hr.UpdateShiftInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}

	shift, err := h.hrSvc.UpdateShift(r.Context(), wsID, id, input, &userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, shift)
}

// DeleteShift handles DELETE /api/v1/workspaces/{wsID}/hr/shifts/{id}.
func (h *HRHandler) DeleteShift(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}

	if err := h.hrSvc.DeleteShift(r.Context(), wsID, id); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ClockIn handles POST /api/v1/workspaces/{wsID}/hr/timesheets/clock-in.
func (h *HRHandler) ClockIn(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input struct {
		EmployeeID string  `json:"employee_id"`
		ShiftID    *string `json:"shift_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}

	empID, err := parseUUIDString(input.EmployeeID)
	if err != nil {
		writeError(w, err)
		return
	}

	var shiftID *uuid.UUID
	if input.ShiftID != nil && *input.ShiftID != "" {
		id, err := parseUUIDString(*input.ShiftID)
		if err != nil {
			writeError(w, err)
			return
		}
		shiftID = &id
	}

	ts, err := h.hrSvc.ClockIn(r.Context(), wsID, empID, shiftID, &userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, ts)
}

// ClockOut handles POST /api/v1/workspaces/{wsID}/hr/timesheets/{id}/clock-out.
func (h *HRHandler) ClockOut(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	ts, err := h.hrSvc.ClockOut(r.Context(), wsID, id, &userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, ts)
}

// ListTimesheets handles GET /api/v1/workspaces/{wsID}/hr/timesheets.
func (h *HRHandler) ListTimesheets(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
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

	timesheets, total, err := h.hrSvc.ListTimesheets(r.Context(), wsID, filter)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  timesheets,
		"total":  total,
		"limit":  filter.Page.Limit,
		"offset": filter.Page.Offset,
	})
}

// ApproveTimesheet handles POST /api/v1/workspaces/{wsID}/hr/timesheets/{id}/approve.
func (h *HRHandler) ApproveTimesheet(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	if err := h.hrSvc.ApproveTimesheet(r.Context(), wsID, id, &userID); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
