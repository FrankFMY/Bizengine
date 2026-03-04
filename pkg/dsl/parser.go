// Package dsl provides YAML parsing and validation for process definitions.
package dsl

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ProcessDefinition represents a complete process definition parsed from YAML.
type ProcessDefinition struct {
	ID          string           `yaml:"id" json:"id"`
	Name        string           `yaml:"name" json:"name"`
	Description string           `yaml:"description" json:"description"`
	TriggerOn   string           `yaml:"trigger_on" json:"trigger_on"`
	EntityKind  string           `yaml:"entity_kind" json:"entity_kind"`
	InitState   string           `yaml:"init_state" json:"init_state"`
	States      map[string]State `yaml:"states" json:"states"`
}

// State represents a single state in the process definition.
type State struct {
	Name        string       `yaml:"name" json:"name"`
	Terminal    bool         `yaml:"terminal" json:"terminal,omitempty"`
	Timeout     *Timeout     `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	OnEnter     []Action     `yaml:"on_enter,omitempty" json:"on_enter,omitempty"`
	OnExit      []Action     `yaml:"on_exit,omitempty" json:"on_exit,omitempty"`
	Transitions []Transition `yaml:"transitions,omitempty" json:"transitions,omitempty"`
}

// Timeout configures automatic state transition after a duration.
type Timeout struct {
	Duration string `yaml:"duration" json:"duration"`
	To       string `yaml:"to" json:"to"`
}

// Transition defines a possible state change triggered by an event.
type Transition struct {
	To        string     `yaml:"to" json:"to"`
	Event     string     `yaml:"event" json:"event"`
	Condition *Condition `yaml:"condition,omitempty" json:"condition,omitempty"`
	Actions   []Action   `yaml:"actions,omitempty" json:"actions,omitempty"`
}

// Condition is an optional guard on a transition.
type Condition struct {
	Field    string `yaml:"field" json:"field"`
	Operator string `yaml:"operator" json:"operator"`
	Value    any    `yaml:"value" json:"value"`
}

// Action represents an executable step (on_enter, on_exit, or transition action).
type Action struct {
	Type   string         `yaml:"type" json:"type"`
	Params map[string]any `yaml:"params,omitempty" json:"params,omitempty"`
}

// ParseFile reads a YAML file and returns a ProcessDefinition.
func ParseFile(path string) (*ProcessDefinition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", path, err)
	}
	return Parse(data)
}

// Parse parses YAML bytes into a ProcessDefinition.
func Parse(data []byte) (*ProcessDefinition, error) {
	var def ProcessDefinition
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	return &def, nil
}
