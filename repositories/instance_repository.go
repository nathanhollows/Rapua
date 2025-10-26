package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/uptrace/bun"
)

type InstanceRepository interface {
	// Create saves an instance to the database
	Create(ctx context.Context, instance *models.Instance) error
	// CreateTx saves an instance to the database within a transaction
	CreateTx(ctx context.Context, tx *bun.Tx, instance *models.Instance) error

	// GetByID finds an instance by ID
	GetByID(ctx context.Context, id string) (*models.Instance, error)
	// FindByUserID finds all instances associated with a user ID
	FindByUserID(ctx context.Context, userID string) ([]models.Instance, error)
	// FindTemplates finds all instances that are templates
	FindTemplates(ctx context.Context, userID string) ([]models.Instance, error)

	// Update updates an instance in the database
	Update(ctx context.Context, instance *models.Instance) error

	// Delete deletes an instance from the database.
	// Deleting an instance cascades to all related data.
	Delete(ctx context.Context, tx *bun.Tx, id string) error
	// DeleteByUserID removes all instances associated with a user ID
	DeleteByUser(ctx context.Context, tx *bun.Tx, userID string) error

	// DismissQuickstart marks the user as having dismissed the quickstart
	DismissQuickstart(ctx context.Context, instanceID string) error
}

type instanceRepository struct {
	db *bun.DB
}

func NewInstanceRepository(db *bun.DB) InstanceRepository {
	return &instanceRepository{
		db: db,
	}
}

func (r *instanceRepository) Create(ctx context.Context, instance *models.Instance) error {
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

func (r *instanceRepository) CreateTx(ctx context.Context, tx *bun.Tx, instance *models.Instance) error {
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

func (r *instanceRepository) Update(ctx context.Context, instance *models.Instance) error {
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

func (r *instanceRepository) GetByID(ctx context.Context, id string) (*models.Instance, error) {
	instance := &models.Instance{}
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

func (r *instanceRepository) FindByUserID(ctx context.Context, userID string) ([]models.Instance, error) {
	instances := []models.Instance{}
	err := r.db.NewSelect().
		Model(&instances).
		Where("user_id = ? AND is_template = ?", userID, false).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return instances, nil
}

func (r *instanceRepository) FindTemplates(ctx context.Context, userID string) ([]models.Instance, error) {
	instances := []models.Instance{}
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

func (r *instanceRepository) Delete(ctx context.Context, tx *bun.Tx, id string) error {
	// Delete instance
	_, err := tx.NewDelete().Model(&models.Instance{}).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return err
	}

	// Delete CheckIns
	_, err = tx.NewDelete().Model(&models.CheckIn{}).Where("instance_id = ?", id).Exec(ctx)
	if err != nil {
		return err
	}

	// Delete locations
	_, err = tx.NewDelete().Model(&models.Location{}).Where("instance_id = ?", id).Exec(ctx)
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
func (r *instanceRepository) DeleteByUser(ctx context.Context, tx *bun.Tx, userID string) error {
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

// DismissQuickstart marks the user as having dismissed the quickstart.
func (r *instanceRepository) DismissQuickstart(ctx context.Context, instanceID string) error {
	_, err := r.db.NewUpdate().
		Model(&models.Instance{}).
		Set("is_quick_start_dismissed = ?", true).
		Where("id = ?", instanceID).
		Exec(ctx)
	return err
}
