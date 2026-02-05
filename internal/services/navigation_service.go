package services

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"strings"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/navigation"
	"github.com/nathanhollows/Rapua/v6/repositories"
)

var (
	ErrAllLocationsVisited = errors.New("all locations visited")
	ErrInstanceNotFound    = errors.New("instance not found")
)

type NavigationService struct {
	locationRepo         repositories.LocationRepository
	teamRepo             repositories.TeamRepository
	gameStructureService *GameStructureService
	blockService         *BlockService
}

// PlayerNavigationView contains all data needed to render the player navigation UI.
type PlayerNavigationView struct {
	// Settings
	Settings models.InstanceSettings // Global instance settings

	// Current state
	CurrentGroup     *models.GameStructure // Current group
	CanAdvanceEarly  bool                  // Whether team can manually advance to next group (minimum met, AutoAdvance=false, not 100% complete)
	MustCheckOut     bool                  // Whether team must check out before proceeding
	BlockingLocation *models.Location      // Location team must check out from (nil if not blocked)

	// Available locations
	NextLocations []models.Location // Locations team can visit next
	// Completed locations (for scavenger hunt mode or similar)
	CompletedLocations []models.Location // Locations already visited (optional, for certain display modes)

	// Navigation clues (for custom display mode)
	Blocks      []blocks.Block                // All navigation clue blocks for next locations
	BlockStates map[string]blocks.PlayerState // States for navigation clue blocks
}

// NewNavigationService creates a new instance of NavigationService.
func NewNavigationService(
	locationRepo repositories.LocationRepository,
	teamRepo repositories.TeamRepository,
	gameStructureService *GameStructureService,
	blockService *BlockService,
) *NavigationService {
	return &NavigationService{
		locationRepo:         locationRepo,
		teamRepo:             teamRepo,
		gameStructureService: gameStructureService,
		blockService:         blockService,
	}
}

// IsValidLocation checks if the location code is valid for the team to check in to.
// This includes both regular available locations AND accessible secret locations.
func (s *NavigationService) IsValidLocation(ctx context.Context, team *models.Team, markerID string) (bool, error) {
	if err := s.validateTeamState(team); err != nil {
		return false, err
	}

	// Find valid locations (without loading full relations)
	locations, err := s.determineNextLocations(ctx, team)
	if err != nil {
		return false, fmt.Errorf("determine next valid locations: %w", err)
	}

	// Check if the location code is valid in regular available locations
	markerID = s.normalizeMarkerID(markerID)
	for _, loc := range locations {
		if loc.MarkerID == markerID {
			return true, nil
		}
	}

	// Also check accessible secret locations
	secretLocations, err := s.getAccessibleSecretLocations(ctx, team)
	if err != nil {
		return false, fmt.Errorf("determine accessible secret locations: %w", err)
	}

	for _, loc := range secretLocations {
		if loc.MarkerID == markerID {
			return true, nil
		}
	}

	return false, fmt.Errorf("code %s is not a valid next location", markerID)
}

// GetNextLocations returns the next locations for the team to visit with full relations loaded.
func (s *NavigationService) GetNextLocations(ctx context.Context, team *models.Team) ([]models.Location, error) {
	// Load team relations if not already loaded
	if err := s.ensureTeamRelationsLoaded(ctx, team); err != nil {
		return nil, fmt.Errorf("loading team relations: %w", err)
	}

	// Get the core locations
	locations, err := s.determineNextLocations(ctx, team)
	if err != nil {
		return nil, err
	}

	// Load full relations for each location
	for i := range locations {
		if loadErr := s.locationRepo.LoadRelations(ctx, &locations[i]); loadErr != nil {
			return nil, fmt.Errorf("loading relations for location: %w", loadErr)
		}
	}

	return locations, nil
}

