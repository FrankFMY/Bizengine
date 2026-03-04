package errs

// ValidationErrors holds field-level validation errors.
// Keys are field names, values are error codes (string) or nested structures.
type ValidationErrors map[string]any

// Set sets a validation error code for a field.
func (v ValidationErrors) Set(field, code string) {
	v[field] = code
}

// SetNested sets nested validation errors for a field (e.g. for embedded objects).
func (v ValidationErrors) SetNested(field string, nested ValidationErrors) {
	v[field] = nested
}

// SetArray sets per-index validation errors for an array field.
// nil elements mean the item at that index is valid.
func (v ValidationErrors) SetArray(field string, items []any) {
	v[field] = items
}

// HasErrors returns true if any validation errors are present.
func (v ValidationErrors) HasErrors() bool {
	return len(v) > 0
}
