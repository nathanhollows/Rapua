package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
)

type InstanceService struct {
	instanceRepo         repositories.InstanceRepository
	instanceSettingsRepo repositories.InstanceSettingsRepository
}

func NewInstanceService(
	instanceRepo repositories.InstanceRepository,
	instanceSettingsRepo repositories.InstanceSettingsRepository,
) *InstanceService {
	return &InstanceService{
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
	}
}

// CreateInstance implements InstanceService.
func (s *InstanceService) CreateInstance(
	ctx context.Context,
	name string,
	user *models.User,
) (*models.Instance, error) {
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	if user == nil {
		return nil, ErrUserNotAuthenticated
	}

	instance := &models.Instance{
		Name:       name,
		UserID:     user.ID,
		IsTemplate: false,
	}

	if err := s.instanceRepo.Create(ctx, instance); err != nil {
		return nil, fmt.Errorf("creating instance: %w", err)
	}

	settings := &models.InstanceSettings{
		InstanceID: instance.ID,
	}
	if err := s.instanceSettingsRepo.Create(ctx, settings); err != nil {
		return nil, fmt.Errorf("creating instance settings: %w", err)
	}

	return instance, nil
}

// FindByUserID implements InstanceService.
func (s *InstanceService) FindByUserID(ctx context.Context, userID string) ([]models.Instance, error) {
	instances, err := s.instanceRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding instances for user: %w", err)
	}
	return instances, nil
}

// FindInstanceIDsForUser implements InstanceService.
func (s *InstanceService) FindInstanceIDsForUser(ctx context.Context, userID string) ([]string, error) {
	instances, err := s.instanceRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding instances for user: %w", err)
	}

	ids := make([]string, len(instances))
	for i, instance := range instances {
		ids[i] = instance.ID
	}
	return ids, nil
}
