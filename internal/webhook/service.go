package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type Webhook struct {
	ID             uuid.UUID `json:"id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	URL            string    `json:"url"`
	Secret         string    `json:"secret,omitempty"`
	Events         []string  `json:"events"`
	Active         bool      `json:"active"`
	CreatedAt      time.Time `json:"created_at"`
}

type Repository interface {
	Create(ctx context.Context, w *Webhook) error
	Get(ctx context.Context, orgID, id uuid.UUID) (*Webhook, error)
	List(ctx context.Context, orgID uuid.UUID) ([]Webhook, error)
	Update(ctx context.Context, w *Webhook) error
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	ListByEvent(ctx context.Context, orgID uuid.UUID, eventType string) ([]Webhook, error)
}

type DeliveryLog struct {
	ID         uuid.UUID `json:"id"`
	WebhookID  uuid.UUID `json:"webhook_id"`
	EventType  string    `json:"event_type"`
	StatusCode int       `json:"status_code"`
	Success    bool      `json:"success"`
	Duration   int64     `json:"duration_ms"`
	CreatedAt  time.Time `json:"created_at"`
}

type Service struct {
	repo   Repository
	client *http.Client
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (s *Service) Create(ctx context.Context, orgID uuid.UUID, url string, events []string, secret string) (*Webhook, error) {
	w := &Webhook{
		ID:             uuid.New(),
		OrganizationID: orgID,
		URL:            url,
		Secret:         secret,
		Events:         events,
		Active:         true,
		CreatedAt:      time.Now(),
	}
	if err := s.repo.Create(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Service) List(ctx context.Context, orgID uuid.UUID) ([]Webhook, error) {
	return s.repo.List(ctx, orgID)
}

func (s *Service) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	return s.repo.Delete(ctx, orgID, id)
}

func (s *Service) Toggle(ctx context.Context, orgID, id uuid.UUID, active bool) (*Webhook, error) {
	w, err := s.repo.Get(ctx, orgID, id)
	if err != nil {
		return nil, err
	}
	w.Active = active
	if err := s.repo.Update(ctx, w); err != nil {
		return nil, err
	}
	return w, nil
}

// Dispatch sends an event to all matching webhooks for an organization.
func (s *Service) Dispatch(ctx context.Context, orgID uuid.UUID, eventType string, payload any) {
	hooks, err := s.repo.ListByEvent(ctx, orgID, eventType)
	if err != nil {
		log.Error().Err(err).Str("event", eventType).Msg("webhook: failed to list hooks")
		return
	}

	body, _ := json.Marshal(map[string]any{
		"event":     eventType,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      payload,
	})

	for _, hook := range hooks {
		if !hook.Active {
			continue
		}
		go s.deliver(hook, body)
	}
}

func (s *Service) deliver(hook Webhook, body []byte) {
	start := time.Now()

	req, err := http.NewRequest("POST", hook.URL, bytes.NewReader(body))
	if err != nil {
		log.Error().Err(err).Str("url", hook.URL).Msg("webhook: create request failed")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "BizEngine-Webhook/1.0")

	if hook.Secret != "" {
		sig := computeHMAC(body, hook.Secret)
		req.Header.Set("X-Webhook-Signature", "sha256="+sig)
	}

	resp, err := s.client.Do(req)
	duration := time.Since(start).Milliseconds()

	if err != nil {
		log.Warn().Err(err).Str("url", hook.URL).Int64("ms", duration).Msg("webhook: delivery failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Warn().Str("url", hook.URL).Int("status", resp.StatusCode).Int64("ms", duration).Msg("webhook: non-success response")
	}
}

func computeHMAC(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

// CreateWebhookInput is the API input for creating a webhook.
type CreateWebhookInput struct {
	URL    string   `json:"url"`
	Events []string `json:"events"`
	Secret string   `json:"secret,omitempty"`
}

func (s *Service) Get(ctx context.Context, orgID, id uuid.UUID) (*Webhook, error) {
	return s.repo.Get(ctx, orgID, id)
}

// Test sends a test event to a webhook.
func (s *Service) Test(ctx context.Context, orgID, id uuid.UUID) error {
	hook, err := s.repo.Get(ctx, orgID, id)
	if err != nil {
		return err
	}

	body, _ := json.Marshal(map[string]any{
		"event":     "webhook.test",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      map[string]string{"message": "test webhook delivery"},
	})

	req, err := http.NewRequest("POST", hook.URL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("webhook: create test request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if hook.Secret != "" {
		sig := computeHMAC(body, hook.Secret)
		req.Header.Set("X-Webhook-Signature", "sha256="+sig)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("webhook: test delivery failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("webhook: test returned status %d", resp.StatusCode)
	}
	return nil
}
