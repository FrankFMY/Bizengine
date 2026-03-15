package redis

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/auth"
)

func setupTestStore(t *testing.T) (*SessionStore, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	return NewSessionStore(client), mr
}

func newTestSession() *auth.Session {
	return &auth.Session{
		ID:             uuid.New().String(),
		UserID:         uuid.New(),
		PhoneID:        uuid.New(),
		OrganizationID: uuid.New(),
		Role:           "owner",
		Email:          "test@example.com",
		FullName:       "Test User",
		CreatedAt:      time.Now(),
	}
}

func TestCreateAndGetSession(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()
	sess := newTestSession()

	err := store.CreateSession(ctx, sess, 10*time.Minute)
	require.NoError(t, err)

	got, err := store.GetSession(ctx, sess.ID)
	require.NoError(t, err)

	assert.Equal(t, sess.ID, got.ID)
	assert.Equal(t, sess.UserID, got.UserID)
	assert.Equal(t, sess.Email, got.Email)
	assert.Equal(t, sess.Role, got.Role)
	assert.Equal(t, sess.OrganizationID, got.OrganizationID)
}

func TestGetSession_NotFound(t *testing.T) {
	store, _ := setupTestStore(t)

	_, err := store.GetSession(context.Background(), "nonexistent-id")
	require.Error(t, err)
}

func TestGetSession_Expired(t *testing.T) {
	store, mr := setupTestStore(t)
	ctx := context.Background()
	sess := newTestSession()

	err := store.CreateSession(ctx, sess, 1*time.Second)
	require.NoError(t, err)

	mr.FastForward(2 * time.Second)

	_, err = store.GetSession(ctx, sess.ID)
	require.Error(t, err)
}

func TestUpdateSession(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()
	sess := newTestSession()

	err := store.CreateSession(ctx, sess, 10*time.Minute)
	require.NoError(t, err)

	sess.Role = "admin"
	sess.OrganizationID = uuid.New()
	err = store.UpdateSession(ctx, sess, 10*time.Minute)
	require.NoError(t, err)

	got, err := store.GetSession(ctx, sess.ID)
	require.NoError(t, err)
	assert.Equal(t, "admin", got.Role)
	assert.Equal(t, sess.OrganizationID, got.OrganizationID)
}

func TestDeleteSession(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()
	sess := newTestSession()

	err := store.CreateSession(ctx, sess, 10*time.Minute)
	require.NoError(t, err)

	err = store.DeleteSession(ctx, sess.ID)
	require.NoError(t, err)

	_, err = store.GetSession(ctx, sess.ID)
	require.Error(t, err)
}

func TestCreateAndGetSeance(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	seance := &auth.Seance{
		ID:        uuid.New().String(),
		SessionID: uuid.New().String(),
		CreatedAt: time.Now(),
	}

	err := store.CreateSeance(ctx, seance, 5*time.Minute)
	require.NoError(t, err)

	got, err := store.GetSeance(ctx, seance.ID)
	require.NoError(t, err)

	assert.Equal(t, seance.ID, got.ID)
	assert.Equal(t, seance.SessionID, got.SessionID)
}

func TestGetSeance_NotFound(t *testing.T) {
	store, _ := setupTestStore(t)

	_, err := store.GetSeance(context.Background(), "nonexistent")
	require.Error(t, err)
}

func TestSlideSeance(t *testing.T) {
	store, mr := setupTestStore(t)
	ctx := context.Background()

	seance := &auth.Seance{
		ID:        uuid.New().String(),
		SessionID: uuid.New().String(),
		CreatedAt: time.Now(),
	}

	err := store.CreateSeance(ctx, seance, 2*time.Second)
	require.NoError(t, err)

	err = store.SlideSeance(ctx, seance.ID, 10*time.Minute)
	require.NoError(t, err)

	mr.FastForward(5 * time.Second)

	got, err := store.GetSeance(ctx, seance.ID)
	require.NoError(t, err)
	assert.Equal(t, seance.ID, got.ID)
}

func TestDeleteSessionSeances(t *testing.T) {
	store, _ := setupTestStore(t)
	ctx := context.Background()

	sessionID := uuid.New().String()

	for i := 0; i < 3; i++ {
		seance := &auth.Seance{
			ID:        uuid.New().String(),
			SessionID: sessionID,
			CreatedAt: time.Now(),
		}
		err := store.CreateSeance(ctx, seance, 10*time.Minute)
		require.NoError(t, err)
	}

	err := store.DeleteSessionSeances(ctx, sessionID)
	require.NoError(t, err)
}

func TestDeleteSessionSeances_NoSeances(t *testing.T) {
	store, _ := setupTestStore(t)
	err := store.DeleteSessionSeances(context.Background(), "no-session")
	require.NoError(t, err)
}

func TestSessionStoreImplementsInterface(t *testing.T) {
	var _ auth.SessionStore = (*SessionStore)(nil)
}
