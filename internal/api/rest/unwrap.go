package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

type ctxKeyParams struct{}
type ctxKeyIdemp struct{}

// RequestParams returns the "params" object from the request envelope, if any.
func RequestParams(ctx context.Context) map[string]any {
	v, _ := ctx.Value(ctxKeyParams{}).(map[string]any)
	return v
}

// RequestIdemp returns the "idemp" string from the request envelope, if any.
func RequestIdemp(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyIdemp{}).(string)
	return v
}

// UnwrapRequest is middleware that unwraps {data, params, idemp} request envelopes.
// For POST/PUT/PATCH: parses the envelope, rewrites r.Body with the "data" payload.
// For GET/DELETE: passes through unchanged.
// If the body has no "data" key, it passes through unchanged (backward compat).
func UnwrapRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
			next.ServeHTTP(w, r)
			return
		}

		if r.Body == nil || r.ContentLength == 0 {
			next.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnprocessableEntity)
			json.NewEncoder(w).Encode(types.Response{
				OK:    false,
				Error: &types.ResponseError{Code: string(errs.CodeUnprocessable)},
			})
			return
		}

		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(body, &envelope); err != nil {
			// Not valid JSON — pass through as-is for decodeJSON to handle
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		dataRaw, hasData := envelope["data"]
		if !hasData {
			// No "data" key — backward compat, pass original body through
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		ctx := r.Context()

		if paramsRaw, ok := envelope["params"]; ok {
			var params map[string]any
			if json.Unmarshal(paramsRaw, &params) == nil {
				ctx = context.WithValue(ctx, ctxKeyParams{}, params)
			}
		}

		if idempRaw, ok := envelope["idemp"]; ok {
			var idemp string
			if json.Unmarshal(idempRaw, &idemp) == nil && idemp != "" {
				ctx = context.WithValue(ctx, ctxKeyIdemp{}, idemp)
			}
		}

		r.Body = io.NopCloser(bytes.NewReader(dataRaw))
		r.ContentLength = int64(len(dataRaw))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
