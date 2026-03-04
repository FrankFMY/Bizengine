package process

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"

	"github.com/bizengine/engine/internal/core/event"
	"github.com/bizengine/engine/pkg/dsl"
	"github.com/bizengine/engine/pkg/types"
)

// Engine executes state machines defined via YAML process definitions.
type Engine struct {
	repo     Repository
	eventBus event.Bus
	defs     map[string]*dsl.ProcessDefinition // trigger_on → definition
}

// NewEngine creates a new process engine.
func NewEngine(repo Repository, eventBus event.Bus) *Engine {
	return &Engine{
		repo:     repo,
		eventBus: eventBus,
		defs:     make(map[string]*dsl.ProcessDefinition),
	}
}

// LoadDefinitions reads definitions from the repository and indexes them by trigger_on.
func (e *Engine) LoadDefinitions(ctx context.Context, wsID uuid.UUID) error {
	defs, err := e.repo.ListDefinitions(ctx, wsID)
	if err != nil {
		return err
	}
	for i := range defs {
		if defs[i].IsActive {
			def := defs[i].Definition
			e.defs[def.TriggerOn] = &def
		}
	}
	return nil
}

// RegisterDefinition adds an in-memory definition (for YAML-loaded definitions).
func (e *Engine) RegisterDefinition(def *dsl.ProcessDefinition) {
	e.defs[def.TriggerOn] = def
}

// HandleEvent processes an event through the engine. It either starts a new process
// or advances existing instances.
func (e *Engine) HandleEvent(ctx context.Context, ev types.Event) error {
	// 1. Check if this event triggers a new process
	if def, ok := e.defs[ev.Type]; ok && ev.EntityID != nil {
		if err := e.startProcess(ctx, ev, def); err != nil {
			log.Error().Err(err).Str("def", def.ID).Msg("failed to start process")
		}
	}

	// 2. Advance any active instances for this entity
	if ev.EntityID != nil {
		instances, err := e.repo.GetActiveByEntity(ctx, *ev.EntityID)
		if err != nil {
			return err
		}
		for i := range instances {
			if err := e.advanceInstance(ctx, &instances[i], ev); err != nil {
				log.Error().Err(err).Str("instance", instances[i].ID.String()).Msg("failed to advance process")
			}
		}
	}

	return nil
}

// GetInstance returns a process instance by ID.
func (e *Engine) GetInstance(ctx context.Context, id uuid.UUID) (*Instance, error) {
	return e.repo.GetInstance(ctx, id)
}

// GetInstancesByEntity returns active process instances for an entity.
func (e *Engine) GetInstancesByEntity(ctx context.Context, entityID uuid.UUID) ([]Instance, error) {
	return e.repo.GetActiveByEntity(ctx, entityID)
}

// ListInstances returns process instances for a workspace.
func (e *Engine) ListInstances(ctx context.Context, wsID uuid.UUID, status *string, limit, offset int) ([]Instance, int, error) {
	return e.repo.ListInstances(ctx, wsID, status, limit, offset)
}

// ListDefinitions returns all definitions for a workspace.
func (e *Engine) ListDefinitions(ctx context.Context, wsID uuid.UUID) ([]DefinitionRecord, error) {
	return e.repo.ListDefinitions(ctx, wsID)
}

// GetDefinition returns a definition by ID.
func (e *Engine) GetDefinition(ctx context.Context, id string, wsID *uuid.UUID) (*DefinitionRecord, error) {
	return e.repo.GetDefinition(ctx, id, wsID)
}

func (e *Engine) startProcess(ctx context.Context, ev types.Event, def *dsl.ProcessDefinition) error {
	now := time.Now()
	inst := &Instance{
		ID:           uuid.New(),
		WorkspaceID:  ev.WorkspaceID,
		DefinitionID: def.ID,
		EntityID:     *ev.EntityID,
		CurrentState: def.InitState,
		Status:       "active",
		Context:      json.RawMessage(`{}`),
		History:      json.RawMessage(`[]`),
		StartedAt:    now,
		UpdatedAt:    now,
	}

	if err := e.repo.CreateInstance(ctx, inst); err != nil {
		return err
	}

	// Execute on_enter for initial state
	if state, ok := def.States[def.InitState]; ok {
		e.executeActions(ctx, ev, state.OnEnter)
	}

	log.Debug().Str("def", def.ID).Str("entity", ev.EntityID.String()).Str("state", def.InitState).Msg("process started")
	return nil
}

