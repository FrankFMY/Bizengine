package event

import (
	"context"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"

	"github.com/bizengine/engine/pkg/types"
)

// Publisher publishes events to subscribers.
type Publisher interface {
	Publish(ctx context.Context, event types.Event) error
}

// Subscriber handles events.
type Subscriber interface {
	HandleEvent(ctx context.Context, event types.Event) error
}

// SubscriberFunc is an adapter to use ordinary functions as Subscriber.
type SubscriberFunc func(ctx context.Context, event types.Event) error

// HandleEvent calls f(ctx, event).
func (f SubscriberFunc) HandleEvent(ctx context.Context, event types.Event) error {
	return f(ctx, event)
}

// Bus manages event subscriptions and dispatch.
type Bus interface {
	Publisher
	Subscribe(eventType string, handler Subscriber)
	SubscribePattern(pattern string, handler Subscriber)
	SubscribeAll(handler Subscriber)
}

type subscription struct {
	eventType string
	pattern   string
	all       bool
	handler   Subscriber
}

// LocalBus is an in-process event bus with goroutine fan-out.
type LocalBus struct {
	mu   sync.RWMutex
	subs []subscription
}

// NewLocalBus creates a new in-process event bus.
func NewLocalBus() *LocalBus {
	return &LocalBus{}
}

// Subscribe registers a handler for a specific event type.
func (b *LocalBus) Subscribe(eventType string, handler Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = append(b.subs, subscription{eventType: eventType, handler: handler})
}

// SubscribePattern registers a handler for events matching a wildcard pattern.
// Pattern "warehouse.stock.*" matches "warehouse.stock.received", "warehouse.stock.shipped", etc.
func (b *LocalBus) SubscribePattern(pattern string, handler Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	prefix := strings.TrimSuffix(pattern, "*")
	b.subs = append(b.subs, subscription{pattern: prefix, handler: handler})
}

// SubscribeAll registers a handler for all events.
func (b *LocalBus) SubscribeAll(handler Subscriber) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs = append(b.subs, subscription{all: true, handler: handler})
}

// Publish dispatches an event to all matching subscribers.
// Each subscriber runs in its own goroutine for non-blocking fan-out.
func (b *LocalBus) Publish(ctx context.Context, event types.Event) error {
	b.mu.RLock()
	matching := make([]Subscriber, 0, len(b.subs))
	for _, sub := range b.subs {
		if sub.all || sub.eventType == event.Type || (sub.pattern != "" && strings.HasPrefix(event.Type, sub.pattern)) {
			matching = append(matching, sub.handler)
		}
	}
	b.mu.RUnlock()

	// Detach from caller context so subscribers are not cancelled
	// when the HTTP request completes.
	bgCtx := context.WithoutCancel(ctx)

	for _, handler := range matching {
		go func(h Subscriber) {
			if err := h.HandleEvent(bgCtx, event); err != nil {
				log.Error().
					Err(err).
					Str("event_type", event.Type).
					Str("event_id", event.ID.String()).
					Msg("event handler failed")
			}
		}(handler)
	}

	return nil
}
