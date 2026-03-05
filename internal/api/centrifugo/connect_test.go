package centrifugo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/auth"
	"github.com/bizengine/engine/pkg/errs"
)

// --- mock session store ---

type mockSessionStore struct {
	sessions map[string]*auth.Session
	seances  map[string]*auth.Seance
}

func newMockStore() *mockSessionStore {
	return &mockSessionStore{
		sessions: make(map[string]*auth.Session),
		seances:  make(map[string]*auth.Seance),
	}
}

func (m *mockSessionStore) CreateSession(_ context.Context, s *auth.Session, _ time.Duration) error {
	m.sessions[s.ID] = s
	return nil
}

func (m *mockSessionStore) GetSession(_ context.Context, id string) (*auth.Session, error) {
	s, ok := m.sessions[id]
	if !ok {
		return nil, errs.NewUnauthorized("session expired")
	}
	return s, nil
}

func (m *mockSessionStore) UpdateSession(_ context.Context, s *auth.Session, _ time.Duration) error {
	m.sessions[s.ID] = s
	return nil
}

func (m *mockSessionStore) DeleteSession(_ context.Context, id string) error {
	delete(m.sessions, id)
	return nil
}

func (m *mockSessionStore) CreateSeance(_ context.Context, s *auth.Seance, _ time.Duration) error {
	m.seances[s.ID] = s
	return nil
}

func (m *mockSessionStore) GetSeance(_ context.Context, id string) (*auth.Seance, error) {
	s, ok := m.seances[id]
	if !ok {
		return nil, errs.NewUnauthorized("seance expired")
	}
	return s, nil
}

func (m *mockSessionStore) SlideSeance(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (m *mockSessionStore) DeleteSessionSeances(_ context.Context, _ string) error {
	return nil
}

// --- tests ---

func TestConnectHandler_Success(t *testing.T) {
	store := newMockStore()
	userID := uuid.New()
	orgID := uuid.New()
	sessID := uuid.New().String()
	seanceID := uuid.New().String()

	store.sessions[sessID] = &auth.Session{
		ID:          sessID,
		UserID:      userID,
		OrganizationID: orgID,
		Role:        "owner",
		Email:       "test@example.com",
		FullName:    "Test User",
	}
	store.seances[seanceID] = &auth.Seance{
		ID:        seanceID,
		SessionID: sessID,
	}

	handler := NewConnectHandler(store, 10*time.Minute)

	req := httptest.NewRequest("POST", "/api/internal/centrifugo/connect", nil)
	req.AddCookie(&http.Cookie{Name: "teco_session", Value: sessID})
	req.AddCookie(&http.Cookie{Name: "teco_seance", Value: seanceID})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp connectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Nil(t, resp.Error)
	require.NotNil(t, resp.Result)
	assert.Equal(t, userID.String(), resp.Result.User)
	assert.Contains(t, resp.Result.Channels, "org:"+orgID.String())
}

func TestConnectHandler_MissingCookies(t *testing.T) {
	store := newMockStore()
	handler := NewConnectHandler(store, 10*time.Minute)

	req := httptest.NewRequest("POST", "/api/internal/centrifugo/connect", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var resp connectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Nil(t, resp.Result)
	require.NotNil(t, resp.Error)
	assert.Equal(t, 403, resp.Error.Code)
}

func TestConnectHandler_ExpiredSeance(t *testing.T) {
	store := newMockStore()
	sessID := uuid.New().String()
	store.sessions[sessID] = &auth.Session{ID: sessID, UserID: uuid.New()}

	handler := NewConnectHandler(store, 10*time.Minute)

	req := httptest.NewRequest("POST", "/api/internal/centrifugo/connect", nil)
	req.AddCookie(&http.Cookie{Name: "teco_session", Value: sessID})
	req.AddCookie(&http.Cookie{Name: "teco_seance", Value: "expired-id"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var resp connectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, 403, resp.Error.Code)
}

func TestConnectHandler_SeanceMismatch(t *testing.T) {
	store := newMockStore()
	sessID := uuid.New().String()
	seanceID := uuid.New().String()

	store.sessions[sessID] = &auth.Session{ID: sessID, UserID: uuid.New()}
	store.seances[seanceID] = &auth.Seance{ID: seanceID, SessionID: "different-session"}

	handler := NewConnectHandler(store, 10*time.Minute)

	req := httptest.NewRequest("POST", "/api/internal/centrifugo/connect", nil)
	req.AddCookie(&http.Cookie{Name: "teco_session", Value: sessID})
	req.AddCookie(&http.Cookie{Name: "teco_seance", Value: seanceID})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	var resp connectResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, 403, resp.Error.Code)
}
