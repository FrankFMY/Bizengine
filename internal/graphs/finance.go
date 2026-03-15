package graphs

import (
	"context"
	"fmt"

	"github.com/FrankFMY/arcana"
)

var financeTrialBalance = arcana.GraphDef{
	Key: "finance_trial_balance",
	Deps: []arcana.TableDep{
		{Table: "accounts", Columns: []string{"code", "name", "type"}},
		{Table: "transactions", Columns: []string{"is_posted"}},
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)

		rows, err := q.Query(ctx, `
			SELECT a.id, a.code, a.name, a.type,
			       COALESCE(SUM(tl.debit), 0) AS total_debit,
			       COALESCE(SUM(tl.credit), 0) AS total_credit
			FROM accounts a
			LEFT JOIN transaction_lines tl ON tl.account_id = a.id AND tl.organization_id = $1
			LEFT JOIN transactions t ON t.id = tl.transaction_id AND t.is_posted = TRUE
			WHERE a.organization_id = $1
			GROUP BY a.id, a.code, a.name, a.type
			ORDER BY a.code
		`, orgID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()

		for rows.Next() {
			var id, code, name, acctType string
			var totalDebit, totalCredit int64

			if err := rows.Scan(&id, &code, &name, &acctType, &totalDebit, &totalCredit); err != nil {
				return nil, err
			}

			result.AddRef(arcana.Ref{Table: "accounts", ID: id, Fields: []string{"code", "name", "type", "debit", "credit", "balance"}})
			result.AddRow("accounts", id, map[string]any{
				"id":      id,
				"code":    code,
				"name":    name,
				"type":    acctType,
				"debit":   totalDebit,
				"credit":  totalCredit,
				"balance": totalDebit - totalCredit,
			})
		}

		return result, rows.Err()
	},
}

