package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
)

var (
	ErrAllLocationsVisited = errors.New("all locations visited")
	ErrInstanceNotFound    = errors.New("instance not found")
)

type NavigationService struct {
	locationRepo         repositories.LocationRepository
	teamRepo             repositories.RunRepository
	varStateRepo         repositories.RunVarStateRepository
	gameStructureService *GameStructureService
	blockService         *BlockService
	logger               *slog.Logger
}

// PlayerNavigationView contains all data needed to render the player navigation UI.
type PlayerNavigationView struct {
	// Settings
	Settings models.QuestSettings // Global instance settings

	// Current state
	CurrentGroup     *models.GameStructure // Current group
	CanAdvanceEarly  bool                  // Whether team can manually advance to next group (minimum met, AutoAdvance=false, not 100% complete)
	MustCheckOut     bool                  // Whether team must check out before proceeding
	BlockingLocation *models.Location      // Location team must check out from (nil if not blocked)

	// Available locations
	NextLocations []models.Location // Locations team can visit next

	// Navigation blocks for next locations
	Blocks      []blocks.Block                // All navigation blocks for next locations
	BlockStates map[string]blocks.PlayerState // States for navigation blocks
}

// NewNavigationService creates a NavigationService.
func NewNavigationService(
	locationRepo repositories.LocationRepository,
	teamRepo repositories.RunRepository,
	varStateRepo repositories.RunVarStateRepository,
	gameStructureService *GameStructureService,
	blockService *BlockService,
	logger *slog.Logger,
) *NavigationService {
	return &NavigationService{
		locationRepo:         locationRepo,
		teamRepo:             teamRepo,
		varStateRepo:         varStateRepo,
		gameStructureService: gameStructureService,
		blockService:         blockService,
		logger:               logger,
	}
}

// filterGameStructure returns a copy of gs with:
//   - sub-groups whose when clause evaluates to false removed (recursively)
//   - location IDs in hiddenLocationIDs removed from LocationIDs
//
// This ensures the navigation engine never sees hidden locations or groups
// when computing group completion, next group, and available locations.
// The root group itself is never filtered (root has no When clause by convention).
func filterGameStructure(
	gs models.GameStructure,
	resolver game.VarResolver,
	hiddenLocationIDs map[string]bool,
) models.GameStructure {
	filtered := gs

	// Strip hidden location IDs so completion calculations ignore them.
	if len(hiddenLocationIDs) > 0 && len(gs.LocationIDs) > 0 {
		filteredIDs := make([]string, 0, len(gs.LocationIDs))
		for _, id := range gs.LocationIDs {
			if !hiddenLocationIDs[id] {
				filteredIDs = append(filteredIDs, id)
			}
		}
		filtered.LocationIDs = filteredIDs
	}

	filtered.SubGroups = nil
	for _, sub := range gs.SubGroups {
		if game.EvaluateWhen(sub.When, resolver) {
			filtered.SubGroups = append(filtered.SubGroups, filterGameStructure(sub, resolver, hiddenLocationIDs))
		}
	}
	return filtered
}

// filterLocationsByWhen returns locs with any entry whose when clause evaluates
// to false removed. nil resolver or nil When clause → always visible.
func filterLocationsByWhen(locs []models.Location, resolver game.VarResolver) []models.Location {
	out := make([]models.Location, 0, len(locs))
	for _, loc := range locs {
		if game.EvaluateWhen(loc.When, resolver) {
			out = append(out, loc)
		}
	}
	return out
}

