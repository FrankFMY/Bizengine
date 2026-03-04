package rest

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/module/catalog"
)

// CatalogHandler handles catalog endpoints.
type CatalogHandler struct {
	catalogSvc *catalog.Service
}

// NewCatalogHandler creates a new CatalogHandler.
func NewCatalogHandler(catalogSvc *catalog.Service) *CatalogHandler {
	return &CatalogHandler{catalogSvc: catalogSvc}
}

// CreateProduct handles POST /api/v1/workspaces/{wsID}/catalog/products.
func (h *CatalogHandler) CreateProduct(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input catalog.CreateProductInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	p, err := h.catalogSvc.CreateProduct(r.Context(), wsID, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w,p)
}

// ListProducts handles GET /api/v1/workspaces/{wsID}/catalog/products.
func (h *CatalogHandler) ListProducts(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	filter := catalog.ProductFilter{
		CategoryID: queryUUID(r, "category_id"),
		Search:     queryString(r, "search"),
		InStock:    queryBool(r, "in_stock"),
		Status:     queryString(r, "status"),
		Page:       parsePage(r),
	}

	result, err := h.catalogSvc.ListProducts(r.Context(), wsID, filter)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,result)
}

// GetProduct handles GET /api/v1/workspaces/{wsID}/catalog/products/{id}.
func (h *CatalogHandler) GetProduct(w http.ResponseWriter, r *http.Request) {
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

	p, err := h.catalogSvc.GetProduct(r.Context(), wsID, id)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,p)
}

// UpdateProduct handles PUT /api/v1/workspaces/{wsID}/catalog/products/{id}.
func (h *CatalogHandler) UpdateProduct(w http.ResponseWriter, r *http.Request) {
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

	var input catalog.UpdateProductInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	p, err := h.catalogSvc.UpdateProduct(r.Context(), wsID, id, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,p)
}

// ArchiveProduct handles POST /api/v1/workspaces/{wsID}/catalog/products/{id}/archive.
func (h *CatalogHandler) ArchiveProduct(w http.ResponseWriter, r *http.Request) {
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

	if err := h.catalogSvc.ArchiveProduct(r.Context(), wsID, id, &userID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, nil)
}

// CreateCategory handles POST /api/v1/workspaces/{wsID}/catalog/categories.
func (h *CatalogHandler) CreateCategory(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}
	userID, _ := auth.UserIDFromCtx(r.Context())

	var input catalog.CreateCategoryInput
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	c, err := h.catalogSvc.CreateCategory(r.Context(), wsID, input, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondCreated(w,c)
}

// ListCategories handles GET /api/v1/workspaces/{wsID}/catalog/categories.
func (h *CatalogHandler) ListCategories(w http.ResponseWriter, r *http.Request) {
	wsID, err := parseUUID(r, "wsID")
	if err != nil {
		respondError(w, err)
		return
	}

	parentID := queryUUID(r, "parent_id")

	categories, err := h.catalogSvc.ListCategories(r.Context(), wsID, parentID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,categories)
}

// UpdateCategory handles PUT /api/v1/workspaces/{wsID}/catalog/categories/{id}.
func (h *CatalogHandler) UpdateCategory(w http.ResponseWriter, r *http.Request) {
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
		Name     *string `json:"name,omitempty"`
		ParentID *string `json:"parent_id,omitempty"`
	}
	if err := decodeJSON(r, &input); err != nil {
		respondError(w, err)
		return
	}

	var parentID *uuid.UUID
	if input.ParentID != nil {
		pid, err := parseUUIDString(*input.ParentID)
		if err != nil {
			respondError(w, err)
			return
		}
		parentID = &pid
	}

	c, err := h.catalogSvc.UpdateCategory(r.Context(), wsID, id, input.Name, parentID, &userID)
	if err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK,c)
}

// DeleteCategory handles DELETE /api/v1/workspaces/{wsID}/catalog/categories/{id}.
func (h *CatalogHandler) DeleteCategory(w http.ResponseWriter, r *http.Request) {
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

	if err := h.catalogSvc.DeleteCategory(r.Context(), wsID, id, &userID); err != nil {
		respondError(w, err)
		return
	}

	respondOK(w, http.StatusOK, nil)
}
