package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/uptrace/bun"
)

type UploadsRepository struct {
	db *bun.DB
}

func NewUploadRepository(db *bun.DB) UploadsRepository {
	return UploadsRepository{db: db}
}

func (r *UploadsRepository) Create(ctx context.Context, upload *models.Upload) error {
	if upload == nil {
		return errors.New("upload is nil")
	}
	if upload.ID == "" {
		upload.ID = uuid.New().String()
	}
	_, err := r.db.NewInsert().Model(upload).Exec(ctx)
	return err
}

func (r *UploadsRepository) SearchByCriteria(
	ctx context.Context,
	criteria map[string]string,
) ([]*models.Upload, error) {
	var uploads []*models.Upload
	query := r.db.NewSelect().Model(&uploads)

	if len(criteria) == 0 {
		return nil, errors.New("search criteria cannot be empty")
	}

	for key, value := range criteria {
		switch key {
		case "id", "location_id", "instance_id", "team_code", "block_id", "storage", "type":
			if value == "NULL" {
				query = query.Where("? IS NULL", bun.Ident(key))
			} else {
				query = query.Where("? = ?", bun.Ident(key), value)
			}
		default:
			return nil, errors.New("invalid search field: " + key)
		}
	}

	err := query.Scan(ctx)
	return uploads, err
}

// GetByBlockID retrieves all uploads associated with a specific block.
func (r *UploadsRepository) GetByBlockID(ctx context.Context, blockID string) ([]*models.Upload, error) {
	var uploads []*models.Upload
	err := r.db.NewSelect().
		Model(&uploads).
		Where("block_id = ?", blockID).
		Scan(ctx)
	return uploads, err
}

// Delete removes an upload record by ID.
func (r *UploadsRepository) Delete(ctx context.Context, uploadID string) error {
	_, err := r.db.NewDelete().
		Model((*models.Upload)(nil)).
		Where("id = ?", uploadID).
		Exec(ctx)
	return err
}

// DeleteByBlockID removes all upload records associated with a specific block.
func (r *UploadsRepository) DeleteByBlockID(ctx context.Context, blockID string) error {
	_, err := r.db.NewDelete().
		Model((*models.Upload)(nil)).
		Where("block_id = ?", blockID).
		Exec(ctx)
	return err
}

// GetOrphanedUploads retrieves all uploads that reference non-existent blocks.
// This includes uploads where block_id is not null but the block doesn't exist.
func (r *UploadsRepository) GetOrphanedUploads(ctx context.Context) ([]*models.Upload, error) {
	var uploads []*models.Upload
	err := r.db.NewSelect().
		Model(&uploads).
		Where("block_id IS NOT NULL").
		Where("block_id NOT IN (SELECT id FROM blocks)").
		Scan(ctx)
	return uploads, err
}
