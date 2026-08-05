package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/uptrace/bun"
)

type QuestSettingsRepository interface {
	// Create new instance settings to the database
	Create(ctx context.Context, settings *models.QuestSettings) error
	// CreateTx creates new instance settings within a transaction
	CreateTx(ctx context.Context, tx *bun.Tx, settings *models.QuestSettings) error

	// Update updates an instance in the database
	Update(ctx context.Context, settings *models.QuestSettings) error
	// UpdateTx updates instance settings within a transaction
	UpdateTx(ctx context.Context, tx *bun.Tx, settings *models.QuestSettings) error

	// GetByQuestID retrieves instance settings by instance ID
	GetByQuestID(ctx context.Context, questID string) (*models.QuestSettings, error)
}

type instanceSettingsRepository struct {
	db *bun.DB
}

func NewQuestSettingsRepository(db *bun.DB) QuestSettingsRepository {
	return &instanceSettingsRepository{
		db: db,
	}
}

func (r *instanceSettingsRepository) Create(ctx context.Context, settings *models.QuestSettings) error {
	if settings.QuestID == "" {
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
	settings *models.QuestSettings,
) error {
	if settings.QuestID == "" {
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

func (r *instanceSettingsRepository) Update(ctx context.Context, settings *models.QuestSettings) error {
	if settings.QuestID == "" {
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
	settings *models.QuestSettings,
) error {
	if settings.QuestID == "" {
		return errors.New("instance ID is required")
	}
	settings.UpdatedAt = time.Now().UTC()
	_, err := tx.NewUpdate().Model(settings).WherePK().Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

// GetByQuestID retrieves instance settings by instance ID.
func (r *instanceSettingsRepository) GetByQuestID(
	ctx context.Context,
	questID string,
) (*models.QuestSettings, error) {
	if questID == "" {
		return nil, errors.New("instance ID is required")
	}

	var settings models.QuestSettings
	err := r.db.NewSelect().
		Model(&settings).
		Where("quest_id = ?", questID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return &settings, nil
}
