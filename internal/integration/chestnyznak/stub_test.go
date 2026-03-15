package chestnyznak

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStub_ImplementsInterface(t *testing.T) {
	var _ MarkingService = (*Stub)(nil)
}

func TestStubVerifyCode(t *testing.T) {
	stub := NewStub()
	code := "010460043993125621CPY4x>"

	info, err := stub.VerifyCode(context.Background(), code)
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, code, info.Code)
	assert.True(t, info.Valid)
	assert.NotEmpty(t, info.ProductName)
	assert.NotEmpty(t, info.Status)
}

func TestStubRegisterReceipt(t *testing.T) {
	stub := NewStub()
	err := stub.RegisterReceipt(
		context.Background(),
		uuid.New(),
		[]string{"code1", "code2"},
		"doc-123",
	)
	require.NoError(t, err)
}

func TestStubRegisterShipment(t *testing.T) {
	stub := NewStub()
	err := stub.RegisterShipment(
		context.Background(),
		uuid.New(),
		[]string{"code1", "code2", "code3"},
		"7707083893",
	)
	require.NoError(t, err)
}

func TestClient_VerifyCode_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "GET", r.Method)
		assert.Contains(t, r.URL.Path, "/facade/identifytools/check")
		assert.Contains(t, r.URL.RawQuery, "code=ABC123")
		assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

		json.NewEncoder(w).Encode(map[string]any{
			"code":         "ABC123",
			"valid":        true,
			"productName":  "Test Product",
			"productGroup": "tobacco",
			"status":       "in_circulation",
		})
	}))
	defer srv.Close()

	client := NewClient(ChestnyZnakConfig{
		BaseURL: srv.URL,
		Token:   "test-token",
	})

	info, err := client.VerifyCode(context.Background(), "ABC123")
	require.NoError(t, err)
	require.NotNil(t, info)

	assert.Equal(t, "ABC123", info.Code)
	assert.True(t, info.Valid)
	assert.Equal(t, "Test Product", info.ProductName)
	assert.Equal(t, "tobacco", info.Category)
	assert.Equal(t, "in_circulation", info.Status)
}

func TestClient_VerifyCode_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	client := NewClient(ChestnyZnakConfig{BaseURL: srv.URL, Token: "bad"})

	_, err := client.VerifyCode(context.Background(), "ABC")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
}

func TestClient_RegisterReceipt_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/facade/doc/create", r.URL.Path)

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "receipt", body["document_type"])

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(ChestnyZnakConfig{BaseURL: srv.URL, Token: "tok"})
	err := client.RegisterReceipt(context.Background(), uuid.New(), []string{"c1"}, "doc1")
	require.NoError(t, err)
}

func TestClient_RegisterShipment_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/facade/doc/create", r.URL.Path)

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "shipment", body["document_type"])
		assert.Equal(t, "1234567890", body["counterparty_inn"])

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewClient(ChestnyZnakConfig{BaseURL: srv.URL, Token: "tok"})
	err := client.RegisterShipment(context.Background(), uuid.New(), []string{"c1"}, "1234567890")
	require.NoError(t, err)
}

func TestClient_RegisterReceipt_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := NewClient(ChestnyZnakConfig{BaseURL: srv.URL, Token: "tok"})
	err := client.RegisterReceipt(context.Background(), uuid.New(), []string{"c1"}, "doc1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}
