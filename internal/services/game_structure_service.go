package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v6/models"
)

// LocationRelationLoader defines the interface for loading location relations.
type LocationRelationLoader interface {
	LoadRelations(ctx context.Context, location *models.Location) error
}

// LocationRepository defines the interface for location data access.
type LocationRepository interface {
	FindByIDs(ctx context.Context, instanceID string, locationIDs []string) ([]*models.Location, error)
	FindByInstance(ctx context.Context, instanceID string) ([]models.Location, error)
	LoadBlocks(ctx context.Context, location *models.Location) error
}

// InstanceRepository defines the interface for instance data access.
type InstanceRepository interface {
	GetByID(ctx context.Context, id string) (*models.Instance, error)
	Update(ctx context.Context, instance *models.Instance) error
}

// GameStructureService provides operations for loading, saving, and validating GameStructures.
type GameStructureService struct {
	locationRepo   LocationRepository
	instanceRepo   InstanceRepository
	relationLoader LocationRelationLoader
}

// NewGameStructureService creates a new GameStructureService.
func NewGameStructureService(locationRepo LocationRepository, instanceRepo InstanceRepository) *GameStructureService {
	return &GameStructureService{
		locationRepo:   locationRepo,
		instanceRepo:   instanceRepo,
		relationLoader: nil, // Will be set via SetRelationLoader
	}
}

// SetRelationLoader sets the location relation loader (for loading blocks, etc.)
func (s *GameStructureService) SetRelationLoader(loader LocationRelationLoader) {
	s.relationLoader = loader
}

// Load populates the GameStructure with location data from the database
// If recursive is true, loads all subgroups recursively
// If recursive is false, only loads locations for this specific group.
func (s *GameStructureService) Load(
	ctx context.Context,
	instanceID string,
	group *models.GameStructure,
	recursive bool,
) error {
	if group == nil {
		return errors.New("group cannot be nil")
	}

	// Load locations for this group if it has any
	if len(group.LocationIDs) > 0 {
		locations, err := s.locationRepo.FindByIDs(ctx, instanceID, group.LocationIDs)
		if err != nil {
			return fmt.Errorf("failed to load locations for group %s: %w", group.ID, err)
		}

		// Create a map for quick lookup
		locationMap := make(map[string]*models.Location, len(locations))
		for _, loc := range locations {
			locationMap[loc.ID] = loc
		}

		// Maintain the order from LocationIDs
		group.Locations = make([]*models.Location, 0, len(group.LocationIDs))
		for _, id := range group.LocationIDs {
			if loc, ok := locationMap[id]; ok {
				group.Locations = append(group.Locations, loc)
			}
		}
	} else {
		group.Locations = []*models.Location{}
	}

	group.SetPopulated(true)

	// Recursively load subgroups if requested
	if recursive {
		for i := range group.SubGroups {
			if err := s.Load(ctx, instanceID, &group.SubGroups[i], true); err != nil {
				return err
			}
		}
	}

	return nil
}

// LoadWithRelations loads locations and their relations (blocks, etc.) for the game structure
// If recursive is true, loads all subgroups recursively.
func (s *GameStructureService) LoadWithRelations(
	ctx context.Context,
	instanceID string,
	group *models.GameStructure,
	recursive bool,
) error {
	// First load the basic location data
	if err := s.Load(ctx, instanceID, group, recursive); err != nil {
		return err
	}

	// Then load relations if a relation loader is configured
	if s.relationLoader != nil {
		return s.loadRelationsRecursive(ctx, group, recursive)
	}

	return nil
}

// loadRelationsRecursive loads relations for all locations in the structure.
func (s *GameStructureService) loadRelationsRecursive(
	ctx context.Context,
	group *models.GameStructure,
	recursive bool,
) error {
	// Load relations for this group's locations
	for i := range group.Locations {
		if err := s.relationLoader.LoadRelations(ctx, group.Locations[i]); err != nil {
			return fmt.Errorf("failed to load relations for location %s: %w", group.Locations[i].ID, err)
		}
	}

	// Recursively load relations for subgroups if requested
	if recursive {
		for i := range group.SubGroups {
			if err := s.loadRelationsRecursive(ctx, &group.SubGroups[i], true); err != nil {
				return err
			}
		}
	}

	return nil
}

