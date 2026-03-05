package finance

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/types"
)

// --- mocks ---

type mockFinanceRepo struct {
	accounts     map[uuid.UUID]*Account
	accountCodes map[string]*Account
	transactions map[uuid.UUID]*Transaction
	lines        map[uuid.UUID][]TransactionLine
	invoices     map[uuid.UUID]*Invoice
	hasLines     map[uuid.UUID]bool
	periods      map[string]*FinancePeriod // key: "orgID:year:month"
	cashOps      []CashOperation
}

func newMockFinanceRepo() *mockFinanceRepo {
	return &mockFinanceRepo{
		accounts:     make(map[uuid.UUID]*Account),
		accountCodes: make(map[string]*Account),
		transactions: make(map[uuid.UUID]*Transaction),
		lines:        make(map[uuid.UUID][]TransactionLine),
		invoices:     make(map[uuid.UUID]*Invoice),
		hasLines:     make(map[uuid.UUID]bool),
		periods:      make(map[string]*FinancePeriod),
	}
}

func (m *mockFinanceRepo) CreateAccount(_ context.Context, _ pgx.Tx, a *Account) error {
	cp := *a
	m.accounts[a.ID] = &cp
	key := a.OrganizationID.String() + ":" + a.Code
	m.accountCodes[key] = &cp
	return nil
}

func (m *mockFinanceRepo) GetAccount(_ context.Context, orgID, accountID uuid.UUID) (*Account, error) {
	a, ok := m.accounts[accountID]
	if !ok || a.OrganizationID != orgID {
		return nil, assert.AnError
	}
	cp := *a
	return &cp, nil
}

func (m *mockFinanceRepo) GetAccountByCode(_ context.Context, orgID uuid.UUID, code string) (*Account, error) {
	key := orgID.String() + ":" + code
	a, ok := m.accountCodes[key]
	if !ok {
		return nil, assert.AnError
	}
	cp := *a
	return &cp, nil
}

func (m *mockFinanceRepo) ListAccounts(_ context.Context, orgID uuid.UUID) ([]Account, error) {
	var result []Account
	for _, a := range m.accounts {
		if a.OrganizationID == orgID {
			result = append(result, *a)
		}
	}
	return result, nil
}

func (m *mockFinanceRepo) DeleteAccount(_ context.Context, orgID, accountID uuid.UUID) error {
	if a, ok := m.accounts[accountID]; ok && a.OrganizationID == orgID {
		key := orgID.String() + ":" + a.Code
		delete(m.accountCodes, key)
		delete(m.accounts, accountID)
		return nil
	}
	return assert.AnError
}

func (m *mockFinanceRepo) HasTransactionLines(_ context.Context, _ uuid.UUID, accountID uuid.UUID) (bool, error) {
	return m.hasLines[accountID], nil
}

func (m *mockFinanceRepo) CreateTransaction(_ context.Context, _ pgx.Tx, t *Transaction) error {
	cp := *t
	m.transactions[t.ID] = &cp
	return nil
}

func (m *mockFinanceRepo) CreateTransactionLines(_ context.Context, _ pgx.Tx, lines []TransactionLine) error {
	if len(lines) == 0 {
		return nil
	}
	txnID := lines[0].TransactionID
	m.lines[txnID] = append(m.lines[txnID], lines...)
	return nil
}

func (m *mockFinanceRepo) GetTransaction(_ context.Context, orgID, txnID uuid.UUID) (*Transaction, error) {
	t, ok := m.transactions[txnID]
	if !ok || t.OrganizationID != orgID {
		return nil, assert.AnError
	}
	cp := *t
	return &cp, nil
}

func (m *mockFinanceRepo) GetTransactionLines(_ context.Context, txnID uuid.UUID) ([]TransactionLine, error) {
	return m.lines[txnID], nil
}

func (m *mockFinanceRepo) ListTransactions(_ context.Context, orgID uuid.UUID, _ TransactionFilter) ([]Transaction, int, error) {
	var result []Transaction
	for _, t := range m.transactions {
		if t.OrganizationID == orgID {
			result = append(result, *t)
		}
	}
	return result, len(result), nil
}

func (m *mockFinanceRepo) UpdateTransaction(_ context.Context, _ pgx.Tx, t *Transaction) error {
	cp := *t
	m.transactions[t.ID] = &cp
	return nil
}

func (m *mockFinanceRepo) CreateInvoice(_ context.Context, inv *Invoice) error {
	cp := *inv
	m.invoices[inv.ID] = &cp
	return nil
}

