package views

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/pkg/types"
)

// collectingPublisher records all messages for assertion.
type collectingPublisher struct {
	mu        sync.Mutex
	snapshots []ViewSnapshotMsg
	tableDiff []TableDiffMsg
	viewDiff  []ViewDiffMsg
}

func (p *collectingPublisher) SendSnapshot(_ context.Context, _ string, msg ViewSnapshotMsg) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.snapshots = append(p.snapshots, msg)
	return nil
}

func (p *collectingPublisher) SendTableDiff(_ context.Context, _ uuid.UUID, msg TableDiffMsg) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tableDiff = append(p.tableDiff, msg)
	return nil
}

func (p *collectingPublisher) SendViewDiff(_ context.Context, _ string, msg ViewDiffMsg) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.viewDiff = append(p.viewDiff, msg)
	return nil
}

// simulatedFactory returns different results on sequential calls to simulate data changes.
type simulatedFactory struct {
	calls   int
	results []*ViewResult
}

func (f *simulatedFactory) build(_ context.Context, _ *pgxpool.Pool, _ uuid.UUID, _ map[string]any) (*ViewResult, error) {
	idx := f.calls
	if idx >= len(f.results) {
		idx = len(f.results) - 1
	}
	f.calls++
	return f.results[idx], nil
}