// LoadBlocksForStructure loads blocks for all locations in the structure.
// This should only be called when blocks data is needed (e.g., location groups admin view).
func (s *GameStructureService) LoadBlocksForStructure(
	ctx context.Context,
	group *models.GameStructure,
	recursive bool,
) error {
	// Load blocks for this group's locations
	for i := range group.Locations {
		err := s.locationRepo.LoadBlocks(ctx, group.Locations[i])
		if err != nil {
			return fmt.Errorf("failed to load blocks for location %s: %w", group.Locations[i].ID, err)
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

// LoadByLocationID finds the group containing the specified location and loads it
// Returns the specific group containing that location (not the root).
func (s *GameStructureService) LoadByLocationID(
	ctx context.Context,
	instanceID string,
	locationID string,
) (*models.GameStructure, error) {
	// Get the instance (game structure is automatically loaded)
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to load instance: %w", err)
	}

	// Find the group containing this location ID
	group := s.FindGroupByLocationID(&instance.GameStructure, locationID)
	if group == nil {
		return nil, fmt.Errorf("location %s not found in any group", locationID)
	}

	// Load the group's locations (non-recursive)
	if err = s.Load(ctx, instanceID, group, false); err != nil {
		return nil, err
	}

	return group, nil
}

// FindGroupByLocationID recursively searches for a group containing the location ID.
func (s *GameStructureService) FindGroupByLocationID(
	group *models.GameStructure,
	locationID string,
) *models.GameStructure {
	// Check if this group contains the location
	for _, id := range group.LocationIDs {
		if id == locationID {
			return group
		}
	}

	// Recursively check subgroups
	for i := range group.SubGroups {
		if found := s.FindGroupByLocationID(&group.SubGroups[i], locationID); found != nil {
			return found
		}
	}

	return nil
}

// Save persists the GameStructure to the database.
func (s *GameStructureService) Save(ctx context.Context, instanceID string, group *models.GameStructure) error {
	// Ensure all locations are included in the structure
	if err := s.ensureAllLocationsIncluded(ctx, instanceID, group); err != nil {
		return fmt.Errorf("ensuring all locations included: %w", err)
	}

	// Validate before saving
	if err := s.Validate(group, instanceID); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Get the instance, update its game structure, and save
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("failed to load instance: %w", err)
	}

	instance.GameStructure = *group

	if err = s.instanceRepo.Update(ctx, instance); err != nil {
		return fmt.Errorf("failed to save game structure: %w", err)
	}

	return nil
}

// ensureAllLocationsIncluded ensures that all locations in the database
// are included in the structure. Any orphaned locations are added to the root group.
func (s *GameStructureService) ensureAllLocationsIncluded(
	ctx context.Context,
	instanceID string,
	group *models.GameStructure,
) error {
	// Collect all location IDs currently in the structure
	includedIDs := make(map[string]bool)
	s.collectAllLocationIDs(group, includedIDs)

	// Get all location IDs from the database for this instance
	locations, err := s.locationRepo.FindByInstance(ctx, instanceID)
	if err != nil {
		return fmt.Errorf("failed to fetch locations: %w", err)
	}

	// Find orphaned locations (in database but not in structure)
	var orphanedIDs []string
	for _, loc := range locations {
		if !includedIDs[loc.ID] {
			orphanedIDs = append(orphanedIDs, loc.ID)
		}
	}

	// Add orphaned locations to root group
	if len(orphanedIDs) > 0 {
		group.LocationIDs = append(group.LocationIDs, orphanedIDs...)
	}

	return nil
}

// collectAllLocationIDs recursively collects all location IDs from the structure.
func (s *GameStructureService) collectAllLocationIDs(group *models.GameStructure, ids map[string]bool) {
	for _, id := range group.LocationIDs {
		ids[id] = true
	}

	for i := range group.SubGroups {
		s.collectAllLocationIDs(&group.SubGroups[i], ids)
	}
}

// Validate checks the GameStructure for errors.
func (s *GameStructureService) Validate(group *models.GameStructure, instanceID string) error {
	if group == nil {
		return errors.New("group cannot be nil")
	}

	// Check for duplicate location IDs across entire tree
	allIDs := make(map[string]bool)
	if err := s.checkDuplicateLocationIDs(group, allIDs); err != nil {
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

// checkDuplicateLocationIDs recursively checks for duplicate location IDs.
func (s *GameStructureService) checkDuplicateLocationIDs(group *models.GameStructure, seen map[string]bool) error {
	for _, id := range group.LocationIDs {
		if seen[id] {
			return fmt.Errorf("duplicate location ID found: %s", id)
		}
		seen[id] = true
	}

	for i := range group.SubGroups {
		if err := s.checkDuplicateLocationIDs(&group.SubGroups[i], seen); err != nil {
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

// GetAllLocationIDs returns all location IDs in the group and its subgroups (flattened, in order).
func (s *GameStructureService) GetAllLocationIDs(group *models.GameStructure) []string {
	ids := make([]string, 0)

	// Add this group's locations first
	ids = append(ids, group.LocationIDs...)

	// Then add subgroups' locations recursively
	for i := range group.SubGroups {
		ids = append(ids, s.GetAllLocationIDs(&group.SubGroups[i])...)
	}

	return ids
}

// GetNextItemType returns what type of item should be next (placeholder implementation).
func (s *GameStructureService) GetNextItemType(
	group *models.GameStructure,
	completedLocationIDs map[string]bool,
	completedGroupIDs map[string]bool,
) interface{} {
	// TODO: Implement based on routing strategy
	return nil
}

// GetNextLocation returns the next location based on routing strategy (placeholder implementation).
func (s *GameStructureService) GetNextLocation(
	group *models.GameStructure,
	completedLocationIDs map[string]bool,
	teamID string,
) string {
	// TODO: Implement based on routing strategy
	return ""
}

// GetNextGroup returns the next group based on routing strategy (placeholder implementation).
func (s *GameStructureService) GetNextGroup(
	group *models.GameStructure,
	completedGroups map[string]bool,
) *models.GameStructure {
	// TODO: Implement based on routing strategy
	return nil
}

// IsCompleted checks if a group is completed based on completion type and count.
func (s *GameStructureService) IsCompleted(group *models.GameStructure, completedCount int) bool {
	switch group.CompletionType {
	case models.CompletionAll:
		totalItems := len(group.LocationIDs) + len(group.SubGroups)
		return completedCount >= totalItems
	case models.CompletionMinimum:
		return completedCount >= group.MinimumRequired
	default:
		return false
	}
}
