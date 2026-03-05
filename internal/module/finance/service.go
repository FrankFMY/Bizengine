package finance

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
	"github.com/jackc/pgx/v5"
)

// Service provides finance operations.
type Service struct {
	repo Repository
	bus  event.Bus
}

// NewService creates a new finance service.
func NewService(repo Repository, bus event.Bus) *Service {
	return &Service{repo: repo, bus: bus}
}

// SeedDefaultAccounts creates the default chart of accounts for an organization.
func (s *Service) SeedDefaultAccounts(ctx context.Context, orgID uuid.UUID) error {
	return s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		for _, input := range DefaultAccounts() {
			acct := &Account{
				ID:             uuid.New(),
				OrganizationID: orgID,
				Code:           input.Code,
				Name:           input.Name,
				Type:           input.Type,
				IsSystem:       true,
				Currency:       "RUB",
				CreatedAt:      time.Now(),
			}
			if err := s.repo.CreateAccount(ctx, tx, acct); err != nil {
				return err
			}
		}
		return nil
	})
}

// CreateAccount creates a new account.
func (s *Service) CreateAccount(ctx context.Context, orgID uuid.UUID, input CreateAccountInput) (*Account, error) {
	if input.Code == "" {
		return nil, errs.NewBadRequest("code is required")
	}
	if input.Name == "" {
		return nil, errs.NewBadRequest("name is required")
	}
	validTypes := map[string]bool{"asset": true, "liability": true, "equity": true, "revenue": true, "expense": true}
	if !validTypes[input.Type] {
		return nil, errs.NewBadRequest("type must be asset, liability, equity, revenue, or expense")
	}

	if input.Currency == "" {
		input.Currency = "RUB"
	}

	acct := &Account{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Code:           input.Code,
		Name:           input.Name,
		Type:           input.Type,
		ParentID:       input.ParentID,
		Currency:       input.Currency,
		CreatedAt:      time.Now(),
	}

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		return s.repo.CreateAccount(ctx, tx, acct)
	}); err != nil {
		return nil, err
	}
	return acct, nil
}

// ListAccounts returns all accounts for an organization.
func (s *Service) ListAccounts(ctx context.Context, orgID uuid.UUID) ([]Account, error) {
	return s.repo.ListAccounts(ctx, orgID)
}

// DeleteAccount deletes an account if it's not system and has no lines.
func (s *Service) DeleteAccount(ctx context.Context, orgID, accountID uuid.UUID) error {
	acct, err := s.repo.GetAccount(ctx, orgID, accountID)
	if err != nil {
		return err
	}
	if acct.IsSystem {
		return errs.NewConflict("cannot delete system account")
	}
	hasLines, err := s.repo.HasTransactionLines(ctx, orgID, accountID)
	if err != nil {
		return err
	}
	if hasLines {
		return errs.NewConflict("cannot delete account with existing transaction lines")
	}
	return s.repo.DeleteAccount(ctx, orgID, accountID)
}

