package file

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Repository interface {
	Create(ctx context.Context, f *File) error
	Get(ctx context.Context, orgID, id uuid.UUID) (*File, error)
	Confirm(ctx context.Context, orgID, id uuid.UUID, sizeBytes int64) error
	Delete(ctx context.Context, orgID, id uuid.UUID) error
	ListByEntity(ctx context.Context, orgID, entityID uuid.UUID) ([]File, error)
}

type File struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	Name           string     `json:"name"`
	S3Key          string     `json:"s3_key"`
	ContentType    string     `json:"content_type"`
	SizeBytes      int64      `json:"size_bytes"`
	Status         string     `json:"status"`
	UploadedBy     *uuid.UUID `json:"uploaded_by,omitempty"`
	EntityID       *uuid.UUID `json:"entity_id,omitempty"`
	Meta           []byte     `json:"meta,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ConfirmedAt    *time.Time `json:"confirmed_at,omitempty"`
}
