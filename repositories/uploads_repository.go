package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v4/models"
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

func (r *UploadsRepository) SearchByCriteria(ctx context.Context, criteria map[string]string) ([]*models.Upload, error) {
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
