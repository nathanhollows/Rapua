package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/uptrace/bun"
)

type QuestRepository interface {
	// Create saves an instance to the database
	Create(ctx context.Context, instance *models.Quest) error
	// CreateTx saves an instance to the database within a transaction
	CreateTx(ctx context.Context, tx *bun.Tx, instance *models.Quest) error

	// GetByID finds an instance by ID
	GetByID(ctx context.Context, id string) (*models.Quest, error)
	// FindByUserID finds all instances associated with a user ID
	FindByUserID(ctx context.Context, userID string) ([]models.Quest, error)
	// FindTemplates finds all instances that are templates
	FindTemplates(ctx context.Context, userID string) ([]models.Quest, error)

	// Update updates an instance in the database
	Update(ctx context.Context, instance *models.Quest) error
	// UpdateTx updates an instance within a transaction
	UpdateTx(ctx context.Context, tx *bun.Tx, instance *models.Quest) error

	// Delete deletes an instance from the database.
	// Deleting an instance cascades to all related data.
	Delete(ctx context.Context, tx *bun.Tx, id string) error
	// DeleteByUserID removes all instances associated with a user ID
	DeleteByUser(ctx context.Context, tx *bun.Tx, userID string) error

	// DismissQuickstart marks the user as having dismissed the quickstart
	DismissQuickstart(ctx context.Context, questID string) error

	// GetByIDWithRelations loads an instance with all relations needed for the admin panel
	GetByIDWithRelations(ctx context.Context, id string) (*models.Quest, error)
}

type questRepository struct {
	db *bun.DB
}

func NewQuestRepository(db *bun.DB) QuestRepository {
	return &questRepository{
		db: db,
	}
}

func (r *questRepository) Create(ctx context.Context, instance *models.Quest) error {
	if instance.ID == "" {
		instance.ID = uuid.New().String()
	}
	if instance.UserID == "" {
		return errors.New("UserID is required")
	}
	_, err := r.db.NewInsert().Model(instance).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (r *questRepository) CreateTx(ctx context.Context, tx *bun.Tx, instance *models.Quest) error {
	if instance.ID == "" {
		instance.ID = uuid.New().String()
	}
	if instance.UserID == "" {
		return errors.New("UserID is required")
	}
	_, err := tx.NewInsert().Model(instance).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (r *questRepository) Update(ctx context.Context, instance *models.Quest) error {
	if instance.ID == "" {
		return errors.New("ID is required")
	}
	res, err := r.db.NewUpdate().Model(instance).WherePK().Exec(ctx)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		return errors.New("instance not found")
	}
	return nil
}

func (r *questRepository) UpdateTx(ctx context.Context, tx *bun.Tx, instance *models.Quest) error {
	if instance.ID == "" {
		return errors.New("ID is required")
	}
	res, err := tx.NewUpdate().Model(instance).WherePK().Exec(ctx)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil || affected == 0 {
		return errors.New("instance not found")
	}
	return nil
}

func (r *questRepository) GetByID(ctx context.Context, id string) (*models.Quest, error) {
	instance := &models.Quest{}
	err := r.db.NewSelect().
		Model(instance).
		Where("id = ?", id).
		Relation("Locations").
		Relation("Settings").
		Relation("ShareLinks").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return instance, nil
}

func (r *questRepository) FindByUserID(ctx context.Context, userID string) ([]models.Quest, error) {
	instances := []models.Quest{}
	err := r.db.NewSelect().
		Model(&instances).
		Where("user_id = ? AND is_template = ?", userID, false).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return instances, nil
}

func (r *questRepository) FindTemplates(ctx context.Context, userID string) ([]models.Quest, error) {
	instances := []models.Quest{}
	err := r.db.NewSelect().
		Model(&instances).
		Where("user_id = ? AND is_template = ?", userID, true).
		Order("created_at DESC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return instances, nil
}

func (r *questRepository) Delete(ctx context.Context, tx *bun.Tx, id string) error {
	// Delete instance
	_, err := tx.NewDelete().Model(&models.Quest{}).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return err
	}

	// Delete CheckIns
	_, err = tx.NewDelete().Model(&models.CheckIn{}).Where("quest_id = ?", id).Exec(ctx)
	if err != nil {
		return err
	}

	// Delete locations
	_, err = tx.NewDelete().Model(&models.Location{}).Where("quest_id = ?", id).Exec(ctx)
	if err != nil {
		return err
	}

	// Delete ShareLinks
	_, err = tx.NewDelete().Model(&models.ShareLink{}).Where("template_id = ?", id).Exec(ctx)
	if err != nil {
		return err
	}

	return nil
}

// DeleteByUserID removes all instances associated with a user ID.
func (r *questRepository) DeleteByUser(ctx context.Context, tx *bun.Tx, userID string) error {
	instances, err := r.FindByUserID(ctx, userID)
	if err != nil {
		return fmt.Errorf("finding instances by user ID: %w", err)
	}
	for _, instance := range instances {
		if deleteErr := r.Delete(ctx, tx, instance.ID); deleteErr != nil {
			return fmt.Errorf("deleting instance: %w", deleteErr)
		}
	}
	return nil
}

// GetByIDWithRelations loads an instance with all relations needed for the admin panel.
func (r *questRepository) GetByIDWithRelations(ctx context.Context, id string) (*models.Quest, error) {
	instance := &models.Quest{}
	err := r.db.NewSelect().
		Model(instance).
		Where("quest.id = ?", id).
		Relation("Settings").
		Relation("Runs").
		Relation("Locations", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Order("order ASC")
		}).
		Relation("Locations.Marker").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return instance, nil
}

// DismissQuickstart marks the user as having dismissed the quickstart.
func (r *questRepository) DismissQuickstart(ctx context.Context, questID string) error {
	_, err := r.db.NewUpdate().
		Model(&models.Quest{}).
		Set("is_quick_start_dismissed = ?", true).
		Where("id = ?", questID).
		Exec(ctx)
	return err
}
