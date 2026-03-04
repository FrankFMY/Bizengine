package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/module/finance"
	"github.com/bizengine/engine/pkg/errs"
)

// FinanceRepo implements finance.Repository using PostgreSQL.
type FinanceRepo struct {
	pool *pgxpool.Pool
}

// NewFinanceRepo creates a new FinanceRepo.
func NewFinanceRepo(pool *pgxpool.Pool) *FinanceRepo {
	return &FinanceRepo{pool: pool}
}

// CreateAccount inserts an account record.
func (r *FinanceRepo) CreateAccount(ctx context.Context, tx pgx.Tx, a *finance.Account) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO accounts (id, workspace_id, code, name, type, parent_id, is_system, currency, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		a.ID, a.WorkspaceID, a.Code, a.Name, a.Type, a.ParentID, a.IsSystem, a.Currency, a.CreatedAt,
	)
	return err
}

// GetAccount returns an account by ID.
func (r *FinanceRepo) GetAccount(ctx context.Context, wsID, accountID uuid.UUID) (*finance.Account, error) {
	var a finance.Account
	err := r.pool.QueryRow(ctx,
		`SELECT id, workspace_id, code, name, type, parent_id, is_system, currency, created_at
		 FROM accounts WHERE workspace_id = $1 AND id = $2`,
		wsID, accountID,
	).Scan(&a.ID, &a.WorkspaceID, &a.Code, &a.Name, &a.Type, &a.ParentID, &a.IsSystem, &a.Currency, &a.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("account not found")
		}
		return nil, err
	}
	return &a, nil
}

// GetAccountByCode returns an account by code.
func (r *FinanceRepo) GetAccountByCode(ctx context.Context, wsID uuid.UUID, code string) (*finance.Account, error) {
	var a finance.Account
	err := r.pool.QueryRow(ctx,
		`SELECT id, workspace_id, code, name, type, parent_id, is_system, currency, created_at
		 FROM accounts WHERE workspace_id = $1 AND code = $2`,
		wsID, code,
	).Scan(&a.ID, &a.WorkspaceID, &a.Code, &a.Name, &a.Type, &a.ParentID, &a.IsSystem, &a.Currency, &a.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("account not found: " + code)
		}
		return nil, err
	}
	return &a, nil
}

// ListAccounts returns all accounts for a workspace.
func (r *FinanceRepo) ListAccounts(ctx context.Context, wsID uuid.UUID) ([]finance.Account, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, workspace_id, code, name, type, parent_id, is_system, currency, created_at
		 FROM accounts WHERE workspace_id = $1 ORDER BY code`,
		wsID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []finance.Account
	for rows.Next() {
		var a finance.Account
		if err := rows.Scan(&a.ID, &a.WorkspaceID, &a.Code, &a.Name, &a.Type, &a.ParentID, &a.IsSystem, &a.Currency, &a.CreatedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, a)
	}
	return accounts, rows.Err()
}

// DeleteAccount deletes an account.
func (r *FinanceRepo) DeleteAccount(ctx context.Context, wsID, accountID uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM accounts WHERE workspace_id = $1 AND id = $2`,
		wsID, accountID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NewNotFound("account not found")
	}
	return nil
}

// HasTransactionLines checks if an account has any transaction lines.
func (r *FinanceRepo) HasTransactionLines(ctx context.Context, wsID, accountID uuid.UUID) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM transaction_lines WHERE workspace_id = $1 AND account_id = $2)`,
		wsID, accountID,
	).Scan(&exists)
	return exists, err
}

// CreateTransaction inserts a transaction record.
func (r *FinanceRepo) CreateTransaction(ctx context.Context, tx pgx.Tx, t *finance.Transaction) error {
	_, err := tx.Exec(ctx,
		`INSERT INTO transactions (id, workspace_id, date, description, reference_type, reference_id, is_posted, actor_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		t.ID, t.WorkspaceID, t.Date, t.Description, t.ReferenceType, t.ReferenceID, t.IsPosted, t.ActorID, t.CreatedAt,
	)
	return err
}

