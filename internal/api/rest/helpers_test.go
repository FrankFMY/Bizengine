package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/pkg/errs"
	"github.com/bizengine/engine/pkg/types"
)

func TestRespondOK(t *testing.T) {
	w := httptest.NewRecorder()
	respondOK(w, http.StatusOK, map[string]string{"key": "value"})

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var resp types.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)
	assert.Nil(t, resp.Error)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "value", data["key"])
}

func TestRespondOK_NilData(t *testing.T) {
	w := httptest.NewRecorder()
	respondOK(w, http.StatusOK, nil)

	var resp types.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)
	assert.Nil(t, resp.Data)
}

func TestRespondCreated(t *testing.T) {
	w := httptest.NewRecorder()
	respondCreated(w, map[string]string{"id": "123"})

	assert.Equal(t, http.StatusCreated, w.Code)

	var resp types.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.OK)
}

func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()
	respondError(w, errs.NewNotFound("entity not found"))

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp types.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.OK)
	require.NotNil(t, resp.Error)
	assert.Equal(t, "NOT_FOUND", resp.Error.Code)
	assert.Equal(t, "entity not found", resp.Error.Details)
}

func TestRespondError_Internal(t *testing.T) {
	w := httptest.NewRecorder()
	respondError(w, errs.NewInternal("something broke"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp types.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.OK)
	assert.Equal(t, "INTERNAL_ERROR", resp.Error.Code)
}

func TestRespondValidation(t *testing.T) {
	w := httptest.NewRecorder()
	v := errs.ValidationErrors{}
	v.Set("email", "REQUIRED")
	v.Set("name", "TOO_LONG")
	respondValidation(w, v)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)

	var resp types.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.OK)
	assert.Nil(t, resp.Error)

	val, ok := resp.Validation.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "REQUIRED", val["email"])
	assert.Equal(t, "TOO_LONG", val["name"])
}
