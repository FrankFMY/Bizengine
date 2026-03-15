package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/core/auth"
)

func main() {
	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/bizengine?sslmode=disable"
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		fatal("connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		fatal("ping database: %v", err)
	}
	progress("connected to database")

	tx, err := pool.Begin(ctx)
	if err != nil {
		fatal("begin transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	now := time.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)

	// ---------------------------------------------------------------
	// 1. User
	// ---------------------------------------------------------------
	userID := uuid.New()
	secret := "password"
	hash, err := auth.HashSecret(secret)
	if err != nil {
		fatal("hash secret: %v", err)
	}

	userName := "Артём Пряничников"
	_, err = tx.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name, phone, is_active, name, secret, ver, upd, created_at, updated_at)
		 VALUES ($1, '', '', '', NULL, true, $2, $3, 1, $4, $5, $6)
		 ON CONFLICT DO NOTHING`,
		userID, &userName, hash, now, now, now,
	)
	if err != nil {
		fatal("insert user: %v", err)
	}
	progress("user: %s (%s)", userName, userID)

	// ---------------------------------------------------------------
	// 1b. Phone
	// ---------------------------------------------------------------
	phoneID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO phones (id, user_id, unformat, format, country, ver, upd, iat)
		 VALUES ($1, $2, $3, $4, 'RU', 1, $5, $6)
		 ON CONFLICT DO NOTHING`,
		phoneID, userID, "79991234567", "7 (999) 123-45-67", now, now,
	)
	if err != nil {
		fatal("insert phone: %v", err)
	}
	progress("phone: 79991234567")

	// ---------------------------------------------------------------
	// 2. Organization
	// ---------------------------------------------------------------
	orgID := uuid.New()
	orgSlug := "kofenya-aroma-" + orgID.String()[:8]
	_, err = tx.Exec(ctx,
		`INSERT INTO organizations (id, name, slug, owner_id, plan, settings, inn, ver, upd, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, 'free', '{}', $5, 1, $6, $7, $8)
		 ON CONFLICT DO NOTHING`,
		orgID, "Кофейня Арома", orgSlug, userID, "1234567890", now, now, now,
	)
	if err != nil {
		fatal("insert organization: %v", err)
	}
	progress("organization: Кофейня Арома (%s)", orgID)

	// ---------------------------------------------------------------
	// 3. Labor
	// ---------------------------------------------------------------
	laborID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO labors (id, organization_id, user_id, role, permissions, admin, ver, upd, iat, joined_at)
		 VALUES ($1, $2, $3, 'owner', '[]', true, 1, $4, $5, $6)
		 ON CONFLICT DO NOTHING`,
		laborID, orgID, userID, now, now, now,
	)
	if err != nil {
		fatal("insert labor: %v", err)
	}
	progress("labor: owner (%s)", laborID)

	// ---------------------------------------------------------------
	// 4. Warehouses
	// ---------------------------------------------------------------
	warehouseMain := uuid.New()
	warehouseKitchen := uuid.New()

	for _, w := range []struct {
		id   uuid.UUID
		name string
		sort int
	}{
		{warehouseMain, "Основной склад", 1},
		{warehouseKitchen, "Склад кухни", 2},
	} {
		_, err = tx.Exec(ctx,
			`INSERT INTO entities (id, organization_id, kind, name, status, meta, sort_order, ver, upd, created_at, updated_at)
			 VALUES ($1, $2, 'warehouse', $3, 'active', '{}', $4, 1, $5, $6, $7)
			 ON CONFLICT DO NOTHING`,
			w.id, orgID, w.name, w.sort, now, now, now,
		)
		if err != nil {
			fatal("insert warehouse %s: %v", w.name, err)
		}
		progress("warehouse: %s", w.name)
	}

	// ---------------------------------------------------------------
	// 5. Categories
	// ---------------------------------------------------------------
	catCoffee := uuid.New()
	catPastry := uuid.New()
	catMilk := uuid.New()

	for _, c := range []struct {
		id   uuid.UUID
		name string
		sort int
	}{
		{catCoffee, "Кофе", 1},
		{catPastry, "Выпечка", 2},
		{catMilk, "Молоко и сливки", 3},
	} {
		_, err = tx.Exec(ctx,
			`INSERT INTO entities (id, organization_id, kind, name, status, meta, sort_order, ver, upd, created_at, updated_at)
			 VALUES ($1, $2, 'category', $3, 'active', '{}', $4, 1, $5, $6, $7)
			 ON CONFLICT DO NOTHING`,
			c.id, orgID, c.name, c.sort, now, now, now,
		)
		if err != nil {
			fatal("insert category %s: %v", c.name, err)
		}
		progress("category: %s", c.name)
	}

	// ---------------------------------------------------------------
	// 6. Products (15 items)
	// ---------------------------------------------------------------
	type product struct {
		id       uuid.UUID
		name     string
		category uuid.UUID
		selling  int64
		purchase int64
		barcode  string
		physical bool
	}

	products := []product{
		// Coffee beans
		{uuid.New(), "Арабика 1кг", catCoffee, 85000, 45000, "4600000000001", true},
		{uuid.New(), "Робуста 1кг", catCoffee, 55000, 30000, "4600000000002", true},
		{uuid.New(), "Эспрессо-смесь 1кг", catCoffee, 70000, 38000, "4600000000003", true},
		// Milk
		{uuid.New(), "Молоко 1л", catMilk, 12000, 7000, "4600000000004", true},
		{uuid.New(), "Сливки 0.5л", catMilk, 18000, 11000, "4600000000005", true},
		// Pastry
		{uuid.New(), "Круассан", catPastry, 15000, 6000, "4600000000006", true},
		{uuid.New(), "Маффин шоколадный", catPastry, 12000, 5000, "4600000000007", true},
		{uuid.New(), "Сэндвич с курицей", catPastry, 25000, 12000, "4600000000008", true},
		{uuid.New(), "Чизкейк", catPastry, 22000, 9000, "4600000000009", true},
		// Drinks (services — not physical stock)
		{uuid.New(), "Американо", catCoffee, 20000, 3000, "4600000000010", false},
		{uuid.New(), "Капучино", catCoffee, 25000, 5000, "4600000000011", false},
		{uuid.New(), "Латте", catCoffee, 28000, 5500, "4600000000012", false},
		{uuid.New(), "Раф", catCoffee, 30000, 6000, "4600000000013", false},
		{uuid.New(), "Флэт Уайт", catCoffee, 27000, 5000, "4600000000014", false},
		{uuid.New(), "Какао", catCoffee, 22000, 4500, "4600000000015", false},
	}

	for i, p := range products {
		_, err = tx.Exec(ctx,
			`INSERT INTO entities (id, organization_id, kind, name, status, parent_id, meta, sort_order, ver, upd, created_at, updated_at)
			 VALUES ($1, $2, 'product', $3, 'active', $4, '{}', $5, 1, $6, $7, $8)
			 ON CONFLICT DO NOTHING`,
			p.id, orgID, p.name, p.category, i+1, now, now, now,
		)
		if err != nil {
			fatal("insert product %s: %v", p.name, err)
		}

		priceData := mustJSON(map[string]any{
			"selling_price":  p.selling,
			"purchase_price": p.purchase,
			"currency":       "RUB",
		})
		_, err = tx.Exec(ctx,
			`INSERT INTO components (id, entity_id, organization_id, type, data, version, ver, upd, created_at, updated_at)
			 VALUES ($1, $2, $3, 'price', $4, 1, 1, $5, $6, $7)
			 ON CONFLICT (entity_id, type) DO NOTHING`,
			uuid.New(), p.id, orgID, priceData, now, now, now,
		)
		if err != nil {
			fatal("insert price component for %s: %v", p.name, err)
		}

		barcodeData := mustJSON(map[string]any{
			"code": p.barcode,
			"type": "EAN13",
		})
		_, err = tx.Exec(ctx,
			`INSERT INTO components (id, entity_id, organization_id, type, data, version, ver, upd, created_at, updated_at)
			 VALUES ($1, $2, $3, 'barcode', $4, 1, 1, $5, $6, $7)
			 ON CONFLICT (entity_id, type) DO NOTHING`,
			uuid.New(), p.id, orgID, barcodeData, now, now, now,
		)
		if err != nil {
			fatal("insert barcode component for %s: %v", p.name, err)
		}

		progress("product: %s (sell=%d, buy=%d)", p.name, p.selling, p.purchase)
	}

	// ---------------------------------------------------------------
	// 7. Stock levels + initial movements (physical products only)
	// ---------------------------------------------------------------
	type stockEntry struct {
		productID   uuid.UUID
		warehouseID uuid.UUID
		quantity    float64
	}

	var stockEntries []stockEntry
	for _, p := range products {
		if !p.physical {
			continue
		}
		stockEntries = append(stockEntries,
			stockEntry{p.id, warehouseMain, 50},
			stockEntry{p.id, warehouseKitchen, 20},
		)
	}

	for _, s := range stockEntries {
		_, err = tx.Exec(ctx,
			`INSERT INTO stock_levels (organization_id, product_id, warehouse_id, quantity, reserved, unit, min_quantity, ver, upd, updated_at)
			 VALUES ($1, $2, $3, $4, 0, 'шт', 5, 1, $5, $6)
			 ON CONFLICT (organization_id, product_id, warehouse_id) DO NOTHING`,
			orgID, s.productID, s.warehouseID, s.quantity, now, now,
		)
		if err != nil {
			fatal("insert stock level: %v", err)
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO stock_movements (id, organization_id, product_id, warehouse_id, type, quantity, unit, reason, ver, upd, created_at)
			 VALUES ($1, $2, $3, $4, 'receive', $5, 'шт', 'Initial seed stock', 1, $6, $7)
			 ON CONFLICT DO NOTHING`,
			uuid.New(), orgID, s.productID, s.warehouseID, s.quantity, now, now,
		)
		if err != nil {
			fatal("insert stock movement: %v", err)
		}
	}
	progress("stock levels and movements for %d entries", len(stockEntries))

	// ---------------------------------------------------------------
	// 8. Customers
	// ---------------------------------------------------------------
	customerIvan := uuid.New()
	customerOOO := uuid.New()

	for _, c := range []struct {
		id   uuid.UUID
		name string
		meta map[string]any
	}{
		{customerIvan, "Иван Петров", map[string]any{"phone": "+7 (900) 111-22-33", "type": "individual"}},
		{customerOOO, "ООО Рога и Копыта", map[string]any{"inn": "9876543210", "type": "company"}},
	} {
		_, err = tx.Exec(ctx,
			`INSERT INTO entities (id, organization_id, kind, name, status, meta, sort_order, ver, upd, created_at, updated_at)
			 VALUES ($1, $2, 'customer', $3, 'active', $4, 0, 1, $5, $6, $7)
			 ON CONFLICT DO NOTHING`,
			c.id, orgID, c.name, mustJSON(c.meta), now, now, now,
		)
		if err != nil {
			fatal("insert customer %s: %v", c.name, err)
		}
		progress("customer: %s", c.name)
	}

	// ---------------------------------------------------------------
	// 9. Orders (5 in different statuses)
	// ---------------------------------------------------------------
	type orderDef struct {
		number     string
		customerID uuid.UUID
		status     string
		items      []struct {
			productIdx int
			qty        float64
		}
	}

	orders := []orderDef{
		{"ORD-001", customerIvan, "draft", []struct {
			productIdx int
			qty        float64
		}{{9, 2}, {5, 1}}},
		{"ORD-002", customerOOO, "confirmed", []struct {
			productIdx int
			qty        float64
		}{{10, 3}, {6, 2}}},
		{"ORD-003", customerIvan, "paid", []struct {
			productIdx int
			qty        float64
		}{{11, 1}, {7, 2}, {8, 1}}},
		{"ORD-004", customerOOO, "shipped", []struct {
			productIdx int
			qty        float64
		}{{0, 5}}},
		{"ORD-005", customerIvan, "delivered", []struct {
			productIdx int
			qty        float64
		}{{12, 2}, {3, 1}}},
	}

	orderIDs := make([]uuid.UUID, len(orders))

	for i, o := range orders {
		orderID := uuid.New()
		orderIDs[i] = orderID
		entityID := uuid.New()

		var totalAmount int64
		for _, item := range o.items {
			p := products[item.productIdx]
			totalAmount += p.selling * int64(item.qty)
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO entities (id, organization_id, kind, name, status, meta, sort_order, ver, upd, created_at, updated_at)
			 VALUES ($1, $2, 'order', $3, 'active', '{}', 0, 1, $4, $5, $6)
			 ON CONFLICT DO NOTHING`,
			entityID, orgID, "Order "+o.number, now, now, now,
		)
		if err != nil {
			fatal("insert order entity %s: %v", o.number, err)
		}

		var paidAt, shippedAt, deliveredAt *time.Time
		switch o.status {
		case "paid":
			t := now.Add(-2 * time.Hour)
			paidAt = &t
		case "shipped":
			t1 := now.Add(-4 * time.Hour)
			t2 := now.Add(-2 * time.Hour)
			paidAt = &t1
			shippedAt = &t2
		case "delivered":
			t1 := now.Add(-6 * time.Hour)
			t2 := now.Add(-4 * time.Hour)
			t3 := now.Add(-1 * time.Hour)
			paidAt = &t1
			shippedAt = &t2
			deliveredAt = &t3
		}

		_, err = tx.Exec(ctx,
			`INSERT INTO orders (id, organization_id, entity_id, number, customer_id, status, subtotal, discount, tax, total, currency, notes, source, warehouse_id, paid_at, shipped_at, delivered_at, ver, upd, created_at, updated_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, 0, 0, $8, 'RUB', '', 'manual', $9, $10, $11, $12, 1, $13, $14, $15)
			 ON CONFLICT DO NOTHING`,
			orderID, orgID, entityID, o.number, o.customerID, o.status,
			totalAmount, totalAmount, warehouseMain,
			paidAt, shippedAt, deliveredAt,
			now, now, now,
		)
		if err != nil {
			fatal("insert order %s: %v", o.number, err)
		}

		for j, item := range o.items {
			p := products[item.productIdx]
			lineTotal := p.selling * int64(item.qty)
			_, err = tx.Exec(ctx,
				`INSERT INTO order_items (id, order_id, organization_id, product_id, name, sku, quantity, unit, unit_price, discount, tax, total, sort_order, ver, upd, created_at)
				 VALUES ($1, $2, $3, $4, $5, '', $6, 'шт', $7, 0, 0, $8, $9, 1, $10, $11)
				 ON CONFLICT DO NOTHING`,
				uuid.New(), orderID, orgID, p.id, p.name, item.qty, p.selling, lineTotal, j+1, now, now,
			)
			if err != nil {
				fatal("insert order item for %s: %v", o.number, err)
			}
		}

		progress("order: %s (status=%s, total=%d)", o.number, o.status, totalAmount)
	}

	// ---------------------------------------------------------------
	// 10. Employees
	// ---------------------------------------------------------------
	empMaria := uuid.New()
	empAlexey := uuid.New()
	empElena := uuid.New()

	type employee struct {
		id       uuid.UUID
		name     string
		role     string
		salary   int64
		schedule string
	}

	employees := []employee{
		{empMaria, "Мария Сидорова", "бариста", 4500000, "5/2"},
		{empAlexey, "Алексей Козлов", "курьер", 3500000, "2/2"},
		{empElena, "Елена Волкова", "менеджер", 6000000, "5/2"},
	}

	for _, e := range employees {
		_, err = tx.Exec(ctx,
			`INSERT INTO entities (id, organization_id, kind, name, status, meta, sort_order, ver, upd, created_at, updated_at)
			 VALUES ($1, $2, 'employee', $3, 'active', '{}', 0, 1, $4, $5, $6)
			 ON CONFLICT DO NOTHING`,
			e.id, orgID, e.name, now, now, now,
		)
		if err != nil {
			fatal("insert employee %s: %v", e.name, err)
		}

		salaryData := mustJSON(map[string]any{
			"amount":   e.salary,
			"currency": "RUB",
			"type":     "monthly",
		})
		_, err = tx.Exec(ctx,
			`INSERT INTO components (id, entity_id, organization_id, type, data, version, ver, upd, created_at, updated_at)
			 VALUES ($1, $2, $3, 'salary', $4, 1, 1, $5, $6, $7)
			 ON CONFLICT (entity_id, type) DO NOTHING`,
			uuid.New(), e.id, orgID, salaryData, now, now, now,
		)
		if err != nil {
			fatal("insert salary component for %s: %v", e.name, err)
		}

		scheduleData := mustJSON(map[string]any{
			"pattern":  e.schedule,
			"position": e.role,
		})
		_, err = tx.Exec(ctx,
			`INSERT INTO components (id, entity_id, organization_id, type, data, version, ver, upd, created_at, updated_at)
			 VALUES ($1, $2, $3, 'schedule', $4, 1, 1, $5, $6, $7)
			 ON CONFLICT (entity_id, type) DO NOTHING`,
			uuid.New(), e.id, orgID, scheduleData, now, now, now,
		)
		if err != nil {
			fatal("insert schedule component for %s: %v", e.name, err)
		}

		progress("employee: %s (%s, salary=%d)", e.name, e.role, e.salary)
	}

	// ---------------------------------------------------------------
	// 11. Accounts (ON CONFLICT DO NOTHING — may already be seeded)
	// ---------------------------------------------------------------
	type accountDef struct {
		code     string
		name     string
		accType  string
		isSystem bool
	}

	defaultAccounts := []accountDef{
		{"10", "Materials", "asset", true},
		{"41", "Goods", "asset", true},
		{"50", "Cash", "asset", true},
		{"51", "Bank account", "asset", true},
		{"60", "Suppliers", "liability", true},
		{"62", "Customers", "asset", true},
		{"70", "Payroll", "liability", true},
		{"90", "Sales", "revenue", true},
		{"91", "Other income/expense", "revenue", true},
		{"99", "Profit and loss", "equity", true},
	}

	for _, a := range defaultAccounts {
		_, err = tx.Exec(ctx,
			`INSERT INTO accounts (id, organization_id, code, name, type, is_system, currency, ver, upd, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, 'RUB', 1, $7, $8)
			 ON CONFLICT (organization_id, code) DO NOTHING`,
			uuid.New(), orgID, a.code, a.name, a.accType, a.isSystem, now, now,
		)
		if err != nil {
			fatal("insert account %s: %v", a.code, err)
		}
	}
	progress("accounts: %d system accounts", len(defaultAccounts))

	// ---------------------------------------------------------------
	// 12. Shifts (today)
	// ---------------------------------------------------------------
	shiftMaria := uuid.New()
	shiftAlexey := uuid.New()

	mariaStart := today.Add(9 * time.Hour)
	mariaEnd := today.Add(18 * time.Hour)
	alexeyStart := today.Add(10 * time.Hour)
	alexeyEnd := today.Add(19 * time.Hour)

	for _, s := range []struct {
		id        uuid.UUID
		empID     uuid.UUID
		empName   string
		startTime time.Time
		endTime   time.Time
	}{
		{shiftMaria, empMaria, "Мария", mariaStart, mariaEnd},
		{shiftAlexey, empAlexey, "Алексей", alexeyStart, alexeyEnd},
	} {
		_, err = tx.Exec(ctx,
			`INSERT INTO shifts (id, organization_id, employee_id, start_time, end_time, break_minutes, status, notes, ver, upd, created_at)
			 VALUES ($1, $2, $3, $4, $5, 60, 'scheduled', '', 1, $6, $7)
			 ON CONFLICT DO NOTHING`,
			s.id, orgID, s.empID, s.startTime, s.endTime, now, now,
		)
		if err != nil {
			fatal("insert shift for %s: %v", s.empName, err)
		}
		progress("shift: %s %s-%s", s.empName, s.startTime.Format("15:04"), s.endTime.Format("15:04"))
	}

	// ---------------------------------------------------------------
	// 13. Locations
	// ---------------------------------------------------------------
	locArbat := uuid.New()
	locCity := uuid.New()

	for _, loc := range []struct {
		id   uuid.UUID
		name string
		lat  float64
		lon  float64
	}{
		{locArbat, "Кофейня на Арбате", 55.7520, 37.5877},
		{locCity, "Кофейня в Сити", 55.7494, 37.5414},
	} {
		locMeta := mustJSON(map[string]any{
			"latitude":  loc.lat,
			"longitude": loc.lon,
			"address":   loc.name,
		})
		_, err = tx.Exec(ctx,
			`INSERT INTO entities (id, organization_id, kind, name, status, meta, sort_order, ver, upd, created_at, updated_at)
			 VALUES ($1, $2, 'location', $3, 'active', $4, 0, 1, $5, $6, $7)
			 ON CONFLICT DO NOTHING`,
			loc.id, orgID, loc.name, locMeta, now, now, now,
		)
		if err != nil {
			fatal("insert location %s: %v", loc.name, err)
		}
		progress("location: %s (%.4f, %.4f)", loc.name, loc.lat, loc.lon)
	}

	// ---------------------------------------------------------------
	// 14. Vehicle
	// ---------------------------------------------------------------
	vehicleID := uuid.New()
	vehicleMeta := mustJSON(map[string]any{
		"plate":  "А777МО77",
		"model":  "Ford Transit",
		"year":   2022,
		"status": "available",
	})
	_, err = tx.Exec(ctx,
		`INSERT INTO entities (id, organization_id, kind, name, status, meta, sort_order, ver, upd, created_at, updated_at)
		 VALUES ($1, $2, 'vehicle', 'Ford Transit', 'active', $3, 0, 1, $4, $5, $6)
		 ON CONFLICT DO NOTHING`,
		vehicleID, orgID, vehicleMeta, now, now, now,
	)
	if err != nil {
		fatal("insert vehicle: %v", err)
	}
	progress("vehicle: Ford Transit (%s)", vehicleID)

	// ---------------------------------------------------------------
	// 15. Route with 2 stops
	// ---------------------------------------------------------------
	routeID := uuid.New()
	routeStart := today.Add(14 * time.Hour)
	routeEnd := today.Add(17 * time.Hour)

	_, err = tx.Exec(ctx,
		`INSERT INTO routes (id, organization_id, name, vehicle_id, driver_id, status, planned_start, planned_end, ver, upd, created_at)
		 VALUES ($1, $2, 'Доставка заказов', $3, $4, 'planned', $5, $6, 1, $7, $8)
		 ON CONFLICT DO NOTHING`,
		routeID, orgID, vehicleID, empAlexey, routeStart, routeEnd, now, now,
	)
	if err != nil {
		fatal("insert route: %v", err)
	}
	progress("route: Доставка заказов (%s)", routeID)

	stop1Arrival := today.Add(14*time.Hour + 30*time.Minute)
	stop2Arrival := today.Add(15*time.Hour + 30*time.Minute)

	for _, s := range []struct {
		locID          uuid.UUID
		address        string
		lat, lon       float64
		sortOrder      int
		plannedArrival time.Time
		orderID        uuid.UUID
	}{
		{locArbat, "ул. Арбат, д. 10", 55.7520, 37.5877, 1, stop1Arrival, orderIDs[3]},
		{locCity, "Пресненская наб., д. 8", 55.7494, 37.5414, 2, stop2Arrival, orderIDs[4]},
	} {
		_, err = tx.Exec(ctx,
			`INSERT INTO route_stops (id, route_id, organization_id, location_id, address, latitude, longitude, sort_order, planned_arrival, status, delivery_ids, notes, ver, upd)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, 'pending', $10, '', 1, $11)
			 ON CONFLICT DO NOTHING`,
			uuid.New(), routeID, orgID, s.locID, s.address, s.lat, s.lon, s.sortOrder, s.plannedArrival,
			[]uuid.UUID{s.orderID}, now,
		)
		if err != nil {
			fatal("insert route stop %s: %v", s.address, err)
		}
		progress("route stop: %s", s.address)
	}

	// ---------------------------------------------------------------
	// 16. Order number sequence
	// ---------------------------------------------------------------
	_, err = tx.Exec(ctx,
		`INSERT INTO order_number_sequences (organization_id, last_number)
		 VALUES ($1, 5)
		 ON CONFLICT (organization_id) DO NOTHING`,
		orgID,
	)
	if err != nil {
		fatal("insert order number sequence: %v", err)
	}
	progress("order_number_sequences: last_number=5")

	// ---------------------------------------------------------------
	// 17. Bank Partner (stub)
	// ---------------------------------------------------------------
	bankPartnerID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO bank_partners (id, name, code, api_base_url, credentials, settings, white_label, active, ver, upd, iat)
		 VALUES ($1, 'Stub Bank', 'stub', NULL, '{}', '{}', $2, true, 1, $3, $4)
		 ON CONFLICT DO NOTHING`,
		bankPartnerID,
		mustJSON(map[string]any{"logo_url": "", "primary_color": "#333333", "app_name": "BizEngine", "bank_name": "Stub Bank"}),
		now, now,
	)
	if err != nil {
		fatal("insert bank partner: %v", err)
	}
	progress("bank partner: Stub Bank (%s)", bankPartnerID)

	// ---------------------------------------------------------------
	// 18. Messenger: Conversations + Messages
	// ---------------------------------------------------------------
	// 18a. Second user for messaging (Мария as separate user)
	userMaria := uuid.New()
	mariaSecret, _ := auth.HashSecret("password")
	mariaName := "Мария Сидорова"
	_, err = tx.Exec(ctx,
		`INSERT INTO users (id, email, password_hash, full_name, phone, is_active, name, secret, ver, upd, created_at, updated_at)
		 VALUES ($1, 'maria@kofenya.test', '', 'Мария Сидорова', NULL, true, $2, $3, 1, $4, $5, $6)
		 ON CONFLICT DO NOTHING`,
		userMaria, &mariaName, mariaSecret, now, now, now,
	)
	if err != nil {
		fatal("insert user maria: %v", err)
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO labors (id, organization_id, user_id, role, permissions, admin, ver, upd, iat, joined_at)
		 VALUES ($1, $2, $3, 'manager', '[]', false, 1, $4, $5, $6)
		 ON CONFLICT DO NOTHING`,
		uuid.New(), orgID, userMaria, now, now, now,
	)
	if err != nil {
		fatal("insert maria labor: %v", err)
	}
	progress("user for messenger: Мария Сидорова (%s)", userMaria)

	// 18b. Entity conversation for order ORD-003
	convOrderID := uuid.New()
	convOrderName := "Order ORD-003 discussion"
	refTypeOrder := "order"
	_, err = tx.Exec(ctx,
		`INSERT INTO conversations (id, organization_id, type, name, reference_type, reference_id, created_by, ver, upd, iat)
		 VALUES ($1, $2, 'entity', $3, $4, $5, $6, 1, $7, $8)
		 ON CONFLICT DO NOTHING`,
		convOrderID, orgID, convOrderName, refTypeOrder, orderIDs[2], userID, now, now,
	)
	if err != nil {
		fatal("insert entity conversation: %v", err)
	}
	// Add members
	for _, mid := range []uuid.UUID{userID, userMaria} {
		_, _ = tx.Exec(ctx,
			`INSERT INTO conversation_members (id, conversation_id, user_id, role, joined_at, ver, upd, iat)
			 VALUES ($1, $2, $3, 'member', $4, 1, $5, $6) ON CONFLICT DO NOTHING`,
			uuid.New(), convOrderID, mid, now, now, now)
	}
	progress("conversation: entity (order ORD-003)")

	// 18c. Direct conversation
	convDirectID := uuid.New()
	_, err = tx.Exec(ctx,
		`INSERT INTO conversations (id, organization_id, type, created_by, ver, upd, iat)
		 VALUES ($1, $2, 'direct', $3, 1, $4, $5)
		 ON CONFLICT DO NOTHING`,
		convDirectID, orgID, userID, now, now,
	)
	if err != nil {
		fatal("insert direct conversation: %v", err)
	}
	for _, mid := range []uuid.UUID{userID, userMaria} {
		_, _ = tx.Exec(ctx,
			`INSERT INTO conversation_members (id, conversation_id, user_id, role, joined_at, ver, upd, iat)
			 VALUES ($1, $2, $3, 'member', $4, 1, $5, $6) ON CONFLICT DO NOTHING`,
			uuid.New(), convDirectID, mid, now, now, now)
	}
	progress("conversation: direct (owner <-> Maria)")

	// 18d. Group conversation
	convGroupID := uuid.New()
	groupName := "Baristas"
	_, err = tx.Exec(ctx,
		`INSERT INTO conversations (id, organization_id, type, name, created_by, ver, upd, iat)
		 VALUES ($1, $2, 'group', $3, $4, 1, $5, $6)
		 ON CONFLICT DO NOTHING`,
		convGroupID, orgID, groupName, userID, now, now,
	)
	if err != nil {
		fatal("insert group conversation: %v", err)
	}
	for _, mid := range []uuid.UUID{userID, userMaria} {
		_, _ = tx.Exec(ctx,
			`INSERT INTO conversation_members (id, conversation_id, user_id, role, joined_at, ver, upd, iat)
			 VALUES ($1, $2, $3, 'member', $4, 1, $5, $6) ON CONFLICT DO NOTHING`,
			uuid.New(), convGroupID, mid, now, now, now)
	}
	progress("conversation: group (Baristas)")

	// 18e. Messages
	seedMessages := []struct {
		convID  uuid.UUID
		sender  uuid.UUID
		content string
		cType   string
		attach  string
	}{
		{convOrderID, userID, "[System] Order ORD-003 confirmed", "system", "[]"},
		{convOrderID, userID, "[System] Order paid (950.00, card)", "system", "[]"},
		{convOrderID, userMaria, "Customer wants extra sugar", "text", "[]"},
		{convOrderID, userID, "Got it, will add", "text", "[]"},
		{convDirectID, userID, "Maria, can you cover the evening shift?", "text", "[]"},
		{convDirectID, userMaria, "Sure, what time?", "text", "[]"},
		{convDirectID, userID, "18:00 to 22:00", "text", "[]"},
		{convGroupID, userID, "New coffee beans arriving tomorrow", "text", "[]"},
		{convGroupID, userMaria, "Great, we're running low on Arabica", "text", "[]"},
		{convGroupID, userID, "Check the order",
			"text",
			string(mustJSON([]map[string]any{{"type": "entity_ref", "entity_kind": "order", "entity_id": orderIDs[3].String(), "preview": map[string]any{"number": "ORD-004", "status": "shipped"}}})),
		},
	}

	var lastMsgIDs [3]uuid.UUID
	var lastMsgPreviews [3]string
	convList := []uuid.UUID{convOrderID, convDirectID, convGroupID}

	for _, sm := range seedMessages {
		msgID := uuid.New()
		msgTime := now.Add(-time.Duration(10-len(seedMessages)) * time.Minute)
		_, err = tx.Exec(ctx,
			`INSERT INTO messages (id, conversation_id, organization_id, sender_id, content, content_type, attachments, ver, upd, iat)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, 1, $8, $9) ON CONFLICT DO NOTHING`,
			msgID, sm.convID, orgID, sm.sender, sm.content, sm.cType, []byte(sm.attach), msgTime, msgTime,
		)
		if err != nil {
			fatal("insert message: %v", err)
		}
		// Track last message per conversation
		for i, cid := range convList {
			if cid == sm.convID {
				lastMsgIDs[i] = msgID
				preview := sm.content
				if len(preview) > 100 {
					preview = preview[:100]
				}
				lastMsgPreviews[i] = preview
			}
		}
	}

	// Update last_message on conversations
	for i, cid := range convList {
		if lastMsgIDs[i] != uuid.Nil {
			_, _ = tx.Exec(ctx,
				`UPDATE conversations SET last_message_id = $1, last_message_at = $2, last_message_preview = $3 WHERE id = $4`,
				lastMsgIDs[i], now, lastMsgPreviews[i], cid)
		}
	}
	progress("messages: %d messages across 3 conversations", len(seedMessages))

	// ---------------------------------------------------------------
	// Commit
	// ---------------------------------------------------------------
	if err := tx.Commit(ctx); err != nil {
		fatal("commit transaction: %v", err)
	}

	fmt.Println()
	progress("seed completed successfully")
	fmt.Printf("  Organization ID: %s\n", orgID)
	fmt.Printf("  User ID:         %s\n", userID)
	fmt.Printf("  Phone:           79991234567\n")
	fmt.Printf("  Secret:          %s\n", secret)
}

func mustJSON(v any) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}

func progress(format string, args ...any) {
	fmt.Printf("[seed] "+format+"\n", args...)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[seed] FATAL: "+format+"\n", args...)
	os.Exit(1)
}