// GetPlayerNavigationView returns a complete view of navigation data for the player UI.
func (s *NavigationService) GetPlayerNavigationView(
	ctx context.Context,
	team *models.Team,
) (*PlayerNavigationView, error) {
	// Load team relations if not already loaded
	if err := s.ensureTeamRelationsLoaded(ctx, team); err != nil {
		return nil, fmt.Errorf("loading team relations: %w", err)
	}

	view := &PlayerNavigationView{
		Settings:    team.Instance.Settings,
		Blocks:      make([]blocks.Block, 0),
		BlockStates: make(map[string]blocks.PlayerState),
	}

	// Check if team is blocked (must check out)
	if team.MustCheckOut != "" {
		view.MustCheckOut = true
		// Load blocking location
		blockingLocation, err := s.locationRepo.GetByID(ctx, team.MustCheckOut)
		if err != nil {
			return nil, fmt.Errorf("loading blocking location: %w", err)
		}
		view.BlockingLocation = blockingLocation
		// Team is blocked, no next locations available
		view.NextLocations = []models.Location{}
		return view, nil
	}

	// Get current group (if using GameStructure)
	var currentGroup *models.GameStructure
	if team.Instance.GameStructure.ID != "" {
		// Compute current group from completed locations
		completedIDs := s.getCompletedLocationIDs(team.CheckIns)
		currentGroupID := navigation.ComputeCurrentGroup(
			&team.Instance.GameStructure,
			completedIDs,
			team.SkippedGroupIDs,
		)

		if currentGroupID != "" {
			currentGroup = navigation.FindGroupByID(&team.Instance.GameStructure, currentGroupID)
		}
		view.CurrentGroup = currentGroup

		// Check if team can advance early (minimum met, AutoAdvance=false, not 100% complete)
		if currentGroup != nil && !currentGroup.AutoAdvance && len(currentGroup.LocationIDs) > 0 {
			// Count completed locations in current group
			completedSet := make(map[string]bool)
			for _, id := range completedIDs {
				completedSet[id] = true
			}
			completedCount := 0
			for _, locID := range currentGroup.LocationIDs {
				if completedSet[locID] {
					completedCount++
				}
			}

			// Check if minimum met but not all complete
			isMinimumMet := false
			switch currentGroup.CompletionType {
			case models.CompletionAll:
				isMinimumMet = completedCount == len(currentGroup.LocationIDs)
			case models.CompletionMinimum:
				isMinimumMet = completedCount >= currentGroup.MinimumRequired
			}

			allComplete := completedCount == len(currentGroup.LocationIDs)
			view.CanAdvanceEarly = isMinimumMet && !allComplete
		}
	}

	// For task mode, load all locations in the group and partition by completion
	if currentGroup != nil && currentGroup.Navigation == models.NavigationDisplayTasks {
		uncompleted, completed, err := s.getScavengerHuntLocations(ctx, team, currentGroup)
		if err != nil {
			return nil, fmt.Errorf("loading scavenger hunt locations: %w", err)
		}
		view.NextLocations = uncompleted
		view.CompletedLocations = completed

		// Load task blocks for all locations (both completed and uncompleted)
		allLocations := append(uncompleted, completed...)
		for _, location := range allLocations {
			locationBlocks, blockStates, blockErr := s.blockService.FindByOwnerIDAndTeamCodeWithStateAndContext(
				ctx,
				location.ID,
				team.Code,
				blocks.ContextTask,
			)
			if blockErr != nil {
				return nil, fmt.Errorf("loading task blocks: %w", blockErr)
			}
			view.Blocks = append(view.Blocks, locationBlocks...)
			maps.Copy(view.BlockStates, blockStates)
		}

		return view, nil
	}

	// Get next locations (standard flow for other display modes)
	locations, err := s.determineNextLocations(ctx, team)
	if err != nil {
		return nil, err
	}

	// Load full relations for each location
	for i := range locations {
		if loadErr := s.locationRepo.LoadRelations(ctx, &locations[i]); loadErr != nil {
			return nil, fmt.Errorf("loading relations for location: %w", loadErr)
		}
	}
	view.NextLocations = locations

	// Load navigation blocks if using custom or tasks display mode
	var viewContext blocks.BlockContext
	if view.CurrentGroup != nil && view.CurrentGroup.Navigation == models.NavigationDisplayTasks {
		viewContext = blocks.ContextTask
	} else if view.CurrentGroup != nil && view.CurrentGroup.Navigation == models.NavigationDisplayCustom {
		viewContext = blocks.ContextLocationClues
	}
	if viewContext == blocks.ContextLocationClues || viewContext == blocks.ContextTask {
		for _, location := range locations {
			locationBlocks, blockStates, blockErr := s.blockService.FindByOwnerIDAndTeamCodeWithStateAndContext(
				ctx,
				location.ID,
				team.Code,
				viewContext,
			)
			if blockErr != nil {
				return nil, fmt.Errorf("loading navigation blocks: %w", blockErr)
			}
			view.Blocks = append(view.Blocks, locationBlocks...)
			maps.Copy(view.BlockStates, blockStates)
		}
	}

	return view, nil
}

// determineNextLocations is the core logic for finding next locations without relation loading.
func (s *NavigationService) determineNextLocations(ctx context.Context, team *models.Team) ([]models.Location, error) {
	if err := s.validateTeamState(team); err != nil {
		return nil, err
	}

	// All games use GameStructure (migration converts legacy games)
	return s.getValidLocationsFromGameStructure(ctx, team)
}

