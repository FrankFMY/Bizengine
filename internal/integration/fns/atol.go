package fns

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ATOLConfig holds configuration for ATOL Online API.
type ATOLConfig struct {
	BaseURL  string // e.g. "https://online.atol.ru/possystem/v4"
	Login    string
	Password string
	GroupID  string
	INN      string
}

// ATOLClient implements FiscalService via ATOL Online API.
type ATOLClient struct {
	cfg    ATOLConfig
	client *http.Client
	token  string
	mu     sync.Mutex
}

func NewATOLClient(cfg ATOLConfig) *ATOLClient {
	return &ATOLClient{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *ATOLClient) getToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.token != "" {
		return c.token, nil
	}

	body, _ := json.Marshal(map[string]string{
		"login": c.cfg.Login,
		"pass":  c.cfg.Password,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", c.cfg.BaseURL+"/getToken", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("atol: get token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Token string `json:"token"`
		Error *struct {
			Code int    `json:"code"`
			Text string `json:"text"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("atol: decode token response: %w", err)
	}
	if result.Error != nil {
		return "", fmt.Errorf("atol: token error %d: %s", result.Error.Code, result.Error.Text)
	}

	c.token = result.Token
	return c.token, nil
}

func (c *ATOLClient) SendReceipt(ctx context.Context, orgID uuid.UUID, receipt Receipt) (*ReceiptResult, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]map[string]any, len(receipt.Items))
	for i, item := range receipt.Items {
		items[i] = map[string]any{
			"name":     item.Name,
			"price":    float64(item.Price) / 100,
			"quantity": item.Quantity,
			"sum":      float64(item.Total) / 100,
			"vat":      map[string]any{"type": mapVAT(item.VAT)},
		}
	}

	payments := make([]map[string]any, len(receipt.Payments))
	for i, p := range receipt.Payments {
		payments[i] = map[string]any{
			"type": mapPaymentType(p.Type),
			"sum":  float64(p.Amount) / 100,
		}
	}

	total := int64(0)
	for _, p := range receipt.Payments {
		total += p.Amount
	}

	payload := map[string]any{
		"external_id": uuid.New().String(),
		"receipt": map[string]any{
			"client": map[string]any{
				"email": receipt.CustomerContact,
			},
			"company": map[string]any{
				"inn":            c.cfg.INN,
				"payment_address": "online",
			},
			"items":    items,
			"payments": payments,
			"total":    float64(total) / 100,
		},
		"timestamp": time.Now().Format("02.01.2006 15:04:05"),
	}

	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("%s/%s/%s", c.cfg.BaseURL, c.cfg.GroupID, receipt.Type)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("atol: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Token", token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("atol: send receipt: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		UUID  string `json:"uuid"`
		Error *struct {
			Code int    `json:"code"`
			Text string `json:"text"`
		} `json:"error"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("atol: decode response: %w", err)
	}
	if result.Error != nil && result.Error.Code != 0 {
		// Reset token on auth errors
		if result.Error.Code == 4 || result.Error.Code == 5 {
			c.mu.Lock()
			c.token = ""
			c.mu.Unlock()
		}
		return nil, fmt.Errorf("atol: error %d: %s", result.Error.Code, result.Error.Text)
	}

	return &ReceiptResult{
		ID:        result.UUID,
		Status:    result.Status,
		CreatedAt: time.Now(),
	}, nil
}

func (c *ATOLClient) GetReceiptStatus(ctx context.Context, _ uuid.UUID, receiptID string) (*ReceiptStatus, error) {
	token, err := c.getToken(ctx)
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s/report/%s", c.cfg.BaseURL, c.cfg.GroupID, receiptID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Token", token)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("atol: get status: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		UUID   string `json:"uuid"`
		Status string `json:"status"`
		Payload struct {
			FiscalDocumentNumber int `json:"fiscal_document_number"`
		} `json:"payload"`
		Error *struct {
			Code int    `json:"code"`
			Text string `json:"text"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("atol: decode status: %w", err)
	}

	status := "pending"
	if result.Status == "done" {
		status = "done"
	} else if result.Status == "fail" {
		status = "fail"
	}

	errText := ""
	if result.Error != nil {
		errText = result.Error.Text
	}

	return &ReceiptStatus{
		ID:        result.UUID,
		Status:    status,
		FiscalNum: fmt.Sprintf("%d", result.Payload.FiscalDocumentNumber),
		Error:     errText,
	}, nil
}

func mapVAT(vat string) string {
	switch vat {
	case "none":
		return "none"
	case "vat0":
		return "vat0"
	case "vat10":
		return "vat10"
	case "vat20":
		return "vat20"
	default:
		return "none"
	}
}

func mapPaymentType(t string) int {
	switch t {
	case "cash":
		return 0
	case "card", "online":
		return 1
	default:
		return 1
	}
}
