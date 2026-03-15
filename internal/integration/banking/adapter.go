// Package banking provides the multi-bank integration layer.
package banking

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// BankAdapter defines the interface for bank-specific API operations.
type BankAdapter interface {
	// Payments
	InitiatePayment(ctx context.Context, req PaymentRequest) (*PaymentResult, error)
	GetPaymentStatus(ctx context.Context, paymentID string) (*PaymentStatus, error)

	// Statements
	GetStatement(ctx context.Context, req StatementRequest) (*Statement, error)

	// Payroll
	CreatePayrollBatch(ctx context.Context, batch PayrollBatch) (*PayrollBatchResult, error)
	GetPayrollBatchStatus(ctx context.Context, batchID string) (*PayrollBatchStatus, error)

	// OAuth
	GetAuthURL(ctx context.Context, redirectURL string) (string, error)
	ExchangeCode(ctx context.Context, code string) (*BankUser, error)

	// Info
	GetBankInfo() BankInfo
}

// PaymentRequest is the input for initiating a payment.
type PaymentRequest struct {
	OrganizationID   uuid.UUID  `json:"organization_id"`
	Amount           int64      `json:"amount"`
	Currency         string     `json:"currency"`
	Purpose          string     `json:"purpose"`
	RecipientINN     string     `json:"recipient_inn"`
	RecipientBIC     string     `json:"recipient_bic"`
	RecipientAccount string     `json:"recipient_account"`
	OrderID          *uuid.UUID `json:"order_id,omitempty"`
}

// PaymentResult is the result of payment initiation.
type PaymentResult struct {
	PaymentID  string `json:"payment_id"`
	PaymentURL string `json:"payment_url,omitempty"`
	Status     string `json:"status"`
}

// PaymentStatus is the current status of a payment.
type PaymentStatus struct {
	PaymentID string    `json:"payment_id"`
	Status    string    `json:"status"`
	Amount    int64     `json:"amount"`
	UpdatedAt time.Time `json:"updated_at"`
}

// StatementRequest is the input for fetching bank statements.
type StatementRequest struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	DateFrom       time.Time `json:"date_from"`
	DateTo         time.Time `json:"date_to"`
}

// Statement is a bank account statement.
type Statement struct {
	DateFrom     time.Time        `json:"date_from"`
	DateTo       time.Time        `json:"date_to"`
	OpenBalance  int64            `json:"open_balance"`
	CloseBalance int64            `json:"close_balance"`
	Entries      []StatementEntry `json:"entries"`
}

// StatementEntry is a single transaction in a bank statement.
type StatementEntry struct {
	Date             time.Time `json:"date"`
	Amount           int64     `json:"amount"`
	Direction        string    `json:"direction"`
	Purpose          string    `json:"purpose"`
	CounterpartyINN  string    `json:"counterparty_inn"`
	CounterpartyName string    `json:"counterparty_name"`
	DocumentNumber   string    `json:"document_number"`
}

// PayrollBatch is a batch of salary payments.
type PayrollBatch struct {
	OrganizationID uuid.UUID         `json:"organization_id"`
	Employees      []PayrollEmployee `json:"employees"`
	Month          int               `json:"month"`
	Year           int               `json:"year"`
}

// PayrollEmployee is a single employee salary payment in a batch.
type PayrollEmployee struct {
	FullName   string    `json:"full_name"`
	CardNumber string    `json:"card_number"`
	Amount     int64     `json:"amount"`
	EmployeeID uuid.UUID `json:"employee_id"`
}

// PayrollBatchResult is the result of creating a payroll batch.
type PayrollBatchResult struct {
	BatchID string `json:"batch_id"`
	Status  string `json:"status"`
	Count   int    `json:"count"`
}

// PayrollBatchStatus is the current status of a payroll batch.
type PayrollBatchStatus struct {
	BatchID   string    `json:"batch_id"`
	Status    string    `json:"status"`
	Processed int       `json:"processed"`
	Failed    int       `json:"failed"`
	Total     int       `json:"total"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BankUser is the user info returned by bank OAuth.
type BankUser struct {
	BankClientID string `json:"bank_client_id"`
	FullName     string `json:"full_name"`
	INN          string `json:"inn,omitempty"`
	Phone        string `json:"phone,omitempty"`
	Email        string `json:"email,omitempty"`
}

// BankInfo contains bank metadata.
type BankInfo struct {
	Code string `json:"code"`
	Name string `json:"name"`
}
