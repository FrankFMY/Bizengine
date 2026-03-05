package notification

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/pkg/types"
)

type Notification struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	UserID         uuid.UUID  `json:"user_id"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	Severity       string     `json:"severity"`
	Read           bool       `json:"read"`
	ReferenceType  *string    `json:"reference_type,omitempty"`
	ReferenceID    *uuid.UUID `json:"reference_id,omitempty"`
	Ver            int        `json:"ver"`
	Upd            time.Time  `json:"upd"`
	Iat            time.Time  `json:"iat"`
}

type ListFilter struct {
	Read *bool
	Page types.PageRequest
}

type Repository interface {
	Create(ctx context.Context, n *Notification) error
	List(ctx context.Context, orgID, userID uuid.UUID, filter ListFilter) ([]Notification, int, error)
	MarkRead(ctx context.Context, orgID, userID, notifID uuid.UUID) error
	MarkAllRead(ctx context.Context, orgID, userID uuid.UUID) error
	CountUnread(ctx context.Context, orgID, userID uuid.UUID) (int, error)
}
