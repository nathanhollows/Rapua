package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
)

type InstanceSettingsService struct {
	instanceSettingsRepo repositories.InstanceSettingsRepository
}

func NewInstanceSettingsService(
	instanceSettingsRepo repositories.InstanceSettingsRepository,
) *InstanceSettingsService {
	return &InstanceSettingsService{
		instanceSettingsRepo: instanceSettingsRepo,
	}
}

// GetInstanceSettings retrieves the settings for the given instance ID.
func (s *InstanceSettingsService) GetInstanceSettings(
	ctx context.Context,
	instanceID string,
) (*models.InstanceSettings, error) {
	if instanceID == "" {
		return nil, errors.New("instance ID cannot be empty")
	}

	settings, err := s.instanceSettingsRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, errors.New("failed to retrieve instance settings: " + err.Error())
	}
	if settings == nil {
		return nil, errors.New("instance settings not found")
	}
	return settings, nil
}

// SaveSettings validates and saves the instance settings to the database.
func (s *InstanceSettingsService) SaveSettings(ctx context.Context, settings *models.InstanceSettings) error {
	if settings == nil {
		return errors.New("settings cannot be nil")
	}

	// Validate business rules
	if settings.MaxNextLocations < 0 {
		return errors.New("max next locations cannot be negative")
	}

	// Save to database
	if err := s.instanceSettingsRepo.Update(ctx, settings); err != nil {
		return fmt.Errorf("saving settings: %w", err)
	}

	return nil
}
