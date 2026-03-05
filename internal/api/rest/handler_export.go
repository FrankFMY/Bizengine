package rest

import (
	"net/http"

	"github.com/bizengine/engine/internal/export"
)

type ExportHandler struct {
	exportSvc *export.Service
}

func NewExportHandler(exportSvc *export.Service) *ExportHandler {
	return &ExportHandler{exportSvc: exportSvc}
}

// Export handles POST /export — generates CSV or PDF/text report.
func (h *ExportHandler) Export(w http.ResponseWriter, r *http.Request) {
	var req export.ExportRequest
	if err := decodeJSON(r, &req); err != nil {
		respondError(w, err)
		return
	}

	result, err := h.exportSvc.Export(r.Context(), req)
	if err != nil {
		respondError(w, err)
		return
	}

	w.Header().Set("Content-Type", result.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename=\""+result.Filename+"\"")
	w.WriteHeader(http.StatusOK)
	w.Write(result.Data)
}
