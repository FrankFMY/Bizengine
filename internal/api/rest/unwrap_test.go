package rest

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/bizengine/engine/pkg/types"
)

func TestUnwrapRequest_WithEnvelope(t *testing.T) {
	var gotBody []byte
	var gotParams map[string]any
	var gotIdemp string

	handler := UnwrapRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		gotParams = RequestParams(r.Context())
		gotIdemp = RequestIdemp(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	envelope := map[string]any{
		"data":   map[string]string{"email": "test@test.com"},
		"params": map[string]any{"lang": "ru"},
		"idemp":  "abc-123",
	}
	body, _ := json.Marshal(envelope)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var data map[string]string
	require.NoError(t, json.Unmarshal(gotBody, &data))
	assert.Equal(t, "test@test.com", data["email"])
	assert.Equal(t, "ru", gotParams["lang"])
	assert.Equal(t, "abc-123", gotIdemp)
}

func TestUnwrapRequest_NoDataKey_Passthrough(t *testing.T) {
	var gotBody []byte

	handler := UnwrapRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))

	raw := map[string]string{"email": "test@test.com", "password": "secret"}
	body, _ := json.Marshal(raw)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var data map[string]string
	require.NoError(t, json.Unmarshal(gotBody, &data))
	assert.Equal(t, "test@test.com", data["email"])
}

func TestUnwrapRequest_GET_Passthrough(t *testing.T) {
	called := false
	handler := UnwrapRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, called)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUnwrapRequest_InvalidJSON(t *testing.T) {
	var gotBody []byte

	handler := UnwrapRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/test", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Invalid JSON passes through for decodeJSON to handle
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "not json", string(gotBody))
}

func TestUnwrapRequest_EmptyBody(t *testing.T) {
	called := false
	handler := UnwrapRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, called)
}

func TestUnwrapRequest_DataOnly(t *testing.T) {
	var gotIdemp string

	handler := UnwrapRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIdemp = RequestIdemp(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	envelope := map[string]any{
		"data": map[string]string{"name": "test"},
	}
	body, _ := json.Marshal(envelope)

	req := httptest.NewRequest("POST", "/test", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, gotIdemp)
}

func TestUnwrapRequest_DELETE_Passthrough(t *testing.T) {
	called := false
	handler := UnwrapRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("DELETE", "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.True(t, called)
}

func TestUnwrapRequest_ReadError(t *testing.T) {
	handler := UnwrapRequest(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("handler should not be called")
	}))

	req := httptest.NewRequest("POST", "/test", &errReader{})
	req.ContentLength = 10
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var resp types.Response
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.OK)
	assert.Equal(t, "UNPROCESSABLE", resp.Error.Code)
}

type errReader struct{}

func (e *errReader) Read([]byte) (int, error) {
	return 0, io.ErrUnexpectedEOF
}
