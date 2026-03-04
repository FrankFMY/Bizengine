package views

import (
	"context"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- mock publisher ---

type mockPublisher struct {
	mu        sync.Mutex
	snapshots []ViewSnapshotMsg
	tableDiff []TableDiffMsg
	viewDiff  []ViewDiffMsg
}

func (m *mockPublisher) SendSnapshot(_ context.Context, _ string, msg ViewSnapshotMsg) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshots = append(m.snapshots, msg)
	return nil
}

func (m *mockPublisher) SendTableDiff(_ context.Context, _ uuid.UUID, msg TableDiffMsg) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tableDiff = append(m.tableDiff, msg)
	return nil
}

func (m *mockPublisher) SendViewDiff(_ context.Context, _ string, msg ViewDiffMsg) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.viewDiff = append(m.viewDiff, msg)
	return nil
}

// --- test factory ---

func testFactory(refs []DataRef, tables map[string]map[string]any) ViewFactory {
	var version int64
	return func(_ context.Context, _ *pgxpool.Pool, _ uuid.UUID, _ map[string]any) (*ViewResult, error) {
		version++
		return &ViewResult{Refs: refs, Tables: tables, Version: version}, nil
	}
}

func TestManager_Subscribe_Snapshot(t *testing.T) {
	reg := NewRegistry()
	pub := &mockPublisher{}

	refs := []DataRef{{Table: "orders", ID: "id-1", Fields: []string{"status"}}}
	tables := map[string]map[string]any{
		"orders": {"id-1": map[string]any{"status": "active"}},
	}

	reg.Register(&ViewDef{
		Key:     "orders_list",
		Tables:  []TableDep{{Table: "orders", Columns: []string{"status"}}},
		Factory: testFactory(refs, tables),
	})

	mgr := NewManager(reg, nil, pub)
	wsID := uuid.New()
	userID := uuid.New()
	seanceID := "seance-1"

	result, err := mgr.Subscribe(context.Background(), seanceID, wsID, userID, "orders_list", nil)
	require.NoError(t, err)
	assert.NotEmpty(t, result.ParamsHash)
	assert.Equal(t, int64(1), result.Version)

	// Snapshot sent
	require.Len(t, pub.snapshots, 1)
	assert.Equal(t, "orders_list", pub.snapshots[0].View)
	assert.Len(t, pub.snapshots[0].Refs, 1)

	// Active subscriptions
	active := mgr.ActiveSubscriptions(seanceID)
	require.Len(t, active, 1)
	assert.Equal(t, "orders_list", active[0].View)
}

func TestManager_Subscribe_Duplicate(t *testing.T) {
	reg := NewRegistry()
	pub := &mockPublisher{}

	reg.Register(&ViewDef{
		Key:     "test_view",
		Tables:  []TableDep{{Table: "t", Columns: []string{"c"}}},
		Factory: testFactory(nil, nil),
	})

	mgr := NewManager(reg, nil, pub)
	wsID := uuid.New()
	seanceID := "s1"

	r1, err := mgr.Subscribe(context.Background(), seanceID, wsID, uuid.New(), "test_view", nil)
	require.NoError(t, err)

	r2, err := mgr.Subscribe(context.Background(), seanceID, wsID, uuid.New(), "test_view", nil)
	require.NoError(t, err)

	assert.Equal(t, r1.ParamsHash, r2.ParamsHash)
	// Only one snapshot sent (duplicate returns early).
	assert.Len(t, pub.snapshots, 1)
}

func TestManager_Subscribe_NotFound(t *testing.T) {
	mgr := NewManager(NewRegistry(), nil, &mockPublisher{})
	_, err := mgr.Subscribe(context.Background(), "s1", uuid.New(), uuid.New(), "nonexistent", nil)
	assert.Error(t, err)
}

func TestManager_Unsubscribe(t *testing.T) {
	reg := NewRegistry()
	pub := &mockPublisher{}

	refs := []DataRef{{Table: "orders", ID: "id-1", Fields: []string{"status"}}}
	tables := map[string]map[string]any{
		"orders": {"id-1": map[string]any{"status": "active"}},
	}

	reg.Register(&ViewDef{
		Key:     "orders_list",
		Tables:  []TableDep{{Table: "orders", Columns: []string{"status"}}},
		Factory: testFactory(refs, tables),
	})

	mgr := NewManager(reg, nil, pub)
	wsID := uuid.New()
	seanceID := "s1"

	result, _ := mgr.Subscribe(context.Background(), seanceID, wsID, uuid.New(), "orders_list", nil)
	mgr.Unsubscribe(seanceID, result.ParamsHash)

	assert.Empty(t, mgr.ActiveSubscriptions(seanceID))
}

