package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/uptrace/bun"
)

type InstanceSettingsRepository interface {
	// Create new instance settings to the database
	Create(ctx context.Context, settings *models.InstanceSettings) error
	// CreateTx creates new instance settings within a transaction
	CreateTx(ctx context.Context, tx *bun.Tx, settings *models.InstanceSettings) error

	// Update updates an instance in the database
	Update(ctx context.Context, settings *models.InstanceSettings) error
	// UpdateTx updates instance settings within a transaction
	UpdateTx(ctx context.Context, tx *bun.Tx, settings *models.InstanceSettings) error

	// GetByInstanceID retrieves instance settings by instance ID
	GetByInstanceID(ctx context.Context, instanceID string) (*models.InstanceSettings, error)
}

type instanceSettingsRepository struct {
	db *bun.DB
}

func NewInstanceSettingsRepository(db *bun.DB) InstanceSettingsRepository {
	return &instanceSettingsRepository{
		db: db,
	}
}

func (r *instanceSettingsRepository) Create(ctx context.Context, settings *models.InstanceSettings) error {
	if settings.InstanceID == "" {
		return errors.New("instance ID is required")
	}
	settings.CreatedAt = time.Now().UTC()
	settings.UpdatedAt = time.Now().UTC()
	_, err := r.db.NewInsert().Model(settings).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (r *instanceSettingsRepository) CreateTx(
	ctx context.Context,
	tx *bun.Tx,
	settings *models.InstanceSettings,
) error {
	if settings.InstanceID == "" {
		return errors.New("instance ID is required")
	}
	settings.CreatedAt = time.Now().UTC()
	settings.UpdatedAt = time.Now().UTC()
	_, err := tx.NewInsert().Model(settings).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (r *instanceSettingsRepository) Update(ctx context.Context, settings *models.InstanceSettings) error {
	if settings.InstanceID == "" {
		return errors.New("instance ID is required")
	}
	settings.UpdatedAt = time.Now().UTC()
	_, err := r.db.NewUpdate().Model(settings).WherePK().Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (r *instanceSettingsRepository) UpdateTx(
	ctx context.Context,
	tx *bun.Tx,
	settings *models.InstanceSettings,
) error {
	if settings.InstanceID == "" {
		return errors.New("instance ID is required")
	}
	settings.UpdatedAt = time.Now().UTC()
	_, err := tx.NewUpdate().Model(settings).WherePK().Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

// GetByInstanceID retrieves instance settings by instance ID.
func (r *instanceSettingsRepository) GetByInstanceID(
	ctx context.Context,
	instanceID string,
) (*models.InstanceSettings, error) {
	if instanceID == "" {
		return nil, errors.New("instance ID is required")
	}

	var settings models.InstanceSettings
	err := r.db.NewSelect().
		Model(&settings).
		Where("instance_id = ?", instanceID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return &settings, nil
}