// CreateTransactionLines inserts transaction line records.
func (r *FinanceRepo) CreateTransactionLines(ctx context.Context, tx pgx.Tx, lines []finance.TransactionLine) error {
	for _, line := range lines {
		if _, err := tx.Exec(ctx,
			`INSERT INTO transaction_lines (id, transaction_id, workspace_id, account_id, debit, credit, description, entity_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			line.ID, line.TransactionID, line.WorkspaceID, line.AccountID, line.Debit, line.Credit, line.Description, line.EntityID,
		); err != nil {
			return err
		}
	}
	return nil
}

// GetTransaction returns a transaction by ID.
func (r *FinanceRepo) GetTransaction(ctx context.Context, wsID, txnID uuid.UUID) (*finance.Transaction, error) {
	var t finance.Transaction
	err := r.pool.QueryRow(ctx,
		`SELECT id, workspace_id, date, description, reference_type, reference_id, is_posted, actor_id, created_at
		 FROM transactions WHERE workspace_id = $1 AND id = $2`,
		wsID, txnID,
	).Scan(&t.ID, &t.WorkspaceID, &t.Date, &t.Description, &t.ReferenceType, &t.ReferenceID, &t.IsPosted, &t.ActorID, &t.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("transaction not found")
		}
		return nil, err
	}
	return &t, nil
}

// GetTransactionLines returns lines for a transaction.
func (r *FinanceRepo) GetTransactionLines(ctx context.Context, txnID uuid.UUID) ([]finance.TransactionLine, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, transaction_id, workspace_id, account_id, debit, credit, description, entity_id
		 FROM transaction_lines WHERE transaction_id = $1`,
		txnID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lines []finance.TransactionLine
	for rows.Next() {
		var l finance.TransactionLine
		if err := rows.Scan(&l.ID, &l.TransactionID, &l.WorkspaceID, &l.AccountID, &l.Debit, &l.Credit, &l.Description, &l.EntityID); err != nil {
			return nil, err
		}
		lines = append(lines, l)
	}
	return lines, rows.Err()
}

// ListTransactions returns transactions matching the filter.
func (r *FinanceRepo) ListTransactions(ctx context.Context, wsID uuid.UUID, filter finance.TransactionFilter) ([]finance.Transaction, int, error) {
	filter.Page.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("t.workspace_id = $%d", argIdx))
	args = append(args, wsID)
	argIdx++

	if filter.AccountID != nil {
		conditions = append(conditions, fmt.Sprintf("EXISTS(SELECT 1 FROM transaction_lines tl WHERE tl.transaction_id = t.id AND tl.account_id = $%d)", argIdx))
		args = append(args, *filter.AccountID)
		argIdx++
	}
	if filter.From != nil {
		conditions = append(conditions, fmt.Sprintf("t.date >= $%d", argIdx))
		args = append(args, *filter.From)
		argIdx++
	}
	if filter.To != nil {
		conditions = append(conditions, fmt.Sprintf("t.date <= $%d", argIdx))
		args = append(args, *filter.To)
		argIdx++
	}
	if filter.IsPosted != nil {
		conditions = append(conditions, fmt.Sprintf("t.is_posted = $%d", argIdx))
		args = append(args, *filter.IsPosted)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM transactions t WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT t.id, t.workspace_id, t.date, t.description, t.reference_type, t.reference_id, t.is_posted, t.actor_id, t.created_at
		 FROM transactions t WHERE %s ORDER BY t.date DESC, t.created_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, filter.Page.Limit, filter.Page.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var txns []finance.Transaction
	for rows.Next() {
		var t finance.Transaction
		if err := rows.Scan(&t.ID, &t.WorkspaceID, &t.Date, &t.Description, &t.ReferenceType, &t.ReferenceID, &t.IsPosted, &t.ActorID, &t.CreatedAt); err != nil {
			return nil, 0, err
		}
		txns = append(txns, t)
	}
	return txns, total, rows.Err()
}

// UpdateTransaction updates a transaction record.
func (r *FinanceRepo) UpdateTransaction(ctx context.Context, tx pgx.Tx, t *finance.Transaction) error {
	_, err := tx.Exec(ctx,
		`UPDATE transactions SET is_posted = $3, description = $4 WHERE workspace_id = $1 AND id = $2`,
		t.WorkspaceID, t.ID, t.IsPosted, t.Description,
	)
	return err
}

// CreateInvoice inserts an invoice record.
func (r *FinanceRepo) CreateInvoice(ctx context.Context, inv *finance.Invoice) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO invoices (id, workspace_id, number, type, counterparty_id, order_id, subtotal, tax, total, currency, status, issued_at, due_at, paid_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`,
		inv.ID, inv.WorkspaceID, inv.Number, inv.Type, inv.CounterpartyID, inv.OrderID,
		inv.Subtotal, inv.Tax, inv.Total, inv.Currency, inv.Status, inv.IssuedAt, inv.DueAt, inv.PaidAt, inv.CreatedAt,
	)
	return err
}

// GetInvoice returns an invoice by ID.
func (r *FinanceRepo) GetInvoice(ctx context.Context, wsID, invoiceID uuid.UUID) (*finance.Invoice, error) {
	var inv finance.Invoice
	err := r.pool.QueryRow(ctx,
		`SELECT id, workspace_id, number, type, counterparty_id, order_id, subtotal, tax, total, currency, status, issued_at, due_at, paid_at, created_at
		 FROM invoices WHERE workspace_id = $1 AND id = $2`,
		wsID, invoiceID,
	).Scan(&inv.ID, &inv.WorkspaceID, &inv.Number, &inv.Type, &inv.CounterpartyID, &inv.OrderID,
		&inv.Subtotal, &inv.Tax, &inv.Total, &inv.Currency, &inv.Status, &inv.IssuedAt, &inv.DueAt, &inv.PaidAt, &inv.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, errs.NewNotFound("invoice not found")
		}
		return nil, err
	}
	return &inv, nil
}

// ListInvoices returns invoices matching the filter.
func (r *FinanceRepo) ListInvoices(ctx context.Context, wsID uuid.UUID, filter finance.InvoiceFilter) ([]finance.Invoice, int, error) {
	filter.Page.Normalize()

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, fmt.Sprintf("workspace_id = $%d", argIdx))
	args = append(args, wsID)
	argIdx++

	if filter.Type != nil {
		conditions = append(conditions, fmt.Sprintf("type = $%d", argIdx))
		args = append(args, *filter.Type)
		argIdx++
	}
	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}

	where := strings.Join(conditions, " AND ")

	var total int
	if err := r.pool.QueryRow(ctx, "SELECT COUNT(*) FROM invoices WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		`SELECT id, workspace_id, number, type, counterparty_id, order_id, subtotal, tax, total, currency, status, issued_at, due_at, paid_at, created_at
		 FROM invoices WHERE %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, argIdx, argIdx+1,
	)
	args = append(args, filter.Page.Limit, filter.Page.Offset)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var invoices []finance.Invoice
	for rows.Next() {
		var inv finance.Invoice
		if err := rows.Scan(&inv.ID, &inv.WorkspaceID, &inv.Number, &inv.Type, &inv.CounterpartyID, &inv.OrderID,
			&inv.Subtotal, &inv.Tax, &inv.Total, &inv.Currency, &inv.Status, &inv.IssuedAt, &inv.DueAt, &inv.PaidAt, &inv.CreatedAt); err != nil {
			return nil, 0, err
		}
		invoices = append(invoices, inv)
	}
	return invoices, total, rows.Err()
}

// UpdateInvoice updates an invoice record.
func (r *FinanceRepo) UpdateInvoice(ctx context.Context, inv *finance.Invoice) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE invoices SET status = $3, paid_at = $4 WHERE workspace_id = $1 AND id = $2`,
		inv.WorkspaceID, inv.ID, inv.Status, inv.PaidAt,
	)
	return err
}

// GetTrialBalance returns the trial balance as of a date.
func (r *FinanceRepo) GetTrialBalance(ctx context.Context, wsID uuid.UUID, date time.Time) ([]finance.TrialBalanceRow, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT a.id, a.code, a.name,
		        COALESCE(SUM(tl.debit), 0) AS debit,
		        COALESCE(SUM(tl.credit), 0) AS credit
		 FROM accounts a
		 LEFT JOIN transaction_lines tl ON tl.account_id = a.id AND tl.workspace_id = a.workspace_id
		 LEFT JOIN transactions t ON t.id = tl.transaction_id AND t.is_posted = TRUE AND t.date <= $2
		 WHERE a.workspace_id = $1
		 GROUP BY a.id, a.code, a.name
		 ORDER BY a.code`,
		wsID, date,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []finance.TrialBalanceRow
	for rows.Next() {
		var row finance.TrialBalanceRow
		if err := rows.Scan(&row.AccountID, &row.Code, &row.Name, &row.Debit, &row.Credit); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

// GetAccountBalance returns the balance for a single account in a date range.
func (r *FinanceRepo) GetAccountBalance(ctx context.Context, wsID, accountID uuid.UUID, from, to time.Time) (*finance.AccountBalance, error) {
	var bal finance.AccountBalance
	bal.AccountID = accountID
	err := r.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(tl.debit), 0), COALESCE(SUM(tl.credit), 0)
		 FROM transaction_lines tl
		 JOIN transactions t ON t.id = tl.transaction_id AND t.is_posted = TRUE
		 WHERE tl.workspace_id = $1 AND tl.account_id = $2 AND t.date >= $3 AND t.date <= $4`,
		wsID, accountID, from, to,
	).Scan(&bal.DebitTotal, &bal.CreditTotal)
	if err != nil {
		return nil, err
	}
	bal.Balance = bal.DebitTotal - bal.CreditTotal
	return &bal, nil
}

// WithTx executes fn within a transaction.
func (r *FinanceRepo) WithTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
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
