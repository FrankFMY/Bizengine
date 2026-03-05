package finance

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/bizengine/engine/pkg/types"
)

// Repository defines data access for the finance module.
type Repository interface {
	// Accounts
	CreateAccount(ctx context.Context, tx pgx.Tx, a *Account) error
	GetAccount(ctx context.Context, orgID, accountID uuid.UUID) (*Account, error)
	GetAccountByCode(ctx context.Context, orgID uuid.UUID, code string) (*Account, error)
	ListAccounts(ctx context.Context, orgID uuid.UUID) ([]Account, error)
	DeleteAccount(ctx context.Context, orgID, accountID uuid.UUID) error

	// Transactions
	CreateTransaction(ctx context.Context, tx pgx.Tx, t *Transaction) error
	CreateTransactionLines(ctx context.Context, tx pgx.Tx, lines []TransactionLine) error
	GetTransaction(ctx context.Context, orgID, txnID uuid.UUID) (*Transaction, error)
	GetTransactionLines(ctx context.Context, txnID uuid.UUID) ([]TransactionLine, error)
	ListTransactions(ctx context.Context, orgID uuid.UUID, filter TransactionFilter) ([]Transaction, int, error)
	UpdateTransaction(ctx context.Context, tx pgx.Tx, t *Transaction) error

	// Invoices
	CreateInvoice(ctx context.Context, inv *Invoice) error
	GetInvoice(ctx context.Context, orgID, invoiceID uuid.UUID) (*Invoice, error)
	ListInvoices(ctx context.Context, orgID uuid.UUID, filter InvoiceFilter) ([]Invoice, int, error)
	UpdateInvoice(ctx context.Context, inv *Invoice) error

	// Reports
	GetTrialBalance(ctx context.Context, orgID uuid.UUID, date time.Time) ([]TrialBalanceRow, error)
	GetAccountBalance(ctx context.Context, orgID, accountID uuid.UUID, from, to time.Time) (*AccountBalance, error)

	// Has lines check for delete protection
	HasTransactionLines(ctx context.Context, orgID, accountID uuid.UUID) (bool, error)

	WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}

// Account represents a chart of accounts entry.
type Account struct {
	ID          uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	Code        string     `json:"code"`
	Name        string     `json:"name"`
	Type        string     `json:"type"`
	ParentID    *uuid.UUID `json:"parent_id,omitempty"`
	IsSystem    bool       `json:"is_system"`
	Currency    string     `json:"currency"`
	CreatedAt   time.Time  `json:"created_at"`
}

