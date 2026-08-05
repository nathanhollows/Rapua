package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/uptrace/bun"
)

type NotificationRepository interface {
	//	Create saves a notification to the database
	Create(context.Context, *models.Notification) error

	//	GetByID finds a notification by its ID
	GetByID(ctx context.Context, id string) (models.Notification, error)
	// FindByRunCode finds all notifications for a specific team code
	FindByRunCode(ctx context.Context, runCode string) ([]models.Notification, error)

	//	Update updates a notification in the database
	Update(context.Context, *models.Notification) error
	// Dismiss marks a notification as dismissed
	Dismiss(ctx context.Context, id string) error

	//	Delete deletes a notification from the database
	Delete(ctx context.Context, id string) error
}

type notificationRepository struct {
	db *bun.DB
}

func NewNotificationRepository(db *bun.DB) NotificationRepository {
	return &notificationRepository{
		db: db,
	}
}

// Create inserts a new notification into the database.
func (r *notificationRepository) Create(ctx context.Context, notification *models.Notification) error {
	// Validate the notification
	if notification.Content == "" {
		return errors.New("message is required")
	}
	if notification.RunCode == "" {
		return errors.New("run_code is required")
	}

	// Generate a new ID if one doesn't exist
	if notification.ID == "" {
		notification.ID = uuid.New().String()
	}

	// Save the notification
	_, err := r.db.NewInsert().Model(notification).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

// Dismiss marks a notification as dismissed.
func (r *notificationRepository) Dismiss(ctx context.Context, id string) error {
	_, err := r.db.NewUpdate().Model(&models.Notification{}).Set("dismissed = true").Where("id = ?", id).Exec(ctx)
	return err
}

// Update updates an existing notification in the database.
func (r *notificationRepository) Update(ctx context.Context, notification *models.Notification) error {
	if notification.ID == "" {
		return errors.New("ID is required")
	}

	_, err := r.db.NewUpdate().Model(notification).WherePK().Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

// Delete removes a notification from the database.
func (r *notificationRepository) Delete(ctx context.Context, id string) error {
	_, err := r.db.NewDelete().Model(&models.Notification{}).Where("id = ?", id).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

// GetByID finds a notification by its ID.
func (r *notificationRepository) GetByID(ctx context.Context, id string) (models.Notification, error) {
	notification := models.Notification{}
	err := r.db.NewSelect().Model(&notification).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return notification, err
	}
	return notification, nil
}

// FindByRunCode finds all notifications for a specific team code.
func (r *notificationRepository) FindByRunCode(ctx context.Context, runCode string) ([]models.Notification, error) {
	var notifications []models.Notification
	err := r.db.NewSelect().Model(&notifications).Where("run_code = ? AND NOT dismissed", runCode).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("FindByRunCode: %w", err)
	}
	return notifications, nil
}
