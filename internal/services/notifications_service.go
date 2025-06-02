package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
)

type NotificationService interface {
	SendNotification(ctx context.Context, teamCode string, content string) (models.Notification, error)
	SendNotificationToAllTeams(ctx context.Context, instanceID string, content string) error
	GetNotifications(ctx context.Context, teamCode string) ([]models.Notification, error)
	DismissNotification(ctx context.Context, notificationID string) error
}

type notificationService struct {
	notificationRepository repositories.NotificationRepository
	teamRepository         repositories.TeamRepository
}

func NewNotificationService(
	notificationRepository repositories.NotificationRepository,
	teamRepository repositories.TeamRepository,
) NotificationService {
	return &notificationService{
		notificationRepository: notificationRepository,
		teamRepository:         teamRepository,
	}
}

// SendNotification sends a notification to a team.
func (s *notificationService) SendNotification(ctx context.Context, teamCode string, content string) (models.Notification, error) {
	notification := models.Notification{
		TeamCode: teamCode,
		Content:  content,
	}

	err := s.notificationRepository.Create(ctx, &notification)
	return notification, err
}

// SendNotificationToAllTeams sends a notification to all teams.
func (s *notificationService) SendNotificationToAllTeams(ctx context.Context, instanceID string, content string) error {
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

	for _, team := range teams {
		if team.HasStarted {
			_, err := s.SendNotification(ctx, team.Code, content)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// GetNotifications retrieves all notifications for a team.
func (s *notificationService) GetNotifications(ctx context.Context, teamCode string) ([]models.Notification, error) {
	return s.notificationRepository.FindByTeamCode(ctx, teamCode)
}

// DismissNotification marks a notification as dismissed.
func (s *notificationService) DismissNotification(ctx context.Context, notificationID string) error {
	err := s.notificationRepository.Dismiss(ctx, notificationID)
	if err != nil {
		return fmt.Errorf("dismiss notification: %w", err)
	}
	return nil
}
