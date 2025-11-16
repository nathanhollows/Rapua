package services

import (
	"context"
	"errors"
	"fmt"
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
func (s *NavigationService) IsValidLocation(ctx context.Context, team *models.Team, markerID string) (bool, error) {
	if err := s.validateTeamState(team); err != nil {
		return false, err
	}

	// Find valid locations (without loading full relations)
	locations, err := s.determineNextLocations(ctx, team)
	if err != nil {
		return false, fmt.Errorf("determine next valid locations: %w", err)
	}

	// Check if the location code is valid
	markerID = s.normalizeMarkerID(markerID)
	for _, loc := range locations {
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

	// Get next locations
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

	// Load navigation blocks if using custom display mode
	if view.CurrentGroup.Navigation == models.NavigationDisplayCustom {
		for _, location := range locations {
			locationBlocks, blockStates, blockErr := s.blockService.FindByOwnerIDAndTeamCodeWithStateAndContext(
				ctx,
				location.ID,
				team.Code,
				blocks.ContextLocationClues,
			)
			if blockErr != nil {
				return nil, fmt.Errorf("loading navigation blocks: %w", blockErr)
			}
			view.Blocks = append(view.Blocks, locationBlocks...)
			for k, v := range blockStates {
				view.BlockStates[k] = v
			}
		}
	}

	// Load completed locations if needed
	if view.CurrentGroup.Routing == models.RouteStrategyScavengerHunt {
		completedLocations := make([]models.Location, 0, len(team.CheckIns))
		for _, checkIn := range team.CheckIns {
			location, err := s.locationRepo.GetByID(ctx, checkIn.LocationID)
			if err != nil {
				return nil, fmt.Errorf("loading completed location %s: %w", checkIn.LocationID, err)
			}
			// Load full relations
			if loadErr := s.locationRepo.LoadRelations(ctx, location); loadErr != nil {
				return nil, fmt.Errorf("loading relations for completed location: %w", loadErr)
			}
			completedLocations = append(completedLocations, *location)
		}
		view.CompletedLocations = completedLocations
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

// getCompletedLocationIDs extracts location IDs from check-ins.
func (s *NavigationService) getCompletedLocationIDs(checkIns []models.CheckIn) []string {
	completed := make([]string, 0, len(checkIns))
	for _, checkIn := range checkIns {
		completed = append(completed, checkIn.LocationID)
	}
	return completed
}
