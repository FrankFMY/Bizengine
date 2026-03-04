package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/internal/views"
)

// --- mock view manager ---

type mockViewManager struct {
	subscribeResult *views.SubscribeResult
	subscribeErr    error
	activeSubs      []views.ActiveSub
	syncErr         error

	subscribedView   string
	subscribedParams map[string]any
	unsubscribedHash string
	syncReq          *views.SyncRequest
}

func (m *mockViewManager) Subscribe(_ context.Context, _ string, _ uuid.UUID, _ uuid.UUID, viewKey string, params map[string]any) (*views.SubscribeResult, error) {
	m.subscribedView = viewKey
	m.subscribedParams = params
	return m.subscribeResult, m.subscribeErr
}

func (m *mockViewManager) Unsubscribe(_ string, paramsHash string) {
	m.unsubscribedHash = paramsHash
}

func (m *mockViewManager) ActiveSubscriptions(_ string) []views.ActiveSub {
	return m.activeSubs
}

func (m *mockViewManager) Sync(_ context.Context, _ string, _ uuid.UUID, req views.SyncRequest) error {
	m.syncReq = &req
	return m.syncErr
}

func viewsRouter(h *ViewsHandler) http.Handler {
	r := chi.NewRouter()
	r.Route("/api/v1/workspaces/{wsID}/views", func(r chi.Router) {
		r.Post("/subscribe", h.Subscribe)
		r.Post("/unsubscribe", h.Unsubscribe)
		r.Get("/active", h.Active)
		r.Post("/sync", h.Sync)
	})
	return r
}

func withViewsAuth(r *http.Request) *http.Request {
	ctx := context.WithValue(r.Context(), auth.ExportedCtxKeyUserID, uuid.New())
	ctx = context.WithValue(ctx, auth.ExportedCtxKeyWorkspaceID, uuid.New())
	ctx = context.WithValue(ctx, auth.ExportedCtxKeyRole, "admin")
	seanceKey := auth.ExportedCtxKeySeanceID()
	ctx = context.WithValue(ctx, seanceKey, "test-seance-id")
	return r.WithContext(ctx)
}

func TestViewsHandler_Subscribe(t *testing.T) {
	mgr := &mockViewManager{
		subscribeResult: &views.SubscribeResult{ParamsHash: "abc123", Version: 1},
	}
	h := NewViewsHandler(mgr)
	router := viewsRouter(h)

	wsID := uuid.New()
	body, _ := json.Marshal(map[string]any{"view": "orders_list", "params": map[string]any{"status": "active"}})
	req := httptest.NewRequest("POST", "/api/v1/workspaces/"+wsID.String()+"/views/subscribe", bytes.NewReader(body))
	req = withViewsAuth(req)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "orders_list", mgr.subscribedView)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	assert.True(t, resp["ok"].(bool))
	data := resp["data"].(map[string]any)
	assert.Equal(t, "abc123", data["params_hash"])
}

func TestViewsHandler_Subscribe_MissingView(t *testing.T) {
	h := NewViewsHandler(&mockViewManager{})
	router := viewsRouter(h)

	wsID := uuid.New()
	body, _ := json.Marshal(map[string]any{})
	req := httptest.NewRequest("POST", "/api/v1/workspaces/"+wsID.String()+"/views/subscribe", bytes.NewReader(body))
	req = withViewsAuth(req)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestViewsHandler_Unsubscribe(t *testing.T) {
	mgr := &mockViewManager{}
	h := NewViewsHandler(mgr)
	router := viewsRouter(h)

	wsID := uuid.New()
	body, _ := json.Marshal(map[string]any{"view": "orders_list", "params_hash": "abc123"})
	req := httptest.NewRequest("POST", "/api/v1/workspaces/"+wsID.String()+"/views/unsubscribe", bytes.NewReader(body))
	req = withViewsAuth(req)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "abc123", mgr.unsubscribedHash)
}

func TestViewsHandler_Active(t *testing.T) {
	mgr := &mockViewManager{
		activeSubs: []views.ActiveSub{
			{View: "orders_list", ParamsHash: "abc123", Version: 5},
		},
	}
	h := NewViewsHandler(mgr)
	router := viewsRouter(h)

	wsID := uuid.New()
	req := httptest.NewRequest("GET", "/api/v1/workspaces/"+wsID.String()+"/views/active", nil)
	req = withViewsAuth(req)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	data := resp["data"].(map[string]any)
	viewsList := data["views"].([]any)
	require.Len(t, viewsList, 1)
}

func TestViewsHandler_Sync(t *testing.T) {
	mgr := &mockViewManager{}
	h := NewViewsHandler(mgr)
	router := viewsRouter(h)

	wsID := uuid.New()
	syncReq := views.SyncRequest{
		Views: []views.SyncView{{View: "orders_list", ParamsHash: "abc", Version: 40}},
		Tables: map[string]map[string]int64{
			"orders": {"uuid-1": 41},
		},
	}
	body, _ := json.Marshal(syncReq)
	req := httptest.NewRequest("POST", "/api/v1/workspaces/"+wsID.String()+"/views/sync", bytes.NewReader(body))
	req = withViewsAuth(req)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	require.NotNil(t, mgr.syncReq)
	assert.Len(t, mgr.syncReq.Views, 1)
}
