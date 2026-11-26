package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
)

type NotificationRepository interface {
	//	GetByID finds a notification by its ID
	GetByID(ctx context.Context, id string) (models.Notification, error)
	// FindByRunCode finds all notifications for a specific run code
	FindByRunCode(ctx context.Context, runCode string) ([]models.Notification, error)

	// CreateWithTx saves a notification inside a caller's transaction, so the
	// message and the event recording it commit together.
	CreateWithTx(ctx context.Context, tx *bun.Tx, notification *models.Notification) error
	// DismissWithTx marks a notification dismissed inside a caller's transaction, so
	// the dismissal and its event commit together.
	DismissWithTx(ctx context.Context, tx *bun.Tx, id string) error

	//	Update updates a notification in the database
	Update(context.Context, *models.Notification) error

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

// CreateWithTx inserts a new notification inside a caller's transaction.
func (r *notificationRepository) CreateWithTx(
	ctx context.Context, tx *bun.Tx, notification *models.Notification,
) error {
	return createNotification(ctx, tx, notification)
}

func createNotification(ctx context.Context, db bun.IDB, notification *models.Notification) error {
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
	_, err := db.NewInsert().Model(notification).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

// DismissWithTx marks a notification as dismissed inside a caller's transaction.
func (r *notificationRepository) DismissWithTx(ctx context.Context, tx *bun.Tx, id string) error {
	return dismissNotification(ctx, tx, id)
}

func dismissNotification(ctx context.Context, db bun.IDB, id string) error {
	_, err := db.NewUpdate().Model(&models.Notification{}).Set("dismissed = true").Where("id = ?", id).Exec(ctx)
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

// FindByRunCode finds all notifications for a specific run code.
func (r *notificationRepository) FindByRunCode(ctx context.Context, runCode string) ([]models.Notification, error) {
	var notifications []models.Notification
	err := r.db.NewSelect().Model(&notifications).Where("run_code = ? AND NOT dismissed", runCode).Scan(ctx)
	if err != nil {
		return nil, fmt.Errorf("FindByRunCode: %w", err)
	}
	return notifications, nil
}
