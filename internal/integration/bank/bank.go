// Package bank provides the bank integration service interface for 1C bank exchange format.
package bank

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// PaymentOrder represents a payment order for export.
type PaymentOrder struct {
	Number          string `json:"number"`
	Date            string `json:"date"`
	Amount          int64  `json:"amount"`
	PayerINN        string `json:"payer_inn"`
	PayerAccount    string `json:"payer_account"`
	ReceiverINN     string `json:"receiver_inn"`
	ReceiverAccount string `json:"receiver_account"`
	Purpose         string `json:"purpose"`
}

// ImportResult is the result of importing a bank statement.
type ImportResult struct {
	TransactionsImported int       `json:"transactions_imported"`
	TotalDebit           int64     `json:"total_debit"`
	TotalCredit          int64     `json:"total_credit"`
	Period               string    `json:"period"`
	ImportedAt           time.Time `json:"imported_at"`
}

// BankService defines the interface for bank integration operations.
type BankService interface {
	ImportStatement(ctx context.Context, wsID uuid.UUID, data []byte) (*ImportResult, error)
	ExportPaymentOrders(ctx context.Context, wsID uuid.UUID, orders []PaymentOrder) ([]byte, error)
}

// Stub is a stub implementation of BankService that logs calls and returns success.
type Stub struct{}

// NewStub creates a new stub BankService.
func NewStub() *Stub { return &Stub{} }

// ImportStatement logs the call and returns a fake import result.
func (s *Stub) ImportStatement(_ context.Context, wsID uuid.UUID, data []byte) (*ImportResult, error) {
	log.Debug().Str("ws", wsID.String()).Int("data_len", len(data)).Msg("bank stub: ImportStatement")
	return &ImportResult{
		TransactionsImported: 0,
		TotalDebit:           0,
		TotalCredit:          0,
		Period:               "stub",
		ImportedAt:           time.Now(),
	}, nil
}

// ExportPaymentOrders logs the call and returns empty 1C format data.
func (s *Stub) ExportPaymentOrders(_ context.Context, wsID uuid.UUID, orders []PaymentOrder) ([]byte, error) {
	log.Debug().Str("ws", wsID.String()).Int("orders", len(orders)).Msg("bank stub: ExportPaymentOrders")
	return []byte("1CClientBankExchange\nVersionEncoding=UTF-8\nSender=BizEngine\nRecipient=Bank\nFormatVersion=1.03\n"), nil
}
