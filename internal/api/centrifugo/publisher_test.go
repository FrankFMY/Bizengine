package centrifugo

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/pkg/types"
)

func TestPublisher_HandleEvent(t *testing.T) {
	var received publishRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "apikey test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &received)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	pub := NewPublisher(srv.URL, "test-key")

	wsID := uuid.New()
	entityID := uuid.New()
	ev := types.Event{
		ID:          uuid.New(),
		WorkspaceID: wsID,
		EntityID:    &entityID,
		Type:        "entity.created",
		Data:        json.RawMessage(`{"name":"Widget"}`),
	}

	err := pub.HandleEvent(context.Background(), ev)
	require.NoError(t, err)

	assert.Equal(t, "publish", received.Method)
	assert.Equal(t, "workspace:"+wsID.String(), received.Params.Channel)
}

func TestPublisher_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	pub := NewPublisher(srv.URL, "test-key")

	ev := types.Event{
		ID:          uuid.New(),
		WorkspaceID: uuid.New(),
		Type:        "test.event",
		Data:        json.RawMessage(`{}`),
	}

	// Should not return error (logs instead)
	err := pub.HandleEvent(context.Background(), ev)
	assert.NoError(t, err)
}
