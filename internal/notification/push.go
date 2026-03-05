package notification

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type PushMessage struct {
	UserID uuid.UUID `json:"user_id"`
	Title  string    `json:"title"`
	Body   string    `json:"body"`
	Data   map[string]string `json:"data,omitempty"`
}

type PushSender interface {
	Send(ctx context.Context, msg PushMessage) error
	SendBulk(ctx context.Context, msgs []PushMessage) error
}

// PushStub logs push notifications without sending them.
type PushStub struct{}

func NewPushStub() *PushStub { return &PushStub{} }

func (s *PushStub) Send(_ context.Context, msg PushMessage) error {
	log.Debug().Str("user", msg.UserID.String()).Str("title", msg.Title).Msg("push stub: Send")
	return nil
}

func (s *PushStub) SendBulk(_ context.Context, msgs []PushMessage) error {
	log.Debug().Int("count", len(msgs)).Msg("push stub: SendBulk")
	return nil
}
