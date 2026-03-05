// Package edo provides the electronic document interchange (EDO) service interface.
package edo

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// EDODocument represents an electronic document (invoice, act, waybill).
type EDODocument struct {
	ID            string    `json:"id"`
	Type          string    `json:"type"` // invoice, act, waybill
	Number        string    `json:"number"`
	Date          string    `json:"date"`
	CounterpartyINN string  `json:"counterparty_inn"`
	Amount        int64     `json:"amount"`
	Content       []byte    `json:"content,omitempty"`
	Status        string    `json:"status"` // sent, delivered, accepted, rejected
	CreatedAt     time.Time `json:"created_at"`
}

// SendResult is the result of sending an EDO document.
type SendResult struct {
	DocumentID string `json:"document_id"`
	Status     string `json:"status"`
}

// EDOService defines the interface for electronic document interchange.
type EDOService interface {
	SendDocument(ctx context.Context, orgID uuid.UUID, doc EDODocument) (*SendResult, error)
	GetIncomingDocuments(ctx context.Context, orgID uuid.UUID, since time.Time) ([]EDODocument, error)
	AcceptDocument(ctx context.Context, orgID uuid.UUID, documentID string) error
	RejectDocument(ctx context.Context, orgID uuid.UUID, documentID string, reason string) error
}

// Stub is a stub implementation of EDOService that logs calls and returns success.
type Stub struct{}

// NewStub creates a new stub EDOService.
func NewStub() *Stub { return &Stub{} }

// SendDocument logs the call and returns a fake success result.
func (s *Stub) SendDocument(_ context.Context, orgID uuid.UUID, doc EDODocument) (*SendResult, error) {
	log.Debug().Str("org", orgID.String()).Str("type", doc.Type).Str("number", doc.Number).Msg("edo stub: SendDocument")
	return &SendResult{
		DocumentID: uuid.New().String(),
		Status:     "sent",
	}, nil
}

// GetIncomingDocuments logs the call and returns an empty list.
func (s *Stub) GetIncomingDocuments(_ context.Context, orgID uuid.UUID, since time.Time) ([]EDODocument, error) {
	log.Debug().Str("org", orgID.String()).Time("since", since).Msg("edo stub: GetIncomingDocuments")
	return nil, nil
}

// AcceptDocument logs the call and returns nil.
func (s *Stub) AcceptDocument(_ context.Context, orgID uuid.UUID, documentID string) error {
	log.Debug().Str("org", orgID.String()).Str("doc_id", documentID).Msg("edo stub: AcceptDocument")
	return nil
}

// RejectDocument logs the call and returns nil.
func (s *Stub) RejectDocument(_ context.Context, orgID uuid.UUID, documentID string, reason string) error {
	log.Debug().Str("org", orgID.String()).Str("doc_id", documentID).Str("reason", reason).Msg("edo stub: RejectDocument")
	return nil
}
