package navigation

import (
	"github.com/nathanhollows/Rapua/v6/models"
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

// ComputeCurrentGroup determines which group a team should currently be in based on their
// completed locations. This walks through the structure from the first visible group,
// automatically advancing through completed groups.
//
// Auto-advance behavior:
//   - 100% completion: ALWAYS auto-advance to next group
//   - Minimum completion (not 100%):
//   - AutoAdvance = true: advance immediately
//   - AutoAdvance = false: stay in group (let players complete remaining locations)
//   - Skipped groups: If a group is in skippedGroupIDs, advance past it even if AutoAdvance = false
//
// Returns empty string if:
//   - Structure has no visible groups
//   - All groups are completed
//
// This function is deterministic: same inputs always produce same result.
func ComputeCurrentGroup(
	structure *models.GameStructure,
	completedLocationIDs []string,
	skippedGroupIDs []string,
) string {
	// Start at first visible group
	current := GetFirstVisibleGroup(structure)
	if current == nil {
		return "" // No visible groups configured
	}

	// Create set for fast skipped group lookup
	skipped := makeSet(skippedGroupIDs)

	// Walk through structure, advancing when appropriate
	for {
		// Check if this group was manually skipped
		if skipped[current.ID] {
			// Group was skipped - advance to next
			next, shouldAdvance, reason := GetNextGroup(structure, current.ID, completedLocationIDs)

			// If we can't advance or reached the end, stay at current
			if !shouldAdvance || reason == ReasonAllComplete {
				return current.ID
			}

			// Move to next group and continue checking
			current = next
			continue
		}

		// Check completion status
		completed := makeSet(completedLocationIDs)
		completedCount := 0
		for _, locID := range current.LocationIDs {
			if completed[locID] {
				completedCount++
			}
		}

		// If group is incomplete (minimum not met), stay here
		isMinimumMet := false
		switch current.CompletionType {
		case models.CompletionAll:
			isMinimumMet = completedCount == len(current.LocationIDs)
		case models.CompletionMinimum:
			isMinimumMet = completedCount >= current.MinimumRequired
		}

		if !isMinimumMet {
			return current.ID // Stay - minimum not met
		}

		// Minimum is met - check if we should advance
		allComplete := completedCount == len(current.LocationIDs)

		// Always advance if 100% complete
		if allComplete {
			// Try to advance to next group
			next, shouldAdvance, reason := GetNextGroup(structure, current.ID, completedLocationIDs)

			// If we can't advance or reached the end, stay at current
			if !shouldAdvance || reason == ReasonAllComplete {
				return current.ID
			}

			// Move to next group and continue checking
			current = next
			continue
		}

		// Partial completion (minimum met, but not all locations)
		// Only advance if AutoAdvance is true
		if !current.AutoAdvance {
			return current.ID // Stay - let players complete remaining locations
		}

		// AutoAdvance is true and minimum met - advance to next group
		next, shouldAdvance, reason := GetNextGroup(structure, current.ID, completedLocationIDs)

		if !shouldAdvance || reason == ReasonAllComplete {
			return current.ID
		}

		current = next
	}
}

// GetNextGroup finds the next group in sequence after the current group.
// This is a low-level function that just finds the next sibling or parent's sibling.
// It does NOT check AutoAdvance - that's handled by ComputeCurrentGroup.
//
// Returns (nextGroup, shouldAdvance, reason) where:
//   - nextGroup: the group that should be active next (may be current if not advancing)
//   - shouldAdvance: true if there is a next group to move to
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

	case models.RouteStrategyScavengerHunt:
		// Return all unvisited location IDs
		// The view is responsible for showing a checklist including all visited locations
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
