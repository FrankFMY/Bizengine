package rest

import (
	"io"
	"net/http"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/dataimport"
	"github.com/bizengine/engine/pkg/errs"
)

// ImportHandler handles data import endpoints.
type ImportHandler struct {
	importSvc *dataimport.Service
}

// NewImportHandler creates a new ImportHandler.
func NewImportHandler(importSvc *dataimport.Service) *ImportHandler {
	return &ImportHandler{importSvc: importSvc}
}

// ImportProducts handles POST /api/v1/organizations/{orgID}/import/products.
func (h *ImportHandler) ImportProducts(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	data, filename, err := h.readFile(r)
	if err != nil {
		respondError(w, err)
		return
	}

	result, err := h.importSvc.ImportProducts(r.Context(), orgID, data, filename, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, result)
}

// ImportCustomers handles POST /api/v1/organizations/{orgID}/import/customers.
func (h *ImportHandler) ImportCustomers(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	data, filename, err := h.readFile(r)
	if err != nil {
		respondError(w, err)
		return
	}

	result, err := h.importSvc.ImportCustomers(r.Context(), orgID, data, filename, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, result)
}

// ImportSuppliers handles POST /api/v1/organizations/{orgID}/import/suppliers.
func (h *ImportHandler) ImportSuppliers(w http.ResponseWriter, r *http.Request) {
	orgID, err := parseUUID(r, "orgID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	data, filename, err := h.readFile(r)
	if err != nil {
		respondError(w, err)
		return
	}

	result, err := h.importSvc.ImportSuppliers(r.Context(), orgID, data, filename, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, result)
}

func (h *ImportHandler) readFile(r *http.Request) ([]byte, string, error) {
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		return nil, "", errs.NewBadRequest("invalid multipart form: " + err.Error())
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		return nil, "", errs.NewBadRequest("file field is required")
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, "", errs.NewBadRequest("failed to read file")
	}

	return data, header.Filename, nil
}
