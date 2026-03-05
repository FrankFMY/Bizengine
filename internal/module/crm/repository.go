package crm

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/bizengine/engine/pkg/types"
)

// Counterparty wraps an entity with CRM-specific components.
type Counterparty struct {
	types.Entity
	Profile map[string]any `json:"profile,omitempty"`
	Balance map[string]any `json:"balance,omitempty"`
	Contact map[string]any `json:"contact,omitempty"`
}

// CreateInput is the input for creating a customer or supplier.
type CreateInput struct {
	Kind    string         `json:"kind"` // customer or supplier
	Name    string         `json:"name"`
	Contact map[string]any `json:"contact"`
	Profile map[string]any `json:"profile"`
	Balance map[string]any `json:"balance"`
}

// UpdateInput is the input for updating a counterparty.
type UpdateInput struct {
	Name    *string        `json:"name"`
	Contact map[string]any `json:"contact"`
	Profile map[string]any `json:"profile"`
	Balance map[string]any `json:"balance"`
}

// TagsInput is the input for modifying tags.
type TagsInput struct {
	Add    []string `json:"add"`
	Remove []string `json:"remove"`
}

// CounterpartyFilter holds query params for listing counterparties.
type CounterpartyFilter struct {
	Search   *string
	Tag      *string
	Category *string
	Page     types.PageRequest
}

// Repository defines data access for the CRM module.
type Repository interface {
	GetOrdersByCustomer(ctx context.Context, orgID, customerID uuid.UUID, page types.PageRequest) ([]json.RawMessage, int, error)
	GetTransactionsByCounterparty(ctx context.Context, orgID, counterpartyID uuid.UUID, page types.PageRequest) ([]json.RawMessage, int, error)
	GetDeliveriesBySupplier(ctx context.Context, orgID, supplierID uuid.UUID, page types.PageRequest) ([]json.RawMessage, int, error)
}
