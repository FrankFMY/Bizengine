package errs

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidationErrors_Set(t *testing.T) {
	v := ValidationErrors{}
	v.Set("email", "REQUIRED")
	v.Set("password", "TOO_SHORT")

	assert.True(t, v.HasErrors())
	assert.Equal(t, "REQUIRED", v["email"])
	assert.Equal(t, "TOO_SHORT", v["password"])
}

func TestValidationErrors_Empty(t *testing.T) {
	v := ValidationErrors{}
	assert.False(t, v.HasErrors())
}

func TestValidationErrors_SetNested(t *testing.T) {
	v := ValidationErrors{}
	addr := ValidationErrors{}
	addr.Set("city", "REQUIRED")
	v.SetNested("address", addr)

	assert.True(t, v.HasErrors())

	b, err := json.Marshal(v)
	require.NoError(t, err)
	assert.JSONEq(t, `{"address":{"city":"REQUIRED"}}`, string(b))
}

func TestValidationErrors_SetArray(t *testing.T) {
	v := ValidationErrors{}

	item0 := ValidationErrors{}
	item0.Set("quantity", "NEGATIVE")

	v.SetArray("items", []any{item0, nil})

	b, err := json.Marshal(v)
	require.NoError(t, err)
	assert.JSONEq(t, `{"items":[{"quantity":"NEGATIVE"},null]}`, string(b))
}

func TestValidationErrors_JSON(t *testing.T) {
	v := ValidationErrors{}
	v.Set("email", "REQUIRED")
	v.Set("name", "TOO_LONG")

	b, err := json.Marshal(v)
	require.NoError(t, err)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(b, &decoded))
	assert.Equal(t, "REQUIRED", decoded["email"])
	assert.Equal(t, "TOO_LONG", decoded["name"])
}
