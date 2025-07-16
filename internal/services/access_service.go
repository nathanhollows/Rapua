package services

import (
	"context"
	"errors"

	"github.com/nathanhollows/Rapua/v3/repositories"
)

type accessService struct {
	blockRepo    repositories.BlockRepository
	instanceRepo repositories.InstanceRepository
	locationRepo repositories.LocationRepository
	markerRepo   repositories.MarkerRepository
}

// NewAccessService creates a new instance of accessService.
func NewAccessService(
	blockRepository repositories.BlockRepository,
	instanceRepository repositories.InstanceRepository,
	locationRepository repositories.LocationRepository,
	markerRepository repositories.MarkerRepository,
) *accessService {
	return &accessService{
		blockRepo:    blockRepository,
		instanceRepo: instanceRepository,
		locationRepo: locationRepository,
		markerRepo:   markerRepository,
	}
}

// CanAdminAccessInstance checks if the user can access the instance.
func (s *accessService) CanAdminAccessInstance(ctx context.Context, userID, instanceID string) (bool, error) {
	if userID == "" {
		return false, ErrUserNotAuthenticated
	}
	if instanceID == "" {
		return false, errors.New("instance ID cannot be empty")
	}

	instanceIDs, err := s.instanceRepo.FindByUserID(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, instance := range instanceIDs {
		if instance.ID == instanceID {
			return true, nil
		}
	}

	return false, nil
}

// CanAdminAccessLocation checks if the user can access the location in the given instance.
func (s *accessService) CanAdminAccessLocation(ctx context.Context, userID, locationID string) (bool, error) {
	if userID == "" {
		return false, errors.New("user ID cannot be empty")
	}
	if locationID == "" {
		return false, errors.New("location ID cannot be empty")
	}

	instanceIDs, err := s.instanceRepo.FindByUserID(ctx, userID)
	if err != nil {
		return false, err
	}

	location, err := s.locationRepo.GetByID(ctx, locationID)
	if err != nil {
		return false, err
	}
	for _, instance := range instanceIDs {
		if instance.ID == location.InstanceID {
			return true, nil
		}
	}
	return false, nil
}

// CanAdminAccessMarker checks if the user can access the marker in the given instance.
func (s *accessService) CanAdminAccessMarker(ctx context.Context, userID, markerID string) (bool, error) {
	if userID == "" {
		return false, errors.New("user ID cannot be empty")
	}
	if markerID == "" {
		return false, errors.New("marker ID cannot be empty")
	}
	access, err := s.markerRepo.UserOwnsMarker(ctx, userID, markerID)
	if err != nil {
		return false, err
	}
	return access, nil
}

// CanAdminAccessBlock checks if the user can access the block in the given instance.
func (s *accessService) CanAdminAccessBlock(ctx context.Context, userID, blockID string) (bool, error) {
	if userID == "" {
		return false, errors.New("user ID cannot be empty")
	}
	if blockID == "" {
		return false, errors.New("block ID cannot be empty")
	}

	block, err := s.blockRepo.GetByID(ctx, blockID)
	if err != nil {
		return false, err
	}

	return s.CanAdminAccessLocation(ctx, userID, block.GetLocationID())
}
