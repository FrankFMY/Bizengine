package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/pkg/types"
)

const idempTTL = 5 * time.Second

// Idempotency middleware prevents duplicate mutations using Redis.
// It reads the "idemp" value from context (set by UnwrapRequest).
// Redis key: idemp:{organization_id}:{idemp_value}, TTL 5s.
func Idempotency(rdb *redis.Client) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			idemp := RequestIdemp(r.Context())
			if idemp == "" {
				next.ServeHTTP(w, r)
				return
			}

			orgID, _ := auth.OrganizationIDFromCtx(r.Context())
			key := fmt.Sprintf("idemp:%s:%s", orgID.String(), idemp)
			ctx := r.Context()

			// Try to set the key. NX = only if not exists.
			set, err := rdb.SetNX(ctx, key, "processing", idempTTL).Result()
			if err != nil {
				// Redis down — let request through rather than block
				next.ServeHTTP(w, r)
				return
			}

			if !set {
				// Key already exists — check if it's a cached response or still processing
				val, err := rdb.Get(ctx, key).Result()
				if err != nil || val == "processing" {
					// Still processing — 429 with deadline
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusTooManyRequests)
					json.NewEncoder(w).Encode(types.Response{
						OK: false,
						Error: &types.ResponseError{
							Code:    "RATE_LIMITED",
							Details: "duplicate request in progress",
						},
						Data: map[string]any{
							"deadline": time.Now().Add(idempTTL).Unix(),
						},
					})
					return
				}
				// Replay cached response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(val))
				return
			}

			// Capture the response
			capture := &responseCapture{
				ResponseWriter: w,
				body:           &bytes.Buffer{},
				status:         http.StatusOK,
			}
			next.ServeHTTP(capture, r)

			// Cache successful responses
			if capture.status >= 200 && capture.status < 300 {
				rdb.Set(ctx, key, capture.body.String(), idempTTL)
			} else {
				// On error, delete the key so client can retry
				rdb.Del(ctx, key)
			}
		})
	}
}

type responseCapture struct {
	http.ResponseWriter
	body   *bytes.Buffer
	status int
}

func (rc *responseCapture) WriteHeader(code int) {
	rc.status = code
	rc.ResponseWriter.WriteHeader(code)
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	rc.body.Write(b)
	return rc.ResponseWriter.Write(b)
}

// IdempotencyKey is a helper to generate idemp keys for testing.
func IdempotencyKey(orgID uuid.UUID, idemp string) string {
	return fmt.Sprintf("idemp:%s:%s", orgID.String(), idemp)
}
