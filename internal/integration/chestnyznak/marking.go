// Package chestnyznak provides the product marking service interface for Russian Chestny Znak system.
package chestnyznak

import (
	"context"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// MarkingInfo contains information about a marked product code.
type MarkingInfo struct {
	Code       string `json:"code"`
	Valid      bool   `json:"valid"`
	ProductName string `json:"product_name"`
	Category   string `json:"category"` // tobacco, pharma, shoes, clothes, dairy
	Status     string `json:"status"`   // introduced, in_circulation, retired
}

// MarkingService defines the interface for product marking operations.
type MarkingService interface {
	VerifyCode(ctx context.Context, code string) (*MarkingInfo, error)
	RegisterReceipt(ctx context.Context, wsID uuid.UUID, codes []string, documentID string) error
	RegisterShipment(ctx context.Context, wsID uuid.UUID, codes []string, counterpartyINN string) error
}

// Stub is a stub implementation of MarkingService that logs calls and returns success.
type Stub struct{}

// NewStub creates a new stub MarkingService.
func NewStub() *Stub { return &Stub{} }

// VerifyCode logs the call and returns a fake valid result.
func (s *Stub) VerifyCode(_ context.Context, code string) (*MarkingInfo, error) {
	log.Debug().Str("code", code).Msg("chestnyznak stub: VerifyCode")
	return &MarkingInfo{
		Code:       code,
		Valid:      true,
		ProductName: "Stub Product",
		Category:   "unknown",
		Status:     "in_circulation",
	}, nil
}

// RegisterReceipt logs the call and returns nil.
func (s *Stub) RegisterReceipt(_ context.Context, wsID uuid.UUID, codes []string, documentID string) error {
	log.Debug().Str("ws", wsID.String()).Int("codes", len(codes)).Str("doc_id", documentID).Msg("chestnyznak stub: RegisterReceipt")
	return nil
}

// RegisterShipment logs the call and returns nil.
func (s *Stub) RegisterShipment(_ context.Context, wsID uuid.UUID, codes []string, counterpartyINN string) error {
	log.Debug().Str("ws", wsID.String()).Int("codes", len(codes)).Str("inn", counterpartyINN).Msg("chestnyznak stub: RegisterShipment")
	return nil
}
