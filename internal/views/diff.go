package views

import (
	"fmt"
	"sort"
)

// PatchOp represents a single JSON Patch operation.
type PatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// ComputeTableDiff compares old and new field values, returning JSON Patch ops for changed fields.
func ComputeTableDiff(old, new map[string]any) []PatchOp {
	var ops []PatchOp

	// Collect all keys from both maps.
	keys := make(map[string]struct{})
	for k := range old {
		keys[k] = struct{}{}
	}
	for k := range new {
		keys[k] = struct{}{}
	}

	// Sort for deterministic output.
	sorted := make([]string, 0, len(keys))
	for k := range keys {
		sorted = append(sorted, k)
	}
	sort.Strings(sorted)

	for _, k := range sorted {
		oldVal, inOld := old[k]
		newVal, inNew := new[k]
		path := "/" + k

		switch {
		case inOld && !inNew:
			ops = append(ops, PatchOp{Op: "remove", Path: path})
		case !inOld && inNew:
			ops = append(ops, PatchOp{Op: "add", Path: path, Value: newVal})
		case inOld && inNew && !jsonEqual(oldVal, newVal):
			ops = append(ops, PatchOp{Op: "replace", Path: path, Value: newVal})
		}
	}

	return ops
}

// ComputeRefsDiff compares old and new ref lists, returning JSON Patch ops.
// Rules: add at end = {"op":"add","path":"/N"}, remove = {"op":"replace","path":"/i","value":null}.
func ComputeRefsDiff(old, new []DataRef) []PatchOp {
	var ops []PatchOp

	oldIdx := make(map[string]int) // "table:id" -> index
	for i, ref := range old {
		oldIdx[ref.Table+":"+ref.ID] = i
	}

	newSet := make(map[string]struct{})
	for _, ref := range new {
		newSet[ref.Table+":"+ref.ID] = struct{}{}
	}

	// Removals: refs in old but not in new -> replace with null at their position.
	for key, idx := range oldIdx {
		if _, exists := newSet[key]; !exists {
			ops = append(ops, PatchOp{
				Op:    "replace",
				Path:  fmt.Sprintf("/%d", idx),
				Value: nil,
			})
		}
	}

	// Additions: refs in new but not in old -> add at end.
	for _, ref := range new {
		key := ref.Table + ":" + ref.ID
		if _, exists := oldIdx[key]; !exists {
			ops = append(ops, PatchOp{
				Op:    "add",
				Path:  fmt.Sprintf("/%d", len(old)),
				Value: ref,
			})
		}
	}

	return ops
}

// jsonEqual compares two values for JSON-level equality.
func jsonEqual(a, b any) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