// IsValidLocation checks if the location code is valid for the team to check in to.
// This includes both regular available locations AND accessible secret locations.
//
// Note: when clauses on locations are intentionally NOT evaluated here. A when clause
// controls visibility (whether the location appears in the player's list), not access.
// A team who physically finds a QR code for a condition-gated location should still be
// able to check in. Use navigation routing mode (ordered, etc.) to restrict access.
func (s *NavigationService) IsValidLocation(ctx context.Context, team *models.Run, markerID string) (bool, error) {
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
func (s *NavigationService) GetNextLocations(ctx context.Context, team *models.Run) ([]models.Location, error) {
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
	team *models.Run,
) (*PlayerNavigationView, error) {
	// Load team relations if not already loaded
	if err := s.ensureTeamRelationsLoaded(ctx, team); err != nil {
		return nil, fmt.Errorf("loading team relations: %w", err)
	}

	view := &PlayerNavigationView{
		Settings:    team.Quest.Settings,
		Blocks:      make([]blocks.Block, 0),
		BlockStates: make(map[string]blocks.PlayerState),
	}

	// Build resolver for visibility evaluation.
	// team_count requires an extra query; failure is non-fatal but logged.
	runCount, countErr := s.teamRepo.CountByInstance(ctx, team.QuestID)
	if countErr != nil {
		runCount = 0
		s.logger.WarnContext(ctx, "getting team count for visibility resolver",
			"instance_id", team.QuestID, "error", countErr)
	}
	resolver := NewPlayerVarResolver(team, team.VarStates).WithRunCount(runCount)

	// Build set of location IDs that are hidden by their when clause.
	// team.Quest.Locations is populated by LoadRelations and includes when_clause.
	// Note: filterLocationsByWhen also evaluates When on locations fetched from the
	// repo later; both use the same resolver so they will always agree.
	hiddenLocationIDs := make(map[string]bool)
	for _, loc := range team.Quest.Locations {
		if !game.EvaluateWhen(loc.When, resolver) {
			hiddenLocationIDs[loc.ID] = true
		}
	}

	// Build a local copy of the team whose Instance.GameStructure has been filtered
	// for visibility. Using a copy (not pointer mutation + defer) ensures the caller's
	// team is never modified, which avoids data races if the team is ever accessed
	// concurrently and makes the control flow easier to reason about.
	navTeam := team
	if team.Quest.GameStructure.ID != "" {
		navInstance := team.Quest
		navInstance.GameStructure = filterGameStructure(team.Quest.GameStructure, resolver, hiddenLocationIDs)
		navTeam = &models.Run{}
		*navTeam = *team
		navTeam.Quest = navInstance
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
	if navTeam.Quest.GameStructure.ID != "" {
		// Compute current group from completed locations
		completedIDs := s.getCompletedLocationIDs(navTeam.CheckIns)
		currentGroupID := navigation.ComputeCurrentGroup(
			&navTeam.Quest.GameStructure,
			completedIDs,
			navTeam.SkippedGroupIDs,
		)

		if currentGroupID != "" {
			currentGroup = navigation.FindGroupByID(&navTeam.Quest.GameStructure, currentGroupID)
		}
		view.CurrentGroup = currentGroup

		view.CanAdvanceEarly = s.computeCanAdvanceEarly(currentGroup, completedIDs)
	}

	// Get next locations
	locations, err := s.determineNextLocations(ctx, team)
	if err != nil {
		return nil, err
	}

	// Filter by location-level visibility conditions
	locations = filterLocationsByWhen(locations, resolver)

	// Load full relations for each location
	for i := range locations {
		if loadErr := s.locationRepo.LoadRelations(ctx, &locations[i]); loadErr != nil {
			return nil, fmt.Errorf("loading relations for location: %w", loadErr)
		}
	}
	view.NextLocations = locations

	// Load navigation blocks for all next locations
	for _, location := range locations {
		locationBlocks, blockStates, blockErr := s.blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
			ctx,
			location.ID,
			team.Code, team.QuestID,
			blocks.ContextNavigation,
		)
		if blockErr != nil {
			return nil, fmt.Errorf("loading navigation blocks: %w", blockErr)
		}
		view.Blocks = append(view.Blocks, locationBlocks...)
		maps.Copy(view.BlockStates, blockStates)
	}

	return view, nil
}

// determineNextLocations is the core logic for finding next locations without relation loading.
func (s *NavigationService) determineNextLocations(ctx context.Context, team *models.Run) ([]models.Location, error) {
	if err := s.validateTeamState(team); err != nil {
		return nil, err
	}

	// All games use GameStructure (migration converts legacy games)
	return s.getValidLocationsFromGameStructure(ctx, team)
}

// validateTeamState checks if team has required relations loaded.
func (s *NavigationService) validateTeamState(team *models.Run) error {
	if team.Quest.ID == "" {
		return ErrInstanceNotFound
	}
	if team.Quest.Settings.QuestID == "" {
		return ErrInstanceSettingsNotFound
	}
	// Note: Locations are no longer required here since all games use GameStructure
	// and locations are loaded on-demand by group
	return nil
}

// ensureTeamRelationsLoaded loads team relations (including VarStates) if not already loaded.
func (s *NavigationService) ensureTeamRelationsLoaded(ctx context.Context, team *models.Run) error {
	if team.Quest.ID == "" || len(team.CheckIns) == 0 {
		if err := s.teamRepo.LoadRelations(ctx, team); err != nil {
			return err
		}
	}
	// Always reload VarStates: they change when blocks fire `sets` triggers and must
	// be fresh for `when` evaluation on locations/groups to work correctly.
	varStates, err := s.varStateRepo.GetAll(ctx, team.Code, team.QuestID)
	if err != nil {
		return fmt.Errorf("loading var states: %w", err)
	}
	team.VarStates = varStates
	return nil
}

// computeCanAdvanceEarly returns true when the team has met the group minimum
// but not all locations are complete, and AutoAdvance is disabled.
func (s *NavigationService) computeCanAdvanceEarly(group *models.GameStructure, completedIDs []string) bool {
	if group == nil || group.AutoAdvance || len(group.LocationIDs) == 0 {
		return false
	}
	completedSet := make(map[string]bool, len(completedIDs))
	for _, id := range completedIDs {
		completedSet[id] = true
	}
	completedCount := 0
	for _, locID := range group.LocationIDs {
		if completedSet[locID] {
			completedCount++
		}
	}
	allComplete := completedCount == len(group.LocationIDs)
	var isMinimumMet bool
	switch group.CompletionType {
	case models.CompletionAll:
		isMinimumMet = allComplete
	case models.CompletionMinimum:
		isMinimumMet = completedCount >= group.MinimumRequired
	}
	return isMinimumMet && !allComplete
}

// normalizeMarkerID trims and uppercases marker ID.
func (s *NavigationService) normalizeMarkerID(markerID string) string {
	return strings.TrimSpace(strings.ToUpper(markerID))
}

// getValidLocationsFromGameStructure determines valid locations using the GameStructure system.
func (s *NavigationService) getValidLocationsFromGameStructure(
	ctx context.Context,
	team *models.Run,
) ([]models.Location, error) {
	// 1. Check if team is locked at a location (MustCheckOut)
	// Use existing Team.MustCheckOut field (single source of truth)
	if team.MustCheckOut != "" {
		return []models.Location{}, nil // No locations available until checkout
	}

	// 2. Get completed location IDs from CheckIns
	completedIDs := s.getCompletedLocationIDs(team.CheckIns)

	// 3. Compute current group from completed locations (pure function, deterministic)
	currentGroupID := navigation.ComputeCurrentGroup(&team.Quest.GameStructure, completedIDs, team.SkippedGroupIDs)

	if currentGroupID == "" {
		// No valid group (either no groups configured or all completed)
		return []models.Location{}, nil
	}

	// 4. Get available location IDs using navigation package
	locationIDs := navigation.GetAvailableLocationIDs(
		&team.Quest.GameStructure,
		currentGroupID,
		completedIDs,
		team.Code,
	)

	if len(locationIDs) == 0 {
		// Check if game is complete (no next group to advance to)
		_, shouldAdvance, _ := navigation.GetNextGroup(&team.Quest.GameStructure, currentGroupID, completedIDs)
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
	team *models.Run,
	locationID string,
) (*PlayerNavigationView, error) {
	// Load team relations if not already loaded
	if err := s.ensureTeamRelationsLoaded(ctx, team); err != nil {
		return nil, fmt.Errorf("loading team relations: %w", err)
	}

	// Find the group containing this location
	group := navigation.FindGroupContainingLocation(&team.Quest.GameStructure, locationID)
	if group == nil {
		return nil, errors.New("location not found in game structure")
	}

	// Load the location
	location, err := s.locationRepo.GetByID(ctx, locationID)
	if err != nil {
		return nil, fmt.Errorf("loading location: %w", err)
	}

	// Load location relations (including blocks)
	err = s.locationRepo.LoadRelations(ctx, location)
	if err != nil {
		return nil, fmt.Errorf("loading location relations: %w", err)
	}

	view := &PlayerNavigationView{
		Settings:        team.Quest.Settings,
		CurrentGroup:    group,
		NextLocations:   []models.Location{*location},
		MustCheckOut:    false,
		CanAdvanceEarly: false,
		Blocks:          make([]blocks.Block, 0),
		BlockStates:     make(map[string]blocks.PlayerState),
	}

	// Load navigation blocks for the location
	locationBlocks, blockStates, blockErr := s.blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		ctx,
		location.ID,
		team.Code, team.QuestID,
		blocks.ContextNavigation,
	)
	if blockErr != nil {
		return nil, fmt.Errorf("loading navigation blocks: %w", blockErr)
	}
	view.Blocks = append(view.Blocks, locationBlocks...)
	maps.Copy(view.BlockStates, blockStates)

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

// getAccessibleSecretLocations returns secret locations that are accessible from the team's current position.
// Secret locations are never displayed to players but are valid for check-in via QR code, link, or GPS.
func (s *NavigationService) getAccessibleSecretLocations(
	ctx context.Context,
	team *models.Run,
) ([]models.Location, error) {
	if err := s.validateTeamState(team); err != nil {
		return nil, err
	}

	// Get completed location IDs
	completedIDs := s.getCompletedLocationIDs(team.CheckIns)

	// Compute current group
	currentGroupID := navigation.ComputeCurrentGroup(&team.Quest.GameStructure, completedIDs, team.SkippedGroupIDs)
	if currentGroupID == "" {
		return []models.Location{}, nil
	}

	// Get accessible secret location IDs from navigation package
	secretLocationIDs := navigation.GetAccessibleSecretLocationIDs(
		&team.Quest.GameStructure,
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
