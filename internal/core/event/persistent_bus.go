package event

import (
	"context"

	"github.com/rs/zerolog/log"

	"github.com/bizengine/engine/pkg/types"
)

// PersistentBus wraps a Bus and automatically persists all published events
// to the Store before dispatching them to subscribers.
type PersistentBus struct {
	inner Bus
	store Store
}

// NewPersistentBus creates a bus that persists events before dispatching.
func NewPersistentBus(inner Bus, store Store) *PersistentBus {
	return &PersistentBus{inner: inner, store: store}
}

// Publish persists the event to the store, then dispatches to subscribers.
func (b *PersistentBus) Publish(ctx context.Context, event types.Event) error {
	if err := b.store.Append(ctx, event); err != nil {
		log.Error().
			Err(err).
			Str("event_type", event.Type).
			Str("event_id", event.ID.String()).
			Msg("failed to persist event, dispatching anyway")
	}
	return b.inner.Publish(ctx, event)
}

// Subscribe delegates to the inner bus.
func (b *PersistentBus) Subscribe(eventType string, handler Subscriber) {
	b.inner.Subscribe(eventType, handler)
}

// SubscribePattern delegates to the inner bus.
func (b *PersistentBus) SubscribePattern(pattern string, handler Subscriber) {
	b.inner.SubscribePattern(pattern, handler)
}

// SubscribeAll delegates to the inner bus.
func (b *PersistentBus) SubscribeAll(handler Subscriber) {
	b.inner.SubscribeAll(handler)
}
