package edo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStub_ImplementsInterface(t *testing.T) {
	var _ EDOService = (*Stub)(nil)
}

func TestStubSendDocument(t *testing.T) {
	stub := NewStub()
	doc := EDODocument{
		Type:            "invoice",
		Number:          "INV-001",
		Date:            "2026-03-15",
		CounterpartyINN: "7707083893",
		Amount:          150000,
	}

	result, err := stub.SendDocument(context.Background(), uuid.New(), doc)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEmpty(t, result.DocumentID)
	assert.Equal(t, "sent", result.Status)
}

func TestStubGetIncomingDocuments(t *testing.T) {
	stub := NewStub()
	docs, err := stub.GetIncomingDocuments(context.Background(), uuid.New(), time.Now())
	require.NoError(t, err)
	assert.Nil(t, docs)
}

func TestStubAcceptDocument(t *testing.T) {
	stub := NewStub()
	err := stub.AcceptDocument(context.Background(), uuid.New(), "doc-123")
	require.NoError(t, err)
}

func TestStubRejectDocument(t *testing.T) {
	stub := NewStub()
	err := stub.RejectDocument(context.Background(), uuid.New(), "doc-456", "incorrect amount")
	require.NoError(t, err)
}

func TestDiadocClient_SendDocument_MockHTTP(t *testing.T) {
	authCalled := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/V3/Authenticate" {
			authCalled = true
			json.NewEncoder(w).Encode(map[string]string{"token": "test-token"})
			return
		}

		assert.Equal(t, "/V3/PostMessage", r.URL.Path)
		assert.Contains(t, r.Header.Get("Authorization"), "ddauth_token=test-token")

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "invoice", body["Type"])

		json.NewEncoder(w).Encode(map[string]string{"MessageId": "msg-001"})
	}))
	defer srv.Close()

	client := NewDiadocClient(DiadocConfig{
		BaseURL:  srv.URL,
		APIKey:   "api-key",
		Login:    "test",
		Password: "pass",
		BoxID:    "box-1",
	})

	doc := EDODocument{Type: "invoice", Number: "1", Date: "2026-01-01", Amount: 10000}
	result, err := client.SendDocument(context.Background(), uuid.New(), doc)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, authCalled)
	assert.Equal(t, "msg-001", result.DocumentID)
	assert.Equal(t, "sent", result.Status)
}

func TestDiadocClient_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewDiadocClient(DiadocConfig{BaseURL: srv.URL, APIKey: "bad"})

	_, err := client.SendDocument(context.Background(), uuid.New(), EDODocument{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "auth failed")
}

func TestDiadocClient_GetIncomingDocuments_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/V3/Authenticate" {
			json.NewEncoder(w).Encode(map[string]string{"token": "tok"})
			return
		}

		assert.Contains(t, r.URL.Path, "/V3/GetDocuments")

		json.NewEncoder(w).Encode(map[string]any{
			"Documents": []map[string]string{
				{"DocumentId": "d1", "Type": "invoice", "Number": "1", "Date": "2026-01-01"},
				{"DocumentId": "d2", "Type": "act", "Number": "2", "Date": "2026-01-02"},
			},
		})
	}))
	defer srv.Close()

	client := NewDiadocClient(DiadocConfig{BaseURL: srv.URL, APIKey: "k", BoxID: "b"})
	docs, err := client.GetIncomingDocuments(context.Background(), uuid.New(), time.Now())
	require.NoError(t, err)
	require.Len(t, docs, 2)

	assert.Equal(t, "d1", docs[0].ID)
	assert.Equal(t, "invoice", docs[0].Type)
	assert.Equal(t, "delivered", docs[0].Status)
}

func TestDiadocClient_AcceptDocument_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/V3/Authenticate" {
			json.NewEncoder(w).Encode(map[string]string{"token": "tok"})
			return
		}
		assert.Equal(t, "/V3/PostMessagePatch", r.URL.Path)

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Accept", body["ActionType"])
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewDiadocClient(DiadocConfig{BaseURL: srv.URL, APIKey: "k", BoxID: "b"})
	err := client.AcceptDocument(context.Background(), uuid.New(), "msg-1")
	require.NoError(t, err)
}

func TestDiadocClient_RejectDocument_MockHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/V3/Authenticate" {
			json.NewEncoder(w).Encode(map[string]string{"token": "tok"})
			return
		}
		assert.Equal(t, "/V3/PostMessagePatch", r.URL.Path)

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "Reject", body["ActionType"])
		assert.Equal(t, "wrong amount", body["Comment"])
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := NewDiadocClient(DiadocConfig{BaseURL: srv.URL, APIKey: "k", BoxID: "b"})
	err := client.RejectDocument(context.Background(), uuid.New(), "msg-1", "wrong amount")
	require.NoError(t, err)
}
