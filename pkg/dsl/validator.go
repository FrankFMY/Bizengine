package dsl

import (
	"fmt"
	"strings"
)

var validOperators = map[string]bool{
	"eq": true, "neq": true, "gt": true, "lt": true,
	"gte": true, "lte": true, "in": true, "not_in": true, "exists": true,
}

var validActionTypes = map[string]bool{
	"emit_event": true, "update_status": true, "update_component": true,
	"notify": true, "webhook": true,
}

// ValidationError represents a single validation issue.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validate checks a ProcessDefinition for structural correctness.
// Returns a list of errors (empty list = valid).
func Validate(def *ProcessDefinition) []ValidationError {
	var errs []ValidationError
	add := func(field, msg string) {
		errs = append(errs, ValidationError{Field: field, Message: msg})
	}

	if def.ID == "" {
		add("id", "must not be empty")
	}
	if def.Name == "" {
		add("name", "must not be empty")
	}
	if def.TriggerOn == "" {
		add("trigger_on", "must not be empty")
	}
	if def.EntityKind == "" {
		add("entity_kind", "must not be empty")
	}
	if len(def.States) == 0 {
		add("states", "must have at least one state")
		return errs
	}

	if _, ok := def.States[def.InitState]; !ok {
		add("init_state", fmt.Sprintf("state %q does not exist", def.InitState))
	}

	hasTerminal := false
	for name, state := range def.States {
		prefix := fmt.Sprintf("states.%s", name)
		if state.Terminal {
			hasTerminal = true
		}

		if state.Timeout != nil {
			if state.Timeout.Duration == "" {
				add(prefix+".timeout.duration", "must not be empty")
			}
			if _, ok := def.States[state.Timeout.To]; !ok {
				add(prefix+".timeout.to", fmt.Sprintf("state %q does not exist", state.Timeout.To))
			}
		}

		validateActions(state.OnEnter, prefix+".on_enter", &errs)
		validateActions(state.OnExit, prefix+".on_exit", &errs)

		for i, tr := range state.Transitions {
			trPrefix := fmt.Sprintf("%s.transitions[%d]", prefix, i)
			if tr.Event == "" {
				add(trPrefix+".event", "must not be empty")
			}
			if _, ok := def.States[tr.To]; !ok {
				add(trPrefix+".to", fmt.Sprintf("state %q does not exist", tr.To))
			}
			if tr.Condition != nil {
				if tr.Condition.Field == "" {
					add(trPrefix+".condition.field", "must not be empty")
				}
				if !validOperators[tr.Condition.Operator] {
					add(trPrefix+".condition.operator", fmt.Sprintf("unknown operator %q, valid: %s", tr.Condition.Operator, operatorList()))
				}
			}
			validateActions(tr.Actions, trPrefix+".actions", &errs)
		}
	}

	if !hasTerminal {
		add("states", "must have at least one terminal state")
	}

	return errs
}

func validateActions(actions []Action, prefix string, errs *[]ValidationError) {
	for i, a := range actions {
		aPrefix := fmt.Sprintf("%s[%d]", prefix, i)
		if !validActionTypes[a.Type] {
			*errs = append(*errs, ValidationError{
				Field:   aPrefix + ".type",
				Message: fmt.Sprintf("unknown action type %q, valid: %s", a.Type, actionTypeList()),
			})
		}
	}
}

func operatorList() string {
	ops := make([]string, 0, len(validOperators))
	for k := range validOperators {
		ops = append(ops, k)
	}
	return strings.Join(ops, ", ")
}

func actionTypeList() string {
	types := make([]string, 0, len(validActionTypes))
	for k := range validActionTypes {
		types = append(types, k)
	}
	return strings.Join(types, ", ")
}
