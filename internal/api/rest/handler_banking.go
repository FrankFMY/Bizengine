package rest

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/bizengine/engine/internal/core/auth"
	adapter "github.com/bizengine/engine/internal/integration/banking"
	"github.com/bizengine/engine/internal/module/banking"
	"github.com/bizengine/engine/pkg/errs"
)

// BankingHandler handles banking endpoints.
type BankingHandler struct {
	svc     *banking.Service
	authSvc *auth.Service
}

// NewBankingHandler creates a new BankingHandler.
func NewBankingHandler(svc *banking.Service, authSvc *auth.Service) *BankingHandler {
	return &BankingHandler{svc: svc, authSvc: authSvc}
}

// InitiatePayment handles POST /organizations/{orgID}/payments/initiate.
func (h *BankingHandler) InitiatePayment(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	var input banking.InitiatePaymentInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	result, err := h.svc.InitiatePayment(r.Context(), orgID, input)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, result)
}

// GetPaymentStatus handles GET /organizations/{orgID}/payments/{paymentID}/status.
func (h *BankingHandler) GetPaymentStatus(w http.ResponseWriter, r *http.Request) {
	paymentID := chi.URLParam(r, "paymentID")
	if paymentID == "" {
		respondError(w, errs.NewBadRequest("paymentID is required"))
		return
	}

	status, err := h.svc.GetPaymentStatus(r.Context(), paymentID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, status)
}

// PaymentCallback handles POST /webhooks/bank/payment-callback.
func (h *BankingHandler) PaymentCallback(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	var callback banking.PaymentCallback
	if err := decodeJSON(r, &callback); err != nil {
		respondError(w, err)
		return
	}

	if err := h.svc.HandlePaymentCallback(r.Context(), orgID, callback); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "processed"})
}

// AutoReconcile handles POST /organizations/{orgID}/finance/bank/auto-reconcile.
func (h *BankingHandler) AutoReconcile(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	result, err := h.svc.AutoReconcile(r.Context(), orgID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, result)
}

// GetReconciliation handles GET /organizations/{orgID}/finance/bank/reconciliations/{id}.
func (h *BankingHandler) GetReconciliation(w http.ResponseWriter, r *http.Request) {
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

	result, err := h.svc.GetReconciliation(r.Context(), orgID, id)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, result)
}

// ListReconciliations handles GET /organizations/{orgID}/finance/bank/reconciliations.
func (h *BankingHandler) ListReconciliations(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	limit := 50
	offset := 0
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			offset = n
		}
	}

	recs, total, err := h.svc.ListReconciliations(r.Context(), orgID, limit, offset)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]any{
		"items":  recs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// PayrollViaBankPay handles POST /organizations/{orgID}/hr/payroll/{year}/{month}/pay.
func (h *BankingHandler) PayrollViaBankPay(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	yearStr := chi.URLParam(r, "year")
	monthStr := chi.URLParam(r, "month")
	year, err := strconv.Atoi(yearStr)
	if err != nil {
		respondError(w, errs.NewBadRequest("invalid year"))
		return
	}
	month, err := strconv.Atoi(monthStr)
	if err != nil || month < 1 || month > 12 {
		respondError(w, errs.NewBadRequest("invalid month"))
		return
	}

	var input struct {
		Employees []adapter.PayrollEmployee `json:"employees"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	result, err := h.svc.PayViaBank(r.Context(), orgID, adapter.PayrollBatch{
		OrganizationID: orgID,
		Employees:      input.Employees,
		Month:          month,
		Year:           year,
	})
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, result)
}

// BankAuthLogin handles GET /auth/bank/{bankCode}/login.
func (h *BankingHandler) BankAuthLogin(w http.ResponseWriter, r *http.Request) {
	redirectURL := r.URL.Query().Get("redirect_uri")
	if redirectURL == "" {
		respondError(w, errs.NewBadRequest("redirect_uri is required"))
		return
	}

	authURL, err := h.svc.GetAuthURL(r.Context(), redirectURL)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"auth_url": authURL})
}

// BankAuthCallback handles GET /auth/bank/{bankCode}/callback.
func (h *BankingHandler) BankAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	if code == "" {
		respondError(w, errs.NewBadRequest("code is required"))
		return
	}

	bankUser, err := h.svc.ExchangeCode(r.Context(), code)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, bankUser)
}

// GetWhiteLabel handles GET /organizations/{orgID}/white-label.
func (h *BankingHandler) GetWhiteLabel(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	wl, err := h.svc.GetWhiteLabel(r.Context(), orgID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, wl)
}