func (m *mockFinanceRepo) GetInvoice(_ context.Context, orgID, invoiceID uuid.UUID) (*Invoice, error) {
	inv, ok := m.invoices[invoiceID]
	if !ok || inv.OrganizationID != orgID {
		return nil, assert.AnError
	}
	cp := *inv
	return &cp, nil
}

func (m *mockFinanceRepo) ListInvoices(_ context.Context, orgID uuid.UUID, _ InvoiceFilter) ([]Invoice, int, error) {
	var result []Invoice
	for _, inv := range m.invoices {
		if inv.OrganizationID == orgID {
			result = append(result, *inv)
		}
	}
	return result, len(result), nil
}

func (m *mockFinanceRepo) UpdateInvoice(_ context.Context, inv *Invoice) error {
	cp := *inv
	m.invoices[inv.ID] = &cp
	return nil
}

func (m *mockFinanceRepo) GetTrialBalance(_ context.Context, _ uuid.UUID, _ time.Time) ([]TrialBalanceRow, error) {
	return nil, nil
}

func (m *mockFinanceRepo) GetAccountBalance(_ context.Context, _, _ uuid.UUID, _, _ time.Time) (*AccountBalance, error) {
	return &AccountBalance{}, nil
}

func (m *mockFinanceRepo) GetPeriod(_ context.Context, orgID uuid.UUID, year, month int) (*FinancePeriod, error) {
	key := orgID.String() + ":" + string(rune(year)) + ":" + string(rune(month))
	p, ok := m.periods[key]
	if !ok {
		return nil, assert.AnError
	}
	cp := *p
	return &cp, nil
}

func (m *mockFinanceRepo) UpsertPeriod(_ context.Context, _ pgx.Tx, p *FinancePeriod) error {
	key := p.OrganizationID.String() + ":" + string(rune(p.Year)) + ":" + string(rune(p.Month))
	cp := *p
	m.periods[key] = &cp
	return nil
}

func (m *mockFinanceRepo) ListPeriods(_ context.Context, orgID uuid.UUID) ([]FinancePeriod, error) {
	var result []FinancePeriod
	for _, p := range m.periods {
		if p.OrganizationID == orgID {
			result = append(result, *p)
		}
	}
	return result, nil
}

func (m *mockFinanceRepo) GetProfitAndLoss(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]PnLRow, error) {
	return nil, nil
}

func (m *mockFinanceRepo) CreateCashOperation(_ context.Context, _ pgx.Tx, op *CashOperation) error {
	m.cashOps = append(m.cashOps, *op)
	return nil
}

func (m *mockFinanceRepo) ListCashOperations(_ context.Context, orgID uuid.UUID, _ CashOperationFilter) ([]CashOperation, int, error) {
	var result []CashOperation
	for _, op := range m.cashOps {
		if op.OrganizationID == orgID {
			result = append(result, op)
		}
	}
	return result, len(result), nil
}

func (m *mockFinanceRepo) WithTx(_ context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
}

type mockBus struct {
	published []types.Event
}

func (b *mockBus) Publish(_ context.Context, ev types.Event) error {
	b.published = append(b.published, ev)
	return nil
}
func (b *mockBus) Subscribe(string, event.Subscriber)        {}
func (b *mockBus) SubscribePattern(string, event.Subscriber) {}
func (b *mockBus) SubscribeAll(event.Subscriber)             {}

// --- helpers ---

func setupFinanceService() (*Service, *mockFinanceRepo, *mockBus) {
	repo := newMockFinanceRepo()
	bus := &mockBus{}
	svc := NewService(repo, bus)
	return svc, repo, bus
}

// --- tests ---

