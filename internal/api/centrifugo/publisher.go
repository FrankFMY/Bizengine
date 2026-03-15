// Package centrifugo provides Centrifugo integration for real-time event delivery.
package centrifugo

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/bizengine/engine/pkg/types"
)

// Publisher forwards events to Centrifugo via its HTTP API.
type Publisher struct {
	apiURL string
	apiKey string
	client *http.Client
}

// NewPublisher creates a new Centrifugo publisher.
func NewPublisher(apiURL, apiKey string) *Publisher {
	return &Publisher{
		apiURL: apiURL,
		apiKey: apiKey,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

type publishRequest struct {
	Method string      `json:"method"`
	Params publishData `json:"params"`
}

type publishData struct {
	Channel string          `json:"channel"`
	Data    json.RawMessage `json:"data"`
}

// HandleEvent publishes an event to the organization channel in Centrifugo.
// For messenger events, it also publishes to the chat:{conversationID} channel.
func (p *Publisher) HandleEvent(ctx context.Context, ev types.Event) error {
	channel := "org:" + ev.OrganizationID.String()

	payload, err := json.Marshal(ev)
	if err != nil {
		return err
	}

	p.publish(ctx, channel, payload)

	// Messenger events: also publish to chat channel for real-time delivery
	var data map[string]any
	if json.Unmarshal(ev.Data, &data) == nil {
		if chatChannel, ok := data["_channel"].(string); ok && chatChannel != "" {
			p.publish(ctx, chatChannel, payload)
		}
	}

	return nil
}

func (p *Publisher) publish(ctx context.Context, channel string, payload json.RawMessage) {
	body, err := json.Marshal(publishRequest{
		Method: "publish",
		Params: publishData{
			Channel: channel,
			Data:    payload,
		},
	})
	if err != nil {
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "apikey "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		log.Error().Err(err).Str("channel", channel).Msg("centrifugo: publish failed")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status", resp.StatusCode).Str("channel", channel).Msg("centrifugo: unexpected status")
	}
}

type disconnectRequest struct {
	Method string         `json:"method"`
	Params disconnectData `json:"params"`
}

type disconnectData struct {
	User string `json:"user"`
}

// Disconnect forces all connections of a user to be closed via Centrifugo API.
func (p *Publisher) Disconnect(ctx context.Context, userID string) error {
	body, err := json.Marshal(disconnectRequest{
		Method: "disconnect",
		Params: disconnectData{User: userID},
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.apiURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "apikey "+p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		log.Error().Err(err).Str("user", userID).Msg("centrifugo: disconnect failed")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status", resp.StatusCode).Str("user", userID).Msg("centrifugo: disconnect unexpected status")
	}

	return nil
}
