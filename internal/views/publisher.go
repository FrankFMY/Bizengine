package views

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ViewPublisher abstracts sending view updates to clients.
type ViewPublisher interface {
	SendSnapshot(ctx context.Context, seanceID string, msg ViewSnapshotMsg) error
	SendTableDiff(ctx context.Context, wsID uuid.UUID, msg TableDiffMsg) error
	SendViewDiff(ctx context.Context, seanceID string, msg ViewDiffMsg) error
}

// ViewSnapshotMsg is sent when a client first subscribes to a view.
type ViewSnapshotMsg struct {
	Type       string                    `json:"type"`
	View       string                    `json:"view"`
	ParamsHash string                    `json:"params_hash"`
	Version    int64                     `json:"version"`
	Refs       []DataRef                 `json:"refs"`
	Tables     map[string]map[string]any `json:"tables"`
}

// TableDiffMsg is sent when a row's data changes (workspace-wide).
type TableDiffMsg struct {
	Type  string    `json:"type"`
	Table string    `json:"table"`
	ID    string    `json:"id"`
	Ver   int64     `json:"ver"`
	Patch []PatchOp `json:"patch"`
}

// ViewDiffMsg is sent when a view's ref list changes (per-seance).
type ViewDiffMsg struct {
	Type       string                    `json:"type"`
	View       string                    `json:"view"`
	ParamsHash string                    `json:"params_hash"`
	Version    int64                     `json:"version"`
	RefsPatch  []PatchOp                 `json:"refs_patch"`
	Tables     map[string]map[string]any `json:"tables,omitempty"`
}

// CentrifugoViewPublisher sends view messages via Centrifugo HTTP API.
type CentrifugoViewPublisher struct {
	apiURL string
	apiKey string
	client *http.Client
}

// NewCentrifugoViewPublisher creates a publisher that pushes to Centrifugo.
func NewCentrifugoViewPublisher(apiURL, apiKey string) *CentrifugoViewPublisher {
	return &CentrifugoViewPublisher{
		apiURL: apiURL,
		apiKey: apiKey,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// SendSnapshot publishes a view_snapshot to the seance's views channel.
func (p *CentrifugoViewPublisher) SendSnapshot(ctx context.Context, seanceID string, msg ViewSnapshotMsg) error {
	msg.Type = "view_snapshot"
	return p.publish(ctx, "views:"+seanceID, msg)
}

// SendTableDiff publishes a table_diff to the workspace channel.
func (p *CentrifugoViewPublisher) SendTableDiff(ctx context.Context, wsID uuid.UUID, msg TableDiffMsg) error {
	msg.Type = "table_diff"
	return p.publish(ctx, "workspace:"+wsID.String(), msg)
}

// SendViewDiff publishes a view_diff to the seance's views channel.
func (p *CentrifugoViewPublisher) SendViewDiff(ctx context.Context, seanceID string, msg ViewDiffMsg) error {
	msg.Type = "view_diff"
	return p.publish(ctx, "views:"+seanceID, msg)
}

type centrifugoPublishReq struct {
	Method string             `json:"method"`
	Params centrifugoPublish  `json:"params"`
}

type centrifugoPublish struct {
	Channel string          `json:"channel"`
	Data    json.RawMessage `json:"data"`
}

func (p *CentrifugoViewPublisher) publish(ctx context.Context, channel string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}

	body, err := json.Marshal(centrifugoPublishReq{
		Method: "publish",
		Params: centrifugoPublish{Channel: channel, Data: payload},
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
		log.Error().Err(err).Str("channel", channel).Msg("views: centrifugo publish failed")
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().Int("status", resp.StatusCode).Str("channel", channel).Msg("views: centrifugo unexpected status")
	}

	return nil
}
