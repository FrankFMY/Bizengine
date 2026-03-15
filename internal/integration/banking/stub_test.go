package banking

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStub_ImplementsInterface(t *testing.T) {
	var _ BankAdapter = (*Stub)(nil)
}

func TestStubInitiatePayment(t *testing.T) {
	stub := NewStub()
	req := PaymentRequest{
		OrganizationID:   uuid.New(),
		Amount:           100000,
		Currency:         "RUB",
		Purpose:          "test payment",
		RecipientINN:     "7707083893",
		RecipientBIC:     "044525225",
		RecipientAccount: "40702810000000000001",
	}

	result, err := stub.InitiatePayment(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEmpty(t, result.PaymentID)
	assert.Equal(t, "pending", result.Status)
	assert.NotEmpty(t, result.PaymentURL)
}

func TestStubGetPaymentStatus(t *testing.T) {
	stub := NewStub()
	paymentID := uuid.New().String()

	status, err := stub.GetPaymentStatus(context.Background(), paymentID)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, paymentID, status.PaymentID)
	assert.Equal(t, "completed", status.Status)
	assert.False(t, status.UpdatedAt.IsZero())
}

func TestStubGetStatement(t *testing.T) {
	stub := NewStub()
	orgID := uuid.New()
	req := StatementRequest{
		OrganizationID: orgID,
	}

	stmt, err := stub.GetStatement(context.Background(), req)
	require.NoError(t, err)
	require.NotNil(t, stmt)

	assert.Equal(t, int64(0), stmt.OpenBalance)
	assert.Equal(t, int64(0), stmt.CloseBalance)
	assert.NotNil(t, stmt.Entries)
	assert.Empty(t, stmt.Entries)
}

func TestStubCreatePayrollBatch(t *testing.T) {
	stub := NewStub()
	employees := []PayrollEmployee{
		{FullName: "Alice", CardNumber: "1111", Amount: 50000, EmployeeID: uuid.New()},
		{FullName: "Bob", CardNumber: "2222", Amount: 60000, EmployeeID: uuid.New()},
		{FullName: "Charlie", CardNumber: "3333", Amount: 70000, EmployeeID: uuid.New()},
	}
	batch := PayrollBatch{
		OrganizationID: uuid.New(),
		Employees:      employees,
		Month:          3,
		Year:           2026,
	}

	result, err := stub.CreatePayrollBatch(context.Background(), batch)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEmpty(t, result.BatchID)
	assert.Equal(t, "completed", result.Status)
	assert.Equal(t, len(employees), result.Count)
}

func TestStubCreatePayrollBatch_EmptyEmployees(t *testing.T) {
	stub := NewStub()
	batch := PayrollBatch{
		OrganizationID: uuid.New(),
		Employees:      nil,
		Month:          1,
		Year:           2026,
	}

	result, err := stub.CreatePayrollBatch(context.Background(), batch)
	require.NoError(t, err)
	assert.Equal(t, 0, result.Count)
}

func TestStubGetPayrollBatchStatus(t *testing.T) {
	stub := NewStub()
	batchID := "batch-123"

	status, err := stub.GetPayrollBatchStatus(context.Background(), batchID)
	require.NoError(t, err)
	require.NotNil(t, status)

	assert.Equal(t, batchID, status.BatchID)
	assert.Equal(t, "completed", status.Status)
	assert.False(t, status.UpdatedAt.IsZero())
}

func TestStubGetAuthURL(t *testing.T) {
	stub := NewStub()
	redirect := "https://myapp.com/callback"

	url, err := stub.GetAuthURL(context.Background(), redirect)
	require.NoError(t, err)

	assert.Contains(t, url, redirect)
	assert.True(t, strings.HasPrefix(url, "https://"))
}

func TestStubExchangeCode(t *testing.T) {
	stub := NewStub()
	code := "test-auth-code-42"

	user, err := stub.ExchangeCode(context.Background(), code)
	require.NoError(t, err)
	require.NotNil(t, user)

	assert.Contains(t, user.BankClientID, code)
	assert.NotEmpty(t, user.FullName)
}

func TestStubGetBankInfo(t *testing.T) {
	stub := NewStub()
	info := stub.GetBankInfo()

	assert.Equal(t, "stub", info.Code)
	assert.NotEmpty(t, info.Name)
}