// CreateTransaction creates a new journal entry with double-entry validation.
func (s *Service) CreateTransaction(ctx context.Context, orgID uuid.UUID, input CreateTransactionInput, actorID *uuid.UUID) (*Transaction, error) {
	if len(input.Lines) < 2 {
		return nil, errs.NewBadRequest("at least two lines are required")
	}

	date, err := time.Parse("2006-01-02", input.Date)
	if err != nil {
		return nil, errs.NewBadRequest("invalid date format, expected YYYY-MM-DD")
	}

	// Check period is open
	open, err := s.IsPeriodOpen(ctx, orgID, date)
	if err != nil {
		return nil, err
	}
	if !open {
		return nil, errs.NewConflict("accounting period is closed for " + date.Format("2006-01"))
	}

	var totalDebit, totalCredit int64
	for _, line := range input.Lines {
		if line.Debit < 0 || line.Credit < 0 {
			return nil, errs.NewBadRequest("debit and credit must be non-negative")
		}
		if line.Debit > 0 && line.Credit > 0 {
			return nil, errs.NewBadRequest("a line cannot have both debit and credit")
		}
		if line.Debit == 0 && line.Credit == 0 {
			return nil, errs.NewBadRequest("a line must have either debit or credit")
		}
		totalDebit += line.Debit
		totalCredit += line.Credit
	}

	if totalDebit != totalCredit {
		return nil, errs.NewBadRequest("sum of debits must equal sum of credits")
	}

	txn := &Transaction{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Date:           date,
		Description:    input.Description,
		ReferenceType:  input.ReferenceType,
		ReferenceID:    input.ReferenceID,
		ActorID:        actorID,
		CreatedAt:      time.Now(),
	}

	var lines []TransactionLine
	for _, lineInput := range input.Lines {
		lines = append(lines, TransactionLine{
			ID:             uuid.New(),
			TransactionID:  txn.ID,
			OrganizationID: orgID,
			AccountID:      lineInput.AccountID,
			Debit:          lineInput.Debit,
			Credit:         lineInput.Credit,
			Description:    lineInput.Description,
			EntityID:       lineInput.EntityID,
		})
	}

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.CreateTransaction(ctx, tx, txn); err != nil {
			return err
		}
		return s.repo.CreateTransactionLines(ctx, tx, lines)
	}); err != nil {
		return nil, err
	}

	txn.Lines = lines

	s.publishEvent(ctx, orgID, "finance.transaction.created", map[string]any{
		"transaction_id": txn.ID.String(),
		"date":           input.Date,
		"total":          totalDebit,
	}, actorID)

	return txn, nil
}

// GetTransaction returns a transaction with its lines.
func (s *Service) GetTransaction(ctx context.Context, orgID, txnID uuid.UUID) (*Transaction, error) {
	txn, err := s.repo.GetTransaction(ctx, orgID, txnID)
	if err != nil {
		return nil, err
	}
	lines, err := s.repo.GetTransactionLines(ctx, txnID)
	if err != nil {
		return nil, err
	}
	txn.Lines = lines
	return txn, nil
}

// ListTransactions returns transactions matching the filter.
func (s *Service) ListTransactions(ctx context.Context, orgID uuid.UUID, filter TransactionFilter) ([]Transaction, int, error) {
	filter.Page.Normalize()
	return s.repo.ListTransactions(ctx, orgID, filter)
}

// PostTransaction marks a transaction as posted (irreversible).
func (s *Service) PostTransaction(ctx context.Context, orgID, txnID uuid.UUID, actorID *uuid.UUID) error {
	txn, err := s.repo.GetTransaction(ctx, orgID, txnID)
	if err != nil {
		return err
	}
	if txn.IsPosted {
		return errs.NewConflict("transaction is already posted")
	}

	txn.IsPosted = true
	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		return s.repo.UpdateTransaction(ctx, tx, txn)
	}); err != nil {
		return err
	}

	s.publishEvent(ctx, orgID, "finance.transaction.posted", map[string]any{
		"transaction_id": txnID.String(),
	}, actorID)
	return nil
}

// CreateInvoice creates a new invoice.
func (s *Service) CreateInvoice(ctx context.Context, orgID uuid.UUID, input CreateInvoiceInput) (*Invoice, error) {
	if input.Number == "" {
		return nil, errs.NewBadRequest("number is required")
	}
	if input.Type != "incoming" && input.Type != "outgoing" {
		return nil, errs.NewBadRequest("type must be incoming or outgoing")
	}

	if input.Currency == "" {
		input.Currency = "RUB"
	}

	inv := &Invoice{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Number:         input.Number,
		Type:           input.Type,
		CounterpartyID: input.CounterpartyID,
		OrderID:        input.OrderID,
		Subtotal:       input.Subtotal,
		Tax:            input.Tax,
		Total:          input.Total,
		Currency:       input.Currency,
		Status:         "draft",
		IssuedAt:       input.IssuedAt,
		DueAt:          input.DueAt,
		CreatedAt:      time.Now(),
	}

	if err := s.repo.CreateInvoice(ctx, inv); err != nil {
		return nil, err
	}
	return inv, nil
}

// ListInvoices returns invoices matching the filter.
func (s *Service) ListInvoices(ctx context.Context, orgID uuid.UUID, filter InvoiceFilter) ([]Invoice, int, error) {
	filter.Page.Normalize()
	return s.repo.ListInvoices(ctx, orgID, filter)
}

