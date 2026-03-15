package fns

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStub_ImplementsInterface(t *testing.T) {
	var _ FiscalService = (*Stub)(nil)
}

func TestStubSendReceipt(t *testing.T) {
	stub := NewStub()
	receipt := Receipt{
		Type: "sell",
		Items: []ReceiptItem{
			{Name: "Item 1", Price: 10000, Quantity: 2, Total: 20000, VAT: "vat20"},
		},
		Payments: []Payment{
			{Type: "card", Amount: 20000},
		},
		CustomerContact: "test@example.com",
		TaxSystem:       "osn",
	}

	result, err := stub.SendReceipt(context.Background(), uuid.New(), receipt)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEmpty(t, result.ID)
	assert.Equal(t, "done", result.Status)
	assert.NotEmpty(t, result.FiscalNum)
	assert.False(t, result.CreatedAt.IsZero())
}

func TestStubGetReceiptStatus(t *testing.T) {
	stub := NewStub()
	receiptID := "receipt-abc"

	status, err := stub.GetReceiptStatus(context.Background(), uuid.New(), receiptID)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, receiptID, status.ID)
	assert.Equal(t, "done", status.Status)
	assert.NotEmpty(t, status.FiscalNum)
}

func TestMapVAT(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"none", "none"},
		{"vat0", "vat0"},
		{"vat10", "vat10"},
		{"vat20", "vat20"},
		{"unknown", "none"},
		{"", "none"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapVAT(tt.input))
		})
	}
}

func TestMapPaymentType(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"cash", 0},
		{"card", 1},
		{"online", 1},
		{"unknown", 1},
		{"", 1},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, mapPaymentType(tt.input))
		})
	}
}

func TestATOLClient_SendReceipt_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/getToken" {
			json.NewEncoder(w).Encode(map[string]any{
				"token": "atol-token-123",
			})
			return
		}

		assert.Contains(t, r.URL.Path, "/group1/sell")
		assert.Equal(t, "atol-token-123", r.Header.Get("Token"))

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		receipt := body["receipt"].(map[string]any)
		assert.NotNil(t, receipt["items"])
		assert.NotNil(t, receipt["payments"])

		json.NewEncoder(w).Encode(map[string]any{
			"uuid":   "atol-uuid-1",
			"status": "wait",
		})
	}))
	defer srv.Close()

	client := NewATOLClient(ATOLConfig{
		BaseURL:  srv.URL,
		Login:    "login",
		Password: "pass",
		GroupID:  "group1",
		INN:      "1234567890",
	})

	receipt := Receipt{
		Type: "sell",
		Items: []ReceiptItem{
			{Name: "Test", Price: 10000, Quantity: 1, Total: 10000, VAT: "vat20"},
		},
		Payments:        []Payment{{Type: "card", Amount: 10000}},
		CustomerContact: "user@test.com",
		TaxSystem:       "osn",
	}

	result, err := client.SendReceipt(context.Background(), uuid.New(), receipt)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "atol-uuid-1", result.ID)
}

func TestATOLClient_GetReceiptStatus_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/getToken" {
			json.NewEncoder(w).Encode(map[string]any{"token": "tok"})
			return
		}

		assert.Contains(t, r.URL.Path, "/group1/report/receipt-42")

		json.NewEncoder(w).Encode(map[string]any{
			"uuid":   "receipt-42",
			"status": "done",
			"payload": map[string]any{
				"fiscal_document_number": 12345,
			},
		})
	}))
	defer srv.Close()

	client := NewATOLClient(ATOLConfig{BaseURL: srv.URL, GroupID: "group1"})

	status, err := client.GetReceiptStatus(context.Background(), uuid.New(), "receipt-42")
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, "receipt-42", status.ID)
	assert.Equal(t, "done", status.Status)
	assert.Equal(t, "12345", status.FiscalNum)
}

func TestATOLClient_TokenError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"code": 12,
				"text": "wrong credentials",
			},
		})
	}))
	defer srv.Close()

	client := NewATOLClient(ATOLConfig{BaseURL: srv.URL, GroupID: "g1"})

	_, err := client.SendReceipt(context.Background(), uuid.New(), Receipt{Type: "sell"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "wrong credentials")
}
