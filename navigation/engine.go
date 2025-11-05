package navigation

import (
	"github.com/nathanhollows/Rapua/v5/models"
)

// AdvanceReason explains why GetNextGroup made a particular decision.
type AdvanceReason string

const (
	ReasonGroupIncomplete     AdvanceReason = "group_incomplete"      // Current group not yet completed
	ReasonAutoAdvanceDisabled AdvanceReason = "auto_advance_disabled" // Group complete but AutoAdvance is false
	ReasonNextSibling         AdvanceReason = "next_sibling"          // Advancing to next sibling group
	ReasonParentNextSibling   AdvanceReason = "parent_next_sibling"   // Advancing to parent's next sibling (recursive)
	ReasonAllComplete         AdvanceReason = "all_complete"          // No more groups available
	ReasonNoParent            AdvanceReason = "no_parent"             // Current group is root (shouldn't happen)
	ReasonGroupNotFound       AdvanceReason = "group_not_found"       // Group ID not found in structure
)

// Engine provides pure navigation logic functions that operate on GameStructure.
// All functions are stateless and thread-safe - they take GameStructure as input
// and return results without side effects.
//
// Design principles:
// - Pure functions: same inputs always produce same outputs
// - Zero external dependencies: no database, no services
// - Efficient: O(n) or better time complexity
// - Multi-tenant safe: no shared state between game instances

// IsGroupCompleted checks if a group's completion criteria are met based on completed locations.
// Uses map lookup for O(n) performance instead of nested loops.
func IsGroupCompleted(
	structure *models.GameStructure,
	groupID string,
	completedLocationIDs []string,
) bool {
	group := FindGroupByID(structure, groupID)
	if group == nil || len(group.LocationIDs) == 0 {
		return false
	}

	// Use map for more efficient lookup
	completed := makeSet(completedLocationIDs)
	count := 0
	for _, locID := range group.LocationIDs {
		if completed[locID] {
			count++
		}
	}

	switch group.CompletionType {
	case models.CompletionAll:
		return count == len(group.LocationIDs)
	case models.CompletionMinimum:
		return count >= group.MinimumRequired
	default:
		return false
	}
}

// GetNextGroup determines what group should be active after completing the current group.
// Returns (nextGroup, shouldAdvance, reason) where:
//   - nextGroup: the group that should be active next (may be current if not advancing)
//   - shouldAdvance: true if team should move to a different group
//   - reason: typed explanation of the decision (see AdvanceReason constants)
func GetNextGroup(
	structure *models.GameStructure,
	currentGroupID string,
	completedLocationIDs []string,
) (*models.GameStructure, bool, AdvanceReason) {
	currentGroup := FindGroupByID(structure, currentGroupID)
	if currentGroup == nil {
		return nil, false, ReasonGroupNotFound
	}

	// Check if current group is complete
	if !IsGroupCompleted(structure, currentGroupID, completedLocationIDs) {
		return currentGroup, false, ReasonGroupIncomplete
	}

	// Check if auto-advance is enabled
	if !currentGroup.AutoAdvance {
		return currentGroup, false, ReasonAutoAdvanceDisabled
	}

	// Find parent and current index
	parent, index := findParentAndIndex(currentGroup, structure)
	if parent == nil {
		return nil, false, ReasonNoParent // At root - shouldn't happen
	}

	// Try next sibling
	if index+1 < len(parent.SubGroups) {
		return &parent.SubGroups[index+1], true, ReasonNextSibling
	}

	// Try parent's next sibling (recursive)
	if !parent.IsRoot {
		nextGroup, shouldAdvance, _ := GetNextGroup(structure, parent.ID, completedLocationIDs)
		if shouldAdvance {
			return nextGroup, true, ReasonParentNextSibling
		}
	}

	// No more groups
	return nil, false, ReasonAllComplete
}

