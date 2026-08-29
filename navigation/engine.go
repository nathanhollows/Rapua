package navigation

import (
	"github.com/nathanhollows/Rapua/v8/models"
)

// AdvanceReason explains why GetNextGroup made a particular decision.
type AdvanceReason string

const (
	ReasonNextSibling       AdvanceReason = "next_sibling"        // Advancing to next sibling group
	ReasonParentNextSibling AdvanceReason = "parent_next_sibling" // Advancing to parent's next sibling (recursive)
	ReasonAllComplete       AdvanceReason = "all_complete"        // No more groups available
	ReasonNoParent          AdvanceReason = "no_parent"           // Current group is root (shouldn't happen)
	ReasonGroupNotFound     AdvanceReason = "group_not_found"     // Group ID not found in structure
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

// ComputeCurrentGroupForObjectives walks GameStructure.ObjectiveIDs, advancing
// through groups whose completion/auto-advance criteria are already met, and
// returns the ID of the first group the player should actually see.
func ComputeCurrentGroupForObjectives( //nolint:gocognit
	structure *models.GameStructure,
	completedObjectiveIDs []string,
	skippedGroupIDs []string,
) string {
	current := GetFirstVisibleGroup(structure)
	if current == nil {
		return ""
	}

	skipped := makeSet(skippedGroupIDs)

	for {
		if skipped[current.ID] {
			next, shouldAdvance, reason := GetNextGroup(structure, current.ID, completedObjectiveIDs)

			if !shouldAdvance || reason == ReasonAllComplete {
				return current.ID
			}

			current = next
			continue
		}

		completed := makeSet(completedObjectiveIDs)
		completedCount := 0
		for _, objID := range current.ObjectiveIDs {
			if completed[objID] {
				completedCount++
			}
		}

		isMinimumMet := false
		switch current.CompletionType {
		case models.CompletionAll:
			isMinimumMet = completedCount == len(current.ObjectiveIDs)
		case models.CompletionMinimum:
			isMinimumMet = completedCount >= current.MinimumRequired
		}

		if !isMinimumMet {
			return current.ID
		}

		allComplete := completedCount == len(current.ObjectiveIDs)

		if allComplete {
			next, shouldAdvance, reason := GetNextGroup(structure, current.ID, completedObjectiveIDs)

			if !shouldAdvance || reason == ReasonAllComplete {
				return current.ID
			}

			current = next
			continue
		}

		if !current.AutoAdvance {
			return current.ID
		}

		next, shouldAdvance, reason := GetNextGroup(structure, current.ID, completedObjectiveIDs)

		if !shouldAdvance || reason == ReasonAllComplete {
			return current.ID
		}

		current = next
	}
}

// GetNextGroup finds the next non-secret group in sequence after the current group.
// This is a low-level function that just finds the next sibling or parent's sibling.
// It does NOT check AutoAdvance - that's handled by ComputeCurrentGroupForObjectives.
// Secret groups are skipped as they never become the current group.
//
// Returns (nextGroup, shouldAdvance, reason) where:
//   - nextGroup: the group that should be active next (may be current if not advancing)
//   - shouldAdvance: true if there is a next group to move to
//   - reason: typed explanation of the decision (see AdvanceReason constants)
func GetNextGroup(
	structure *models.GameStructure,
	currentGroupID string,
	completedObjectiveIDs []string,
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

	// Try next non-secret sibling
	for i := index + 1; i < len(parent.SubGroups); i++ {
		if parent.SubGroups[i].Routing != models.RouteStrategySecret {
			return &parent.SubGroups[i], true, ReasonNextSibling
		}
	}

	// Try parent's next sibling (recursive)
	if !parent.IsRoot {
		nextGroup, shouldAdvance, _ := GetNextGroup(structure, parent.ID, completedObjectiveIDs)
		if shouldAdvance {
			return nextGroup, true, ReasonParentNextSibling
		}
	}

	// No more non-secret groups
	return nil, false, ReasonAllComplete
}

// GetAvailableObjectiveIDs returns objective IDs a team can pursue next, based on
// the current group's routing strategy, MaxNext setting, and completed objective IDs.
func GetAvailableObjectiveIDs(
	structure *models.GameStructure,
	groupID string,
	completedObjectiveIDs []string,
	runCode string,
) []string {
	group := FindGroupByID(structure, groupID)
	if group == nil || len(group.ObjectiveIDs) == 0 {
		return []string{}
	}

	unvisitedIDs := filterUnvisitedIDs(group.ObjectiveIDs, completedObjectiveIDs)
	if len(unvisitedIDs) == 0 {
		return []string{}
	}

	switch group.Routing {
	case models.RouteStrategyOrdered:
		firstID := findMinOrderID(group.ObjectiveIDs, completedObjectiveIDs)
		if firstID == "" {
			return []string{}
		}
		return []string{firstID}

	case models.RouteStrategyRandomised:
		return deterministicShuffleIDs(group.ObjectiveIDs, completedObjectiveIDs, runCode, group.MaxNext)

	case models.RouteStrategyFreeRoam:
		return unvisitedIDs

	case models.RouteStrategySecret:
		return []string{}

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

// FindGroupContainingObjective recursively searches for a group containing the specified objective ID.
// Returns nil if not found.
func FindGroupContainingObjective(root *models.GameStructure, objectiveID string) *models.GameStructure {
	if root == nil {
		return nil
	}

	for _, id := range root.ObjectiveIDs {
		if id == objectiveID {
			return root
		}
	}

	for i := range root.SubGroups {
		if found := FindGroupContainingObjective(&root.SubGroups[i], objectiveID); found != nil {
			return found
		}
	}

	return nil
}

// GetFirstVisibleGroup returns the first non-root, non-secret subgroup, which is where teams should start.
// Secret groups are never the current group - they're accessible but don't affect progression.
// Returns nil if structure has no non-secret subgroups.
func GetFirstVisibleGroup(structure *models.GameStructure) *models.GameStructure {
	if structure == nil || !structure.IsRoot {
		return nil
	}

	if len(structure.SubGroups) == 0 {
		return nil
	}

	// Find first non-secret group
	for i := range structure.SubGroups {
		if structure.SubGroups[i].Routing != models.RouteStrategySecret {
			return &structure.SubGroups[i]
		}
	}

	// All subgroups are secret - invalid configuration
	return nil
}

// ValidateStructure performs basic validation on a GameStructure.
// Returns error if:
//   - Root group has no subgroups
//   - Multiple root groups exist
//   - Non-root group has IsRoot=true
//   - Duplicate group IDs found
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
