package ws

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/pkg/types"
)

func newTestClient(hub *Hub, wsID, userID uuid.UUID) *Client {
	return &Client{
		hub:         hub,
		send:        make(chan []byte, sendBufSize),
		workspaceID: wsID,
		userID:      userID,
	}
}

func registerClient(hub *Hub, c *Client) {
	hub.mu.Lock()
	hub.clients[c] = true
	hub.mu.Unlock()
}

func readMsg(c *Client) (ServerMessage, bool) {
	select {
	case data := <-c.send:
		var msg ServerMessage
		json.Unmarshal(data, &msg)
		return msg, true
	case <-time.After(50 * time.Millisecond):
		return ServerMessage{}, false
	}
}

func TestHubBroadcastToSubscribedClient(t *testing.T) {
	hub := NewHub(nil)
	wsID := uuid.New()

	client := newTestClient(hub, wsID, uuid.New())
	client.allEvts = true
	registerClient(hub, client)

	ev := types.Event{
		ID:          uuid.New(),
		WorkspaceID: wsID,
		Type:        "order.created",
		Timestamp:   time.Now(),
	}
	err := hub.HandleEvent(context.Background(), ev)
	require.NoError(t, err)

	msg, ok := readMsg(client)
	require.True(t, ok, "expected message on client channel")
	assert.Equal(t, "event", msg.Type)
}

func TestHubWorkspaceIsolation(t *testing.T) {
	hub := NewHub(nil)
	wsA := uuid.New()
	wsB := uuid.New()

	clientA := newTestClient(hub, wsA, uuid.New())
	clientA.allEvts = true
	registerClient(hub, clientA)

	clientB := newTestClient(hub, wsB, uuid.New())
	clientB.allEvts = true
	registerClient(hub, clientB)

	ev := types.Event{
		ID:          uuid.New(),
		WorkspaceID: wsA,
		Type:        "entity.created",
		Timestamp:   time.Now(),
	}
	hub.HandleEvent(context.Background(), ev)

	_, okA := readMsg(clientA)
	assert.True(t, okA, "client A should receive event from workspace A")

	_, okB := readMsg(clientB)
	assert.False(t, okB, "client B should NOT receive event from workspace A")
}

func TestHubEventFiltering(t *testing.T) {
	hub := NewHub(nil)
	wsID := uuid.New()

	client := newTestClient(hub, wsID, uuid.New())
	client.filters = []string{"order.*"}
	registerClient(hub, client)

	// Should match
	hub.HandleEvent(context.Background(), types.Event{
		ID: uuid.New(), WorkspaceID: wsID, Type: "order.created", Timestamp: time.Now(),
	})
	_, ok := readMsg(client)
	assert.True(t, ok, "order.created should match filter order.*")

	// Should not match
	hub.HandleEvent(context.Background(), types.Event{
		ID: uuid.New(), WorkspaceID: wsID, Type: "entity.updated", Timestamp: time.Now(),
	})
	_, ok = readMsg(client)
	assert.False(t, ok, "entity.updated should NOT match filter order.*")
}

func TestHubNoFiltersNoDelivery(t *testing.T) {
	hub := NewHub(nil)
	wsID := uuid.New()

	client := newTestClient(hub, wsID, uuid.New())
	registerClient(hub, client)

	hub.HandleEvent(context.Background(), types.Event{
		ID: uuid.New(), WorkspaceID: wsID, Type: "order.created", Timestamp: time.Now(),
	})

	_, ok := readMsg(client)
	assert.False(t, ok, "client with no filters and allEvts=false should not receive events")
}

func TestMatchesEventWildcard(t *testing.T) {
	c := &Client{filters: []string{"warehouse.stock.*"}}

	assert.True(t, c.matchesEvent("warehouse.stock.received"))
	assert.True(t, c.matchesEvent("warehouse.stock.adjusted"))
	assert.False(t, c.matchesEvent("warehouse.movement"))
	assert.False(t, c.matchesEvent("order.created"))
}

func TestMatchesEventExact(t *testing.T) {
	c := &Client{filters: []string{"order.created", "order.confirmed"}}

	assert.True(t, c.matchesEvent("order.created"))
	assert.True(t, c.matchesEvent("order.confirmed"))
	assert.False(t, c.matchesEvent("order.paid"))
}

func TestMatchesEventSubscribeAll(t *testing.T) {
	c := &Client{allEvts: true}

	assert.True(t, c.matchesEvent("anything"))
	assert.True(t, c.matchesEvent("order.created"))
}
