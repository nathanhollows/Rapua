package services

import (
	"context"
	"errors"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
)

type AccessService struct {
	blockRepo     repositories.BlockRepository
	instanceRepo  repositories.QuestRepository
	locationRepo  repositories.LocationRepository
	markerRepo    repositories.MarkerRepository
	objectiveRepo repositories.ObjectiveRepository
}

// NewAccessService creates an accessService.
func NewAccessService(
	blockRepository repositories.BlockRepository,
	questRepository repositories.QuestRepository,
	locationRepository repositories.LocationRepository,
	markerRepository repositories.MarkerRepository,
	objectiveRepository repositories.ObjectiveRepository,
) *AccessService {
	return &AccessService{
		blockRepo:     blockRepository,
		instanceRepo:  questRepository,
		locationRepo:  locationRepository,
		markerRepo:    markerRepository,
		objectiveRepo: objectiveRepository,
	}
}

// CanAdminAccessQuest checks if the user can access the quest.
func (s *AccessService) CanAdminAccessQuest(ctx context.Context, userID, questID string) (bool, error) {
	if userID == "" {
		return false, ErrUserNotAuthenticated
	}
	if questID == "" {
		return false, errors.New("instance ID cannot be empty")
	}

	instanceIDs, err := s.instanceRepo.FindByUserID(ctx, userID)
	if err != nil {
		return false, err
	}

	for _, instance := range instanceIDs {
		if instance.ID == questID {
			return true, nil
		}
	}

	return false, nil
}

// CanAdminAccessLocation checks if the user can access the location in the given instance.
func (s *AccessService) CanAdminAccessLocation(ctx context.Context, userID, locationID string) (bool, error) {
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
		if instance.ID == location.QuestID {
			return true, nil
		}
	}
	return false, nil
}

func (s *AccessService) CanAdminAccessObjective(ctx context.Context, userID, objectiveID string) (bool, error) {
	if userID == "" {
		return false, errors.New("user ID cannot be empty")
	}
	if objectiveID == "" {
		return false, errors.New("objective ID cannot be empty")
	}

	instanceIDs, err := s.instanceRepo.FindByUserID(ctx, userID)
	if err != nil {
		return false, err
	}

	objective, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return false, err
	}
	for _, instance := range instanceIDs {
		if instance.ID == objective.QuestID {
			return true, nil
		}
	}
	return false, nil
}

// CanAdminAccessBlock checks if the user can access the block in the given instance.
func (s *AccessService) CanAdminAccessBlock(ctx context.Context, userID, blockID string) (bool, error) {
	if userID == "" {
		return false, errors.New("user ID cannot be empty")
	}
	if blockID == "" {
		return false, errors.New("block ID cannot be empty")
	}

	return s.blockRepo.UserOwnsBlock(ctx, userID, blockID)
}

// CanAdminAccessBlockOwner checks if the user can access an owner (instance or location) based on context.
func (s *AccessService) CanAdminAccessBlockOwner(
	ctx context.Context,
	userID, ownerID string,
	blockContext blocks.BlockContext,
) (bool, error) {
	if userID == "" {
		return false, errors.New("user ID cannot be empty")
	}
	if ownerID == "" {
		return false, errors.New("owner ID cannot be empty")
	}

	switch blockContext {
	case blocks.ContextStart, blocks.ContextFinish:
		// For start/complete blocks, owner is questID/complete.
		return s.CanAdminAccessQuest(ctx, userID, ownerID)
	case blocks.ContextObjectiveProof, blocks.ContextObjectiveReveal:
		return s.CanAdminAccessObjective(ctx, userID, ownerID)
	default:
		// For location blocks, owner is locationID.
		return s.CanAdminAccessLocation(ctx, userID, ownerID)
	}
}
