package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v8/models"
)

// ObjectiveRepository defines the interface for objective data access.
type ObjectiveRepository interface {
	FindByIDs(ctx context.Context, questID string, objectiveIDs []string) ([]*models.Objective, error)
	FindByQuestID(ctx context.Context, questID string) ([]models.Objective, error)
	LoadBlocks(ctx context.Context, objective *models.Objective) error
}

// QuestRepository defines the interface for quest data access.
type QuestRepository interface {
	GetByID(ctx context.Context, id string) (*models.Quest, error)
	Update(ctx context.Context, instance *models.Quest) error
}

// GameStructureService provides operations for loading, saving, and validating GameStructures.
type GameStructureService struct {
	objectiveRepo ObjectiveRepository
	instanceRepo  QuestRepository
}

// NewGameStructureService creates a new GameStructureService.
func NewGameStructureService(
	objectiveRepo ObjectiveRepository,
	instanceRepo QuestRepository,
) *GameStructureService {
	return &GameStructureService{
		objectiveRepo: objectiveRepo,
		instanceRepo:  instanceRepo,
	}
}

// Load populates the GameStructure with objective data, recursively loading
// subgroups when recursive is set.
func (s *GameStructureService) Load(
	ctx context.Context,
	questID string,
	group *models.GameStructure,
	recursive bool,
) error {
	if group == nil {
		return errors.New("group cannot be nil")
	}

	if len(group.ObjectiveIDs) > 0 {
		objectives, err := s.objectiveRepo.FindByIDs(ctx, questID, group.ObjectiveIDs)
		if err != nil {
			return fmt.Errorf("failed to load objectives for group %s: %w", group.ID, err)
		}

		objectiveMap := make(map[string]*models.Objective, len(objectives))
		for _, obj := range objectives {
			objectiveMap[obj.ID] = obj
		}

		// Maintain the order from ObjectiveIDs.
		group.Objectives = make([]*models.Objective, 0, len(group.ObjectiveIDs))
		for _, id := range group.ObjectiveIDs {
			if obj, ok := objectiveMap[id]; ok {
				group.Objectives = append(group.Objectives, obj)
			}
		}
	} else {
		group.Objectives = []*models.Objective{}
	}

	group.SetPopulated(true)

	// Recursively load subgroups if requested
	if recursive {
		for i := range group.SubGroups {
			if err := s.Load(ctx, questID, &group.SubGroups[i], true); err != nil {
				return err
			}
		}
	}

	return nil
}

// LoadBlocksForStructure is expensive: call it only when block data is needed (the quest builder admin view).
func (s *GameStructureService) LoadBlocksForStructure(
	ctx context.Context,
	group *models.GameStructure,
	recursive bool,
) error {
	for i := range group.Objectives {
		err := s.objectiveRepo.LoadBlocks(ctx, group.Objectives[i])
		if err != nil {
			return fmt.Errorf("failed to load blocks for objective %s: %w", group.Objectives[i].ID, err)
		}
	}

	// Recursively load blocks for subgroups if requested
	if recursive {
		for i := range group.SubGroups {
			if err := s.LoadBlocksForStructure(ctx, &group.SubGroups[i], true); err != nil {
				return err
			}
		}
	}

	return nil
}

// Save persists the GameStructure to the database.
func (s *GameStructureService) Save(ctx context.Context, questID string, group *models.GameStructure) error {
	if err := s.ensureAllObjectivesIncluded(ctx, questID, group); err != nil {
		return fmt.Errorf("ensuring all objectives included: %w", err)
	}

	// Validate before saving
	if err := s.Validate(group, questID); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Get the instance, update its game structure, and save
	instance, err := s.instanceRepo.GetByID(ctx, questID)
	if err != nil {
		return fmt.Errorf("failed to load instance: %w", err)
	}

	instance.GameStructure = *group

	if err = s.instanceRepo.Update(ctx, instance); err != nil {
		return fmt.Errorf("failed to save game structure: %w", err)
	}

	return nil
}

// ensureAllObjectivesIncluded adds orphaned objectives to the root group.
func (s *GameStructureService) ensureAllObjectivesIncluded(
	ctx context.Context,
	questID string,
	group *models.GameStructure,
) error {
	includedIDs := make(map[string]bool)
	s.collectAllObjectiveIDs(group, includedIDs)

	objectives, err := s.objectiveRepo.FindByQuestID(ctx, questID)
	if err != nil {
		return fmt.Errorf("failed to fetch objectives: %w", err)
	}

	var orphanedIDs []string
	for _, obj := range objectives {
		if !includedIDs[obj.ID] {
			orphanedIDs = append(orphanedIDs, obj.ID)
		}
	}

	if len(orphanedIDs) > 0 {
		group.ObjectiveIDs = append(group.ObjectiveIDs, orphanedIDs...)
	}

	return nil
}

