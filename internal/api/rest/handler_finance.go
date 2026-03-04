package rest

import (
	"net/http"
	"time"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/module/finance"
	"github.com/bizengine/engine/pkg/errs"
)

// FinanceHandler handles finance endpoints.
type FinanceHandler struct {
	financeSvc *finance.Service
}

// NewFinanceHandler creates a new FinanceHandler.
func NewFinanceHandler(financeSvc *finance.Service) *FinanceHandler {
	return &FinanceHandler{financeSvc: financeSvc}
}

// ListAccounts handles GET /api/v1/workspaces/{wsID}/finance/accounts.
func (h *FinanceHandler) ListAccounts(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	accounts, err := h.financeSvc.ListAccounts(r.Context(), wsID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,accounts)
}

// CreateAccount handles POST /api/v1/workspaces/{wsID}/finance/accounts.
func (h *FinanceHandler) CreateAccount(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	var input finance.CreateAccountInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	acct, err := h.financeSvc.CreateAccount(r.Context(), wsID, input)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w,acct)
}

// GetAccountBalance handles GET /api/v1/workspaces/{wsID}/finance/accounts/{id}/balance.
func (h *FinanceHandler) GetAccountBalance(w http.ResponseWriter, r *http.Request) {
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

	from := time.Now().AddDate(-1, 0, 0)
	to := time.Now()

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t
		}
	}

	bal, err := h.financeSvc.GetAccountBalance(r.Context(), wsID, id, from, to)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,bal)
}

// CreateTransaction handles POST /api/v1/workspaces/{wsID}/finance/transactions.
func (h *FinanceHandler) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input finance.CreateTransactionInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	txn, err := h.financeSvc.CreateTransaction(r.Context(), wsID, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w,txn)
}

// GetTransaction handles GET /api/v1/workspaces/{wsID}/finance/transactions/{id}.
func (h *FinanceHandler) GetTransaction(w http.ResponseWriter, r *http.Request) {
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

	txn, err := h.financeSvc.GetTransaction(r.Context(), wsID, id)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,txn)
}

// ListTransactions handles GET /api/v1/workspaces/{wsID}/finance/transactions.
func (h *FinanceHandler) ListTransactions(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	filter := finance.TransactionFilter{
		AccountID: queryUUID(r, "account_id"),
		Page:      parsePage(r),
	}

	if fromStr := r.URL.Query().Get("from"); fromStr != "" {
		if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			filter.From = &t
		}
	}
	if toStr := r.URL.Query().Get("to"); toStr != "" {
		if t, err := time.Parse("2006-01-02", toStr); err == nil {
			filter.To = &t
		}
	}

	txns, total, err := h.financeSvc.ListTransactions(r.Context(), wsID, filter)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,map[string]any{
		"items":  txns,
		"total":  total,
		"limit":  filter.Page.Limit,
		"offset": filter.Page.Offset,
	})
}

// PostTransaction handles POST /api/v1/workspaces/{wsID}/finance/transactions/{id}/post.
func (h *FinanceHandler) PostTransaction(w http.ResponseWriter, r *http.Request) {
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

	if err := h.financeSvc.PostTransaction(r.Context(), wsID, id, &userID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, nil)
}

// GetTrialBalance handles GET /api/v1/workspaces/{wsID}/finance/reports/trial-balance.
func (h *FinanceHandler) GetTrialBalance(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	date := time.Now()
	if dateStr := r.URL.Query().Get("date"); dateStr != "" {
		if t, err := time.Parse("2006-01-02", dateStr); err == nil {
			date = t
		} else {
			respondError(w, errs.NewBadRequest("invalid date format"))
			return
		}
	}

	rows, err := h.financeSvc.GetTrialBalance(r.Context(), wsID, date)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,rows)
}

// CreateInvoice handles POST /api/v1/workspaces/{wsID}/finance/invoices.
func (h *FinanceHandler) CreateInvoice(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	var input finance.CreateInvoiceInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	inv, err := h.financeSvc.CreateInvoice(r.Context(), wsID, input)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w,inv)
}

// ListInvoices handles GET /api/v1/workspaces/{wsID}/finance/invoices.
func (h *FinanceHandler) ListInvoices(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	filter := finance.InvoiceFilter{
		Type:   queryString(r, "type"),
		Status: queryString(r, "status"),
		Page:   parsePage(r),
	}

	invoices, total, err := h.financeSvc.ListInvoices(r.Context(), wsID, filter)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,map[string]any{
		"items":  invoices,
		"total":  total,
		"limit":  filter.Page.Limit,
		"offset": filter.Page.Offset,
	})
}

// MarkInvoicePaid handles POST /api/v1/workspaces/{wsID}/finance/invoices/{id}/pay.
func (h *FinanceHandler) MarkInvoicePaid(w http.ResponseWriter, r *http.Request) {
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

	if err := h.financeSvc.MarkInvoicePaid(r.Context(), wsID, id); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, nil)
}
