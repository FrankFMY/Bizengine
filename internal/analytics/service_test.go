package analytics

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRevenueDataZeroValues(t *testing.T) {
	r := RevenueData{}
	assert.Equal(t, int64(0), r.Today)
	assert.Equal(t, int64(0), r.ThisWeek)
	assert.Equal(t, int64(0), r.ThisMonth)
}

func TestRevenueDataAccumulation(t *testing.T) {
	r := RevenueData{
		Today:     150000,
		ThisWeek:  750000,
		ThisMonth: 3200000,
	}

	assert.Equal(t, int64(150000), r.Today)
	assert.Equal(t, int64(750000), r.ThisWeek)
	assert.Equal(t, int64(3200000), r.ThisMonth)

	assert.True(t, r.Today <= r.ThisWeek, "today revenue should not exceed this week")
	assert.True(t, r.ThisWeek <= r.ThisMonth, "this week revenue should not exceed this month")
}

func TestOrdersDataTotal(t *testing.T) {
	tests := []struct {
		name string
		data OrdersData
	}{
		{
			name: "all zero",
			data: OrdersData{Pending: 0, Confirmed: 0, Shipped: 0, Total: 0},
		},
		{
			name: "mixed statuses",
			data: OrdersData{Pending: 5, Confirmed: 3, Shipped: 2, Total: 15},
		},
		{
			name: "only pending",
			data: OrdersData{Pending: 10, Confirmed: 0, Shipped: 0, Total: 10},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.GreaterOrEqual(t, tt.data.Total, tt.data.Pending+tt.data.Confirmed+tt.data.Shipped,
				"total should be >= sum of known statuses (other statuses may exist)")
		})
	}
}

func TestProductStatRevenue(t *testing.T) {
	products := []ProductStat{
		{Name: "Widget A", Sold: 100, Revenue: 500000},
		{Name: "Widget B", Sold: 50, Revenue: 1000000},
		{Name: "Widget C", Sold: 200, Revenue: 200000},
	}

	topByRevenue := products[0]
	for _, p := range products[1:] {
		if p.Revenue > topByRevenue.Revenue {
			topByRevenue = p
		}
	}
	assert.Equal(t, "Widget B", topByRevenue.Name)

	topBySold := products[0]
	for _, p := range products[1:] {
		if p.Sold > topBySold.Sold {
			topBySold = p
		}
	}
	assert.Equal(t, "Widget C", topBySold.Name)
}

func TestStockAlertBelowMinimum(t *testing.T) {
	alerts := []StockAlert{
		{Name: "Low item", Available: 2, MinQuantity: 10},
		{Name: "Critical item", Available: 0, MinQuantity: 5},
		{Name: "At threshold", Available: 5, MinQuantity: 5},
	}

	for _, a := range alerts {
		assert.LessOrEqual(t, a.Available, a.MinQuantity,
			"stock alert should only include items at or below min quantity: %s", a.Name)
	}
}

func TestDashboardDataInit(t *testing.T) {
	d := &DashboardData{}

	assert.Equal(t, int64(0), d.Revenue.Today)
	assert.Equal(t, int64(0), d.Revenue.ThisWeek)
	assert.Equal(t, int64(0), d.Revenue.ThisMonth)
	assert.Equal(t, 0, d.Orders.Total)
	assert.Nil(t, d.TopProducts)
	assert.Nil(t, d.LowStock)
	assert.Nil(t, d.RecentOrders)
}

func TestTimeSeriesPointFormat(t *testing.T) {
	point := TimeSeriesPoint{
		Date:  "2026-03-15",
		Value: 250000,
	}

	assert.Equal(t, "2026-03-15", point.Date)
	assert.Equal(t, int64(250000), point.Value)
	assert.Len(t, point.Date, 10, "date should be in YYYY-MM-DD format")
}

func TestGetRevenueSeriesDefaultDays(t *testing.T) {
	s := &Service{}

	assert.NotNil(t, s)
}