func TestManager_UnsubscribeAll(t *testing.T) {
	reg := NewRegistry()
	pub := &mockPublisher{}

	reg.Register(&ViewDef{
		Key:     "v1",
		Tables:  []TableDep{{Table: "t1", Columns: []string{"c"}}},
		Factory: testFactory([]DataRef{{Table: "t1", ID: "1"}}, map[string]map[string]any{"t1": {"1": map[string]any{}}}),
	})
	reg.Register(&ViewDef{
		Key:         "v2",
		Tables:      []TableDep{{Table: "t2", Columns: []string{"c"}}},
		ParamSchema: map[string]string{"x": "string"},
		Factory:     testFactory([]DataRef{{Table: "t2", ID: "2"}}, map[string]map[string]any{"t2": {"2": map[string]any{}}}),
	})

	mgr := NewManager(reg, nil, pub)
	wsID := uuid.New()
	seanceID := "s1"

	mgr.Subscribe(context.Background(), seanceID, wsID, uuid.New(), "v1", nil)
	mgr.Subscribe(context.Background(), seanceID, wsID, uuid.New(), "v2", map[string]any{"x": "val"})

	assert.Len(t, mgr.ActiveSubscriptions(seanceID), 2)

	mgr.UnsubscribeAll(seanceID)
	assert.Empty(t, mgr.ActiveSubscriptions(seanceID))
}

func TestManager_Invalidate_ViewDiff(t *testing.T) {
	reg := NewRegistry()
	pub := &mockPublisher{}

	callCount := 0
	factory := func(_ context.Context, _ *pgxpool.Pool, _ uuid.UUID, _ map[string]any) (*ViewResult, error) {
		callCount++
		if callCount == 1 {
			return &ViewResult{
				Refs:    []DataRef{{Table: "orders", ID: "1", Fields: []string{"status"}}},
				Tables:  map[string]map[string]any{"orders": {"1": map[string]any{"status": "draft"}}},
				Version: 1,
			}, nil
		}
		// Second call: ref list changed (new order added).
		return &ViewResult{
			Refs: []DataRef{
				{Table: "orders", ID: "1", Fields: []string{"status"}},
				{Table: "orders", ID: "2", Fields: []string{"status"}},
			},
			Tables: map[string]map[string]any{
				"orders": {
					"1": map[string]any{"status": "active"},
					"2": map[string]any{"status": "draft"},
				},
			},
			Version: 2,
		}, nil
	}

	wsID := uuid.New()
	reg.Register(&ViewDef{
		Key:     "orders_list",
		Tables:  []TableDep{{Table: "orders", Columns: []string{"status"}}},
		Factory: factory,
	})

	mgr := NewManager(reg, nil, pub)
	seanceID := "s1"

	mgr.Subscribe(context.Background(), seanceID, wsID, uuid.New(), "orders_list", nil)
	require.Len(t, pub.snapshots, 1)

	// Trigger invalidation.
	mgr.Invalidate(context.Background(), ChangeEvent{
		Table:       "orders",
		RowID:       "1",
		WorkspaceID: wsID,
	})

	// view_diff sent because refs changed.
	require.Len(t, pub.viewDiff, 1)
	assert.Equal(t, "orders_list", pub.viewDiff[0].View)
	assert.NotEmpty(t, pub.viewDiff[0].RefsPatch)
}

func TestManager_ParamValidation(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&ViewDef{
		Key:         "filtered",
		Tables:      []TableDep{{Table: "t", Columns: []string{"c"}}},
		ParamSchema: map[string]string{"id": "uuid"},
		Factory:     testFactory(nil, nil),
	})

	mgr := NewManager(reg, nil, &mockPublisher{})

	// Invalid UUID param
	_, err := mgr.Subscribe(context.Background(), "s1", uuid.New(), uuid.New(), "filtered", map[string]any{"id": "not-a-uuid"})
	assert.Error(t, err)

	// Valid UUID param
	_, err = mgr.Subscribe(context.Background(), "s1", uuid.New(), uuid.New(), "filtered", map[string]any{"id": uuid.New().String()})
	assert.NoError(t, err)
}
