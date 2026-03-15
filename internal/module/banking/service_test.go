package banking

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/event"
	adapter "github.com/bizengine/engine/internal/integration/banking"
)

type mockBankingRepo struct {
	mu              sync.Mutex
	reconciliations map[uuid.UUID]*Reconciliation
	entries         map[uuid.UUID][]ReconciliationEntry
	partners        map[uuid.UUID]*BankPartner
	orgPartners     map[uuid.UUID]*BankPartner
}

func newMockBankingRepo() *mockBankingRepo {
	return &mockBankingRepo{
		reconciliations: make(map[uuid.UUID]*Reconciliation),
		entries:         make(map[uuid.UUID][]ReconciliationEntry),
		partners:        make(map[uuid.UUID]*BankPartner),
		orgPartners:     make(map[uuid.UUID]*BankPartner),
	}
}

func (r *mockBankingRepo) GetBankPartner(_ context.Context, id uuid.UUID) (*BankPartner, error) {
	return r.partners[id], nil
}
func (r *mockBankingRepo) GetBankPartnerByCode(_ context.Context, code string) (*BankPartner, error) {
	return nil, nil
}
func (r *mockBankingRepo) ListBankPartners(_ context.Context) ([]BankPartner, error) {
	return []BankPartner{}, nil
}
func (r *mockBankingRepo) GetOrgBankPartner(_ context.Context, orgID uuid.UUID) (*BankPartner, error) {
	bp, ok := r.orgPartners[orgID]
	if !ok {
		return nil, nil
	}
	return bp, nil
}

func (r *mockBankingRepo) CreateReconciliation(_ context.Context, _ pgx.Tx, rec *Reconciliation) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	rec.Iat = now
	rec.Upd = now
	rec.Ver = 1
	r.reconciliations[rec.ID] = rec
	return nil
}

func (r *mockBankingRepo) CreateReconciliationEntry(_ context.Context, _ pgx.Tx, e *ReconciliationEntry) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	e.Iat = now
	e.Upd = now
	e.Ver = 1
	r.entries[e.ReconciliationID] = append(r.entries[e.ReconciliationID], *e)
	return nil
}

func (r *mockBankingRepo) GetReconciliation(_ context.Context, orgID, id uuid.UUID) (*Reconciliation, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.reconciliations[id]
	if !ok {
		return nil, assert.AnError
	}
	return rec, nil
}

func (r *mockBankingRepo) ListReconciliations(_ context.Context, orgID uuid.UUID, limit, offset int) ([]Reconciliation, int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var recs []Reconciliation
	for _, rec := range r.reconciliations {
		if rec.OrganizationID == orgID {
			recs = append(recs, *rec)
		}
	}
	return recs, len(recs), nil
}

func (r *mockBankingRepo) GetReconciliationEntries(_ context.Context, reconID uuid.UUID) ([]ReconciliationEntry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entries := r.entries[reconID]
	if entries == nil {
		entries = []ReconciliationEntry{}
	}
	return entries, nil
}

func (r *mockBankingRepo) UpdateEntryStatus(_ context.Context, _ pgx.Tx, entryID uuid.UUID, status string, matchedType *string, matchedID *uuid.UUID, transactionID *uuid.UUID) error {
	return nil
}

func (r *mockBankingRepo) FindOrderByAmountAndINN(_ context.Context, orgID uuid.UUID, amount int64, inn string) (*uuid.UUID, error) {
	return nil, assert.AnError
}

func (r *mockBankingRepo) WithTx(_ context.Context, fn func(tx pgx.Tx) error) error {
	return fn(nil)
}

type mockFinanceTx struct{}

func (m *mockFinanceTx) CreateAutoTransaction(_ context.Context, _ uuid.UUID, _, _, _, _ string, _ int64, _ *string, _ *uuid.UUID) error {
	return nil
}

