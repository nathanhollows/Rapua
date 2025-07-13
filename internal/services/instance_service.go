package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v3/db"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
)

type instanceService struct {
	transactor           db.Transactor
	locationService      LocationService
	teamService          TeamService
	instanceRepo         repositories.InstanceRepository
	instanceSettingsRepo repositories.InstanceSettingsRepository
}

type InstanceService interface {
	// CreateInstance creates a new instance for the given user
	CreateInstance(ctx context.Context, name string, user *models.User) (*models.Instance, error)
	// DuplicateInstance duplicates an instance for the given user
	DuplicateInstance(ctx context.Context, user *models.User, id, name string) (*models.Instance, error)

	// FindByUserID returns all instances for the given user
	FindByUserID(ctx context.Context, userID string) ([]models.Instance, error)
	// FindInstanceIDsForUser returns the IDs of all instances for the given user
	FindInstanceIDsForUser(ctx context.Context, userID string) ([]string, error)
}

func NewInstanceService(
	locationService LocationService,
	teamService TeamService,
	instanceRepo repositories.InstanceRepository,
	instanceSettingsRepo repositories.InstanceSettingsRepository,
) InstanceService {
	return &instanceService{
		locationService:      locationService,
		teamService:          teamService,
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
	}
}

// CreateInstance implements InstanceService.
func (s *instanceService) CreateInstance(ctx context.Context, name string, user *models.User) (*models.Instance, error) {
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

// DuplicateInstance implements InstanceService.
func (s *instanceService) DuplicateInstance(ctx context.Context, user *models.User, id string, name string) (*models.Instance, error) {
	if user == nil {
		return nil, ErrUserNotAuthenticated
	}

	oldInstance, err := s.instanceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("finding instance: %w", err)
	}

	if oldInstance.IsTemplate {
		return nil, errors.New("cannot duplicate a template")
	}

	locations, err := s.locationService.FindByInstance(ctx, oldInstance.ID)
	if err != nil {
		return nil, fmt.Errorf("finding locations: %w", err)
	}

	if name == "" {
		return nil, errors.New("name cannot be empty")
	}
	if id == "" {
		return nil, errors.New("id cannot be empty")
	}

	newInstance := &models.Instance{
		Name:   name,
		UserID: user.ID,
	}

	if err := s.instanceRepo.Create(ctx, newInstance); err != nil {
		return nil, fmt.Errorf("creating instance: %w", err)
	}

	// Copy locations
	for _, location := range locations {
		_, err := s.locationService.DuplicateLocation(ctx, location, newInstance.ID)
		if err != nil {
			return nil, fmt.Errorf("duplicating location: %w", err)
		}
	}

	// Copy settings
	settings := oldInstance.Settings
	settings.InstanceID = newInstance.ID
	if err := s.instanceSettingsRepo.Create(ctx, &settings); err != nil {
		return nil, fmt.Errorf("creating settings: %w", err)
	}

	return newInstance, nil
}

// FindByUserID implements InstanceService.
func (s *instanceService) FindByUserID(ctx context.Context, userID string) ([]models.Instance, error) {
	instances, err := s.instanceRepo.FindByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding instances for user: %w", err)
	}
	return instances, nil
}

// FindInstanceIDsForUser implements InstanceService.
func (s *instanceService) FindInstanceIDsForUser(ctx context.Context, userID string) ([]string, error) {
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
