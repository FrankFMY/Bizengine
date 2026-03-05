package analytics

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	pool *pgxpool.Pool
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

type DashboardData struct {
	Revenue      RevenueData      `json:"revenue"`
	Orders       OrdersData       `json:"orders"`
	TopProducts  []ProductStat    `json:"top_products"`
	LowStock     []StockAlert     `json:"low_stock"`
	RecentOrders []RecentOrder    `json:"recent_orders"`
}

type RevenueData struct {
	Today     int64 `json:"today"`
	ThisWeek  int64 `json:"this_week"`
	ThisMonth int64 `json:"this_month"`
}

type OrdersData struct {
	Pending   int `json:"pending"`
	Confirmed int `json:"confirmed"`
	Shipped   int `json:"shipped"`
	Total     int `json:"total"`
}

type ProductStat struct {
	ProductID uuid.UUID `json:"product_id"`
	Name      string    `json:"name"`
	Sold      int       `json:"sold"`
	Revenue   int64     `json:"revenue"`
}

type StockAlert struct {
	ProductID   uuid.UUID `json:"product_id"`
	Name        string    `json:"name"`
	Available   float64   `json:"available"`
	MinQuantity float64   `json:"min_quantity"`
}

type RecentOrder struct {
	ID        uuid.UUID `json:"id"`
	Number    string    `json:"number"`
	Status    string    `json:"status"`
	Total     int64     `json:"total"`
	CreatedAt time.Time `json:"created_at"`
}

type TimeSeriesPoint struct {
	Date  string `json:"date"`
	Value int64  `json:"value"`
}

func (s *Service) GetDashboard(ctx context.Context, orgID uuid.UUID) (*DashboardData, error) {
	d := &DashboardData{}

	// Revenue
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := todayStart.AddDate(0, 0, -int(now.Weekday()))
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(total), 0) FROM orders WHERE organization_id = $1 AND status IN ('paid', 'shipped', 'delivered') AND created_at >= $2`,
		orgID, todayStart).Scan(&d.Revenue.Today)

	s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(total), 0) FROM orders WHERE organization_id = $1 AND status IN ('paid', 'shipped', 'delivered') AND created_at >= $2`,
		orgID, weekStart).Scan(&d.Revenue.ThisWeek)

	s.pool.QueryRow(ctx,
		`SELECT COALESCE(SUM(total), 0) FROM orders WHERE organization_id = $1 AND status IN ('paid', 'shipped', 'delivered') AND created_at >= $2`,
		orgID, monthStart).Scan(&d.Revenue.ThisMonth)

	// Order counts by status
	s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE organization_id = $1 AND status IN ('new', 'draft')`, orgID).Scan(&d.Orders.Pending)
	s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE organization_id = $1 AND status = 'confirmed'`, orgID).Scan(&d.Orders.Confirmed)
	s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE organization_id = $1 AND status = 'shipped'`, orgID).Scan(&d.Orders.Shipped)
	s.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM orders WHERE organization_id = $1`, orgID).Scan(&d.Orders.Total)

	// Top products (by order count in last 30 days)
	rows, err := s.pool.Query(ctx,
		`SELECT oi.product_id, oi.name, COUNT(*) as sold, SUM(oi.total) as revenue
		 FROM order_items oi
		 JOIN orders o ON o.id = oi.order_id
		 WHERE o.organization_id = $1 AND o.created_at >= $2 AND o.status NOT IN ('cancelled', 'draft')
		 GROUP BY oi.product_id, oi.name
		 ORDER BY sold DESC LIMIT 10`,
		orgID, monthStart)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p ProductStat
			if err := rows.Scan(&p.ProductID, &p.Name, &p.Sold, &p.Revenue); err == nil {
				d.TopProducts = append(d.TopProducts, p)
			}
		}
	}

	// Low stock
	rows2, err := s.pool.Query(ctx,
		`SELECT sl.product_id, COALESCE(e.label, sl.product_id::text), sl.available, sl.min_quantity
		 FROM stock_levels sl
		 LEFT JOIN entities e ON e.id = sl.product_id
		 WHERE sl.organization_id = $1 AND sl.available <= sl.min_quantity AND sl.min_quantity > 0
		 ORDER BY sl.available ASC LIMIT 10`,
		orgID)
	if err == nil {
		defer rows2.Close()
		for rows2.Next() {
			var a StockAlert
			if err := rows2.Scan(&a.ProductID, &a.Name, &a.Available, &a.MinQuantity); err == nil {
				d.LowStock = append(d.LowStock, a)
			}
		}
	}

	// Recent orders
	rows3, err := s.pool.Query(ctx,
		`SELECT id, number, status, total, created_at FROM orders WHERE organization_id = $1 ORDER BY created_at DESC LIMIT 10`,
		orgID)
	if err == nil {
		defer rows3.Close()
		for rows3.Next() {
			var o RecentOrder
			if err := rows3.Scan(&o.ID, &o.Number, &o.Status, &o.Total, &o.CreatedAt); err == nil {
				d.RecentOrders = append(d.RecentOrders, o)
			}
		}
	}

	return d, nil
}

func (s *Service) GetRevenueSeries(ctx context.Context, orgID uuid.UUID, days int) ([]TimeSeriesPoint, error) {
	if days <= 0 {
		days = 30
	}

	rows, err := s.pool.Query(ctx,
		`SELECT DATE(created_at) as d, COALESCE(SUM(total), 0) as revenue
		 FROM orders
		 WHERE organization_id = $1 AND created_at >= NOW() - make_interval(days := $2) AND status NOT IN ('cancelled', 'draft')
		 GROUP BY d ORDER BY d`,
		orgID, days)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []TimeSeriesPoint
	for rows.Next() {
		var p TimeSeriesPoint
		var d time.Time
		if err := rows.Scan(&d, &p.Value); err == nil {
			p.Date = d.Format("2006-01-02")
			points = append(points, p)
		}
	}
	return points, rows.Err()
}