func (e *Engine) advanceInstance(ctx context.Context, inst *Instance, ev types.Event) error {
	def, ok := e.findDefinition(inst.DefinitionID)
	if !ok {
		return fmt.Errorf("definition %q not found", inst.DefinitionID)
	}

	state, ok := def.States[inst.CurrentState]
	if !ok {
		return fmt.Errorf("state %q not found in definition %q", inst.CurrentState, def.ID)
	}

	// Find matching transition
	var eventData map[string]any
	if ev.Data != nil {
		json.Unmarshal(ev.Data, &eventData)
	}

	for _, tr := range state.Transitions {
		if tr.Event != ev.Type {
			continue
		}
		if tr.Condition != nil && !evaluateCondition(tr.Condition, eventData) {
			continue
		}

		// Execute: on_exit → transition actions → update state → on_enter
		e.executeActions(ctx, ev, state.OnExit)
		e.executeActions(ctx, ev, tr.Actions)

		// Record history
		entry := HistoryEntry{
			From:      inst.CurrentState,
			To:        tr.To,
			Event:     ev.Type,
			Timestamp: time.Now(),
		}
		var history []HistoryEntry
		json.Unmarshal(inst.History, &history)
		history = append(history, entry)
		inst.History, _ = json.Marshal(history)

		inst.CurrentState = tr.To
		inst.UpdatedAt = time.Now()

		// Check if new state is terminal
		newState, _ := def.States[tr.To]
		if newState.Terminal {
			inst.Status = "completed"
			now := time.Now()
			inst.CompletedAt = &now
		}

		if err := e.repo.UpdateInstance(ctx, inst); err != nil {
			return err
		}

		// Execute on_enter of new state
		e.executeActions(ctx, ev, newState.OnEnter)

		log.Debug().Str("inst", inst.ID.String()).Str("from", entry.From).Str("to", entry.To).Str("event", ev.Type).Msg("process advanced")
		return nil
	}

	return nil
}

func (e *Engine) findDefinition(id string) (*dsl.ProcessDefinition, bool) {
	for _, def := range e.defs {
		if def.ID == id {
			return def, true
		}
	}
	return nil, false
}

func (e *Engine) executeActions(ctx context.Context, ev types.Event, actions []dsl.Action) {
	for _, action := range actions {
		switch action.Type {
		case "emit_event":
			e.executeEmitEvent(ctx, ev, action.Params)
		case "update_status":
			e.executeUpdateStatus(ctx, ev, action.Params)
		case "notify":
			e.executeNotify(ctx, ev, action.Params)
		case "update_component", "webhook":
			log.Debug().Str("type", action.Type).Msg("action type not yet implemented")
		}
	}
}

func (e *Engine) executeEmitEvent(ctx context.Context, source types.Event, params map[string]any) {
	eventType, _ := params["event_type"].(string)
	if eventType == "" {
		return
	}
	data, _ := params["data"].(map[string]any)

	ev := types.Event{
		ID:          uuid.New(),
		WorkspaceID: source.WorkspaceID,
		EntityID:    source.EntityID,
		Type:        eventType,
		Timestamp:   time.Now(),
		Version:     1,
	}
	ev.Data, _ = json.Marshal(data)
	e.eventBus.Publish(ctx, ev)
}

func (e *Engine) executeUpdateStatus(ctx context.Context, source types.Event, params map[string]any) {
	status, _ := params["status"].(string)
	if status == "" {
		return
	}
	ev := types.Event{
		ID:          uuid.New(),
		WorkspaceID: source.WorkspaceID,
		EntityID:    source.EntityID,
		Type:        "entity.status_update_requested",
		Timestamp:   time.Now(),
		Version:     1,
	}
	ev.Data, _ = json.Marshal(map[string]any{"status": status})
	e.eventBus.Publish(ctx, ev)
}

func (e *Engine) executeNotify(ctx context.Context, source types.Event, params map[string]any) {
	ev := types.Event{
		ID:          uuid.New(),
		WorkspaceID: source.WorkspaceID,
		EntityID:    source.EntityID,
		Type:        "notification.created",
		Timestamp:   time.Now(),
		Version:     1,
	}
	ev.Data, _ = json.Marshal(params)
	e.eventBus.Publish(ctx, ev)
}

// TriggerManual allows manual triggering of an event for process advancement.
func (e *Engine) TriggerManual(ctx context.Context, wsID uuid.UUID, entityID uuid.UUID, eventType string, data json.RawMessage) error {
	ev := types.Event{
		ID:          uuid.New(),
		WorkspaceID: wsID,
		EntityID:    &entityID,
		Type:        eventType,
		Timestamp:   time.Now(),
		Version:     1,
		Data:        data,
	}
	return e.HandleEvent(ctx, ev)
}

// evaluateCondition checks if a condition is met against event data.
func evaluateCondition(cond *dsl.Condition, data map[string]any) bool {
	if data == nil {
		return cond.Operator == "exists" && cond.Value == false
	}

	val, exists := data[cond.Field]

	switch cond.Operator {
	case "exists":
		wantExists, _ := cond.Value.(bool)
		return exists == wantExists
	case "eq":
		return fmt.Sprintf("%v", val) == fmt.Sprintf("%v", cond.Value)
	case "neq":
		return fmt.Sprintf("%v", val) != fmt.Sprintf("%v", cond.Value)
	case "gt":
		return toFloat(val) > toFloat(cond.Value)
	case "lt":
		return toFloat(val) < toFloat(cond.Value)
	case "gte":
		return toFloat(val) >= toFloat(cond.Value)
	case "lte":
		return toFloat(val) <= toFloat(cond.Value)
	case "in":
		return inSlice(val, cond.Value)
	case "not_in":
		return !inSlice(val, cond.Value)
	}
	return false
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case json.Number:
		f, _ := n.Float64()
		return f
	}
	return 0
}

func inSlice(val any, list any) bool {
	strVal := fmt.Sprintf("%v", val)
	switch items := list.(type) {
	case []any:
		for _, item := range items {
			if fmt.Sprintf("%v", item) == strVal {
				return true
			}
		}
	case []string:
		for _, item := range items {
			if item == strVal {
				return true
			}
		}
	}
	return false
}

