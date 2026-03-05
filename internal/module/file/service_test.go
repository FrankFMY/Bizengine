package file

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRepo struct {
	files map[uuid.UUID]*File
}

func newMockRepo() *mockRepo {
	return &mockRepo{files: make(map[uuid.UUID]*File)}
}

func (m *mockRepo) Create(_ context.Context, f *File) error {
	m.files[f.ID] = f
	return nil
}

func (m *mockRepo) Get(_ context.Context, orgID, id uuid.UUID) (*File, error) {
	f, ok := m.files[id]
	if !ok || f.OrganizationID != orgID {
		return nil, &notFoundErr{}
	}
	return f, nil
}

func (m *mockRepo) Confirm(_ context.Context, orgID, id uuid.UUID, sizeBytes int64) error {
	f, ok := m.files[id]
	if !ok || f.OrganizationID != orgID {
		return &notFoundErr{}
	}
	f.Status = "confirmed"
	f.SizeBytes = sizeBytes
	now := time.Now()
	f.ConfirmedAt = &now
	return nil
}

func (m *mockRepo) Delete(_ context.Context, orgID, id uuid.UUID) error {
	f, ok := m.files[id]
	if !ok || f.OrganizationID != orgID {
		return &notFoundErr{}
	}
	delete(m.files, id)
	return nil
}

func (m *mockRepo) ListByEntity(_ context.Context, orgID, entityID uuid.UUID) ([]File, error) {
	var result []File
	for _, f := range m.files {
		if f.OrganizationID == orgID && f.EntityID != nil && *f.EntityID == entityID {
			result = append(result, *f)
		}
	}
	return result, nil
}

type notFoundErr struct{}

func (e *notFoundErr) Error() string { return "not found" }

type mockS3 struct {
	objects map[string]*ObjectMeta
}

func (m *mockS3) PresignedPutURL(_ context.Context, key, _ string, _ time.Duration) (string, error) {
	return "https://s3.example.com/upload/" + key, nil
}

func (m *mockS3) PresignedGetURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://s3.example.com/download/" + key, nil
}

func (m *mockS3) HeadObject(_ context.Context, key string) (*ObjectMeta, error) {
	if m.objects != nil {
		if meta, ok := m.objects[key]; ok {
			return meta, nil
		}
	}
	return &ObjectMeta{ContentType: "application/octet-stream", ContentLength: 1024}, nil
}

func (m *mockS3) Delete(_ context.Context, _ string) error {
	return nil
}

func TestRequestUpload_Validation(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, nil)

	_, err := svc.RequestUpload(context.Background(), uuid.New(), UploadRequest{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "name is required")
}

func TestRequestUpload_CreatesRecord(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, &mockS3{})
	orgID := uuid.New()
	userID := uuid.New()

	resp, err := svc.RequestUpload(context.Background(), orgID, UploadRequest{
		Name:        "photo.jpg",
		ContentType: "image/jpeg",
	}, &userID)

	require.NoError(t, err)
	assert.NotEmpty(t, resp.UploadURL)
	assert.Len(t, repo.files, 1)
	for _, f := range repo.files {
		assert.Equal(t, "photo.jpg", f.Name)
		assert.Equal(t, "image/jpeg", f.ContentType)
		assert.Equal(t, "pending", f.Status)
		assert.Equal(t, &userID, f.UploadedBy)
		assert.Contains(t, f.S3Key, orgID.String())
	}
}

func TestRequestUpload_DefaultContentType(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, &mockS3{})

	resp, err := svc.RequestUpload(context.Background(), uuid.New(), UploadRequest{
		Name: "data.bin",
	}, nil)

	require.NoError(t, err)
	assert.NotEmpty(t, resp.FileID)
	for _, f := range repo.files {
		assert.Equal(t, "application/octet-stream", f.ContentType)
	}
}

func TestListByEntity(t *testing.T) {
	repo := newMockRepo()
	svc := NewService(repo, nil)

	orgID := uuid.New()
	entityID := uuid.New()
	otherEntity := uuid.New()

	repo.files[uuid.New()] = &File{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EntityID:       &entityID,
		Name:           "a.txt",
	}
	repo.files[uuid.New()] = &File{
		ID:             uuid.New(),
		OrganizationID: orgID,
		EntityID:       &otherEntity,
		Name:           "b.txt",
	}

	files, err := svc.ListByEntity(context.Background(), orgID, entityID)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "a.txt", files[0].Name)
}
