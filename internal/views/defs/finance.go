package defs

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/views"
)

// FinanceTrialBalance returns the trial balance (оборотно-сальдовая ведомость).
var FinanceTrialBalance = &views.ViewDef{
	Key: "finance_trial_balance",
	Tables: []views.TableDep{
		{Table: "accounts", Columns: []string{"code", "name", "type"}},
		{Table: "transactions", Columns: []string{"is_posted"}},
	},
	Factory: financeTrialBalanceFactory,
}

func financeTrialBalanceFactory(ctx context.Context, pool *pgxpool.Pool, wsID uuid.UUID, _ map[string]any) (*views.ViewResult, error) {
	rows, err := pool.Query(ctx, `
		SELECT a.id, a.code, a.name, a.type,
		       COALESCE(SUM(tl.debit), 0) AS total_debit,
		       COALESCE(SUM(tl.credit), 0) AS total_credit
		FROM accounts a
		LEFT JOIN transaction_lines tl ON tl.account_id = a.id AND tl.workspace_id = $1
		LEFT JOIN transactions t ON t.id = tl.transaction_id AND t.is_posted = TRUE
		WHERE a.workspace_id = $1
		GROUP BY a.id, a.code, a.name, a.type
		ORDER BY a.code
	`, wsID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]views.DataRef, 0)
	tables := map[string]map[string]any{"accounts": {}}

	for rows.Next() {
		var id, code, name, acctType string
		var totalDebit, totalCredit int64

		if err := rows.Scan(&id, &code, &name, &acctType, &totalDebit, &totalCredit); err != nil {
			return nil, err
		}

		refs = append(refs, views.DataRef{Table: "accounts", ID: id, Fields: []string{"code", "name", "type", "debit", "credit", "balance"}})
		tables["accounts"][id] = map[string]any{
			"id":      id,
			"code":    code,
			"name":    name,
			"type":    acctType,
			"debit":   totalDebit,
			"credit":  totalCredit,
			"balance": totalDebit - totalCredit,
		}
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}

// FinanceTransactionsList returns posted transactions.
var FinanceTransactionsList = &views.ViewDef{
	Key: "finance_transactions_list",
	Tables: []views.TableDep{
		{Table: "transactions", Columns: []string{"date", "description", "is_posted"}},
	},
	ParamSchema: map[string]string{
		"account_id": "uuid",
	},
	Factory: financeTransactionsListFactory,
}

func financeTransactionsListFactory(ctx context.Context, pool *pgxpool.Pool, wsID uuid.UUID, params map[string]any) (*views.ViewResult, error) {
	limit := intParam(params, "limit", 50)
	offset := intParam(params, "offset", 0)
	accountID, _ := params["account_id"].(string)

	query := `
		SELECT t.id, t.date, t.description, t.is_posted, t.created_at,
		       COALESCE(SUM(tl.debit), 0) AS total_debit,
		       COALESCE(SUM(tl.credit), 0) AS total_credit,
		       COUNT(*) OVER() AS total_count
		FROM transactions t
		JOIN transaction_lines tl ON tl.transaction_id = t.id AND tl.workspace_id = $1
		WHERE t.workspace_id = $1
	`
	args := []any{wsID}
	argIdx := 2

	if accountID != "" {
		query += fmt.Sprintf(` AND tl.account_id = $%d`, argIdx)
		args = append(args, accountID)
		argIdx++
	}

	query += fmt.Sprintf(` GROUP BY t.id ORDER BY t.date DESC, t.created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	refs := make([]views.DataRef, 0)
	tables := map[string]map[string]any{"transactions": {}}

	for rows.Next() {
		var id, description string
		var date, createdAt any
		var isPosted bool
		var totalDebit, totalCredit int64
		var cnt int

		if err := rows.Scan(&id, &date, &description, &isPosted, &createdAt, &totalDebit, &totalCredit, &cnt); err != nil {
			return nil, err
		}

		refs = append(refs, views.DataRef{Table: "transactions", ID: id, Fields: []string{"date", "description", "debit", "credit"}})
		tables["transactions"][id] = map[string]any{
			"id":          id,
			"date":        date,
			"description": description,
			"is_posted":   isPosted,
			"debit":       totalDebit,
			"credit":      totalCredit,
			"created_at":  createdAt,
		}
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}

// FinanceAccountBalance returns the balance for a specific account.
var FinanceAccountBalance = &views.ViewDef{
	Key: "finance_account_balance",
	Tables: []views.TableDep{
		{Table: "accounts", Columns: []string{"code", "name"}},
		{Table: "transactions", Columns: []string{"is_posted"}},
	},
	ParamSchema: map[string]string{
		"account_id": "uuid",
	},
	Factory: financeAccountBalanceFactory,
}

func financeAccountBalanceFactory(ctx context.Context, pool *pgxpool.Pool, wsID uuid.UUID, params map[string]any) (*views.ViewResult, error) {
	accountID, _ := params["account_id"].(string)

	var id, code, name, acctType string
	var totalDebit, totalCredit int64

	err := pool.QueryRow(ctx, `
		SELECT a.id, a.code, a.name, a.type,
		       COALESCE(SUM(tl.debit), 0),
		       COALESCE(SUM(tl.credit), 0)
		FROM accounts a
		LEFT JOIN transaction_lines tl ON tl.account_id = a.id AND tl.workspace_id = $1
		LEFT JOIN transactions t ON t.id = tl.transaction_id AND t.is_posted = TRUE
		WHERE a.workspace_id = $1 AND a.id = $2
		GROUP BY a.id
	`, wsID, accountID).Scan(&id, &code, &name, &acctType, &totalDebit, &totalCredit)
	if err != nil {
		return nil, err
	}

	refs := []views.DataRef{{Table: "accounts", ID: id, Fields: []string{"code", "name", "balance"}}}
	tables := map[string]map[string]any{
		"accounts": {
			id: map[string]any{
				"id":      id,
				"code":    code,
				"name":    name,
				"type":    acctType,
				"debit":   totalDebit,
				"credit":  totalCredit,
				"balance": totalDebit - totalCredit,
			},
		},
	}

	return &views.ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
}