// MarkInvoicePaid marks an invoice as paid.
func (s *Service) MarkInvoicePaid(ctx context.Context, orgID, invoiceID uuid.UUID) error {
	inv, err := s.repo.GetInvoice(ctx, orgID, invoiceID)
	if err != nil {
		return err
	}
	if inv.Status == "paid" {
		return errs.NewConflict("invoice is already paid")
	}

	now := time.Now()
	inv.Status = "paid"
	inv.PaidAt = &now
	return s.repo.UpdateInvoice(ctx, inv)
}

// GetTrialBalance returns the trial balance as of a date.
func (s *Service) GetTrialBalance(ctx context.Context, orgID uuid.UUID, date time.Time) ([]TrialBalanceRow, error) {
	return s.repo.GetTrialBalance(ctx, orgID, date)
}

// GetAccountBalance returns the balance for a single account.
func (s *Service) GetAccountBalance(ctx context.Context, orgID, accountID uuid.UUID, from, to time.Time) (*AccountBalance, error) {
	return s.repo.GetAccountBalance(ctx, orgID, accountID, from, to)
}

// CreateAutoTransaction creates a transaction from an event (for event subscriptions).
func (s *Service) CreateAutoTransaction(ctx context.Context, orgID uuid.UUID, date string, description string, debitCode, creditCode string, amount int64, refType *string, refID *uuid.UUID) error {
	debitAcct, err := s.repo.GetAccountByCode(ctx, orgID, debitCode)
	if err != nil {
		return err
	}
	creditAcct, err := s.repo.GetAccountByCode(ctx, orgID, creditCode)
	if err != nil {
		return err
	}

	_, err = s.CreateTransaction(ctx, orgID, CreateTransactionInput{
		Date:          date,
		Description:   description,
		ReferenceType: refType,
		ReferenceID:   refID,
		Lines: []CreateTransactionLineInput{
			{AccountID: debitAcct.ID, Debit: amount},
			{AccountID: creditAcct.ID, Credit: amount},
		},
	}, nil)
	return err
}

// ClosePeriod closes a monthly accounting period, preventing new transactions.
func (s *Service) ClosePeriod(ctx context.Context, orgID uuid.UUID, year, month int, actorID *uuid.UUID) (*FinancePeriod, error) {
	if month < 1 || month > 12 {
		return nil, errs.NewBadRequest("month must be between 1 and 12")
	}

	p, err := s.repo.GetPeriod(ctx, orgID, year, month)
	if err != nil {
		// Period doesn't exist yet — create it as closed
		p = &FinancePeriod{
			OrganizationID: orgID,
			Year:           year,
			Month:          month,
		}
	}
	if p.Status == "closed" {
		return nil, errs.NewConflict("period is already closed")
	}

	now := time.Now()
	p.Status = "closed"
	p.ClosedAt = &now
	p.ClosedBy = actorID

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		return s.repo.UpsertPeriod(ctx, tx, p)
	}); err != nil {
		return nil, err
	}

	s.publishEvent(ctx, orgID, "finance.period.closed", map[string]any{
		"year":  year,
		"month": month,
	}, actorID)

	return p, nil
}

// ReopenPeriod reopens a closed period.
func (s *Service) ReopenPeriod(ctx context.Context, orgID uuid.UUID, year, month int, actorID *uuid.UUID) (*FinancePeriod, error) {
	p, err := s.repo.GetPeriod(ctx, orgID, year, month)
	if err != nil {
		return nil, errs.NewNotFound("period not found")
	}
	if p.Status != "closed" {
		return nil, errs.NewConflict("period is not closed")
	}

	p.Status = "open"
	p.ClosedAt = nil
	p.ClosedBy = nil

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		return s.repo.UpsertPeriod(ctx, tx, p)
	}); err != nil {
		return nil, err
	}

	return p, nil
}

// ListPeriods returns all finance periods for an organization.
func (s *Service) ListPeriods(ctx context.Context, orgID uuid.UUID) ([]FinancePeriod, error) {
	return s.repo.ListPeriods(ctx, orgID)
}

// IsPeriodOpen checks whether a transaction date falls in an open period.
func (s *Service) IsPeriodOpen(ctx context.Context, orgID uuid.UUID, date time.Time) (bool, error) {
	p, err := s.repo.GetPeriod(ctx, orgID, date.Year(), int(date.Month()))
	if err != nil {
		// No period record means open by default
		return true, nil
	}
	return p.Status != "closed", nil
}

