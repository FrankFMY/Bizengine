package rest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// TrimStrings is middleware that recursively trims whitespace from all string
// values in JSON request bodies. Only applies to POST, PUT, and PATCH.
func TrimStrings(next http.Handler) http.Handler {
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
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		var parsed any
		if err := json.Unmarshal(body, &parsed); err != nil {
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		trimmed := trimValue(parsed)
		out, err := json.Marshal(trimmed)
		if err != nil {
			r.Body = io.NopCloser(bytes.NewReader(body))
			next.ServeHTTP(w, r)
			return
		}

		r.Body = io.NopCloser(bytes.NewReader(out))
		r.ContentLength = int64(len(out))
		next.ServeHTTP(w, r)
	})
}

func trimValue(v any) any {
	switch val := v.(type) {
	case string:
		return strings.TrimSpace(val)
	case map[string]any:
		for k, item := range val {
			val[k] = trimValue(item)
		}
		return val
	case []any:
		for i, item := range val {
			val[i] = trimValue(item)
		}
		return val
	default:
		return v
	}
}