// GetAvailableLocationIDs returns location IDs a team can visit based on:
//   - Current group's routing strategy
//   - Current group's MaxNext setting
//   - Completed location IDs
//   - Team code (for deterministic randomization)
//
// Returns empty slice if:
//   - Group not found
//   - Group has no location IDs
//   - All locations in group completed
//
// Note: This function works with location IDs only. Caller is responsible
// for fetching actual location objects after filtering.
func GetAvailableLocationIDs(
	structure *models.GameStructure,
	groupID string,
	completedLocationIDs []string,
	teamCode string,
) []string {
	group := FindGroupByID(structure, groupID)
	if group == nil || len(group.LocationIDs) == 0 {
		return []string{}
	}

	// Filter to unvisited location IDs
	unvisitedIDs := filterUnvisitedIDs(group.LocationIDs, completedLocationIDs)
	if len(unvisitedIDs) == 0 {
		return []string{}
	}

	// Apply routing strategy
	switch group.Routing {
	case models.RouteStrategyOrdered:
		// For ordered routing, the position in LocationIDs IS the order
		// Return single location ID with lowest order (first unvisited)
		firstID := findMinOrderID(group.LocationIDs, completedLocationIDs)
		if firstID == "" {
			return []string{}
		}
		return []string{firstID}

	case models.RouteStrategyRandom:
		// Return up to maxNext randomly selected location IDs (deterministic per team)
		return deterministicShuffleIDs(group.LocationIDs, completedLocationIDs, teamCode, group.MaxNext)

	case models.RouteStrategyFreeRoam:
		// Return all unvisited location IDs
		return unvisitedIDs

	default:
		return unvisitedIDs
	}
}

// FindGroupByID recursively searches for a group with the specified ID.
// Returns nil if not found.
func FindGroupByID(root *models.GameStructure, groupID string) *models.GameStructure {
	if root == nil {
		return nil
	}

	if root.ID == groupID {
		return root
	}

	// Recursively search subgroups
	for i := range root.SubGroups {
		if found := FindGroupByID(&root.SubGroups[i], groupID); found != nil {
			return found
		}
	}

	return nil
}

// GetFirstVisibleGroup returns the first non-root subgroup, which is where teams should start.
// Returns nil if structure has no subgroups (invalid game configuration).
func GetFirstVisibleGroup(structure *models.GameStructure) *models.GameStructure {
	if structure == nil || !structure.IsRoot {
		return nil
	}

	if len(structure.SubGroups) == 0 {
		return nil
	}

	return &structure.SubGroups[0]
}

// ValidateStructure performs basic validation on a GameStructure.
// Returns error if:
//   - Root group has no subgroups
//   - Multiple root groups exist
//   - Non-root group has IsRoot=true
//   - Duplicate group IDs found
//   - Duplicate location IDs found
//   - Visible groups missing name or color
//   - Ordered groups do not use CompletionAll
//   - Random routing has MaxNext = 0
func ValidateStructure(structure *models.GameStructure) error {
	if structure == nil {
		return nil
	}

	// Root must have at least one subgroup for teams to start
	if structure.IsRoot && len(structure.SubGroups) == 0 {
		return ErrNoVisibleGroups
	}

	// Validate root constraints
	if err := validateRootConstraints(structure); err != nil {
		return err
	}

	// Check for duplicate group IDs
	groupIDs := make(map[string]bool)
	if err := checkDuplicateGroupIDs(structure, groupIDs); err != nil {
		return err
	}

	// Check for duplicate location IDs
	locationIDs := make(map[string]bool)
	if err := checkDuplicateLocationIDs(structure, locationIDs); err != nil {
		return err
	}

	// Ordered groups must have CompletionAll
	if err := validateCompletionType(structure); err != nil {
		return err
	}

	// Validate routing strategies
	if err := validateRoutingStrategies(structure); err != nil {
		return err
	}

	// Validate group metadata
	return validateGroupMetadata(structure)
}
