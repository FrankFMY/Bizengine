package event

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/pkg/types"
)

func newTestEvent(eventType string) types.Event {
	return types.Event{
		ID:             uuid.New(),
		OrganizationID: uuid.New(),
		Type:           eventType,
		Data:           []byte(`{}`),
		Timestamp:      time.Now(),
		Version:        1,
	}
}

func TestLocalBus_Subscribe(t *testing.T) {
	bus := NewLocalBus()
	var received []types.Event
	var mu sync.Mutex

	bus.Subscribe("order.created", SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		mu.Lock()
		received = append(received, ev)
		mu.Unlock()
		return nil
	}))

	ev := newTestEvent("order.created")
	err := bus.Publish(context.Background(), ev)
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	assert.Len(t, received, 1)
	assert.Equal(t, ev.ID, received[0].ID)
	mu.Unlock()
}

func TestLocalBus_Subscribe_NoMatch(t *testing.T) {
	bus := NewLocalBus()
	called := false

	bus.Subscribe("order.created", SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		called = true
		return nil
	}))

	err := bus.Publish(context.Background(), newTestEvent("warehouse.stock.received"))
	require.NoError(t, err)

	time.Sleep(50 * time.Millisecond)
	assert.False(t, called)
}

func TestLocalBus_SubscribeAll(t *testing.T) {
	bus := NewLocalBus()
	var count int
	var mu sync.Mutex

	bus.SubscribeAll(SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	}))

	bus.Publish(context.Background(), newTestEvent("order.created"))
	bus.Publish(context.Background(), newTestEvent("warehouse.stock.received"))

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	assert.Equal(t, 2, count)
	mu.Unlock()
}

func TestLocalBus_SubscribePattern(t *testing.T) {
	bus := NewLocalBus()
	var received []string
	var mu sync.Mutex

	bus.SubscribePattern("warehouse.stock.*", SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		mu.Lock()
		received = append(received, ev.Type)
		mu.Unlock()
		return nil
	}))

	bus.Publish(context.Background(), newTestEvent("warehouse.stock.received"))
	bus.Publish(context.Background(), newTestEvent("warehouse.stock.shipped"))
	bus.Publish(context.Background(), newTestEvent("order.created"))

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	assert.Len(t, received, 2)
	assert.Contains(t, received, "warehouse.stock.received")
	assert.Contains(t, received, "warehouse.stock.shipped")
	mu.Unlock()
}

func TestLocalBus_MultipleSubscribers(t *testing.T) {
	bus := NewLocalBus()
	var count int
	var mu sync.Mutex

	handler := SubscriberFunc(func(ctx context.Context, ev types.Event) error {
		mu.Lock()
		count++
		mu.Unlock()
		return nil
	})

	bus.Subscribe("order.created", handler)
	bus.Subscribe("order.created", handler)
	bus.SubscribeAll(handler)

	bus.Publish(context.Background(), newTestEvent("order.created"))

	time.Sleep(50 * time.Millisecond)
	mu.Lock()
	assert.Equal(t, 3, count)
	mu.Unlock()
}
