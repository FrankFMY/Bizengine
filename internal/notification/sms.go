package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/rs/zerolog/log"
)

type SMSMessage struct {
	Phone string `json:"phone"`
	Text  string `json:"text"`
}

type SMSSender interface {
	Send(ctx context.Context, msg SMSMessage) error
}

// SMSConfig holds SMS gateway connection parameters.
type SMSConfig struct {
	APIURL string
	APIKey string
	Sender string
}

// HTTPSMSSender sends SMS via HTTP gateway (compatible with most Russian SMS providers).
type HTTPSMSSender struct {
	cfg    SMSConfig
	client *http.Client
}

func NewHTTPSMSSender(cfg SMSConfig) *HTTPSMSSender {
	return &HTTPSMSSender{cfg: cfg, client: &http.Client{}}
}

func (s *HTTPSMSSender) Send(ctx context.Context, msg SMSMessage) error {
	payload := map[string]string{
		"phone":  msg.Phone,
		"text":   msg.Text,
		"sender": s.cfg.Sender,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("sms: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.cfg.APIURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("sms: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sms: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("sms: gateway returned %d", resp.StatusCode)
	}
	return nil
}

// SMSStub logs SMS without sending them.
type SMSStub struct{}

func NewSMSStub() *SMSStub { return &SMSStub{} }

func (s *SMSStub) Send(_ context.Context, msg SMSMessage) error {
	log.Debug().Str("phone", msg.Phone).Str("text", msg.Text).Msg("sms stub: Send")
	return nil
}
