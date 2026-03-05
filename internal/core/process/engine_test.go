package process

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/dsl"
	"github.com/bizengine/engine/pkg/types"
)

// --- mocks ---

type mockProcessRepo struct {
	definitions map[string]*DefinitionRecord
	instances   map[uuid.UUID]*Instance
}

func newMockRepo() *mockProcessRepo {
	return &mockProcessRepo{
		definitions: make(map[string]*DefinitionRecord),
		instances:   make(map[uuid.UUID]*Instance),
	}
}

func (m *mockProcessRepo) UpsertDefinition(_ context.Context, def *DefinitionRecord) error {
	m.definitions[def.ID] = def
	return nil
}

func (m *mockProcessRepo) GetDefinition(_ context.Context, id string, _ *uuid.UUID) (*DefinitionRecord, error) {
	if def, ok := m.definitions[id]; ok {
		return def, nil
	}
	return nil, nil
}

func (m *mockProcessRepo) ListDefinitions(_ context.Context, _ uuid.UUID) ([]DefinitionRecord, error) {
	var defs []DefinitionRecord
	for _, d := range m.definitions {
		defs = append(defs, *d)
	}
	return defs, nil
}

func (m *mockProcessRepo) CreateInstance(_ context.Context, inst *Instance) error {
	cp := *inst
	m.instances[inst.ID] = &cp
	return nil
}

func (m *mockProcessRepo) GetInstance(_ context.Context, id uuid.UUID) (*Instance, error) {
	if inst, ok := m.instances[id]; ok {
		return inst, nil
	}
	return nil, nil
}

func (m *mockProcessRepo) GetActiveByEntity(_ context.Context, entityID uuid.UUID) ([]Instance, error) {
	var result []Instance
	for _, inst := range m.instances {
		if inst.EntityID == entityID && inst.Status == "active" {
			result = append(result, *inst)
		}
	}
	return result, nil
}

func (m *mockProcessRepo) ListInstances(_ context.Context, _ uuid.UUID, status *string, _, _ int) ([]Instance, int, error) {
	var result []Instance
	for _, inst := range m.instances {
		if status == nil || inst.Status == *status {
			result = append(result, *inst)
		}
	}
	return result, len(result), nil
}

func (m *mockProcessRepo) UpdateInstance(_ context.Context, inst *Instance) error {
	cp := *inst
	m.instances[inst.ID] = &cp
	return nil
}

