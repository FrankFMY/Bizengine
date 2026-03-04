// Package rest provides HTTP handlers for the REST API.
package rest

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes an error response based on typed errors.
func writeError(w http.ResponseWriter, err error) {
	status := errs.HTTPStatus(err)
	writeJSON(w, status, types.ErrorResponse{
		Code:    string(errs.GetCode(err)),
		Message: errs.GetMessage(err),
	})
}

// parseUUID extracts and parses a UUID from URL params.
func parseUUID(r *http.Request, param string) (uuid.UUID, error) {
	s := chi.URLParam(r, param)
	if s == "" {
		return uuid.Nil, errs.NewBadRequest(param + " is required")
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, errs.NewBadRequest("invalid " + param)
	}
	return id, nil
}

// parsePage extracts pagination parameters from query string.
func parsePage(r *http.Request) types.PageRequest {
	p := types.DefaultPageRequest()
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.Limit = n
		}
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			p.Offset = n
		}
	}
	if v := r.URL.Query().Get("sort"); v != "" {
		p.Sort = v
	}
	if v := r.URL.Query().Get("order"); v != "" {
		p.Order = v
	}
	p.Normalize()
	return p
}

// decodeJSON decodes the request body into v.
func decodeJSON(r *http.Request, v any) error {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return errs.NewBadRequest("invalid request body: " + err.Error())
	}
	return nil
}

// queryString returns a query parameter as *string (nil if absent).
func queryString(r *http.Request, key string) *string {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	return &v
}

// queryUUID returns a query parameter as *uuid.UUID (nil if absent).
func queryUUID(r *http.Request, key string) *uuid.UUID {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	id, err := uuid.Parse(v)
	if err != nil {
		return nil
	}
	return &id
}

// queryBool returns a query parameter as *bool (nil if absent).
func queryBool(r *http.Request, key string) *bool {
	v := r.URL.Query().Get(key)
	if v == "" {
		return nil
	}
	b := v == "true" || v == "1"
	return &b
}

// parseUUIDString parses a UUID from a raw string.
func parseUUIDString(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return uuid.Nil, errs.NewBadRequest("invalid uuid: " + s)
	}
	return id, nil
}