var financeTransactionsList = arcana.GraphDef{
	Key: "finance_transactions_list",
	Deps: []arcana.TableDep{
		{Table: "transactions", Columns: []string{"date", "description", "is_posted"}},
	},
	Params: arcana.ParamSchema{
		"account_id": arcana.ParamUUID().Build(),
		"limit":      arcana.ParamInt().Default(50),
		"offset":     arcana.ParamInt().Default(0),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		limit := p.Int("limit")
		offset := p.Int("offset")
		accountID := p.UUID("account_id")

		query := `
			SELECT t.id, t.date, t.description, t.is_posted, t.created_at,
			       COALESCE(SUM(tl.debit), 0) AS total_debit,
			       COALESCE(SUM(tl.credit), 0) AS total_credit,
			       COUNT(*) OVER() AS total_count
			FROM transactions t
			JOIN transaction_lines tl ON tl.transaction_id = t.id AND tl.organization_id = $1
			WHERE t.organization_id = $1
		`
		args := []any{orgID}
		argIdx := 2

		if accountID != "" {
			query += fmt.Sprintf(` AND tl.account_id = $%d`, argIdx)
			args = append(args, accountID)
			argIdx++
		}

		query += fmt.Sprintf(` GROUP BY t.id ORDER BY t.date DESC, t.created_at DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
		args = append(args, limit, offset)

		rows, err := q.Query(ctx, query, args...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()

		for rows.Next() {
			var id, description string
			var date, createdAt any
			var isPosted bool
			var totalDebit, totalCredit int64
			var cnt int

			if err := rows.Scan(&id, &date, &description, &isPosted, &createdAt, &totalDebit, &totalCredit, &cnt); err != nil {
				return nil, err
			}

			result.AddRef(arcana.Ref{Table: "transactions", ID: id, Fields: []string{"date", "description", "debit", "credit"}})
			result.AddRow("transactions", id, map[string]any{
				"id":          id,
				"date":        date,
				"description": description,
				"is_posted":   isPosted,
				"debit":       totalDebit,
				"credit":      totalCredit,
				"created_at":  createdAt,
			})
		}

		return result, rows.Err()
	},
}

var financeAccountBalance = arcana.GraphDef{
	Key: "finance_account_balance",
	Deps: []arcana.TableDep{
		{Table: "accounts", Columns: []string{"code", "name"}},
		{Table: "transactions", Columns: []string{"is_posted"}},
	},
	Params: arcana.ParamSchema{
		"account_id": arcana.ParamUUID().Required(),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		accountID := p.UUID("account_id")

		var id, code, name, acctType string
		var totalDebit, totalCredit int64

		err := q.QueryRow(ctx, `
			SELECT a.id, a.code, a.name, a.type,
			       COALESCE(SUM(tl.debit), 0),
			       COALESCE(SUM(tl.credit), 0)
			FROM accounts a
			LEFT JOIN transaction_lines tl ON tl.account_id = a.id AND tl.organization_id = $1
			LEFT JOIN transactions t ON t.id = tl.transaction_id AND t.is_posted = TRUE
			WHERE a.organization_id = $1 AND a.id = $2
			GROUP BY a.id
		`, orgID, accountID).Scan(&id, &code, &name, &acctType, &totalDebit, &totalCredit)
		if err != nil {
			return nil, err
		}

		result := arcana.NewResult()
		result.AddRef(arcana.Ref{Table: "accounts", ID: id, Fields: []string{"code", "name", "balance"}})
		result.AddRow("accounts", id, map[string]any{
			"id":      id,
			"code":    code,
			"name":    name,
			"type":    acctType,
			"debit":   totalDebit,
			"credit":  totalCredit,
			"balance": totalDebit - totalCredit,
		})

		return result, nil
	},
}

var financePnL = arcana.GraphDef{
	Key: "finance_pnl",
	Deps: []arcana.TableDep{
		{Table: "accounts", Columns: []string{"code", "name", "type"}},
		{Table: "transactions", Columns: []string{"is_posted", "date"}},
	},
	Params: arcana.ParamSchema{
		"from": arcana.ParamString().Required(),
		"to":   arcana.ParamString().Required(),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		from := p.String("from")
		to := p.String("to")

		rows, err := q.Query(ctx, `
			SELECT a.id, a.code, a.name, a.type,
			       COALESCE(SUM(tl.debit), 0), COALESCE(SUM(tl.credit), 0)
			FROM accounts a
			LEFT JOIN transaction_lines tl ON tl.account_id = a.id AND tl.organization_id = $1
			LEFT JOIN transactions t ON t.id = tl.transaction_id AND t.is_posted = TRUE AND t.date >= $2 AND t.date <= $3
			WHERE a.organization_id = $1 AND a.type IN ('revenue', 'expense')
			GROUP BY a.id, a.code, a.name, a.type
			ORDER BY a.code
		`, orgID, from, to)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()
		var totalRevenue, totalExpense int64

		for rows.Next() {
			var id, code, name, acctType string
			var debit, credit int64
			if err := rows.Scan(&id, &code, &name, &acctType, &debit, &credit); err != nil {
				return nil, err
			}
			var amount int64
			if acctType == "revenue" {
				amount = credit - debit
				totalRevenue += amount
			} else {
				amount = debit - credit
				totalExpense += amount
			}
			result.AddRef(arcana.Ref{Table: "accounts", ID: id, Fields: []string{"code", "name", "amount"}})
			result.AddRow("accounts", id, map[string]any{
				"id": id, "code": code, "name": name, "type": acctType, "amount": amount,
			})
		}

		result.AddRow("summary", "pnl", map[string]any{
			"total_revenue": totalRevenue, "total_expense": totalExpense, "net_profit": totalRevenue - totalExpense,
		})
		return result, rows.Err()
	},
}

var financePeriodsList = arcana.GraphDef{
	Key: "finance_periods_list",
	Deps: []arcana.TableDep{
		{Table: "finance_periods", Columns: []string{"status", "closed_at"}},
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)

		rows, err := q.Query(ctx, `
			SELECT year, month, status, closed_at, closed_by
			FROM finance_periods WHERE organization_id = $1
			ORDER BY year DESC, month DESC
		`, orgID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()
		for rows.Next() {
			var year, month int
			var status string
			var closedAt, closedBy any
			if err := rows.Scan(&year, &month, &status, &closedAt, &closedBy); err != nil {
				return nil, err
			}
			id := fmt.Sprintf("%d-%02d", year, month)
			result.AddRef(arcana.Ref{Table: "finance_periods", ID: id, Fields: []string{"status"}})
			result.AddRow("finance_periods", id, map[string]any{
				"year": year, "month": month, "status": status, "closed_at": closedAt, "closed_by": closedBy,
			})
		}
		return result, rows.Err()
	},
}

var financeCashOperations = arcana.GraphDef{
	Key: "finance_cash_operations",
	Deps: []arcana.TableDep{
		{Table: "cash_operations", Columns: []string{"type", "amount", "created_at"}},
	},
	Params: arcana.ParamSchema{
		"limit":  arcana.ParamInt().Default(50),
		"offset": arcana.ParamInt().Default(0),
	},
	Factory: func(ctx context.Context, q arcana.Querier, p arcana.Params) (*arcana.Result, error) {
		orgID := arcana.WorkspaceID(ctx)
		limit := p.Int("limit")
		offset := p.Int("offset")

		rows, err := q.Query(ctx, `
			SELECT id, type, amount, description, account_code, created_at,
			       COUNT(*) OVER() AS total_count
			FROM cash_operations WHERE organization_id = $1
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`, orgID, limit, offset)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		result := arcana.NewResult()
		for rows.Next() {
			var id, opType, desc, accountCode string
			var amount int64
			var createdAt any
			var total int
			if err := rows.Scan(&id, &opType, &amount, &desc, &accountCode, &createdAt, &total); err != nil {
				return nil, err
			}
			result.SetTotal(total)
			result.AddRef(arcana.Ref{Table: "cash_operations", ID: id, Fields: []string{"type", "amount"}})
			result.AddRow("cash_operations", id, map[string]any{
				"id": id, "type": opType, "amount": amount, "description": desc,
				"account_code": accountCode, "created_at": createdAt,
			})
		}
		return result, rows.Err()
	},
}
