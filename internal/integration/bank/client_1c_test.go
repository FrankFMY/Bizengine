package bank

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOneCClient_ExportPaymentOrders(t *testing.T) {
	c := NewOneCClient()

	orders := []PaymentOrder{
		{
			Number:          "42",
			Date:            "05.03.2026",
			Amount:          150000,
			PayerINN:        "7707083893",
			PayerAccount:    "40702810000000000001",
			ReceiverINN:     "7719081310",
			ReceiverAccount: "40702810000000000002",
			Purpose:         "Payment for services",
		},
	}

	data, err := c.ExportPaymentOrders(context.Background(), uuid.New(), orders)
	require.NoError(t, err)

	text := string(data)
	assert.True(t, strings.HasPrefix(text, "1CClientBankExchange"))
	assert.Contains(t, text, "Номер=42")
	assert.Contains(t, text, "Сумма=1500.00")
	assert.Contains(t, text, "ПлательщикИНН=7707083893")
	assert.Contains(t, text, "КонецДокумента")
	assert.Contains(t, text, "КонецФайла")
}

func TestOneCClient_ImportStatement(t *testing.T) {
	c := NewOneCClient()

	statement := `1CClientBankExchange
ВерсияФормата=1.03
Кодировка=UTF-8
ДатаНачала=01.03.2026
СекцияДокумент=Платёжное поручение
Номер=1
Сумма=1500,00
КонецДокумента
СекцияДокумент=Платёжное поручение
Номер=2
Сумма=2500,50
КонецДокумента
КонецФайла`

	result, err := c.ImportStatement(context.Background(), uuid.New(), []byte(statement))
	require.NoError(t, err)

	assert.Equal(t, 2, result.TransactionsImported)
	assert.Equal(t, int64(400050), result.TotalDebit) // 150000 + 250050
	assert.Equal(t, int64(0), result.TotalCredit)
	assert.Equal(t, "01.03.2026", result.Period)
}

func TestOneCClient_RoundTrip(t *testing.T) {
	c := NewOneCClient()

	orders := []PaymentOrder{
		{
			Number:          "1",
			Date:            "05.03.2026",
			Amount:          100000,
			PayerINN:        "1234567890",
			PayerAccount:    "40702810000000000001",
			ReceiverINN:     "0987654321",
			ReceiverAccount: "40702810000000000002",
			Purpose:         "Test payment",
		},
	}

	exported, err := c.ExportPaymentOrders(context.Background(), uuid.New(), orders)
	require.NoError(t, err)

	result, err := c.ImportStatement(context.Background(), uuid.New(), exported)
	require.NoError(t, err)

	assert.Equal(t, 1, result.TransactionsImported)
	assert.Equal(t, int64(100000), result.TotalDebit)
}
