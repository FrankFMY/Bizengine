package rest

import (
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/documents"
	"github.com/bizengine/engine/pkg/errs"
)

// DocumentsHandler handles document generation endpoints.
type DocumentsHandler struct {
	docSvc *documents.Service
}

// NewDocumentsHandler creates a new DocumentsHandler.
func NewDocumentsHandler(docSvc *documents.Service) *DocumentsHandler {
	return &DocumentsHandler{docSvc: docSvc}
}

// Invoice handles GET /api/v1/organizations/{orgID}/documents/invoice/{orderID}.
func (h *DocumentsHandler) Invoice(w http.ResponseWriter, r *http.Request) {
	orgID, orderID, err := h.parseOrgAndOrder(r)
	if err != nil {
		respondError(w, err)
		return
	}

	html, err := h.docSvc.RenderInvoice(r.Context(), orgID, orderID)
	if err != nil {
		respondError(w, err)
		return
	}

	h.writeHTML(w, html)
}

// TORG12 handles GET /api/v1/organizations/{orgID}/documents/torg12/{orderID}.
func (h *DocumentsHandler) TORG12(w http.ResponseWriter, r *http.Request) {
	orgID, orderID, err := h.parseOrgAndOrder(r)
	if err != nil {
		respondError(w, err)
		return
	}

	html, err := h.docSvc.RenderTORG12(r.Context(), orgID, orderID)
	if err != nil {
		respondError(w, err)
		return
	}

	h.writeHTML(w, html)
}

// Act handles GET /api/v1/organizations/{orgID}/documents/act/{orderID}.
func (h *DocumentsHandler) Act(w http.ResponseWriter, r *http.Request) {
	orgID, orderID, err := h.parseOrgAndOrder(r)
	if err != nil {
		respondError(w, err)
		return
	}

	html, err := h.docSvc.RenderAct(r.Context(), orgID, orderID)
	if err != nil {
		respondError(w, err)
		return
	}

	h.writeHTML(w, html)
}

// Receipt handles GET /api/v1/organizations/{orgID}/documents/receipt/{orderID}.
func (h *DocumentsHandler) Receipt(w http.ResponseWriter, r *http.Request) {
	orgID, orderID, err := h.parseOrgAndOrder(r)
	if err != nil {
		respondError(w, err)
		return
	}

	html, err := h.docSvc.RenderReceipt(r.Context(), orgID, orderID)
	if err != nil {
		respondError(w, err)
		return
	}

	h.writeHTML(w, html)
}

// PriceTags handles GET /api/v1/organizations/{orgID}/documents/price-tags?product_ids=...
func (h *DocumentsHandler) PriceTags(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}

	idsStr := r.URL.Query().Get("product_ids")
	if idsStr == "" {
		respondError(w, errs.NewBadRequest("product_ids is required"))
		return
	}

	var productIDs []uuid.UUID
	for _, s := range strings.Split(idsStr, ",") {
		id, err := uuid.Parse(strings.TrimSpace(s))
		if err != nil {
			respondError(w, errs.NewBadRequest("invalid product_id: "+s))
			return
		}
		productIDs = append(productIDs, id)
	}

	html, err := h.docSvc.RenderPriceTags(r.Context(), orgID, productIDs)
	if err != nil {
		respondError(w, err)
		return
	}

	h.writeHTML(w, html)
}

func (h *DocumentsHandler) parseOrgAndOrder(r *http.Request) (uuid.UUID, uuid.UUID, error) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	orderID, err := parseUUID(r, "orderID")
	if err != nil {
		return uuid.Nil, uuid.Nil, err
	}
	return orgID, orderID, nil
}

func (h *DocumentsHandler) writeHTML(w http.ResponseWriter, data []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
