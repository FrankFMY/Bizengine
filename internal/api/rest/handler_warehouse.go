package rest

import (
	"net/http"
	"time"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/module/warehouse"
	"github.com/bizengine/engine/pkg/errs"
)

// WarehouseHandler handles warehouse endpoints.
type WarehouseHandler struct {
	warehouseSvc *warehouse.Service
}

// NewWarehouseHandler creates a new WarehouseHandler.
func NewWarehouseHandler(warehouseSvc *warehouse.Service) *WarehouseHandler {
	return &WarehouseHandler{warehouseSvc: warehouseSvc}
}

// Receive handles POST /api/v1/organizations/{orgID}/warehouse/receive.
func (h *WarehouseHandler) Receive(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input warehouse.ReceiveInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}
	input.ActorID = &userID

	result, err := h.warehouseSvc.Receive(r.Context(), orgID, input)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,result)
}

// Ship handles POST /api/v1/organizations/{orgID}/warehouse/ship.
func (h *WarehouseHandler) Ship(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input warehouse.ShipInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}
	input.ActorID = &userID

	result, err := h.warehouseSvc.Ship(r.Context(), orgID, input)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,result)
}

// Transfer handles POST /api/v1/organizations/{orgID}/warehouse/transfer.
func (h *WarehouseHandler) Transfer(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input warehouse.TransferInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}
	input.ActorID = &userID

	result, err := h.warehouseSvc.Transfer(r.Context(), orgID, input)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,result)
}

// Adjust handles POST /api/v1/organizations/{orgID}/warehouse/adjust.
func (h *WarehouseHandler) Adjust(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input warehouse.AdjustInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}
	input.ActorID = &userID

	result, err := h.warehouseSvc.Adjust(r.Context(), orgID, input)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,result)
}

// ListStock handles GET /api/v1/organizations/{orgID}/warehouse/{warehouseID}/stock.
func (h *WarehouseHandler) ListStock(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	warehouseID, err := parseUUID(r, "warehouseID")
	if err != nil {
		respondError(w, err)
		return
	}

	filter := warehouse.StockFilter{
		Search:   queryString(r, "search"),
		LowStock: queryBool(r, "low_stock"),
		Page:     parsePage(r),
	}

	result, err := h.warehouseSvc.ListStock(r.Context(), orgID, warehouseID, filter)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,result)
}

// GetStockLevel handles GET /api/v1/organizations/{orgID}/warehouse/{warehouseID}/stock/{productID}.
func (h *WarehouseHandler) GetStockLevel(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	warehouseID, err := parseUUID(r, "warehouseID")
	if err != nil {
		respondError(w, err)
		return
	}
	productID, err := parseUUID(r, "productID")
	if err != nil {
		respondError(w, err)
		return
	}

	sl, err := h.warehouseSvc.GetStockLevel(r.Context(), orgID, productID, warehouseID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,sl)
}

// GetLowStock handles GET /api/v1/organizations/{orgID}/warehouse/low-stock.
func (h *WarehouseHandler) GetLowStock(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	items, err := h.warehouseSvc.GetLowStock(r.Context(), orgID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,items)
}

// ListMovements handles GET /api/v1/organizations/{orgID}/warehouse/movements.
func (h *WarehouseHandler) ListMovements(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	filter := warehouse.MovementFilter{
		ProductID:   queryUUID(r, "product_id"),
		WarehouseID: queryUUID(r, "warehouse_id"),
		Type:        queryString(r, "type"),
		Page:        parsePage(r),
	}

	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		t, err := time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			t, err = time.Parse("2006-01-02", sinceStr)
		}
		if err != nil {
			respondError(w, errs.NewBadRequest("invalid since format"))
			return
		}
		filter.Since = &t
	}

	result, err := h.warehouseSvc.GetMovements(r.Context(), orgID, filter)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,result)
}