// TestE2E_SubscribeEventInvalidateDiff tests the full lifecycle:
// 1. Subscribe to view → snapshot sent
// 2. Event fires → triggers produce ChangeEvent
// 3. Manager.Invalidate → re-runs factory → computes view_diff
// 4. Verify diff contains correct refs_patch and new tables
func TestE2E_SubscribeEventInvalidateDiff(t *testing.T) {
	ctx := context.Background()
	wsID := uuid.New()
	userID := uuid.New()
	seanceID := "e2e-seance"

	pub := &collectingPublisher{}
	reg := NewRegistry()

	// Simulate a stock list view with changing data.
	factory := &simulatedFactory{
		results: []*ViewResult{
			// Initial: 2 products
			{
				Refs: []DataRef{
					{Table: "stock_levels", ID: "prod-1", Fields: []string{"quantity"}},
					{Table: "stock_levels", ID: "prod-2", Fields: []string{"quantity"}},
				},
				Tables: map[string]map[string]any{
					"stock_levels": {
						"prod-1": map[string]any{"product_name": "Widget", "quantity": 100.0},
						"prod-2": map[string]any{"product_name": "Gadget", "quantity": 50.0},
					},
				},
				Version: 1,
			},
			// After stock receipt: prod-1 quantity changed, new prod-3 appeared
			{
				Refs: []DataRef{
					{Table: "stock_levels", ID: "prod-1", Fields: []string{"quantity"}},
					{Table: "stock_levels", ID: "prod-2", Fields: []string{"quantity"}},
					{Table: "stock_levels", ID: "prod-3", Fields: []string{"quantity"}},
				},
				Tables: map[string]map[string]any{
					"stock_levels": {
						"prod-1": map[string]any{"product_name": "Widget", "quantity": 150.0},
						"prod-2": map[string]any{"product_name": "Gadget", "quantity": 50.0},
						"prod-3": map[string]any{"product_name": "Doohickey", "quantity": 25.0},
					},
				},
				Version: 2,
			},
		},
	}

	reg.Register(&ViewDef{
		Key:    "warehouse_stock_list",
		Tables: []TableDep{{Table: "stock_levels", Columns: []string{"quantity"}}},
		ParamSchema: map[string]string{
			"warehouse_id": "uuid",
		},
		Factory: factory.build,
	})

	mgr := NewManager(reg, nil, pub)

	// Step 1: Subscribe
	whID := uuid.New()
	result, err := mgr.Subscribe(ctx, seanceID, wsID, userID, "warehouse_stock_list", map[string]any{
		"warehouse_id": whID.String(),
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.ParamsHash)
	assert.Equal(t, int64(1), result.Version)

	// Snapshot was sent
	require.Len(t, pub.snapshots, 1)
	snap := pub.snapshots[0]
	assert.Equal(t, "warehouse_stock_list", snap.View)
	assert.Len(t, snap.Refs, 2)
	assert.Len(t, snap.Tables["stock_levels"], 2)

	// Step 2: Simulate event → trigger → ChangeEvent
	ev := types.Event{
		Type:        "warehouse.stock.received",
		WorkspaceID: wsID,
		Data:        []byte(`{"product_id":"prod-1"}`),
	}
	changes := EventToChanges(ev)
	require.Len(t, changes, 1)
	assert.Equal(t, "stock_levels", changes[0].Table)
	assert.Equal(t, "prod-1", changes[0].RowID)
	assert.Equal(t, wsID, changes[0].WorkspaceID)

	// Step 3: Invalidate
	mgr.Invalidate(ctx, changes[0])

	// Step 4: Verify view_diff was sent (refs changed: prod-3 was added)
	require.Len(t, pub.viewDiff, 1)
	vd := pub.viewDiff[0]
	assert.Equal(t, "warehouse_stock_list", vd.View)
	assert.Equal(t, result.ParamsHash, vd.ParamsHash)
	assert.Equal(t, int64(2), vd.Version)

	// refs_patch should contain an "add" for prod-3
	hasAdd := false
	for _, op := range vd.RefsPatch {
		if op.Op == "add" {
			hasAdd = true
		}
	}
	assert.True(t, hasAdd, "expected refs_patch to contain an add op for prod-3")

	// New tables should contain prod-3 data
	require.NotNil(t, vd.Tables)
	assert.Contains(t, vd.Tables, "stock_levels")

	// Subscription version should be updated
	active := mgr.ActiveSubscriptions(seanceID)
	require.Len(t, active, 1)
	assert.Equal(t, int64(2), active[0].Version)
}

// TestE2E_SubscribeUnsubscribeCleanup verifies RefCount GC works.
func TestE2E_SubscribeUnsubscribeCleanup(t *testing.T) {
	ctx := context.Background()
	wsID := uuid.New()
	pub := &collectingPublisher{}
	reg := NewRegistry()

	refs := []DataRef{
		{Table: "orders", ID: "ord-1", Fields: []string{"status"}},
		{Table: "orders", ID: "ord-2", Fields: []string{"status"}},
	}
	tables := map[string]map[string]any{
		"orders": {
			"ord-1": map[string]any{"status": "active"},
			"ord-2": map[string]any{"status": "draft"},
		},
	}

	factory := func(_ context.Context, _ *pgxpool.Pool, _ uuid.UUID, _ map[string]any) (*ViewResult, error) {
		return &ViewResult{Refs: refs, Tables: tables, Version: 1}, nil
	}

	reg.Register(&ViewDef{
		Key:     "orders_list",
		Tables:  []TableDep{{Table: "orders", Columns: []string{"status"}}},
		Factory: factory,
	})

	mgr := NewManager(reg, nil, pub)

	// Two different seances subscribe to the same view.
	r1, _ := mgr.Subscribe(ctx, "s1", wsID, uuid.New(), "orders_list", nil)
	r2, _ := mgr.Subscribe(ctx, "s2", wsID, uuid.New(), "orders_list", nil)
	assert.NotEmpty(t, r1.ParamsHash)
	assert.NotEmpty(t, r2.ParamsHash)

	// Both subscriptions are active.
	assert.Len(t, mgr.ActiveSubscriptions("s1"), 1)
	assert.Len(t, mgr.ActiveSubscriptions("s2"), 1)

	// Unsubscribe s1.
	mgr.UnsubscribeAll("s1")
	assert.Empty(t, mgr.ActiveSubscriptions("s1"))
	assert.Len(t, mgr.ActiveSubscriptions("s2"), 1) // s2 still active

	// Data store should still have rows (s2 holds refs).
	mgr.mu.RLock()
	store := mgr.dataStore[wsID.String()]
	mgr.mu.RUnlock()
	require.NotNil(t, store)
	assert.NotNil(t, store.GetRow("orders", "ord-1"))

	// Unsubscribe s2 → rows should be GC'd.
	mgr.UnsubscribeAll("s2")
	assert.Nil(t, store.GetRow("orders", "ord-1"))
	assert.Nil(t, store.GetRow("orders", "ord-2"))
}

// TestE2E_MultipleViewsSameTable verifies that a change to one table
// invalidates all views depending on it.
func TestE2E_MultipleViewsSameTable(t *testing.T) {
	ctx := context.Background()
	wsID := uuid.New()
	pub := &collectingPublisher{}
	reg := NewRegistry()

	callCount := map[string]int{}
	mkFactory := func(viewKey string) ViewFactory {
		return func(_ context.Context, _ *pgxpool.Pool, _ uuid.UUID, _ map[string]any) (*ViewResult, error) {
			callCount[viewKey]++
			return &ViewResult{
				Refs:    []DataRef{{Table: "orders", ID: "o1"}},
				Tables:  map[string]map[string]any{"orders": {"o1": map[string]any{"x": callCount[viewKey]}}},
				Version: int64(callCount[viewKey]),
			}, nil
		}
	}

	reg.Register(&ViewDef{Key: "view_a", Tables: []TableDep{{Table: "orders", Columns: []string{"status"}}}, Factory: mkFactory("view_a")})
	reg.Register(&ViewDef{Key: "view_b", Tables: []TableDep{{Table: "orders", Columns: []string{"total"}}}, Factory: mkFactory("view_b")})

	mgr := NewManager(reg, nil, pub)

	mgr.Subscribe(ctx, "s1", wsID, uuid.New(), "view_a", nil)
	mgr.Subscribe(ctx, "s1", wsID, uuid.New(), "view_b", nil)

	// Both subscribed = 2 snapshots.
	assert.Len(t, pub.snapshots, 2)

	// Invalidate "orders" table.
	mgr.Invalidate(ctx, ChangeEvent{Table: "orders", RowID: "o1", WorkspaceID: wsID})

	// Both view factories should have been called again (2 subscribe + 2 invalidate = 4 total).
	assert.Equal(t, 2, callCount["view_a"])
	assert.Equal(t, 2, callCount["view_b"])
}

// TestE2E_SyncFullSnapshot verifies that sync sends full snapshot when version diff > 10.
func TestE2E_SyncFullSnapshot(t *testing.T) {
	ctx := context.Background()
	wsID := uuid.New()
	pub := &collectingPublisher{}
	reg := NewRegistry()

	version := int64(0)
	reg.Register(&ViewDef{
		Key:    "test_view",
		Tables: []TableDep{{Table: "t", Columns: []string{"c"}}},
		Factory: func(_ context.Context, _ *pgxpool.Pool, _ uuid.UUID, _ map[string]any) (*ViewResult, error) {
			version++
			return &ViewResult{Refs: []DataRef{{Table: "t", ID: "1"}}, Tables: map[string]map[string]any{"t": {"1": map[string]any{}}}, Version: version}, nil
		},
	})

	mgr := NewManager(reg, nil, pub)
	r, _ := mgr.Subscribe(ctx, "s1", wsID, uuid.New(), "test_view", nil)

	// Simulate the subscription being at version 50 (server-side).
	mgr.mu.Lock()
	mgr.subs["s1"][0].Version = 50
	mgr.mu.Unlock()

	// Client thinks it's at version 1 → diff > 10 → full snapshot.
	err := mgr.Sync(ctx, "s1", wsID, SyncRequest{
		Views: []SyncView{{View: "test_view", ParamsHash: r.ParamsHash, Version: 1}},
	})
	require.NoError(t, err)

	// 1 initial snapshot + 1 sync snapshot = 2
	assert.Len(t, pub.snapshots, 2)
}

// TestE2E_InvalidateNoSubscribers does not panic or send messages.
func TestE2E_InvalidateNoSubscribers(t *testing.T) {
	pub := &collectingPublisher{}
	reg := NewRegistry()
	reg.Register(&ViewDef{
		Key:    "orphan",
		Tables: []TableDep{{Table: "orphan_table", Columns: []string{"x"}}},
		Factory: func(_ context.Context, _ *pgxpool.Pool, _ uuid.UUID, _ map[string]any) (*ViewResult, error) {
			t.Fatal("factory should not be called")
			return nil, nil
		},
	})

	mgr := NewManager(reg, nil, pub)
	mgr.Invalidate(context.Background(), ChangeEvent{Table: "orphan_table", RowID: "1", WorkspaceID: uuid.New()})

	assert.Empty(t, pub.snapshots)
	assert.Empty(t, pub.viewDiff)
	assert.Empty(t, pub.tableDiff)
}

// TestE2E_EventTriggerMapping verifies the full event → ChangeEvent mapping for all domains.
func TestE2E_EventTriggerMapping(t *testing.T) {
	tests := []struct {
		eventType     string
		expectedTable string
		needsEntity   bool
	}{
		{"entity.created", "entities", true},
		{"entity.updated", "entities", true},
		{"order.created", "orders", true},
		{"order.confirmed", "orders", true},
		{"order.paid", "orders", true},
		{"order.cancelled", "orders", true},
		{"catalog.product.created", "catalog_products", true},
		{"catalog.category.updated", "catalog_categories", true},
		{"hr.employee.hired", "employees", true},
		{"hr.shift.created", "shifts", true},
		{"hr.timesheet.approved", "timesheets", true},
		{"finance.transaction.posted", "transactions", true},
		{"finance.invoice.paid", "invoices", true},
		{"finance.account.created", "accounts", true},
		{"logistics.route.started", "routes", true},
		{"logistics.geo.updated", "geo_points", true},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			ev := types.Event{
				Type:        tt.eventType,
				WorkspaceID: uuid.New(),
			}
			if tt.needsEntity {
				eid := uuid.New()
				ev.EntityID = &eid
			}

			changes := EventToChanges(ev)
			require.NotEmpty(t, changes, "expected changes for event %s", tt.eventType)
			assert.Equal(t, tt.expectedTable, changes[0].Table)
		})
	}
}
