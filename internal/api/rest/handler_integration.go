package rest

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/bizengine/engine/internal/integration/bank"
	"github.com/bizengine/engine/internal/integration/chestnyznak"
	"github.com/bizengine/engine/internal/integration/edo"
	"github.com/bizengine/engine/internal/integration/fns"
	"github.com/bizengine/engine/pkg/errs"
)

type IntegrationHandler struct {
	fiscal  fns.FiscalService
	edo     edo.EDOService
	marking chestnyznak.MarkingService
	banking bank.BankService
}

func NewIntegrationHandler(
	fiscal fns.FiscalService,
	edoSvc edo.EDOService,
	marking chestnyznak.MarkingService,
	banking bank.BankService,
) *IntegrationHandler {
	return &IntegrationHandler{
		fiscal:  fiscal,
		edo:     edoSvc,
		marking: marking,
		banking: banking,
	}
}

// --- Fiscal (ATOL / FZ-54) ---

// SendReceipt handles POST /integrations/fiscal/receipt.
func (h *IntegrationHandler) SendReceipt(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	var receipt fns.Receipt
	if err := decodeJSON(r, &receipt); err != nil {
		respondError(w, err)
		return
	}

	result, err := h.fiscal.SendReceipt(r.Context(), orgID, receipt)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, result)
}

// GetReceiptStatus handles GET /integrations/fiscal/receipt/{receiptID}.
func (h *IntegrationHandler) GetReceiptStatus(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	receiptID := chi.URLParam(r, "receiptID")
	if receiptID == "" {
		respondError(w, errs.NewBadRequest("receiptID is required"))
		return
	}

	status, err := h.fiscal.GetReceiptStatus(r.Context(), orgID, receiptID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, status)
}

// --- EDO (Diadoc) ---

// SendEDODocument handles POST /integrations/edo/documents.
func (h *IntegrationHandler) SendEDODocument(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	var doc edo.EDODocument
	if err := decodeJSON(r, &doc); err != nil {
		respondError(w, err)
		return
	}

	result, err := h.edo.SendDocument(r.Context(), orgID, doc)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w, result)
}

// GetIncomingEDO handles GET /integrations/edo/documents/incoming.
func (h *IntegrationHandler) GetIncomingEDO(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	since := time.Now().AddDate(0, 0, -30)
	if v := r.URL.Query().Get("since"); v != "" {
		if t, err := time.Parse("2006-01-02", v); err == nil {
			since = t
		}
	}

	docs, err := h.edo.GetIncomingDocuments(r.Context(), orgID, since)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, docs)
}

// AcceptEDODocument handles POST /integrations/edo/documents/{docID}/accept.
func (h *IntegrationHandler) AcceptEDODocument(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	docID := chi.URLParam(r, "docID")

	if err := h.edo.AcceptDocument(r.Context(), orgID, docID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// RejectEDODocument handles POST /integrations/edo/documents/{docID}/reject.
func (h *IntegrationHandler) RejectEDODocument(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	docID := chi.URLParam(r, "docID")

	var input struct {
		Reason string `json:"reason"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	if err := h.edo.RejectDocument(r.Context(), orgID, docID, input.Reason); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "rejected"})
}

// --- Chestny Znak (product marking) ---

// VerifyMarking handles POST /integrations/marking/verify.
func (h *IntegrationHandler) VerifyMarking(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	info, err := h.marking.VerifyCode(r.Context(), input.Code)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, info)
}

// RegisterMarkingReceipt handles POST /integrations/marking/receipt.
func (h *IntegrationHandler) RegisterMarkingReceipt(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	var input struct {
		Codes      []string `json:"codes"`
		DocumentID string   `json:"document_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	if err := h.marking.RegisterReceipt(r.Context(), orgID, input.Codes, input.DocumentID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "registered"})
}

// RegisterMarkingShipment handles POST /integrations/marking/shipment.
func (h *IntegrationHandler) RegisterMarkingShipment(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	var input struct {
		Codes           []string `json:"codes"`
		CounterpartyINN string   `json:"counterparty_inn"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	if err := h.marking.RegisterShipment(r.Context(), orgID, input.Codes, input.CounterpartyINN); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, map[string]string{"status": "registered"})
}

// --- Bank (1C format) ---

// ImportBankStatement handles POST /integrations/bank/import.
func (h *IntegrationHandler) ImportBankStatement(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	var input struct {
		Data []byte `json:"data"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	result, err := h.banking.ImportStatement(r.Context(), orgID, input.Data)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, result)
}

// ExportPaymentOrders handles POST /integrations/bank/export.
func (h *IntegrationHandler) ExportPaymentOrders(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	var input struct {
		Orders []bank.PaymentOrder `json:"orders"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	data, err := h.banking.ExportPaymentOrders(r.Context(), orgID, input.Orders)
	if err != nil {
		respondError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=windows-1251")
	w.Header().Set("Content-Disposition", "attachment; filename=\"1c_exchange.txt\"")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