// validateTeamState checks if team has required relations loaded.
func (s *NavigationService) validateTeamState(team *models.Team) error {
	if team.Instance.ID == "" {
		return ErrInstanceNotFound
	}
	if team.Instance.Settings.InstanceID == "" {
		return ErrInstanceSettingsNotFound
	}
	// Note: Locations are no longer required here since all games use GameStructure
	// and locations are loaded on-demand by group
	return nil
}

// ensureTeamRelationsLoaded loads team relations if not already loaded.
func (s *NavigationService) ensureTeamRelationsLoaded(ctx context.Context, team *models.Team) error {
	if team.Instance.ID == "" || len(team.CheckIns) == 0 {
		return s.teamRepo.LoadRelations(ctx, team)
	}
	return nil
}

// normalizeMarkerID trims and uppercases marker ID.
func (s *NavigationService) normalizeMarkerID(markerID string) string {
	return strings.TrimSpace(strings.ToUpper(markerID))
}

// getValidLocationsFromGameStructure determines valid locations using the GameStructure system.
func (s *NavigationService) getValidLocationsFromGameStructure(
	ctx context.Context,
	team *models.Team,
) ([]models.Location, error) {
	// 1. Check if team is locked at a location (MustCheckOut)
	// Use existing Team.MustCheckOut field (single source of truth)
	if team.MustCheckOut != "" {
		return []models.Location{}, nil // No locations available until checkout
	}

	// 2. Get completed location IDs from CheckIns
	completedIDs := s.getCompletedLocationIDs(team.CheckIns)

	// 3. Compute current group from completed locations (pure function, deterministic)
	currentGroupID := navigation.ComputeCurrentGroup(&team.Instance.GameStructure, completedIDs, team.SkippedGroupIDs)

	if currentGroupID == "" {
		// No valid group (either no groups configured or all completed)
		return []models.Location{}, nil
	}

	// 4. Get available location IDs using navigation package
	locationIDs := navigation.GetAvailableLocationIDs(
		&team.Instance.GameStructure,
		currentGroupID,
		completedIDs,
		team.Code,
	)

	if len(locationIDs) == 0 {
		// Check if game is complete (no next group to advance to)
		_, shouldAdvance, _ := navigation.GetNextGroup(&team.Instance.GameStructure, currentGroupID, completedIDs)
		if !shouldAdvance {
			return []models.Location{}, ErrAllLocationsVisited
		}
		return []models.Location{}, nil
	}

	// 5. Fetch only the needed locations from database
	locations := make([]models.Location, 0, len(locationIDs))
	for _, id := range locationIDs {
		location, err := s.locationRepo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to load location %s: %w", id, err)
		}
		locations = append(locations, *location)
	}

	return locations, nil
}

// GetPreviewNavigationView creates a simplified navigation view showing only
// the specified location within its containing group for preview mode.
func (s *NavigationService) GetPreviewNavigationView(
	ctx context.Context,
	team *models.Team,
	locationID string,
) (*PlayerNavigationView, error) {
	// Load team relations if not already loaded
	if err := s.ensureTeamRelationsLoaded(ctx, team); err != nil {
		return nil, fmt.Errorf("loading team relations: %w", err)
	}

	// Find the group containing this location
	group := navigation.FindGroupContainingLocation(&team.Instance.GameStructure, locationID)
	if group == nil {
		return nil, errors.New("location not found in game structure")
	}

	// Load the location
	location, err := s.locationRepo.GetByID(ctx, locationID)
	if err != nil {
		return nil, fmt.Errorf("loading location: %w", err)
	}

	// Load location relations (including blocks)
	if err := s.locationRepo.LoadRelations(ctx, location); err != nil {
		return nil, fmt.Errorf("loading location relations: %w", err)
	}

	view := &PlayerNavigationView{
		Settings:        team.Instance.Settings,
		CurrentGroup:    group,
		NextLocations:   []models.Location{*location},
		MustCheckOut:    false,
		CanAdvanceEarly: false,
		Blocks:          make([]blocks.Block, 0),
		BlockStates:     make(map[string]blocks.PlayerState),
	}

	// Load navigation blocks if using custom or tasks display mode
	var viewContext blocks.BlockContext
	if view.CurrentGroup != nil && view.CurrentGroup.Navigation == models.NavigationDisplayTasks {
		viewContext = blocks.ContextTask
	} else if view.CurrentGroup != nil && view.CurrentGroup.Navigation == models.NavigationDisplayCustom {
		viewContext = blocks.ContextLocationClues
	}
	if viewContext == blocks.ContextLocationClues || viewContext == blocks.ContextTask {
		locationBlocks, blockStates, blockErr := s.blockService.FindByOwnerIDAndTeamCodeWithStateAndContext(
			ctx,
			location.ID,
			team.Code,
			viewContext,
		)
		if blockErr != nil {
			return nil, fmt.Errorf("loading navigation blocks: %w", blockErr)
		}
		view.Blocks = append(view.Blocks, locationBlocks...)
		maps.Copy(view.BlockStates, blockStates)
	}

	return view, nil
}

