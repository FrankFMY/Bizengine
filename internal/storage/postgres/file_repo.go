package postgres

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bizengine/engine/internal/module/file"
	"github.com/bizengine/engine/pkg/errs"
)

type FileRepo struct {
	pool *pgxpool.Pool
}

func NewFileRepo(pool *pgxpool.Pool) *FileRepo {
	return &FileRepo{pool: pool}
}

func (r *FileRepo) Create(ctx context.Context, f *file.File) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO files (id, organization_id, name, s3_key, content_type, size_bytes, status, uploaded_by, entity_id, meta, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		f.ID, f.OrganizationID, f.Name, f.S3Key, f.ContentType, f.SizeBytes, f.Status,
		f.UploadedBy, f.EntityID, f.Meta, f.CreatedAt,
	)
	return err
}

func (r *FileRepo) Get(ctx context.Context, orgID, id uuid.UUID) (*file.File, error) {
	var f file.File
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, name, s3_key, content_type, size_bytes, status, uploaded_by, entity_id, meta, created_at, confirmed_at
		 FROM files WHERE organization_id = $1 AND id = $2`,
		orgID, id,
	).Scan(&f.ID, &f.OrganizationID, &f.Name, &f.S3Key, &f.ContentType, &f.SizeBytes, &f.Status,
		&f.UploadedBy, &f.EntityID, &f.Meta, &f.CreatedAt, &f.ConfirmedAt)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, errs.NewNotFound("file not found")
		}
		return nil, err
	}
	return &f, nil
}

func (r *FileRepo) Confirm(ctx context.Context, orgID, id uuid.UUID, sizeBytes int64) error {
	tag, err := r.pool.Exec(ctx,
		`UPDATE files SET status = 'confirmed', size_bytes = $3, confirmed_at = now() WHERE organization_id = $1 AND id = $2 AND status = 'pending'`,
		orgID, id, sizeBytes,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NewConflict("file already confirmed or not found")
	}
	return nil
}

func (r *FileRepo) Delete(ctx context.Context, orgID, id uuid.UUID) error {
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM files WHERE organization_id = $1 AND id = $2`,
		orgID, id,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return errs.NewNotFound("file not found")
	}
	return nil
}

func (r *FileRepo) ListByEntity(ctx context.Context, orgID, entityID uuid.UUID) ([]file.File, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, name, s3_key, content_type, size_bytes, status, uploaded_by, entity_id, meta, created_at, confirmed_at
		 FROM files WHERE organization_id = $1 AND entity_id = $2 ORDER BY created_at DESC`,
		orgID, entityID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []file.File
	for rows.Next() {
		var f file.File
		if err := rows.Scan(&f.ID, &f.OrganizationID, &f.Name, &f.S3Key, &f.ContentType, &f.SizeBytes, &f.Status,
			&f.UploadedBy, &f.EntityID, &f.Meta, &f.CreatedAt, &f.ConfirmedAt); err != nil {
			return nil, err
		}
		files = append(files, f)
	}
	return files, rows.Err()
}
