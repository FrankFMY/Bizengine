// Package fns provides the fiscal service interface for Russian FZ-54 online cash registers.
package fns

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// ReceiptItem represents a line item on a fiscal receipt.
type ReceiptItem struct {
	Name     string `json:"name"`
	Price    int64  `json:"price"`
	Quantity int    `json:"quantity"`
	Total    int64  `json:"total"`
	VAT      string `json:"vat"` // none, vat0, vat10, vat20
}

// Payment represents a payment method on a receipt.
type Payment struct {
	Type   string `json:"type"` // cash, card, online
	Amount int64  `json:"amount"`
}

// Receipt is the input for creating a fiscal receipt.
type Receipt struct {
	Type            string        `json:"type"` // sell, sell_return, buy, buy_return
	Items           []ReceiptItem `json:"items"`
	Payments        []Payment     `json:"payments"`
	CustomerContact string        `json:"customer_contact"`
	TaxSystem       string        `json:"tax_system"` // osn, usn_income, usn_income_outcome, envd, esn, patent
}

// ReceiptResult is the result of sending a receipt.
type ReceiptResult struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"`
	FiscalNum string    `json:"fiscal_num"`
	CreatedAt time.Time `json:"created_at"`
}

// ReceiptStatus represents the current status of a fiscal receipt.
type ReceiptStatus struct {
	ID        string `json:"id"`
	Status    string `json:"status"` // pending, done, fail
	FiscalNum string `json:"fiscal_num,omitempty"`
	Error     string `json:"error,omitempty"`
}

// FiscalService defines the interface for fiscal receipt operations.
type FiscalService interface {
	SendReceipt(ctx context.Context, orgID uuid.UUID, receipt Receipt) (*ReceiptResult, error)
	GetReceiptStatus(ctx context.Context, orgID uuid.UUID, receiptID string) (*ReceiptStatus, error)
}

// Stub is a stub implementation of FiscalService that logs calls and returns success.
type Stub struct{}

// NewStub creates a new stub FiscalService.
func NewStub() *Stub { return &Stub{} }

// SendReceipt logs the call and returns a fake success result.
func (s *Stub) SendReceipt(_ context.Context, orgID uuid.UUID, receipt Receipt) (*ReceiptResult, error) {
	log.Debug().Str("org", orgID.String()).Str("type", receipt.Type).Int("items", len(receipt.Items)).Msg("fns stub: SendReceipt")
	return &ReceiptResult{
		ID:        uuid.New().String(),
		Status:    "done",
		FiscalNum: "STUB-0000001",
		CreatedAt: time.Now(),
	}, nil
}

// GetReceiptStatus logs the call and returns a fake done status.
func (s *Stub) GetReceiptStatus(_ context.Context, orgID uuid.UUID, receiptID string) (*ReceiptStatus, error) {
	log.Debug().Str("org", orgID.String()).Str("receipt_id", receiptID).Msg("fns stub: GetReceiptStatus")
	return &ReceiptStatus{
		ID:        receiptID,
		Status:    "done",
		FiscalNum: "STUB-0000001",
	}, nil
}