// GetProfitAndLoss returns revenue and expense totals for a date range.
func (s *Service) GetProfitAndLoss(ctx context.Context, orgID uuid.UUID, from, to time.Time) (*PnLReport, error) {
	rows, err := s.repo.GetProfitAndLoss(ctx, orgID, from, to)
	if err != nil {
		return nil, err
	}

	var totalRevenue, totalExpense int64
	for _, row := range rows {
		switch row.Type {
		case "revenue":
			totalRevenue += row.Amount
		case "expense":
			totalExpense += row.Amount
		}
	}

	return &PnLReport{
		From:         from,
		To:           to,
		Rows:         rows,
		TotalRevenue: totalRevenue,
		TotalExpense: totalExpense,
		NetProfit:    totalRevenue - totalExpense,
	}, nil
}

// CreateCashOperation records a cash register deposit or withdrawal.
func (s *Service) CreateCashOperation(ctx context.Context, orgID uuid.UUID, input CreateCashOperationInput, actorID *uuid.UUID) (*CashOperation, error) {
	if input.Type != "deposit" && input.Type != "withdrawal" {
		return nil, errs.NewBadRequest("type must be deposit or withdrawal")
	}
	if input.Amount <= 0 {
		return nil, errs.NewBadRequest("amount must be positive")
	}
	if input.AccountCode == "" {
		input.AccountCode = "50" // default to cash account
	}

	date := time.Now()
	open, err := s.IsPeriodOpen(ctx, orgID, date)
	if err != nil {
		return nil, err
	}
	if !open {
		return nil, errs.NewConflict("accounting period is closed for " + date.Format("2006-01"))
	}

	op := &CashOperation{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Type:           input.Type,
		Amount:         input.Amount,
		Description:    input.Description,
		AccountCode:    input.AccountCode,
		ActorID:        actorID,
		CreatedAt:      time.Now(),
	}

	// Create double-entry transaction: deposit = Dt 50 Ct {counterpart}, withdrawal = reverse
	debitCode := "50"
	creditCode := input.AccountCode
	if input.Type == "withdrawal" {
		debitCode = input.AccountCode
		creditCode = "50"
	}
	// If counterpart is cash itself, use "91" (other income/expense)
	if debitCode == creditCode {
		if input.Type == "deposit" {
			creditCode = "91"
		} else {
			debitCode = "91"
		}
	}

	if err := s.repo.WithTx(ctx, func(tx pgx.Tx) error {
		if err := s.repo.CreateCashOperation(ctx, tx, op); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Also create accounting transaction
	_ = s.CreateAutoTransaction(ctx, orgID, op.CreatedAt.Format("2006-01-02"), "Cash: "+input.Description, debitCode, creditCode, input.Amount, nil, nil)

	s.publishEvent(ctx, orgID, "finance.cash."+input.Type, map[string]any{
		"operation_id": op.ID,
		"amount":       input.Amount,
		"type":         input.Type,
	}, actorID)

	return op, nil
}

// ListCashOperations returns cash operations matching the filter.
func (s *Service) ListCashOperations(ctx context.Context, orgID uuid.UUID, filter CashOperationFilter) (*types.PageResponse[CashOperation], error) {
	filter.Page.Normalize()
	ops, total, err := s.repo.ListCashOperations(ctx, orgID, filter)
	if err != nil {
		return nil, err
	}
	if ops == nil {
		ops = []CashOperation{}
	}
	return &types.PageResponse[CashOperation]{
		Items:  ops,
		Total:  total,
		Limit:  filter.Page.Limit,
		Offset: filter.Page.Offset,
	}, nil
}

func (s *Service) publishEvent(ctx context.Context, orgID uuid.UUID, eventType string, data map[string]any, actorID *uuid.UUID) {
	payload, _ := json.Marshal(data)
	ev := types.Event{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Type:           eventType,
		Data:           payload,
		Timestamp:      time.Now(),
	}
	if actorID != nil {
		ev.ActorID = actorID
	}
	s.bus.Publish(ctx, ev)
}
