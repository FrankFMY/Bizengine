package file

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/bizengine/engine/pkg/errs"
)

type ObjectStorage interface {
	PresignedPutURL(ctx context.Context, key, contentType string, ttl time.Duration) (string, error)
	PresignedGetURL(ctx context.Context, key string, ttl time.Duration) (string, error)
	HeadObject(ctx context.Context, key string) (*ObjectMeta, error)
	Delete(ctx context.Context, key string) error
}

type ObjectMeta struct {
	ContentType   string
	ContentLength int64
	LastModified  time.Time
}

type Service struct {
	repo Repository
	s3   ObjectStorage
}

func NewService(repo Repository, s3 ObjectStorage) *Service {
	return &Service{repo: repo, s3: s3}
}

type UploadRequest struct {
	Name        string     `json:"name"`
	ContentType string     `json:"content_type"`
	EntityID    *uuid.UUID `json:"entity_id,omitempty"`
}

type UploadResponse struct {
	FileID    uuid.UUID `json:"file_id"`
	UploadURL string    `json:"upload_url"`
}

func (s *Service) RequestUpload(ctx context.Context, orgID uuid.UUID, input UploadRequest, userID *uuid.UUID) (*UploadResponse, error) {
	if input.Name == "" {
		return nil, errs.NewBadRequest("name is required")
	}
	if input.ContentType == "" {
		input.ContentType = "application/octet-stream"
	}

	id := uuid.New()
	s3Key := fmt.Sprintf("orgs/%s/files/%s/%s", orgID, id, input.Name)

	f := &File{
		ID:             id,
		OrganizationID: orgID,
		Name:           input.Name,
		S3Key:          s3Key,
		ContentType:    input.ContentType,
		Status:         "pending",
		UploadedBy:     userID,
		EntityID:       input.EntityID,
		CreatedAt:      time.Now(),
	}

	if err := s.repo.Create(ctx, f); err != nil {
		return nil, fmt.Errorf("file: create record: %w", err)
	}

	url, err := s.s3.PresignedPutURL(ctx, s3Key, input.ContentType, 10*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("file: presign upload: %w", err)
	}

	return &UploadResponse{
		FileID:    id,
		UploadURL: url,
	}, nil
}

func (s *Service) Confirm(ctx context.Context, orgID, fileID uuid.UUID) (*File, error) {
	f, err := s.repo.Get(ctx, orgID, fileID)
	if err != nil {
		return nil, err
	}
	if f.Status != "pending" {
		return nil, errs.NewConflict("file already confirmed")
	}

	meta, err := s.s3.HeadObject(ctx, f.S3Key)
	if err != nil {
		return nil, errs.NewBadRequest("file not found in storage — upload may not have completed")
	}

	if err := s.repo.Confirm(ctx, orgID, fileID, meta.ContentLength); err != nil {
		return nil, fmt.Errorf("file: confirm: %w", err)
	}

	f.Status = "confirmed"
	f.SizeBytes = meta.ContentLength
	now := time.Now()
	f.ConfirmedAt = &now
	return f, nil
}

func (s *Service) GetDownloadURL(ctx context.Context, orgID, fileID uuid.UUID) (string, *File, error) {
	f, err := s.repo.Get(ctx, orgID, fileID)
	if err != nil {
		return "", nil, err
	}

	url, err := s.s3.PresignedGetURL(ctx, f.S3Key, 15*time.Minute)
	if err != nil {
		return "", nil, fmt.Errorf("file: presign download: %w", err)
	}

	return url, f, nil
}

func (s *Service) Delete(ctx context.Context, orgID, fileID uuid.UUID) error {
	f, err := s.repo.Get(ctx, orgID, fileID)
	if err != nil {
		return err
	}

	if err := s.s3.Delete(ctx, f.S3Key); err != nil {
		return fmt.Errorf("file: delete from s3: %w", err)
	}

	return s.repo.Delete(ctx, orgID, fileID)
}

func (s *Service) ListByEntity(ctx context.Context, orgID, entityID uuid.UUID) ([]File, error) {
	return s.repo.ListByEntity(ctx, orgID, entityID)
}