// getCompletedLocationIDs extracts location IDs from check-ins where blocks are completed.
// A location is only considered complete when BlocksCompleted is true.
func (s *NavigationService) getCompletedLocationIDs(checkIns []models.CheckIn) []string {
	completed := make([]string, 0, len(checkIns))
	for _, checkIn := range checkIns {
		if checkIn.BlocksCompleted {
			completed = append(completed, checkIn.LocationID)
		}
	}
	return completed
}

// getScavengerHuntLocations returns locations for task display mode.
// Uncompleted locations use the same routing logic as other modes (guided, random, free roam).
// Completed locations are all locations in the group where BlocksCompleted is true.
// Both lists preserve the order defined by group.LocationIDs.
func (s *NavigationService) getScavengerHuntLocations(
	ctx context.Context,
	team *models.Team,
	group *models.GameStructure,
) (uncompleted []models.Location, completed []models.Location, err error) {
	if len(group.LocationIDs) == 0 {
		return []models.Location{}, []models.Location{}, nil
	}

	// Get uncompleted locations using the same routing logic as other modes
	// This respects guided/random/free roam strategies
	uncompleted, err = s.determineNextLocations(ctx, team)
	if err != nil {
		// ErrAllLocationsVisited is expected when all tasks are complete
		if errors.Is(err, ErrAllLocationsVisited) {
			uncompleted = []models.Location{}
		} else {
			return nil, nil, fmt.Errorf("determining next locations: %w", err)
		}
	}

	// Load relations for uncompleted locations
	for i := range uncompleted {
		if loadErr := s.locationRepo.LoadRelations(ctx, &uncompleted[i]); loadErr != nil {
			return nil, nil, fmt.Errorf("loading location relations: %w", loadErr)
		}
	}

	// Get completed locations: all locations in group where BlocksCompleted is true
	// Build completion map from check-ins
	completionMap := make(map[string]bool)
	for _, checkIn := range team.CheckIns {
		if checkIn.BlocksCompleted {
			completionMap[checkIn.LocationID] = true
		}
	}

	// Build set of group location IDs for filtering
	groupLocationSet := make(map[string]bool)
	for _, locID := range group.LocationIDs {
		groupLocationSet[locID] = true
	}

	// Collect completed locations in group order
	completed = make([]models.Location, 0)
	for _, locID := range group.LocationIDs {
		if !completionMap[locID] {
			continue // Not completed
		}
		if !groupLocationSet[locID] {
			continue // Not in this group
		}

		loc, loadErr := s.locationRepo.GetByID(ctx, locID)
		if loadErr != nil {
			return nil, nil, fmt.Errorf("loading completed location: %w", loadErr)
		}
		if loadErr := s.locationRepo.LoadRelations(ctx, loc); loadErr != nil {
			return nil, nil, fmt.Errorf("loading location relations: %w", loadErr)
		}
		completed = append(completed, *loc)
	}

	return uncompleted, completed, nil
}

// getAccessibleSecretLocations returns secret locations that are accessible from the team's current position.
// Secret locations are never displayed to players but are valid for check-in via QR code, link, or GPS.
func (s *NavigationService) getAccessibleSecretLocations(
	ctx context.Context,
	team *models.Team,
) ([]models.Location, error) {
	if err := s.validateTeamState(team); err != nil {
		return nil, err
	}

	// Get completed location IDs
	completedIDs := s.getCompletedLocationIDs(team.CheckIns)

	// Compute current group
	currentGroupID := navigation.ComputeCurrentGroup(&team.Instance.GameStructure, completedIDs, team.SkippedGroupIDs)
	if currentGroupID == "" {
		return []models.Location{}, nil
	}

	// Get accessible secret location IDs from navigation package
	secretLocationIDs := navigation.GetAccessibleSecretLocationIDs(
		&team.Instance.GameStructure,
		currentGroupID,
		completedIDs,
	)

	if len(secretLocationIDs) == 0 {
		return []models.Location{}, nil
	}

	// Fetch actual location objects
	locations := make([]models.Location, 0, len(secretLocationIDs))
	for _, id := range secretLocationIDs {
		location, err := s.locationRepo.GetByID(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to load secret location %s: %w", id, err)
		}
		locations = append(locations, *location)
	}

	return locations, nil
}
