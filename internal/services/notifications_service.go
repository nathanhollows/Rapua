package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
)

type NotificationService struct {
	transactor             db.Transactor
	notificationRepository repositories.NotificationRepository
	teamRepository         repositories.RunRepository
	runEventRepo           repositories.RunEventRepository
}

func NewNotificationService(
	transactor db.Transactor,
	notificationRepository repositories.NotificationRepository,
	teamRepository repositories.RunRepository,
	runEventRepo repositories.RunEventRepository,
) *NotificationService {
	return &NotificationService{
		transactor:             transactor,
		notificationRepository: notificationRepository,
		teamRepository:         teamRepository,
		runEventRepo:           runEventRepo,
	}
}

// SendNotification sends a notification to a team.
func (s *NotificationService) SendNotification(
	ctx context.Context,
	runCode string,
	content string,
) (models.Notification, error) {
	// The event needs the quest, which a run code does not carry.
	team, err := s.teamRepository.GetByCode(ctx, runCode)
	if err != nil {
		return models.Notification{}, fmt.Errorf("finding run for notification: %w", err)
	}

	return s.sendNotification(ctx, team, content)
}

// sendNotification writes the message and the event recording it together. The
// caller holds the run, so the quest comes from it rather than another lookup.
func (s *NotificationService) sendNotification(
	ctx context.Context, team *models.Run, content string,
) (models.Notification, error) {
	notification := models.Notification{
		RunCode: team.Code,
		Content: content,
	}

	tx, err := s.transactor.BeginTx(ctx, nil)
	if err != nil {
		return models.Notification{}, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.notificationRepository.CreateWithTx(ctx, tx, &notification); err != nil {
		return models.Notification{}, err
	}

	if err := s.runEventRepo.RecordWithTx(
		ctx, tx, notificationSentEvent(team, notification.ID, notification.Content),
	); err != nil {
		return models.Notification{}, fmt.Errorf("recording notification: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return models.Notification{}, fmt.Errorf("committing notification: %w", err)
	}

	return notification, nil
}

// SendNotificationToAllTeams sends a notification to all teams.
func (s *NotificationService) SendNotificationToAllTeams(ctx context.Context, instanceID string, content string) error {
	teams, err := s.teamRepository.FindAll(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("error finding teams: %w", err)
	}

	if len(teams) == 0 {
		return errors.New("no teams to send notification to")
	}

	if content == "" {
		return errors.New("content cannot be empty")
	}

	for i := range teams {
		if !teams[i].HasStarted {
			continue
		}
		// The runs are already loaded, so the quest comes from the row in hand
		// rather than a lookup per team.
		if _, err = s.sendNotification(ctx, &teams[i], content); err != nil {
			return err
		}
	}
	return nil
}

// GetNotifications retrieves all notifications for a team.
func (s *NotificationService) GetNotifications(ctx context.Context, runCode string) ([]models.Notification, error) {
	return s.notificationRepository.FindByRunCode(ctx, runCode)
}

// DismissNotification marks a notification as dismissed.
//
// A missing id is not an error: the player's dismiss may simply have arrived twice.
func (s *NotificationService) DismissNotification(ctx context.Context, notificationID string) error {
	notification, err := s.notificationRepository.GetByID(ctx, notificationID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("finding notification: %w", err)
	}

	// Already dismissed. The read happened once and the record is the record: a
	// second press is the same thing again only as far as the button is concerned.
	if notification.Dismissed {
		return nil
	}

	team, err := s.teamRepository.GetByCode(ctx, notification.RunCode)
	if err != nil {
		return fmt.Errorf("finding run for notification: %w", err)
	}

	tx, err := s.transactor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := s.notificationRepository.DismissWithTx(ctx, tx, notificationID); err != nil {
		return fmt.Errorf("dismiss notification: %w", err)
	}

	if err := s.runEventRepo.RecordWithTx(ctx, tx, notificationReadEvent(team, notificationID)); err != nil {
		return fmt.Errorf("recording notification read: %w", err)
	}

	return tx.Commit()
}
