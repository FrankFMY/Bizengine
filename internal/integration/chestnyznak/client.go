package chestnyznak

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// ChestnyZnakConfig holds configuration for Chestny Znak API.
type ChestnyZnakConfig struct {
	BaseURL string // e.g. "https://markirovka.crpt.ru/api/v3"
	Token   string // API token (OMS certificate-based)
}

// Client implements MarkingService via Chestny Znak (CRPT) API.
type Client struct {
	cfg    ChestnyZnakConfig
	client *http.Client
}

func NewClient(cfg ChestnyZnakConfig) *Client {
	return &Client{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	} else {
		reader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.cfg.BaseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)

	return c.client.Do(req)
}

func (c *Client) VerifyCode(ctx context.Context, code string) (*MarkingInfo, error) {
	path := fmt.Sprintf("/facade/identifytools/check?code=%s", code)
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("chestnyznak: verify: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("chestnyznak: verify failed with status %d", resp.StatusCode)
	}

	var result struct {
		Code        string `json:"code"`
		Valid       bool   `json:"valid"`
		ProductName string `json:"productName"`
		Category    string `json:"productGroup"`
		Status      string `json:"status"`
	}
	json.NewDecoder(resp.Body).Decode(&result)

	return &MarkingInfo{
		Code:        result.Code,
		Valid:       result.Valid,
		ProductName: result.ProductName,
		Category:    result.Category,
		Status:      result.Status,
	}, nil
}

func (c *Client) RegisterReceipt(ctx context.Context, orgID uuid.UUID, codes []string, documentID string) error {
	payload := map[string]any{
		"document_type": "receipt",
		"document_id":   documentID,
		"codes":         codes,
	}
	body, _ := json.Marshal(payload)

	resp, err := c.doRequest(ctx, "POST", "/facade/doc/create", body)
	if err != nil {
		return fmt.Errorf("chestnyznak: register receipt: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("chestnyznak: register receipt failed with status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) RegisterShipment(ctx context.Context, orgID uuid.UUID, codes []string, counterpartyINN string) error {
	payload := map[string]any{
		"document_type":    "shipment",
		"counterparty_inn": counterpartyINN,
		"codes":            codes,
	}
	body, _ := json.Marshal(payload)

	resp, err := c.doRequest(ctx, "POST", "/facade/doc/create", body)
	if err != nil {
		return fmt.Errorf("chestnyznak: register shipment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("chestnyznak: register shipment failed with status %d", resp.StatusCode)
	}
	return nil
}