// Transaction represents a journal entry.
type Transaction struct {
	ID            uuid.UUID         `json:"id"`
	OrganizationID   uuid.UUID         `json:"organization_id"`
	Date          time.Time         `json:"date"`
	Description   string            `json:"description"`
	ReferenceType *string           `json:"reference_type,omitempty"`
	ReferenceID   *uuid.UUID        `json:"reference_id,omitempty"`
	IsPosted      bool              `json:"is_posted"`
	ActorID       *uuid.UUID        `json:"actor_id,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	Lines         []TransactionLine `json:"lines,omitempty"`
}

// TransactionLine represents a debit or credit line.
type TransactionLine struct {
	ID            uuid.UUID  `json:"id"`
	TransactionID uuid.UUID  `json:"transaction_id"`
	OrganizationID   uuid.UUID  `json:"organization_id"`
	AccountID     uuid.UUID  `json:"account_id"`
	Debit         int64      `json:"debit"`
	Credit        int64      `json:"credit"`
	Description   string     `json:"description"`
	EntityID      *uuid.UUID `json:"entity_id,omitempty"`
}

// Invoice represents an incoming or outgoing invoice.
type Invoice struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID    uuid.UUID  `json:"organization_id"`
	Number         string     `json:"number"`
	Type           string     `json:"type"`
	CounterpartyID *uuid.UUID `json:"counterparty_id,omitempty"`
	OrderID        *uuid.UUID `json:"order_id,omitempty"`
	Subtotal       int64      `json:"subtotal"`
	Tax            int64      `json:"tax"`
	Total          int64      `json:"total"`
	Currency       string     `json:"currency"`
	Status         string     `json:"status"`
	IssuedAt       *time.Time `json:"issued_at,omitempty"`
	DueAt          *time.Time `json:"due_at,omitempty"`
	PaidAt         *time.Time `json:"paid_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// TrialBalanceRow represents a row in the trial balance report.
type TrialBalanceRow struct {
	AccountID uuid.UUID `json:"account_id"`
	Code      string    `json:"code"`
	Name      string    `json:"name"`
	Debit     int64     `json:"debit"`
	Credit    int64     `json:"credit"`
}

// AccountBalance represents the balance of a single account.
type AccountBalance struct {
	AccountID  uuid.UUID `json:"account_id"`
	DebitTotal int64     `json:"debit_total"`
	CreditTotal int64   `json:"credit_total"`
	Balance    int64     `json:"balance"`
}

// TransactionFilter holds query params for listing transactions.
type TransactionFilter struct {
	AccountID *uuid.UUID
	From      *time.Time
	To        *time.Time
	IsPosted  *bool
	Page      types.PageRequest
}

// InvoiceFilter holds query params for listing invoices.
type InvoiceFilter struct {
	Type   *string
	Status *string
	Page   types.PageRequest
}

// CreateAccountInput is the input for creating an account.
type CreateAccountInput struct {
	Code     string     `json:"code"`
	Name     string     `json:"name"`
	Type     string     `json:"type"`
	ParentID *uuid.UUID `json:"parent_id"`
	Currency string     `json:"currency"`
}

// CreateTransactionInput is the input for creating a transaction.
type CreateTransactionInput struct {
	Date          string                    `json:"date"`
	Description   string                    `json:"description"`
	ReferenceType *string                   `json:"reference_type"`
	ReferenceID   *uuid.UUID                `json:"reference_id"`
	Lines         []CreateTransactionLineInput `json:"lines"`
}

// CreateTransactionLineInput is the input for a transaction line.
type CreateTransactionLineInput struct {
	AccountID   uuid.UUID  `json:"account_id"`
	Debit       int64      `json:"debit"`
	Credit      int64      `json:"credit"`
	Description string     `json:"description"`
	EntityID    *uuid.UUID `json:"entity_id"`
}

// CreateInvoiceInput is the input for creating an invoice.
type CreateInvoiceInput struct {
	Number         string     `json:"number"`
	Type           string     `json:"type"`
	CounterpartyID *uuid.UUID `json:"counterparty_id"`
	OrderID        *uuid.UUID `json:"order_id"`
	Subtotal       int64      `json:"subtotal"`
	Tax            int64      `json:"tax"`
	Total          int64      `json:"total"`
	Currency       string     `json:"currency"`
	IssuedAt       *time.Time `json:"issued_at"`
	DueAt          *time.Time `json:"due_at"`
}

// DefaultAccounts returns the default chart of accounts for a new organization.
func DefaultAccounts() []CreateAccountInput {
	return []CreateAccountInput{
		{Code: "10", Name: "Materials", Type: "asset"},
		{Code: "41", Name: "Goods", Type: "asset"},
		{Code: "50", Name: "Cash", Type: "asset"},
		{Code: "51", Name: "Bank account", Type: "asset"},
		{Code: "60", Name: "Suppliers", Type: "liability"},
		{Code: "62", Name: "Customers", Type: "asset"},
		{Code: "70", Name: "Payroll", Type: "liability"},
		{Code: "90", Name: "Sales", Type: "revenue"},
		{Code: "91", Name: "Other income/expense", Type: "revenue"},
		{Code: "99", Name: "Profit and loss", Type: "equity"},
	}
}
