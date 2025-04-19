package services

import (
	"context"
	"database/sql"
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

	// DeleteInstance deletes an instance for the given user
	DeleteInstance(ctx context.Context, user *models.User, instanceID, confirmName string) (bool, error)
}

func NewInstanceService(
	transactor db.Transactor,
	locationService LocationService,
	teamService TeamService,
	instanceRepo repositories.InstanceRepository,
	instanceSettingsRepo repositories.InstanceSettingsRepository,
) InstanceService {
	return &instanceService{
		transactor:           transactor,
		locationService:      locationService,
		teamService:          teamService,
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
	}
}

// CreateInstance implements InstanceService.
func (s *instanceService) CreateInstance(ctx context.Context, name string, user *models.User) (*models.Instance, error) {
	if name == "" {
		return nil, NewValidationError("name")
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
		return nil, NewValidationError("name")
	}
	if id == "" {
		return nil, NewValidationError("id")
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

// DeleteInstance implements InstanceService.
func (s *instanceService) DeleteInstance(ctx context.Context, user *models.User, instanceID string, confirmName string) (bool, error) {
	if user == nil {
		return false, ErrUserNotAuthenticated
	}

	// Check if the user has permission to delete the instance
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return false, fmt.Errorf("finding instance: %w", err)
	}

	if user.ID != instance.UserID {
		return false, ErrPermissionDenied
	}

	// If the name does not match the confirmation, return an error
	if confirmName != instance.Name {
		return false, errors.New("instance name does not match confirmation")
	}

	// Check if the user is currently using this instance
	if user.CurrentInstanceID == instance.ID {
		return false, errors.New("cannot delete an instance that is currently in use")
	}

	for i, location := range instance.Locations {
		err := s.locationService.LoadRelations(ctx, &location)
		if err != nil {
			return false, fmt.Errorf("loading relations for location: %w", err)
		}
		instance.Locations[i] = location
	}

	// Start transaction
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			return false, fmt.Errorf("rolling back transaction: %w", err2)
		}
		return false, fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			err := tx.Rollback()
			if err != nil {
				panic(fmt.Errorf("rolling back transaction: %w", err))
			}
			panic(p)
		}
	}()

	err = s.instanceRepo.Delete(ctx, tx, instanceID)
	if err != nil {
		err2 := tx.Rollback()
		if err2 != nil {
			return false, fmt.Errorf("rolling back transaction: %w", err2)
		}
		return false, fmt.Errorf("deleting instance: %w", err)
	}

	err = s.instanceSettingsRepo.Delete(ctx, tx, instanceID)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return false, fmt.Errorf("rolling back transaction: %w", rollbackErr)
		}
		return false, fmt.Errorf("deleting instance settings: %w", err)
	}

	err = s.teamService.DeleteByInstanceID(ctx, tx, instanceID)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return false, fmt.Errorf("rolling back transaction: %w", rollbackErr)
		}
		return false, fmt.Errorf("deleting teams: %w", err)
	}

	err = s.locationService.DeleteLocations(ctx, tx, instance.Locations)
	if err != nil {
		rollbackErr := tx.Rollback()
		if rollbackErr != nil {
			return false, fmt.Errorf("rolling back transaction: %w", rollbackErr)
		}
		return false, fmt.Errorf("deleting locations: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return false, fmt.Errorf("committing transaction: %w", err)
	}

	return true, nil
}
