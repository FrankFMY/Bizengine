package documents

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockOrderFetcher struct{}

func (m *mockOrderFetcher) GetOrderForDocument(_ context.Context, _, _ uuid.UUID) (*OrderData, error) {
	return &OrderData{
		ID:     uuid.New(),
		Number: "ORD-001",
		Date:   time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC),
		CustomerName: "Test Customer",
		Items: []OrderItemRow{
			{Index: 1, Name: "Кофе Латте", SKU: "LATTE-01", Quantity: 2, UnitPrice: "250.00", Total: "500.00", VATRate: 20},
			{Index: 2, Name: "Круассан", SKU: "CROIS-01", Quantity: 1, UnitPrice: "150.00", Total: "150.00", VATRate: 20},
		},
		Total:          65000,
		VATAmount:      10833,
		TotalFormatted: "650.00",
		Status:         "paid",
	}, nil
}

type mockOrgFetcher struct{}

func (m *mockOrgFetcher) GetOrgRequisites(_ context.Context, _ uuid.UUID) (*OrgRequisites, error) {
	return &OrgRequisites{
		Name:    "ООО Тест",
		INN:     "7700000000",
		KPP:     "770001001",
		Address: "г. Москва, ул. Тестовая, д. 1",
	}, nil
}

type mockProductFetcher struct{}

func (m *mockProductFetcher) GetProductTags(_ context.Context, _ uuid.UUID, _ []uuid.UUID) ([]ProductTag, error) {
	return []ProductTag{
		{Name: "Кофе Латте", SKU: "LATTE-01", Price: "250.00", Unit: "шт"},
		{Name: "Круассан", SKU: "CROIS-01", Price: "150.00", Unit: "шт"},
	}, nil
}

func setupDocService(t *testing.T) *Service {
	svc, err := NewService(&mockOrderFetcher{}, &mockOrgFetcher{}, &mockProductFetcher{})
	require.NoError(t, err)
	return svc
}

func TestRenderInvoice(t *testing.T) {
	svc := setupDocService(t)
	html, err := svc.RenderInvoice(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Contains(t, string(html), "СЧЁТ-ФАКТУРА")
	assert.Contains(t, string(html), "ORD-001")
	assert.Contains(t, string(html), "Кофе Латте")
	assert.Contains(t, string(html), "650.00")
}

func TestRenderTORG12(t *testing.T) {
	svc := setupDocService(t)
	html, err := svc.RenderTORG12(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Contains(t, string(html), "ТОВАРНАЯ НАКЛАДНАЯ")
	assert.Contains(t, string(html), "ООО Тест")
}

func TestRenderAct(t *testing.T) {
	svc := setupDocService(t)
	html, err := svc.RenderAct(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Contains(t, string(html), "АКТ ВЫПОЛНЕННЫХ РАБОТ")
}

func TestRenderReceipt(t *testing.T) {
	svc := setupDocService(t)
	html, err := svc.RenderReceipt(context.Background(), uuid.New(), uuid.New())
	require.NoError(t, err)
	assert.Contains(t, string(html), "ЧЕК")
	assert.Contains(t, string(html), "СПАСИБО ЗА ПОКУПКУ")
}

func TestRenderPriceTags(t *testing.T) {
	svc := setupDocService(t)
	html, err := svc.RenderPriceTags(context.Background(), uuid.New(), []uuid.UUID{uuid.New(), uuid.New()})
	require.NoError(t, err)
	assert.Contains(t, string(html), "Кофе Латте")
	assert.Contains(t, string(html), "250.00")
}

func TestRenderPriceTagsEmpty(t *testing.T) {
	svc := setupDocService(t)
	_, err := svc.RenderPriceTags(context.Background(), uuid.New(), []uuid.UUID{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "product_ids")
}

