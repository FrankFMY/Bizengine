package edo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// DiadocConfig holds configuration for Diadoc API.
type DiadocConfig struct {
	BaseURL  string // e.g. "https://diadoc-api.kontur.ru"
	APIKey   string // developer API key
	Login    string
	Password string
	BoxID    string // organization box ID in Diadoc
}

// DiadocClient implements EDOService via Diadoc (Kontur) API.
type DiadocClient struct {
	cfg    DiadocConfig
	client *http.Client
	token  string
}

func NewDiadocClient(cfg DiadocConfig) *DiadocClient {
	return &DiadocClient{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *DiadocClient) authenticate(ctx context.Context) error {
	if c.token != "" {
		return nil
	}

	url := c.cfg.BaseURL + "/V3/Authenticate"
	body, _ := json.Marshal(map[string]string{
		"login":    c.cfg.Login,
		"password": c.cfg.Password,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "DiadocAuth ddauth_api_client_id="+c.cfg.APIKey)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("diadoc: authenticate: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("diadoc: auth failed with status %d", resp.StatusCode)
	}

	var result struct {
		Token string `json:"token"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	c.token = result.Token
	return nil
}

func (c *DiadocClient) doRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	if err := c.authenticate(ctx); err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "DiadocAuth ddauth_api_client_id="+c.cfg.APIKey+",ddauth_token="+c.token)

	return c.client.Do(req)
}

func (c *DiadocClient) SendDocument(ctx context.Context, orgID uuid.UUID, doc EDODocument) (*SendResult, error) {
	payload := map[string]any{
		"FromBoxId": c.cfg.BoxID,
		"Type":      doc.Type,
		"Number":    doc.Number,
		"Date":      doc.Date,
		"Amount":    doc.Amount,
	}

	body, _ := json.Marshal(payload)
	resp, err := c.doRequest(ctx, "POST", "/V3/PostMessage", body)
	if err != nil {
		return nil, fmt.Errorf("diadoc: send document: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("diadoc: send failed with status %d", resp.StatusCode)
	}

	var result struct {
		MessageID string `json:"MessageId"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return &SendResult{
		DocumentID: result.MessageID,
		Status:     "sent",
	}, nil
}

func (c *DiadocClient) GetIncomingDocuments(ctx context.Context, _ uuid.UUID, since time.Time) ([]EDODocument, error) {
	path := fmt.Sprintf("/V3/GetDocuments?boxId=%s&filterCategory=Any.InboundNotRevoked&timestampFrom=%s",
		c.cfg.BoxID, since.Format(time.RFC3339))

	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("diadoc: get incoming: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Documents []struct {
			DocumentID string `json:"DocumentId"`
			Type       string `json:"Type"`
			Number     string `json:"Number"`
			Date       string `json:"Date"`
		} `json:"Documents"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	docs := make([]EDODocument, 0, len(result.Documents))
	for _, d := range result.Documents {
		docs = append(docs, EDODocument{
			ID:     d.DocumentID,
			Type:   d.Type,
			Number: d.Number,
			Date:   d.Date,
			Status: "delivered",
		})
	}
	return docs, nil
}

func (c *DiadocClient) AcceptDocument(ctx context.Context, _ uuid.UUID, documentID string) error {
	payload := map[string]any{
		"BoxId":      c.cfg.BoxID,
		"MessageId":  documentID,
		"ActionType": "Accept",
	}
	body, _ := json.Marshal(payload)
	resp, err := c.doRequest(ctx, "POST", "/V3/PostMessagePatch", body)
	if err != nil {
		return fmt.Errorf("diadoc: accept: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("diadoc: accept failed with status %d", resp.StatusCode)
	}
	return nil
}

func (c *DiadocClient) RejectDocument(ctx context.Context, _ uuid.UUID, documentID string, reason string) error {
	payload := map[string]any{
		"BoxId":      c.cfg.BoxID,
		"MessageId":  documentID,
		"ActionType": "Reject",
		"Comment":    reason,
	}
	body, _ := json.Marshal(payload)
	resp, err := c.doRequest(ctx, "POST", "/V3/PostMessagePatch", body)
	if err != nil {
		return fmt.Errorf("diadoc: reject: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("diadoc: reject failed with status %d", resp.StatusCode)
	}
	return nil
}