func (s *GameStructureService) collectAllObjectiveIDs(group *models.GameStructure, ids map[string]bool) {
	for _, id := range group.ObjectiveIDs {
		ids[id] = true
	}

	for i := range group.SubGroups {
		s.collectAllObjectiveIDs(&group.SubGroups[i], ids)
	}
}

// Validate checks the GameStructure for errors.
func (s *GameStructureService) Validate(group *models.GameStructure, _ string) error {
	if group == nil {
		return errors.New("group cannot be nil")
	}

	allObjectiveIDs := make(map[string]bool)
	if err := s.checkDuplicateObjectiveIDs(group, allObjectiveIDs); err != nil {
		return err
	}

	// Check for duplicate group IDs
	allGroupIDs := make(map[string]bool)
	if err := s.checkDuplicateGroupIDs(group, allGroupIDs); err != nil {
		return err
	}

	// Validate visible groups have names and colors
	if err := s.validateGroupMetadata(group); err != nil {
		return err
	}

	return nil
}

func (s *GameStructureService) checkDuplicateObjectiveIDs(group *models.GameStructure, seen map[string]bool) error {
	for _, id := range group.ObjectiveIDs {
		if seen[id] {
			return fmt.Errorf("duplicate objective ID found: %s", id)
		}
		seen[id] = true
	}

	for i := range group.SubGroups {
		if err := s.checkDuplicateObjectiveIDs(&group.SubGroups[i], seen); err != nil {
			return err
		}
	}

	return nil
}

// checkDuplicateGroupIDs recursively checks for duplicate group IDs.
func (s *GameStructureService) checkDuplicateGroupIDs(group *models.GameStructure, seen map[string]bool) error {
	if group.ID != "" {
		if seen[group.ID] {
			return fmt.Errorf("duplicate group ID found: %s", group.ID)
		}
		seen[group.ID] = true
	}

	for i := range group.SubGroups {
		if err := s.checkDuplicateGroupIDs(&group.SubGroups[i], seen); err != nil {
			return err
		}
	}

	return nil
}

// validateGroupMetadata checks that visible groups have required metadata.
func (s *GameStructureService) validateGroupMetadata(group *models.GameStructure) error {
	// Root group can have empty name and color
	if !group.IsRoot {
		if group.Name == "" {
			return fmt.Errorf("visible group %s must have a name", group.ID)
		}
		if group.Color == "" {
			return fmt.Errorf("visible group %s must have a color", group.ID)
		}
	}

	// Recursively validate subgroups
	for i := range group.SubGroups {
		if err := s.validateGroupMetadata(&group.SubGroups[i]); err != nil {
			return err
		}
	}

	return nil
}

// FindGroupByID recursively searches for a group with the specified ID.
func (s *GameStructureService) FindGroupByID(group *models.GameStructure, groupID string) *models.GameStructure {
	if group.ID == groupID {
		return group
	}

	for i := range group.SubGroups {
		if found := s.FindGroupByID(&group.SubGroups[i], groupID); found != nil {
			return found
		}
	}

	return nil
}

// InsertObjectiveIntoGroup falls back to root when groupID is empty or not
// found. beforeObjectiveID and afterObjectiveID choose the insertion point;
// beforeObjectiveID takes precedence. If both are empty, appends to the end.
func (s *GameStructureService) InsertObjectiveIntoGroup(
	ctx context.Context,
	questID, objectiveID, groupID, afterObjectiveID, beforeObjectiveID string,
) error {
	instance, err := s.instanceRepo.GetByID(ctx, questID)
	if err != nil {
		return fmt.Errorf("loading instance: %w", err)
	}

	target := s.FindGroupByID(&instance.GameStructure, groupID)
	if target == nil {
		target = &instance.GameStructure // fallback to root.
	}

	switch {
	case beforeObjectiveID != "":
		inserted := false
		for i, id := range target.ObjectiveIDs {
			if id == beforeObjectiveID {
				newIDs := make([]string, 0, len(target.ObjectiveIDs)+1)
				newIDs = append(newIDs, target.ObjectiveIDs[:i]...)
				newIDs = append(newIDs, objectiveID)
				newIDs = append(newIDs, target.ObjectiveIDs[i:]...)
				target.ObjectiveIDs = newIDs
				inserted = true
				break
			}
		}
		if !inserted {
			target.ObjectiveIDs = append([]string{objectiveID}, target.ObjectiveIDs...)
		}
	case afterObjectiveID != "":
		inserted := false
		for i, id := range target.ObjectiveIDs {
			if id == afterObjectiveID {
				target.ObjectiveIDs = append(
					target.ObjectiveIDs[:i+1],
					append([]string{objectiveID}, target.ObjectiveIDs[i+1:]...)...,
				)
				inserted = true
				break
			}
		}
		if !inserted {
			target.ObjectiveIDs = append(target.ObjectiveIDs, objectiveID)
		}
	default:
		target.ObjectiveIDs = append(target.ObjectiveIDs, objectiveID)
	}

	return s.Save(ctx, questID, &instance.GameStructure)
}
