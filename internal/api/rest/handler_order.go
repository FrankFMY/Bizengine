package rest

import (
	"net/http"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/module/order"
)

// OrderHandler handles order endpoints.
type OrderHandler struct {
	orderSvc *order.Service
}

// NewOrderHandler creates a new OrderHandler.
func NewOrderHandler(orderSvc *order.Service) *OrderHandler {
	return &OrderHandler{orderSvc: orderSvc}
}

// Create handles POST /api/v1/workspaces/{wsID}/orders.
func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input order.CreateOrderInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	o, err := h.orderSvc.Create(r.Context(), wsID, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w,o)
}

// List handles GET /api/v1/workspaces/{wsID}/orders.
func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	filter := order.OrderFilter{
		Status:     queryString(r, "status"),
		CustomerID: queryUUID(r, "customer_id"),
		Search:     queryString(r, "search"),
		Page:       parsePage(r),
	}

	result, err := h.orderSvc.List(r.Context(), wsID, filter)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,result)
}

// Get handles GET /api/v1/workspaces/{wsID}/orders/{id}.
func (h *OrderHandler) Get(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}
	id, err := parseUUID(r, "id")
	if err != nil {
		respondError(w, err)
		return
	}

	include := r.URL.Query().Get("include")
	o, err := h.orderSvc.Get(r.Context(), wsID, id, include == "items")
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,o)
}

// Update handles PUT /api/v1/workspaces/{wsID}/orders/{id}.
func (h *OrderHandler) Update(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
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

	var input order.UpdateOrderInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	o, err := h.orderSvc.Update(r.Context(), wsID, id, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,o)
}

// Confirm handles POST /api/v1/workspaces/{wsID}/orders/{id}/confirm.
func (h *OrderHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
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

	o, err := h.orderSvc.Confirm(r.Context(), wsID, id, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,o)
}

// Pay handles POST /api/v1/workspaces/{wsID}/orders/{id}/pay.
func (h *OrderHandler) Pay(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
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

	var input order.PayInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	o, err := h.orderSvc.Pay(r.Context(), wsID, id, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,o)
}

// Ship handles POST /api/v1/workspaces/{wsID}/orders/{id}/ship.
func (h *OrderHandler) Ship(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
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

	var input order.ShipInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	o, err := h.orderSvc.Ship(r.Context(), wsID, id, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,o)
}

// Deliver handles POST /api/v1/workspaces/{wsID}/orders/{id}/deliver.
func (h *OrderHandler) Deliver(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
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

	o, err := h.orderSvc.Deliver(r.Context(), wsID, id, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,o)
}

// Cancel handles POST /api/v1/workspaces/{wsID}/orders/{id}/cancel.
func (h *OrderHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
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

	o, err := h.orderSvc.Cancel(r.Context(), wsID, id, input.Reason, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,o)
}