func (m *mockProcessRepo) DeleteDefinition(_ context.Context, id string, _ uuid.UUID) error {
	delete(m.definitions, id)
	return nil
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

// --- test definition ---

func simpleOrderDef() *dsl.ProcessDefinition {
	return &dsl.ProcessDefinition{
		ID:         "order_fulfillment",
		Name:       "Order Fulfillment",
		TriggerOn:  "order.created",
		EntityKind: "order",
		InitState:  "new",
		States: map[string]dsl.State{
			"new": {
				Name: "New",
				Transitions: []dsl.Transition{
					{To: "confirmed", Event: "order.confirmed"},
					{To: "cancelled", Event: "order.cancelled"},
				},
			},
			"confirmed": {
				Name: "Confirmed",
				Transitions: []dsl.Transition{
					{To: "paid", Event: "order.paid"},
					{To: "cancelled", Event: "order.cancelled"},
				},
			},
			"paid": {
				Name: "Paid",
				OnEnter: []dsl.Action{
					{Type: "emit_event", Params: map[string]any{"event_type": "notification.order_paid"}},
				},
				Transitions: []dsl.Transition{
					{To: "shipped", Event: "order.shipped"},
				},
			},
			"shipped": {
				Name: "Shipped",
				Transitions: []dsl.Transition{
					{To: "delivered", Event: "order.delivered"},
				},
			},
			"delivered": {Name: "Delivered", Terminal: true},
			"cancelled": {Name: "Cancelled", Terminal: true},
		},
	}
}

// --- tests ---

func TestStartProcess(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	bus := &mockBus{}
	engine := NewEngine(repo, bus)

	def := simpleOrderDef()
	engine.RegisterDefinition(def)

	entityID := uuid.New()
	orgID := uuid.New()

	ev := types.Event{
		ID:          uuid.New(),
		OrganizationID: orgID,
		EntityID:    &entityID,
		Type:        "order.created",
	}

	err := engine.HandleEvent(ctx, ev)
	require.NoError(t, err)

	assert.Len(t, repo.instances, 1)
	for _, inst := range repo.instances {
		assert.Equal(t, "new", inst.CurrentState)
		assert.Equal(t, "active", inst.Status)
		assert.Equal(t, "order_fulfillment", inst.DefinitionID)
		assert.Equal(t, entityID, inst.EntityID)
	}
}

func TestAdvanceProcess(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	bus := &mockBus{}
	engine := NewEngine(repo, bus)

	def := simpleOrderDef()
	engine.RegisterDefinition(def)

	entityID := uuid.New()
	orgID := uuid.New()

	// Start process
	engine.HandleEvent(ctx, types.Event{
		ID: uuid.New(), OrganizationID: orgID, EntityID: &entityID, Type: "order.created",
	})

	// Confirm
	engine.HandleEvent(ctx, types.Event{
		ID: uuid.New(), OrganizationID: orgID, EntityID: &entityID, Type: "order.confirmed",
	})

	for _, inst := range repo.instances {
		assert.Equal(t, "confirmed", inst.CurrentState)
	}

	// Pay (triggers on_enter emit_event)
	bus.published = nil
	engine.HandleEvent(ctx, types.Event{
		ID: uuid.New(), OrganizationID: orgID, EntityID: &entityID, Type: "order.paid",
	})

	for _, inst := range repo.instances {
		assert.Equal(t, "paid", inst.CurrentState)
	}
	// on_enter of "paid" emits notification.order_paid
	found := false
	for _, ev := range bus.published {
		if ev.Type == "notification.order_paid" {
			found = true
		}
	}
	assert.True(t, found, "expected notification.order_paid event")
}

func TestProcessToTerminal(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	bus := &mockBus{}
	engine := NewEngine(repo, bus)
	engine.RegisterDefinition(simpleOrderDef())

	entityID := uuid.New()
	orgID := uuid.New()

	events := []string{"order.created", "order.confirmed", "order.paid", "order.shipped", "order.delivered"}
	for _, evType := range events {
		engine.HandleEvent(ctx, types.Event{
			ID: uuid.New(), OrganizationID: orgID, EntityID: &entityID, Type: evType,
		})
	}

	for _, inst := range repo.instances {
		assert.Equal(t, "delivered", inst.CurrentState)
		assert.Equal(t, "completed", inst.Status)
		assert.NotNil(t, inst.CompletedAt)

		var history []HistoryEntry
		json.Unmarshal(inst.History, &history)
		assert.Len(t, history, 4) // new→confirmed→paid→shipped→delivered = 4 transitions
	}
}

func TestCancelFromNew(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	bus := &mockBus{}
	engine := NewEngine(repo, bus)
	engine.RegisterDefinition(simpleOrderDef())

	entityID := uuid.New()
	orgID := uuid.New()

	engine.HandleEvent(ctx, types.Event{
		ID: uuid.New(), OrganizationID: orgID, EntityID: &entityID, Type: "order.created",
	})
	engine.HandleEvent(ctx, types.Event{
		ID: uuid.New(), OrganizationID: orgID, EntityID: &entityID, Type: "order.cancelled",
	})

	for _, inst := range repo.instances {
		assert.Equal(t, "cancelled", inst.CurrentState)
		assert.Equal(t, "completed", inst.Status)
	}
}

func TestConditionEvaluation(t *testing.T) {
	tests := []struct {
		name     string
		cond     *dsl.Condition
		data     map[string]any
		expected bool
	}{
		{"eq match", &dsl.Condition{Field: "status", Operator: "eq", Value: "paid"}, map[string]any{"status": "paid"}, true},
		{"eq no match", &dsl.Condition{Field: "status", Operator: "eq", Value: "paid"}, map[string]any{"status": "new"}, false},
		{"neq match", &dsl.Condition{Field: "status", Operator: "neq", Value: "draft"}, map[string]any{"status": "new"}, true},
		{"gt match", &dsl.Condition{Field: "total", Operator: "gt", Value: 100.0}, map[string]any{"total": 200.0}, true},
		{"gt no match", &dsl.Condition{Field: "total", Operator: "gt", Value: 100.0}, map[string]any{"total": 50.0}, false},
		{"in match", &dsl.Condition{Field: "status", Operator: "in", Value: []any{"paid", "confirmed"}}, map[string]any{"status": "paid"}, true},
		{"in no match", &dsl.Condition{Field: "status", Operator: "in", Value: []any{"paid", "confirmed"}}, map[string]any{"status": "new"}, false},
		{"exists true", &dsl.Condition{Field: "tracking", Operator: "exists", Value: true}, map[string]any{"tracking": "ABC"}, true},
		{"exists false", &dsl.Condition{Field: "tracking", Operator: "exists", Value: true}, map[string]any{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, evaluateCondition(tt.cond, tt.data))
		})
	}
}

func TestNoMatchingTransition(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	bus := &mockBus{}
	engine := NewEngine(repo, bus)
	engine.RegisterDefinition(simpleOrderDef())

	entityID := uuid.New()
	orgID := uuid.New()

	engine.HandleEvent(ctx, types.Event{
		ID: uuid.New(), OrganizationID: orgID, EntityID: &entityID, Type: "order.created",
	})

	// Try to ship from "new" — no transition should match
	engine.HandleEvent(ctx, types.Event{
		ID: uuid.New(), OrganizationID: orgID, EntityID: &entityID, Type: "order.shipped",
	})

	for _, inst := range repo.instances {
		assert.Equal(t, "new", inst.CurrentState) // unchanged
	}
}

