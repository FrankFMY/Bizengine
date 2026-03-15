// Package banking provides the banking business module.
package banking

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// BankPartner represents a partner bank.
type BankPartner struct {
	ID         uuid.UUID      `json:"id"`
	Name       string         `json:"name"`
	Code       string         `json:"code"`
	APIBaseURL *string        `json:"api_base_url,omitempty"`
	Settings   map[string]any `json:"settings,omitempty"`
	WhiteLabel WhiteLabel     `json:"white_label"`
	Active     bool           `json:"active"`
	Ver        int            `json:"ver"`
	Upd        time.Time      `json:"upd"`
	Iat        time.Time      `json:"iat"`
}

// WhiteLabel holds bank branding information.
type WhiteLabel struct {
	LogoURL      string `json:"logo_url,omitempty"`
	PrimaryColor string `json:"primary_color,omitempty"`
	AppName      string `json:"app_name,omitempty"`
	BankName     string `json:"bank_name,omitempty"`
}

// Reconciliation represents a bank reconciliation run.
type Reconciliation struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	BankPartnerID  *uuid.UUID `json:"bank_partner_id,omitempty"`
	DateFrom       time.Time  `json:"date_from"`
	DateTo         time.Time  `json:"date_to"`
	TotalEntries   int        `json:"total_entries"`
	Matched        int        `json:"matched"`
	Unmatched      int        `json:"unmatched"`
	Status         string     `json:"status"`
	Ver            int        `json:"ver"`
	Upd            time.Time  `json:"upd"`
	Iat            time.Time  `json:"iat"`
}

// ReconciliationEntry represents a single entry in a reconciliation.
type ReconciliationEntry struct {
	ID               uuid.UUID      `json:"id"`
	ReconciliationID uuid.UUID      `json:"reconciliation_id"`
	OrganizationID   uuid.UUID      `json:"organization_id"`
	StatementEntry   map[string]any `json:"statement_entry"`
	MatchedType      *string        `json:"matched_type,omitempty"`
	MatchedID        *uuid.UUID     `json:"matched_id,omitempty"`
	TransactionID    *uuid.UUID     `json:"transaction_id,omitempty"`
	Status           string         `json:"status"`
	Ver              int            `json:"ver"`
	Upd              time.Time      `json:"upd"`
	Iat              time.Time      `json:"iat"`
}

// ReconciliationResult is returned by auto-reconciliation.
type ReconciliationResult struct {
	Reconciliation Reconciliation        `json:"reconciliation"`
	Entries        []ReconciliationEntry `json:"entries"`
}

// Repository defines data access for the banking module.
type Repository interface {
	// Bank partners
	GetBankPartner(ctx context.Context, id uuid.UUID) (*BankPartner, error)
	GetBankPartnerByCode(ctx context.Context, code string) (*BankPartner, error)
	ListBankPartners(ctx context.Context) ([]BankPartner, error)
	GetOrgBankPartner(ctx context.Context, orgID uuid.UUID) (*BankPartner, error)

	// Reconciliation
	CreateReconciliation(ctx context.Context, tx pgx.Tx, r *Reconciliation) error
	CreateReconciliationEntry(ctx context.Context, tx pgx.Tx, e *ReconciliationEntry) error
	GetReconciliation(ctx context.Context, orgID, id uuid.UUID) (*Reconciliation, error)
	ListReconciliations(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]Reconciliation, int, error)
	GetReconciliationEntries(ctx context.Context, reconID uuid.UUID) ([]ReconciliationEntry, error)
	UpdateEntryStatus(ctx context.Context, tx pgx.Tx, entryID uuid.UUID, status string, matchedType *string, matchedID *uuid.UUID, transactionID *uuid.UUID) error

	// Matching
	FindOrderByAmountAndINN(ctx context.Context, orgID uuid.UUID, amount int64, inn string) (*uuid.UUID, error)

	WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error
}
