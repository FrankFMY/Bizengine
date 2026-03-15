package postgres

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/module/banking"
	"github.com/bizengine/engine/pkg/errs"
)

// BankingRepo implements banking.Repository with PostgreSQL.
type BankingRepo struct {
	pool *pgxpool.Pool
}

// NewBankingRepo creates a new BankingRepo.
func NewBankingRepo(pool *pgxpool.Pool) *BankingRepo {
	return &BankingRepo{pool: pool}
}

// GetBankPartner returns a bank partner by ID.
func (r *BankingRepo) GetBankPartner(ctx context.Context, id uuid.UUID) (*banking.BankPartner, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, code, api_base_url, settings, white_label, active, ver, upd, iat
		 FROM bank_partners WHERE id = $1`, id)
	return r.scanBankPartner(row)
}

// GetBankPartnerByCode returns a bank partner by code.
func (r *BankingRepo) GetBankPartnerByCode(ctx context.Context, code string) (*banking.BankPartner, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT id, name, code, api_base_url, settings, white_label, active, ver, upd, iat
		 FROM bank_partners WHERE code = $1`, code)
	return r.scanBankPartner(row)
}

// ListBankPartners returns all active bank partners.
func (r *BankingRepo) ListBankPartners(ctx context.Context) ([]banking.BankPartner, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, code, api_base_url, settings, white_label, active, ver, upd, iat
		 FROM bank_partners WHERE active = true ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var partners []banking.BankPartner
	for rows.Next() {
		bp, err := r.scanBankPartnerRow(rows)
		if err != nil {
			return nil, err
		}
		partners = append(partners, *bp)
	}
	if partners == nil {
		partners = []banking.BankPartner{}
	}
	return partners, rows.Err()
}

// GetOrgBankPartner returns the bank partner for an organization.
func (r *BankingRepo) GetOrgBankPartner(ctx context.Context, orgID uuid.UUID) (*banking.BankPartner, error) {
	row := r.pool.QueryRow(ctx,
		`SELECT bp.id, bp.name, bp.code, bp.api_base_url, bp.settings, bp.white_label, bp.active, bp.ver, bp.upd, bp.iat
		 FROM bank_partners bp
		 JOIN organizations o ON o.bank_partner_id = bp.id
		 WHERE o.id = $1 AND bp.active = true`, orgID)
	bp, err := r.scanBankPartner(row)
	if err != nil {
		return nil, nil
	}
	return bp, nil
}

// CreateReconciliation inserts a new reconciliation record.
func (r *BankingRepo) CreateReconciliation(ctx context.Context, tx pgx.Tx, rec *banking.Reconciliation) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO bank_reconciliations (id, organization_id, bank_partner_id, date_from, date_to, total_entries, matched, unmatched, status, ver, upd, iat)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 1, now(), now())`,
		rec.ID, rec.OrganizationID, rec.BankPartnerID, rec.DateFrom, rec.DateTo,
		rec.TotalEntries, rec.Matched, rec.Unmatched, rec.Status)
	return err
}

// CreateReconciliationEntry inserts a new reconciliation entry.
func (r *BankingRepo) CreateReconciliationEntry(ctx context.Context, tx pgx.Tx, e *banking.ReconciliationEntry) error {
	entryJSON, _ := json.Marshal(e.StatementEntry)
	_, err := tx.Exec(ctx,
		`INSERT INTO bank_reconciliation_entries (id, reconciliation_id, organization_id, statement_entry, matched_type, matched_id, transaction_id, status, ver, upd, iat)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, now(), now())`,
		e.ID, e.ReconciliationID, e.OrganizationID, entryJSON,
		e.MatchedType, e.MatchedID, e.TransactionID, e.Status)
	return err
}

// GetReconciliation returns a reconciliation by ID.
func (r *BankingRepo) GetReconciliation(ctx context.Context, orgID, id uuid.UUID) (*banking.Reconciliation, error) {
	var rec banking.Reconciliation
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, bank_partner_id, date_from, date_to, total_entries, matched, unmatched, status, ver, upd, iat
		 FROM bank_reconciliations WHERE id = $1 AND organization_id = $2`, id, orgID).
		Scan(&rec.ID, &rec.OrganizationID, &rec.BankPartnerID, &rec.DateFrom, &rec.DateTo,
			&rec.TotalEntries, &rec.Matched, &rec.Unmatched, &rec.Status, &rec.Ver, &rec.Upd, &rec.Iat)
	if err != nil {
		return nil, errs.NewNotFound("reconciliation not found")
	}
	return &rec, nil
}

