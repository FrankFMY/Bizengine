package rest

import (
	"net/http"
	"time"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/module/logistics"
	"github.com/bizengine/engine/pkg/errs"
)

// LogisticsHandler handles logistics endpoints.
type LogisticsHandler struct {
	svc *logistics.Service
}

// NewLogisticsHandler creates a new LogisticsHandler.
func NewLogisticsHandler(svc *logistics.Service) *LogisticsHandler {
	return &LogisticsHandler{svc: svc}
}

// CreateRoute handles POST /api/v1/workspaces/{wsID}/logistics/routes.
func (h *LogisticsHandler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input logistics.CreateRouteInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}

	route, err := h.svc.CreateRoute(r.Context(), wsID, input, &userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, route)
}

// ListRoutes handles GET /api/v1/workspaces/{wsID}/logistics/routes.
func (h *LogisticsHandler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}

	filter := logistics.RouteFilter{
		Status:   queryString(r, "status"),
		DriverID: queryUUID(r, "driver_id"),
		Page:     parsePage(r),
	}

	routes, total, err := h.svc.ListRoutes(r.Context(), wsID, filter)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  routes,
		"total":  total,
		"limit":  filter.Page.Limit,
		"offset": filter.Page.Offset,
	})
}

// GetRoute handles GET /api/v1/workspaces/{wsID}/logistics/routes/{id}.
func (h *LogisticsHandler) GetRoute(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	routeID, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}

	route, err := h.svc.GetRoute(r.Context(), wsID, routeID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, route)
}

// StartRoute handles POST /api/v1/workspaces/{wsID}/logistics/routes/{id}/start.
func (h *LogisticsHandler) StartRoute(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	routeID, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	route, err := h.svc.StartRoute(r.Context(), wsID, routeID, &userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, route)
}

// CompleteRoute handles POST /api/v1/workspaces/{wsID}/logistics/routes/{id}/complete.
func (h *LogisticsHandler) CompleteRoute(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	routeID, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	route, err := h.svc.CompleteRoute(r.Context(), wsID, routeID, &userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, route)
}

// ArriveAtStop handles POST /api/v1/workspaces/{wsID}/logistics/routes/{id}/stops/{stopID}/arrive.
func (h *LogisticsHandler) ArriveAtStop(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	routeID, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	stopID, err := parseUUID(r, "stopID")
	if err != nil {
		writeError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	stop, err := h.svc.ArriveAtStop(r.Context(), wsID, routeID, stopID, &userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, stop)
}

// CompleteStop handles POST /api/v1/workspaces/{wsID}/logistics/routes/{id}/stops/{stopID}/complete.
func (h *LogisticsHandler) CompleteStop(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	routeID, err := parseUUID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	stopID, err := parseUUID(r, "stopID")
	if err != nil {
		writeError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	stop, err := h.svc.CompleteStop(r.Context(), wsID, routeID, stopID, &userID)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, stop)
}

// UpdateGeo handles POST /api/v1/workspaces/{wsID}/logistics/geo.
func (h *LogisticsHandler) UpdateGeo(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var body struct {
		EntityID  string          `json:"entity_id"`
		Latitude  float64         `json:"latitude"`
		Longitude float64         `json:"longitude"`
		Speed     float64         `json:"speed"`
		Heading   float64         `json:"heading"`
		RecordedAt *time.Time     `json:"recorded_at,omitempty"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, err)
		return
	}

	entityID, err := parseUUIDString(body.EntityID)
	if err != nil {
		writeError(w, err)
		return
	}

	point := logistics.GeoPoint{
		Latitude:  body.Latitude,
		Longitude: body.Longitude,
		Speed:     body.Speed,
		Heading:   body.Heading,
	}
	if body.RecordedAt != nil {
		point.RecordedAt = *body.RecordedAt
	}

	if err := h.svc.UpdateGeo(r.Context(), wsID, entityID, point, &userID); err != nil {
		writeError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetTrack handles GET /api/v1/workspaces/{wsID}/logistics/geo/{entityID}/track.
func (h *LogisticsHandler) GetTrack(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		writeError(w, err)
		return
	}
	entityID, err := parseUUID(r, "entityID")
	if err != nil {
		writeError(w, err)
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		writeError(w, errs.NewBadRequest("from and to query parameters are required"))
		return
	}

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		writeError(w, errs.NewBadRequest("invalid from time format, use RFC3339"))
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		writeError(w, errs.NewBadRequest("invalid to time format, use RFC3339"))
		return
	}

	points, err := h.svc.GetTrack(r.Context(), wsID, entityID, from, to)
	if err != nil {
		writeError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, points)
}
