package rest

import (
	"net/http"
	"strconv"

	"github.com/bizengine/engine/internal/analytics"
)

type AnalyticsHandler struct {
	svc *analytics.Service
}

func NewAnalyticsHandler(svc *analytics.Service) *AnalyticsHandler {
	return &AnalyticsHandler{svc: svc}
}

// Dashboard handles GET /analytics/dashboard.
func (h *AnalyticsHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	data, err := h.svc.GetDashboard(r.Context(), orgID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, data)
}

// RevenueSeries handles GET /analytics/revenue?days=30.
func (h *AnalyticsHandler) RevenueSeries(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	days := 30
	if v := r.URL.Query().Get("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}

	points, err := h.svc.GetRevenueSeries(r.Context(), orgID, days)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, points)
}
