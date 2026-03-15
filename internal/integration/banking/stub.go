package banking

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// Stub is a development stub for BankAdapter that logs calls and returns success.
type Stub struct{}

// NewStub creates a new stub BankAdapter.
func NewStub() *Stub { return &Stub{} }

// InitiatePayment logs the call and returns a fake payment result.
func (s *Stub) InitiatePayment(_ context.Context, req PaymentRequest) (*PaymentResult, error) {
	log.Debug().
		Str("org", req.OrganizationID.String()).
		Int64("amount", req.Amount).
		Str("purpose", req.Purpose).
		Msg("banking stub: InitiatePayment")
	return &PaymentResult{
		PaymentID:  uuid.New().String(),
		PaymentURL: "https://stub.bank/pay/" + uuid.New().String(),
		Status:     "pending",
	}, nil
}

// GetPaymentStatus logs the call and returns a completed status.
func (s *Stub) GetPaymentStatus(_ context.Context, paymentID string) (*PaymentStatus, error) {
	log.Debug().Str("payment_id", paymentID).Msg("banking stub: GetPaymentStatus")
	return &PaymentStatus{
		PaymentID: paymentID,
		Status:    "completed",
		Amount:    0,
		UpdatedAt: time.Now(),
	}, nil
}

// GetStatement logs the call and returns an empty statement.
func (s *Stub) GetStatement(_ context.Context, req StatementRequest) (*Statement, error) {
	log.Debug().
		Str("org", req.OrganizationID.String()).
		Time("from", req.DateFrom).
		Time("to", req.DateTo).
		Msg("banking stub: GetStatement")
	return &Statement{
		DateFrom:     req.DateFrom,
		DateTo:       req.DateTo,
		OpenBalance:  0,
		CloseBalance: 0,
		Entries:      []StatementEntry{},
	}, nil
}

// CreatePayrollBatch logs the call and returns a fake batch result.
func (s *Stub) CreatePayrollBatch(_ context.Context, batch PayrollBatch) (*PayrollBatchResult, error) {
	log.Debug().
		Str("org", batch.OrganizationID.String()).
		Int("employees", len(batch.Employees)).
		Int("month", batch.Month).
		Int("year", batch.Year).
		Msg("banking stub: CreatePayrollBatch")
	return &PayrollBatchResult{
		BatchID: uuid.New().String(),
		Status:  "completed",
		Count:   len(batch.Employees),
	}, nil
}

// GetPayrollBatchStatus logs the call and returns completed status.
func (s *Stub) GetPayrollBatchStatus(_ context.Context, batchID string) (*PayrollBatchStatus, error) {
	log.Debug().Str("batch_id", batchID).Msg("banking stub: GetPayrollBatchStatus")
	return &PayrollBatchStatus{
		BatchID:   batchID,
		Status:    "completed",
		Processed: 0,
		Failed:    0,
		Total:     0,
		UpdatedAt: time.Now(),
	}, nil
}

// GetAuthURL returns a stub OAuth URL.
func (s *Stub) GetAuthURL(_ context.Context, redirectURL string) (string, error) {
	log.Debug().Str("redirect", redirectURL).Msg("banking stub: GetAuthURL")
	return "https://stub.bank/oauth/authorize?redirect_uri=" + redirectURL, nil
}

// ExchangeCode returns a stub bank user.
func (s *Stub) ExchangeCode(_ context.Context, code string) (*BankUser, error) {
	log.Debug().Str("code", code).Msg("banking stub: ExchangeCode")
	return &BankUser{
		BankClientID: "stub-client-" + code,
		FullName:     "Stub User",
		Phone:        "79991234567",
		Email:        "stub@bank.test",
	}, nil
}

// GetBankInfo returns stub bank metadata.
func (s *Stub) GetBankInfo() BankInfo {
	return BankInfo{Code: "stub", Name: "Stub Bank"}
}