// --- integration tests with real YAML definitions ---

func loadYAML(t *testing.T, file string) *dsl.ProcessDefinition {
	t.Helper()
	def, err := dsl.ParseFile(file)
	require.NoError(t, err, "failed to parse %s", file)
	errs := dsl.Validate(def)
	require.Empty(t, errs, "validation errors in %s: %v", file, errs)
	return def
}

func TestYAMLOrderFulfillmentFullCycle(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	bus := &mockBus{}
	engine := NewEngine(repo, bus)
	engine.RegisterDefinition(loadYAML(t, "../../../processes/order_fulfillment.yaml"))

	orgID := uuid.New()
	entityID := uuid.New()
	ev := func(eventType string) { //nolint
		engine.HandleEvent(ctx, types.Event{
			ID: uuid.New(), OrganizationID: orgID, EntityID: &entityID, Type: eventType,
		})
	}

	ev("order.created")
	for _, inst := range repo.instances {
		assert.Equal(t, "new", inst.CurrentState)
	}

	ev("order.confirmed")
	ev("order.paid")
	ev("order.assembling")
	ev("order.shipped")
	ev("order.delivered")

	for _, inst := range repo.instances {
		assert.Equal(t, "delivered", inst.CurrentState)
		assert.Equal(t, "completed", inst.Status)
		assert.NotNil(t, inst.CompletedAt)
	}
}

func TestYAMLStockReplenishment(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	bus := &mockBus{}
	engine := NewEngine(repo, bus)

	def := loadYAML(t, "../../../processes/stock_replenishment.yaml")
	engine.RegisterDefinition(def)

	orgID := uuid.New()
	entityID := uuid.New()

	engine.HandleEvent(ctx, types.Event{
		ID: uuid.New(), OrganizationID: orgID, EntityID: &entityID, Type: def.TriggerOn,
	})

	require.Len(t, repo.instances, 1)
	for _, inst := range repo.instances {
		assert.Equal(t, def.InitState, inst.CurrentState)
		assert.Equal(t, "active", inst.Status)
		assert.Equal(t, def.ID, inst.DefinitionID)
	}
}

func TestYAMLDeliveryTracking(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	bus := &mockBus{}
	engine := NewEngine(repo, bus)

	def := loadYAML(t, "../../../processes/delivery_tracking.yaml")
	engine.RegisterDefinition(def)

	orgID := uuid.New()
	entityID := uuid.New()

	engine.HandleEvent(ctx, types.Event{
		ID: uuid.New(), OrganizationID: orgID, EntityID: &entityID, Type: def.TriggerOn,
	})

	require.Len(t, repo.instances, 1)
	for _, inst := range repo.instances {
		assert.Equal(t, def.InitState, inst.CurrentState)
		assert.Equal(t, def.ID, inst.DefinitionID)
	}
}

func TestYAMLEmployeeOnboarding(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	bus := &mockBus{}
	engine := NewEngine(repo, bus)

	def := loadYAML(t, "../../../processes/employee_onboarding.yaml")
	engine.RegisterDefinition(def)

	orgID := uuid.New()
	entityID := uuid.New()

	engine.HandleEvent(ctx, types.Event{
		ID: uuid.New(), OrganizationID: orgID, EntityID: &entityID, Type: def.TriggerOn,
	})

	require.Len(t, repo.instances, 1)
	for _, inst := range repo.instances {
		assert.Equal(t, def.InitState, inst.CurrentState)
		assert.Equal(t, def.ID, inst.DefinitionID)
	}
}

func TestYAMLHistoryRecording(t *testing.T) {
	ctx := context.Background()
	repo := newMockRepo()
	bus := &mockBus{}
	engine := NewEngine(repo, bus)
	engine.RegisterDefinition(loadYAML(t, "../../../processes/order_fulfillment.yaml"))

	orgID := uuid.New()
	entityID := uuid.New()
	ev := func(eventType string) {
		engine.HandleEvent(ctx, types.Event{
			ID: uuid.New(), OrganizationID: orgID, EntityID: &entityID, Type: eventType,
		})
	}

	ev("order.created")
	ev("order.confirmed")
	ev("order.paid")

	for _, inst := range repo.instances {
		var history []HistoryEntry
		require.NoError(t, json.Unmarshal(inst.History, &history))
		assert.Len(t, history, 2)
		assert.Equal(t, "new", history[0].From)
		assert.Equal(t, "confirmed", history[0].To)
		assert.Equal(t, "order.confirmed", history[0].Event)
	}
}