// ListReconciliations returns reconciliations for an organization.
func (r *BankingRepo) ListReconciliations(ctx context.Context, orgID uuid.UUID, limit, offset int) ([]banking.Reconciliation, int, error) {
	var total int
	_ = r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM bank_reconciliations WHERE organization_id = $1`, orgID).Scan(&total)

	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, bank_partner_id, date_from, date_to, total_entries, matched, unmatched, status, ver, upd, iat
		 FROM bank_reconciliations WHERE organization_id = $1 ORDER BY iat DESC LIMIT $2 OFFSET $3`,
		orgID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var recs []banking.Reconciliation
	for rows.Next() {
		var rec banking.Reconciliation
		if err := rows.Scan(&rec.ID, &rec.OrganizationID, &rec.BankPartnerID, &rec.DateFrom, &rec.DateTo,
			&rec.TotalEntries, &rec.Matched, &rec.Unmatched, &rec.Status, &rec.Ver, &rec.Upd, &rec.Iat); err != nil {
			return nil, 0, err
		}
		recs = append(recs, rec)
	}
	if recs == nil {
		recs = []banking.Reconciliation{}
	}
	return recs, total, rows.Err()
}

// GetReconciliationEntries returns all entries for a reconciliation.
func (r *BankingRepo) GetReconciliationEntries(ctx context.Context, reconID uuid.UUID) ([]banking.ReconciliationEntry, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, reconciliation_id, organization_id, statement_entry, matched_type, matched_id, transaction_id, status, ver, upd, iat
		 FROM bank_reconciliation_entries WHERE reconciliation_id = $1 ORDER BY iat`, reconID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []banking.ReconciliationEntry
	for rows.Next() {
		var e banking.ReconciliationEntry
		var entryJSON []byte
		if err := rows.Scan(&e.ID, &e.ReconciliationID, &e.OrganizationID, &entryJSON,
			&e.MatchedType, &e.MatchedID, &e.TransactionID, &e.Status, &e.Ver, &e.Upd, &e.Iat); err != nil {
			return nil, err
		}
		json.Unmarshal(entryJSON, &e.StatementEntry)
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []banking.ReconciliationEntry{}
	}
	return entries, rows.Err()
}

// UpdateEntryStatus updates the status and match info of a reconciliation entry.
func (r *BankingRepo) UpdateEntryStatus(ctx context.Context, tx pgx.Tx, entryID uuid.UUID, status string, matchedType *string, matchedID *uuid.UUID, transactionID *uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE bank_reconciliation_entries SET status = $1, matched_type = $2, matched_id = $3, transaction_id = $4 WHERE id = $5`,
		status, matchedType, matchedID, transactionID, entryID)
	return err
}

// FindOrderByAmountAndINN finds an order matching the given amount and customer INN.
func (r *BankingRepo) FindOrderByAmountAndINN(ctx context.Context, orgID uuid.UUID, amount int64, inn string) (*uuid.UUID, error) {
	var orderID uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT o.id FROM orders o
		 JOIN entities e ON e.id = o.entity_id AND e.organization_id = o.organization_id
		 LEFT JOIN components c ON c.entity_id = e.id AND c.type = 'contact'
		 WHERE o.organization_id = $1 AND o.total = $2 AND o.status IN ('confirmed', 'new')
		 AND (c.data->>'inn' = $3 OR $3 = '')
		 ORDER BY o.created_at DESC LIMIT 1`,
		orgID, amount, inn).Scan(&orderID)
	if err != nil {
		return nil, err
	}
	return &orderID, nil
}

// WithTx executes fn inside a transaction.
func (r *BankingRepo) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *BankingRepo) scanBankPartner(row pgx.Row) (*banking.BankPartner, error) {
	var bp banking.BankPartner
	var settingsJSON, wlJSON []byte
	err := row.Scan(&bp.ID, &bp.Name, &bp.Code, &bp.APIBaseURL, &settingsJSON, &wlJSON,
		&bp.Active, &bp.Ver, &bp.Upd, &bp.Iat)
	if err != nil {
		return nil, errs.NewNotFound("bank partner not found")
	}
	json.Unmarshal(settingsJSON, &bp.Settings)
	json.Unmarshal(wlJSON, &bp.WhiteLabel)
	return &bp, nil
}

func (r *BankingRepo) scanBankPartnerRow(rows pgx.Rows) (*banking.BankPartner, error) {
	var bp banking.BankPartner
	var settingsJSON, wlJSON []byte
	err := rows.Scan(&bp.ID, &bp.Name, &bp.Code, &bp.APIBaseURL, &settingsJSON, &wlJSON,
		&bp.Active, &bp.Ver, &bp.Upd, &bp.Iat)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(settingsJSON, &bp.Settings)
	json.Unmarshal(wlJSON, &bp.WhiteLabel)
	return &bp, nil
}