func TestInitiatePayment(t *testing.T) {
	repo := newMockBankingRepo()
	bus := event.NewLocalBus()
	bankAdapter := adapter.NewStub()
	svc := NewService(repo, bankAdapter, bus, &mockFinanceTx{})
	ctx := context.Background()
	orgID := uuid.New()
	orderID := uuid.New()

	result, err := svc.InitiatePayment(ctx, orgID, InitiatePaymentInput{
		OrderID: &orderID,
		Amount:  100000,
		Method:  "card",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.PaymentID)
	assert.NotEmpty(t, result.PaymentURL)
	assert.Equal(t, "pending", result.Status)
}

func TestInitiatePaymentValidation(t *testing.T) {
	repo := newMockBankingRepo()
	bus := event.NewLocalBus()
	bankAdapter := adapter.NewStub()
	svc := NewService(repo, bankAdapter, bus, &mockFinanceTx{})
	ctx := context.Background()
	orgID := uuid.New()

	// Missing order_id
	_, err := svc.InitiatePayment(ctx, orgID, InitiatePaymentInput{
		Amount: 100000,
	})
	assert.Error(t, err)

	// Zero amount
	orderID := uuid.New()
	_, err = svc.InitiatePayment(ctx, orgID, InitiatePaymentInput{
		OrderID: &orderID,
		Amount:  0,
	})
	assert.Error(t, err)
}

func TestAutoReconcile(t *testing.T) {
	repo := newMockBankingRepo()
	bus := event.NewLocalBus()
	bankAdapter := adapter.NewStub()
	svc := NewService(repo, bankAdapter, bus, &mockFinanceTx{})
	ctx := context.Background()
	orgID := uuid.New()

	result, err := svc.AutoReconcile(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Reconciliation.TotalEntries)
	assert.Equal(t, "completed", result.Reconciliation.Status)
}

func TestPaymentCallback(t *testing.T) {
	repo := newMockBankingRepo()
	bus := event.NewLocalBus()
	bankAdapter := adapter.NewStub()
	svc := NewService(repo, bankAdapter, bus, &mockFinanceTx{})
	ctx := context.Background()
	orgID := uuid.New()
	orderID := uuid.New()

	err := svc.HandlePaymentCallback(ctx, orgID, PaymentCallback{
		PaymentID: "pay-123",
		OrderID:   &orderID,
		Amount:    50000,
		Status:    "completed",
		Method:    "card",
	})
	assert.NoError(t, err)
}

func TestPayViaBank(t *testing.T) {
	repo := newMockBankingRepo()
	bus := event.NewLocalBus()
	bankAdapter := adapter.NewStub()
	svc := NewService(repo, bankAdapter, bus, &mockFinanceTx{})
	ctx := context.Background()
	orgID := uuid.New()

	result, err := svc.PayViaBank(ctx, orgID, adapter.PayrollBatch{
		Month: 3,
		Year:  2026,
		Employees: []adapter.PayrollEmployee{
			{FullName: "Test", CardNumber: "1234", Amount: 50000, EmployeeID: uuid.New()},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, 1, result.Count)
}

func TestPayViaBankEmptyBatch(t *testing.T) {
	repo := newMockBankingRepo()
	bus := event.NewLocalBus()
	bankAdapter := adapter.NewStub()
	svc := NewService(repo, bankAdapter, bus, &mockFinanceTx{})
	ctx := context.Background()
	orgID := uuid.New()

	_, err := svc.PayViaBank(ctx, orgID, adapter.PayrollBatch{
		Month:     3,
		Year:      2026,
		Employees: []adapter.PayrollEmployee{},
	})
	assert.Error(t, err)
}

func TestGetWhiteLabel(t *testing.T) {
	repo := newMockBankingRepo()
	bus := event.NewLocalBus()
	bankAdapter := adapter.NewStub()
	svc := NewService(repo, bankAdapter, bus, &mockFinanceTx{})
	ctx := context.Background()
	orgID := uuid.New()

	// No bank partner — returns default
	wl, err := svc.GetWhiteLabel(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, "BizEngine", wl.AppName)

	// With bank partner
	bp := &BankPartner{
		ID:   uuid.New(),
		Name: "Sber",
		Code: "sber",
		WhiteLabel: WhiteLabel{
			LogoURL:      "https://sber.ru/logo.svg",
			PrimaryColor: "#21A038",
			AppName:      "SberBiz",
			BankName:     "Sber",
		},
	}
	repo.orgPartners[orgID] = bp

	wl, err = svc.GetWhiteLabel(ctx, orgID)
	require.NoError(t, err)
	assert.Equal(t, "SberBiz", wl.AppName)
	assert.Equal(t, "#21A038", wl.PrimaryColor)
}

func TestGetPaymentStatus(t *testing.T) {
	repo := newMockBankingRepo()
	bus := event.NewLocalBus()
	bankAdapter := adapter.NewStub()
	svc := NewService(repo, bankAdapter, bus, &mockFinanceTx{})
	ctx := context.Background()

	status, err := svc.GetPaymentStatus(ctx, "pay-123")
	require.NoError(t, err)
	assert.Equal(t, "completed", status.Status)
}

func TestBankAuthFlow(t *testing.T) {
	repo := newMockBankingRepo()
	bus := event.NewLocalBus()
	bankAdapter := adapter.NewStub()
	svc := NewService(repo, bankAdapter, bus, &mockFinanceTx{})
	ctx := context.Background()

	url, err := svc.GetAuthURL(ctx, "https://app.biz/callback")
	require.NoError(t, err)
	assert.Contains(t, url, "https://app.biz/callback")

	user, err := svc.ExchangeCode(ctx, "test-code-123")
	require.NoError(t, err)
	assert.Contains(t, user.BankClientID, "test-code-123")
	assert.NotEmpty(t, user.FullName)
}