func TestCreateAccount(t *testing.T) {
	orgID := uuid.New()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, repo, _ := setupFinanceService()

		acct, err := svc.CreateAccount(ctx, orgID, CreateAccountInput{
			Code: "44",
			Name: "Selling expenses",
			Type: "expense",
		})
		require.NoError(t, err)
		assert.Equal(t, "44", acct.Code)
		assert.Equal(t, "expense", acct.Type)
		assert.Len(t, repo.accounts, 1)
	})

	t.Run("invalid type", func(t *testing.T) {
		svc, _, _ := setupFinanceService()
		_, err := svc.CreateAccount(ctx, orgID, CreateAccountInput{
			Code: "01",
			Name: "Test",
			Type: "invalid",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type must be")
	})

	t.Run("missing code", func(t *testing.T) {
		svc, _, _ := setupFinanceService()
		_, err := svc.CreateAccount(ctx, orgID, CreateAccountInput{
			Name: "Test",
			Type: "asset",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "code is required")
	})
}

func TestSeedDefaultAccounts(t *testing.T) {
	orgID := uuid.New()
	ctx := context.Background()

	svc, repo, _ := setupFinanceService()
	err := svc.SeedDefaultAccounts(ctx, orgID)
	require.NoError(t, err)
	assert.Len(t, repo.accounts, 11)
}

func TestCreateTransaction(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()
	acct1 := uuid.New()
	acct2 := uuid.New()

	t.Run("balanced transaction", func(t *testing.T) {
		svc, _, bus := setupFinanceService()

		txn, err := svc.CreateTransaction(ctx, orgID, CreateTransactionInput{
			Date:        "2025-01-15",
			Description: "Test transaction",
			Lines: []CreateTransactionLineInput{
				{AccountID: acct1, Debit: 50000},
				{AccountID: acct2, Credit: 50000},
			},
		}, &actorID)

		require.NoError(t, err)
		assert.Equal(t, "Test transaction", txn.Description)
		assert.Len(t, txn.Lines, 2)
		assert.False(t, txn.IsPosted)

		hasEvent := false
		for _, ev := range bus.published {
			if ev.Type == "finance.transaction.created" {
				hasEvent = true
			}
		}
		assert.True(t, hasEvent)
	})

	t.Run("unbalanced rejected", func(t *testing.T) {
		svc, _, _ := setupFinanceService()

		_, err := svc.CreateTransaction(ctx, orgID, CreateTransactionInput{
			Date: "2025-01-15",
			Lines: []CreateTransactionLineInput{
				{AccountID: acct1, Debit: 50000},
				{AccountID: acct2, Credit: 30000},
			},
		}, &actorID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "sum of debits must equal sum of credits")
	})

	t.Run("less than two lines", func(t *testing.T) {
		svc, _, _ := setupFinanceService()

		_, err := svc.CreateTransaction(ctx, orgID, CreateTransactionInput{
			Date: "2025-01-15",
			Lines: []CreateTransactionLineInput{
				{AccountID: acct1, Debit: 50000},
			},
		}, &actorID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "at least two lines")
	})

	t.Run("both debit and credit rejected", func(t *testing.T) {
		svc, _, _ := setupFinanceService()

		_, err := svc.CreateTransaction(ctx, orgID, CreateTransactionInput{
			Date: "2025-01-15",
			Lines: []CreateTransactionLineInput{
				{AccountID: acct1, Debit: 50000, Credit: 50000},
				{AccountID: acct2, Debit: 50000},
			},
		}, &actorID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot have both debit and credit")
	})

	t.Run("zero line rejected", func(t *testing.T) {
		svc, _, _ := setupFinanceService()

		_, err := svc.CreateTransaction(ctx, orgID, CreateTransactionInput{
			Date: "2025-01-15",
			Lines: []CreateTransactionLineInput{
				{AccountID: acct1, Debit: 50000},
				{AccountID: acct2},
			},
		}, &actorID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "must have either debit or credit")
	})

	t.Run("invalid date", func(t *testing.T) {
		svc, _, _ := setupFinanceService()

		_, err := svc.CreateTransaction(ctx, orgID, CreateTransactionInput{
			Date: "not-a-date",
			Lines: []CreateTransactionLineInput{
				{AccountID: acct1, Debit: 50000},
				{AccountID: acct2, Credit: 50000},
			},
		}, &actorID)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid date")
	})
}

func TestPostTransaction(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupFinanceService()

	txn, _ := svc.CreateTransaction(ctx, orgID, CreateTransactionInput{
		Date: "2025-01-15",
		Lines: []CreateTransactionLineInput{
			{AccountID: uuid.New(), Debit: 10000},
			{AccountID: uuid.New(), Credit: 10000},
		},
	}, &actorID)

	err := svc.PostTransaction(ctx, orgID, txn.ID, &actorID)
	require.NoError(t, err)

	err = svc.PostTransaction(ctx, orgID, txn.ID, &actorID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already posted")
}

func TestDeleteAccount(t *testing.T) {
	orgID := uuid.New()
	ctx := context.Background()

	t.Run("delete non-system account", func(t *testing.T) {
		svc, _, _ := setupFinanceService()

		acct, _ := svc.CreateAccount(ctx, orgID, CreateAccountInput{
			Code: "99", Name: "Test", Type: "asset",
		})

		err := svc.DeleteAccount(ctx, orgID, acct.ID)
		require.NoError(t, err)
	})

	t.Run("cannot delete system account", func(t *testing.T) {
		svc, _, _ := setupFinanceService()
		svc.SeedDefaultAccounts(ctx, orgID)

		accounts, _ := svc.ListAccounts(ctx, orgID)
		require.NotEmpty(t, accounts)

		err := svc.DeleteAccount(ctx, orgID, accounts[0].ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot delete system account")
	})

	t.Run("cannot delete account with lines", func(t *testing.T) {
		svc, repo, _ := setupFinanceService()

		acct, _ := svc.CreateAccount(ctx, orgID, CreateAccountInput{
			Code: "44", Name: "Expenses", Type: "expense",
		})
		repo.hasLines[acct.ID] = true

		err := svc.DeleteAccount(ctx, orgID, acct.ID)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "existing transaction lines")
	})
}

func TestCreateInvoice(t *testing.T) {
	orgID := uuid.New()
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		svc, _, _ := setupFinanceService()

		inv, err := svc.CreateInvoice(ctx, orgID, CreateInvoiceInput{
			Number:   "INV-001",
			Type:     "outgoing",
			Subtotal: 100000,
			Tax:      20000,
			Total:    120000,
		})

		require.NoError(t, err)
		assert.Equal(t, "INV-001", inv.Number)
		assert.Equal(t, "draft", inv.Status)
		assert.Equal(t, "RUB", inv.Currency)
	})

	t.Run("invalid type", func(t *testing.T) {
		svc, _, _ := setupFinanceService()
		_, err := svc.CreateInvoice(ctx, orgID, CreateInvoiceInput{
			Number: "INV-002",
			Type:   "invalid",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "type must be")
	})
}

func TestMarkInvoicePaid(t *testing.T) {
	orgID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupFinanceService()

	inv, _ := svc.CreateInvoice(ctx, orgID, CreateInvoiceInput{
		Number: "INV-001",
		Type:   "outgoing",
		Total:  50000,
	})

	err := svc.MarkInvoicePaid(ctx, orgID, inv.ID)
	require.NoError(t, err)

	err = svc.MarkInvoicePaid(ctx, orgID, inv.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already paid")
}

func TestAutoTransaction(t *testing.T) {
	orgID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupFinanceService()
	svc.SeedDefaultAccounts(ctx, orgID)

	err := svc.CreateAutoTransaction(ctx, orgID, "2025-01-15", "Order payment", "51", "62", 50000, nil, nil)
	require.NoError(t, err)
}

func TestClosePeriod(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupFinanceService()

	period, err := svc.ClosePeriod(ctx, orgID, 2025, 1, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "closed", period.Status)
	assert.Equal(t, 2025, period.Year)
	assert.Equal(t, 1, period.Month)
	assert.NotNil(t, period.ClosedAt)
}

func TestClosePeriodAlreadyClosed(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupFinanceService()

	_, err := svc.ClosePeriod(ctx, orgID, 2025, 6, &actorID)
	require.NoError(t, err)

	_, err = svc.ClosePeriod(ctx, orgID, 2025, 6, &actorID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already closed")
}

func TestListPeriods(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupFinanceService()

	svc.ClosePeriod(ctx, orgID, 2025, 1, &actorID)
	svc.ClosePeriod(ctx, orgID, 2025, 2, &actorID)

	periods, err := svc.ListPeriods(ctx, orgID)
	require.NoError(t, err)
	assert.Len(t, periods, 2)
}

func TestCreateCashOperation(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupFinanceService()
	svc.SeedDefaultAccounts(ctx, orgID)

	op, err := svc.CreateCashOperation(ctx, orgID, CreateCashOperationInput{
		Type:        "deposit",
		Amount:      100000,
		Description: "Cash sale",
		AccountCode: "50",
	}, &actorID)
	require.NoError(t, err)
	assert.Equal(t, "deposit", op.Type)
	assert.Equal(t, int64(100000), op.Amount)
}

func TestCashOperationInvalidType(t *testing.T) {
	orgID := uuid.New()
	actorID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupFinanceService()

	_, err := svc.CreateCashOperation(ctx, orgID, CreateCashOperationInput{
		Type:   "invalid",
		Amount: 1000,
	}, &actorID)
	assert.Error(t, err)
}

func TestGetProfitAndLoss(t *testing.T) {
	orgID := uuid.New()
	ctx := context.Background()

	svc, _, _ := setupFinanceService()

	from := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2025, 1, 31, 23, 59, 59, 0, time.UTC)

	report, err := svc.GetProfitAndLoss(ctx, orgID, from, to)
	require.NoError(t, err)
	assert.NotNil(t, report)
}
