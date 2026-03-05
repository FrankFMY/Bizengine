package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/auth"
)

func setupIdempTest(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return rdb, mr
}

func idempCtx(orgID uuid.UUID, idemp string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, auth.ExportedCtxKeyOrganizationID, orgID)
	ctx = context.WithValue(ctx, ctxKeyIdemp{}, idemp)
	return ctx
}

func TestIdempotency_NoIdemp_Passthrough(t *testing.T) {
	rdb, _ := setupIdempTest(t)
	called := false

	handler := Idempotency(rdb)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		respondOK(w, http.StatusOK, map[string]string{"status": "done"})
	}))

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestIdempotency_FirstRequest_CachesResponse(t *testing.T) {
	rdb, mr := setupIdempTest(t)
	orgID := uuid.New()
	callCount := 0

	handler := Idempotency(rdb)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		respondOK(w, http.StatusOK, map[string]string{"result": "ok"})
	}))

	ctx := idempCtx(orgID, "test-key-1")
	req := httptest.NewRequest("POST", "/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, 1, callCount)
	assert.Equal(t, http.StatusOK, w.Code)

	key := IdempotencyKey(orgID, "test-key-1")
	val, err := mr.Get(key)
	require.NoError(t, err)
	assert.Contains(t, val, `"ok":true`)
}

func TestIdempotency_DuplicateRequest_ReplaysCache(t *testing.T) {
	rdb, _ := setupIdempTest(t)
	orgID := uuid.New()
	callCount := 0

	handler := Idempotency(rdb)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		respondOK(w, http.StatusOK, map[string]string{"result": "ok"})
	}))

	// First request
	ctx := idempCtx(orgID, "test-key-2")
	req := httptest.NewRequest("POST", "/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, 1, callCount)

	// Second request with same idemp key
	req2 := httptest.NewRequest("POST", "/test", nil).WithContext(ctx)
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	assert.Equal(t, 1, callCount) // handler NOT called again
	assert.Equal(t, http.StatusOK, w2.Code)
	assert.Contains(t, w2.Body.String(), `"ok":true`)
}

func TestIdempotency_ErrorResponse_DeletesKey(t *testing.T) {
	rdb, mr := setupIdempTest(t)
	orgID := uuid.New()

	handler := Idempotency(rdb)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"ok":false}`))
	}))

	ctx := idempCtx(orgID, "test-key-err")
	req := httptest.NewRequest("POST", "/test", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	key := IdempotencyKey(orgID, "test-key-err")
	assert.False(t, mr.Exists(key))
}
