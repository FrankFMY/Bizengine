package dsl

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAndValidateYAMLFiles(t *testing.T) {
	processDir := filepath.Join("..", "..", "processes")
	entries, err := os.ReadDir(processDir)
	require.NoError(t, err, "processes directory should exist")

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".yaml" {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			def, err := ParseFile(filepath.Join(processDir, entry.Name()))
			require.NoError(t, err)
			require.NotNil(t, def)

			assert.NotEmpty(t, def.ID)
			assert.NotEmpty(t, def.Name)
			assert.NotEmpty(t, def.TriggerOn)
			assert.NotEmpty(t, def.EntityKind)
			assert.NotEmpty(t, def.InitState)
			assert.NotEmpty(t, def.States)

			errs := Validate(def)
			assert.Empty(t, errs, "validation errors: %v", errs)
		})
	}
}

func TestValidateRejectsInvalid(t *testing.T) {
	t.Run("empty ID", func(t *testing.T) {
		def := &ProcessDefinition{
			Name:       "Test",
			TriggerOn:  "test.event",
			EntityKind: "test",
			InitState:  "start",
			States: map[string]State{
				"start": {Name: "Start", Terminal: true},
			},
		}
		errs := Validate(def)
		assert.NotEmpty(t, errs)
	})

	t.Run("missing init state", func(t *testing.T) {
		def := &ProcessDefinition{
			ID:         "test",
			Name:       "Test",
			TriggerOn:  "test.event",
			EntityKind: "test",
			InitState:  "nonexistent",
			States: map[string]State{
				"start": {Name: "Start", Terminal: true},
			},
		}
		errs := Validate(def)
		assert.NotEmpty(t, errs)
	})

	t.Run("no terminal state", func(t *testing.T) {
		def := &ProcessDefinition{
			ID:         "test",
			Name:       "Test",
			TriggerOn:  "test.event",
			EntityKind: "test",
			InitState:  "start",
			States: map[string]State{
				"start": {Name: "Start"},
			},
		}
		errs := Validate(def)
		assert.NotEmpty(t, errs)
	})

	t.Run("invalid transition target", func(t *testing.T) {
		def := &ProcessDefinition{
			ID:         "test",
			Name:       "Test",
			TriggerOn:  "test.event",
			EntityKind: "test",
			InitState:  "start",
			States: map[string]State{
				"start": {
					Name: "Start",
					Transitions: []Transition{
						{To: "nonexistent", Event: "go"},
					},
				},
				"end": {Name: "End", Terminal: true},
			},
		}
		errs := Validate(def)
		assert.NotEmpty(t, errs)
	})

	t.Run("invalid operator", func(t *testing.T) {
		def := &ProcessDefinition{
			ID:         "test",
			Name:       "Test",
			TriggerOn:  "test.event",
			EntityKind: "test",
			InitState:  "start",
			States: map[string]State{
				"start": {
					Name: "Start",
					Transitions: []Transition{
						{To: "end", Event: "go", Condition: &Condition{Field: "x", Operator: "invalid"}},
					},
				},
				"end": {Name: "End", Terminal: true},
			},
		}
		errs := Validate(def)
		assert.NotEmpty(t, errs)
	})
}
