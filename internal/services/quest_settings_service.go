package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v7/internal/repositories"
	"github.com/nathanhollows/Rapua/v7/models"
)

type QuestSettingsService struct {
	instanceSettingsRepo repositories.QuestSettingsRepository
}

func NewQuestSettingsService(
	instanceSettingsRepo repositories.QuestSettingsRepository,
) *QuestSettingsService {
	return &QuestSettingsService{
		instanceSettingsRepo: instanceSettingsRepo,
	}
}

// GetQuestSettings retrieves the settings for the given instance ID.
func (s *QuestSettingsService) GetQuestSettings(
	ctx context.Context,
	questID string,
) (*models.QuestSettings, error) {
	if questID == "" {
		return nil, errors.New("instance ID cannot be empty")
	}

	settings, err := s.instanceSettingsRepo.GetByQuestID(ctx, questID)
	if err != nil {
		return nil, errors.New("failed to retrieve instance settings: " + err.Error())
	}
	if settings == nil {
		return nil, errors.New("instance settings not found")
	}
	return settings, nil
}

// SaveSettings validates and saves the instance settings to the database.
func (s *QuestSettingsService) SaveSettings(ctx context.Context, settings *models.QuestSettings) error {
	if settings == nil {
		return errors.New("settings cannot be nil")
	}

	// Save to database
	if err := s.instanceSettingsRepo.Update(ctx, settings); err != nil {
		return fmt.Errorf("saving settings: %w", err)
	}

	return nil
}
