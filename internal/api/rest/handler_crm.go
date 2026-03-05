package rest

import (
	"net/http"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/module/crm"
)

// CRMHandler handles CRM endpoints.
type CRMHandler struct {
	crmSvc *crm.Service
}

// NewCRMHandler creates a new CRMHandler.
func NewCRMHandler(crmSvc *crm.Service) *CRMHandler {
	return &CRMHandler{crmSvc: crmSvc}
}

// CreateCustomer handles POST /api/v1/organizations/{orgID}/crm/customers.
func (h *CRMHandler) CreateCustomer(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input crm.CreateInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}
	input.Kind = "customer"

	cp, err := h.crmSvc.Create(r.Context(), orgID, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, cp)
}

// ListCustomers handles GET /api/v1/organizations/{orgID}/crm/customers.
func (h *CRMHandler) ListCustomers(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	filter := crm.CounterpartyFilter{
		Search:   queryString(r, "search"),
		Tag:      queryString(r, "tag"),
		Category: queryString(r, "category"),
		Page:     parsePage(r),
	}

	resp, err := h.crmSvc.List(r.Context(), orgID, "customer", filter)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, resp)
}

// GetCustomer handles GET /api/v1/organizations/{orgID}/crm/customers/{id}.
func (h *CRMHandler) GetCustomer(w http.ResponseWriter, r *http.Request) {
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

	cp, err := h.crmSvc.Get(r.Context(), orgID, id)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, cp)
}

// UpdateCustomer handles PUT /api/v1/organizations/{orgID}/crm/customers/{id}.
func (h *CRMHandler) UpdateCustomer(w http.ResponseWriter, r *http.Request) {
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

	var input crm.UpdateInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	cp, err := h.crmSvc.Update(r.Context(), orgID, id, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, cp)
}

// UpdateCustomerTags handles POST /api/v1/organizations/{orgID}/crm/customers/{id}/tags.
func (h *CRMHandler) UpdateCustomerTags(w http.ResponseWriter, r *http.Request) {
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

	var input crm.TagsInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	cp, err := h.crmSvc.UpdateTags(r.Context(), orgID, id, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, cp)
}

// GetCustomerOrders handles GET /api/v1/organizations/{orgID}/crm/customers/{id}/orders.
func (h *CRMHandler) GetCustomerOrders(w http.ResponseWriter, r *http.Request) {
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

	page := parsePage(r)
	orders, total, err := h.crmSvc.GetOrders(r.Context(), orgID, id, page)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]any{
		"items":  orders,
		"total":  total,
		"limit":  page.Limit,
		"offset": page.Offset,
	})
}

// GetCustomerTransactions handles GET /api/v1/organizations/{orgID}/crm/customers/{id}/transactions.
func (h *CRMHandler) GetCustomerTransactions(w http.ResponseWriter, r *http.Request) {
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

	page := parsePage(r)
	txns, total, err := h.crmSvc.GetTransactions(r.Context(), orgID, id, page)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]any{
		"items":  txns,
		"total":  total,
		"limit":  page.Limit,
		"offset": page.Offset,
	})
}

// CreateSupplier handles POST /api/v1/organizations/{orgID}/crm/suppliers.
func (h *CRMHandler) CreateSupplier(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input crm.CreateInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}
	input.Kind = "supplier"

	cp, err := h.crmSvc.Create(r.Context(), orgID, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, cp)
}

// ListSuppliers handles GET /api/v1/organizations/{orgID}/crm/suppliers.
func (h *CRMHandler) ListSuppliers(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	filter := crm.CounterpartyFilter{
		Search:   queryString(r, "search"),
		Tag:      queryString(r, "tag"),
		Category: queryString(r, "category"),
		Page:     parsePage(r),
	}

	resp, err := h.crmSvc.List(r.Context(), orgID, "supplier", filter)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, resp)
}

// GetSupplier handles GET /api/v1/organizations/{orgID}/crm/suppliers/{id}.
func (h *CRMHandler) GetSupplier(w http.ResponseWriter, r *http.Request) {
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

	cp, err := h.crmSvc.Get(r.Context(), orgID, id)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, cp)
}

// GetSupplierDeliveries handles GET /api/v1/organizations/{orgID}/crm/suppliers/{id}/deliveries.
func (h *CRMHandler) GetSupplierDeliveries(w http.ResponseWriter, r *http.Request) {
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

	page := parsePage(r)
	deliveries, total, err := h.crmSvc.GetDeliveries(r.Context(), orgID, id, page)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]any{
		"items":  deliveries,
		"total":  total,
		"limit":  page.Limit,
		"offset": page.Offset,
	})
}
