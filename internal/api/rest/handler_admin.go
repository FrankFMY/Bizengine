package rest

import (
	"net/http"
	"time"

	"github.com/FrankFMY/arcana"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/pkg/errs"
)

// AdminHandler provides debug and monitoring endpoints.
type AdminHandler struct {
	arcanaEngine *arcana.Engine
	startedAt    time.Time
	version      string
}

// NewAdminHandler creates a new AdminHandler.
func NewAdminHandler(arcanaEngine *arcana.Engine, version string) *AdminHandler {
	return &AdminHandler{
		arcanaEngine: arcanaEngine,
		startedAt:    time.Now(),
		version:      version,
	}
}

// State returns current system state for debugging.
// GET /admin/state
func (h *AdminHandler) State(w http.ResponseWriter, r *http.Request) {
	role := auth.RoleFromCtx(r.Context())
	if role != "owner" && role != "admin" {
		respondError(w, errs.NewForbidden("admin only"))
		return
	}

	arcanaStats := h.arcanaEngine.Stats()

	respondOK(w, http.StatusOK, map[string]any{
		"arcana": arcanaStats,
		"uptime": time.Since(h.startedAt).String(),
	})
}

// Invalidate forces an Arcana invalidation for debugging.
// POST /admin/invalidate
func (h *AdminHandler) Invalidate(w http.ResponseWriter, r *http.Request) {
	role := auth.RoleFromCtx(r.Context())
	if role != "owner" && role != "admin" {
		respondError(w, errs.NewForbidden("admin only"))
		return
	}

	var input struct {
		Table   string   `json:"table"`
		RowID   string   `json:"row_id"`
		Columns []string `json:"columns"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	if input.Table == "" {
		respondError(w, errs.NewBadRequest("table is required"))
		return
	}

	h.arcanaEngine.Notify(r.Context(), arcana.Change{
		Table:   input.Table,
		RowID:   input.RowID,
		Columns: input.Columns,
	})

	respondOK(w, http.StatusOK, map[string]bool{"invalidated": true})
}

// Graphs lists registered graph definitions with their dependencies.
// GET /admin/graphs
func (h *AdminHandler) Graphs(w http.ResponseWriter, r *http.Request) {
	role := auth.RoleFromCtx(r.Context())
	if role != "owner" && role != "admin" {
		respondError(w, errs.NewForbidden("admin only"))
		return
	}

	registry := h.arcanaEngine.Registry()
	keys := registry.Keys()

	type graphInfo struct {
		Key  string            `json:"key"`
		Deps []arcana.TableDep `json:"deps"`
	}

	graphs := make([]graphInfo, 0, len(keys))
	for _, key := range keys {
		def, ok := registry.Get(key)
		if !ok {
			continue
		}
		graphs = append(graphs, graphInfo{Key: def.Key, Deps: def.Deps})
	}

	respondOK(w, http.StatusOK, map[string]any{"graphs": graphs, "count": len(graphs)})
}
