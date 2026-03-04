package views

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeTableDiff_FieldChange(t *testing.T) {
	old := map[string]any{"status": "draft", "total": 100}
	new := map[string]any{"status": "active", "total": 100}

	ops := ComputeTableDiff(old, new)

	assert.Len(t, ops, 1)
	assert.Equal(t, "replace", ops[0].Op)
	assert.Equal(t, "/status", ops[0].Path)
	assert.Equal(t, "active", ops[0].Value)
}

func TestComputeTableDiff_FieldAdd(t *testing.T) {
	old := map[string]any{"name": "Test"}
	new := map[string]any{"name": "Test", "status": "active"}

	ops := ComputeTableDiff(old, new)

	assert.Len(t, ops, 1)
	assert.Equal(t, "add", ops[0].Op)
	assert.Equal(t, "/status", ops[0].Path)
}

func TestComputeTableDiff_FieldRemove(t *testing.T) {
	old := map[string]any{"name": "Test", "status": "active"}
	new := map[string]any{"name": "Test"}

	ops := ComputeTableDiff(old, new)

	assert.Len(t, ops, 1)
	assert.Equal(t, "remove", ops[0].Op)
	assert.Equal(t, "/status", ops[0].Path)
}

func TestComputeTableDiff_NoChange(t *testing.T) {
	old := map[string]any{"status": "active", "total": 100}
	new := map[string]any{"status": "active", "total": 100}

	ops := ComputeTableDiff(old, new)
	assert.Empty(t, ops)
}

func TestComputeRefsDiff_Add(t *testing.T) {
	old := []DataRef{{Table: "orders", ID: "1", Fields: []string{"status"}}}
	new := []DataRef{
		{Table: "orders", ID: "1", Fields: []string{"status"}},
		{Table: "orders", ID: "2", Fields: []string{"status"}},
	}

	ops := ComputeRefsDiff(old, new)

	assert.Len(t, ops, 1)
	assert.Equal(t, "add", ops[0].Op)
	assert.Equal(t, "/1", ops[0].Path) // appended at end (old len = 1)
}

func TestComputeRefsDiff_Remove(t *testing.T) {
	old := []DataRef{
		{Table: "orders", ID: "1", Fields: []string{"status"}},
		{Table: "orders", ID: "2", Fields: []string{"status"}},
	}
	new := []DataRef{{Table: "orders", ID: "2", Fields: []string{"status"}}}

	ops := ComputeRefsDiff(old, new)

	assert.Len(t, ops, 1)
	assert.Equal(t, "replace", ops[0].Op)
	assert.Equal(t, "/0", ops[0].Path) // position of removed ref
	assert.Nil(t, ops[0].Value)
}

func TestComputeRefsDiff_NoChange(t *testing.T) {
	refs := []DataRef{{Table: "orders", ID: "1", Fields: []string{"status"}}}
	ops := ComputeRefsDiff(refs, refs)
	assert.Empty(t, ops)
}

func TestComputeRefsDiff_AddAndRemove(t *testing.T) {
	old := []DataRef{
		{Table: "orders", ID: "1", Fields: []string{"status"}},
		{Table: "orders", ID: "2", Fields: []string{"status"}},
	}
	new := []DataRef{
		{Table: "orders", ID: "2", Fields: []string{"status"}},
		{Table: "orders", ID: "3", Fields: []string{"status"}},
	}

	ops := ComputeRefsDiff(old, new)

	assert.Len(t, ops, 2)
	// One removal (id=1 at pos 0) and one addition (id=3)
	hasRemove := false
	hasAdd := false
	for _, op := range ops {
		if op.Op == "replace" && op.Path == "/0" {
			hasRemove = true
		}
		if op.Op == "add" {
			hasAdd = true
		}
	}
	assert.True(t, hasRemove)
	assert.True(t, hasAdd)
}
