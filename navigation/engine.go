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

// GetNextGroup finds the next non-secret group in sequence after the current group.
// This is a low-level function that just finds the next sibling or parent's sibling.
// It does NOT check AutoAdvance - that's handled by ComputeCurrentGroup.
// Secret groups are skipped as they never become the current group.
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

	// Try next non-secret sibling
	for i := index + 1; i < len(parent.SubGroups); i++ {
		if parent.SubGroups[i].Routing != models.RouteStrategySecret {
			return &parent.SubGroups[i], true, ReasonNextSibling
		}
	}

	// Try parent's next sibling (recursive)
	if !parent.IsRoot {
		nextGroup, shouldAdvance, _ := GetNextGroup(structure, parent.ID, completedLocationIDs)
		if shouldAdvance {
			return nextGroup, true, ReasonParentNextSibling
		}
	}

	// No more non-secret groups
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
// Note: Secret locations are handled separately by GetAccessibleSecretLocationIDs.
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

	case models.RouteStrategySecret:
		// Secret locations are never shown in the normal available locations
		// They are only accessible via direct access (QR code, link, GPS)
		return []string{}

	default:
		return unvisitedIDs
	}
}

// GetAccessibleSecretLocationIDs returns secret location IDs that are accessible from the current group.
// Secret locations are accessible if they are siblings of the current group or any of its ancestors.
// This walks UP the tree recursively, checking for secret siblings at each level.
//
// Accessible relationships (walking UP):
//   - Sibling: same parent
//   - Uncle: parent's sibling
//   - Great-uncle: grandparent's sibling
//   - Great-great-uncle: great-grandparent's sibling
//   - ... and so on to root
//
// NOT accessible (never walks DOWN):
//   - Cousins: children of uncles
//   - Nested children: descendants of any group
//
// Example structure:
//
//	root[
//	  secret_root_level[loc9],         ← great-uncle to inner_group
//	  branch_a[
//	    secret_a[loc7, loc8],          ← uncle to inner_group
//	    branch_b[
//	      inner_group[loc1, loc2],     ← current group
//	      secret_b[loc3, loc4]         ← sibling to inner_group
//	    ]
//	  ]
//	]
//
// If player is in inner_group:
//   - secret_b accessible (sibling)
//   - secret_a accessible (uncle - sibling of parent branch_b)
//   - secret_root_level accessible (great-uncle - sibling of grandparent branch_a)
//
// Returns empty slice if:
//   - Current group not found
//   - No secret groups are accessible
func GetAccessibleSecretLocationIDs(
	structure *models.GameStructure,
	currentGroupID string,
	completedLocationIDs []string,
) []string {
	currentGroup := FindGroupByID(structure, currentGroupID)
	if currentGroup == nil {
		return []string{}
	}

	accessibleIDs := make([]string, 0)
	completed := makeSet(completedLocationIDs)

	// Walk up the tree, checking for secret siblings at each ancestor level
	ancestor := currentGroup
	for {
		parent, _ := findParentAndIndex(ancestor, structure)
		if parent == nil {
			break // Reached root
		}

		// Collect unvisited secret locations from siblings
		secretIDs := collectSecretSiblingLocations(parent, ancestor.ID, completed)
		accessibleIDs = append(accessibleIDs, secretIDs...)

		// Move up to next ancestor
		if parent.IsRoot {
			break // Stop at root
		}
		ancestor = parent
	}

	return accessibleIDs
}

// collectSecretSiblingLocations finds unvisited locations in secret sibling groups.
func collectSecretSiblingLocations(
	parent *models.GameStructure,
	excludeGroupID string,
	completed map[string]bool,
) []string {
	secretIDs := make([]string, 0)

	for i := range parent.SubGroups {
		subGroup := &parent.SubGroups[i]
		if subGroup.ID == excludeGroupID {
			continue // Skip the excluded group
		}
		if subGroup.Routing == models.RouteStrategySecret {
			// Add unvisited locations from this secret sibling group
			for _, locID := range subGroup.LocationIDs {
				if !completed[locID] {
					secretIDs = append(secretIDs, locID)
				}
			}
		}
	}

	return secretIDs
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

// FindGroupContainingLocation recursively searches for a group containing the specified location ID.
// Returns nil if not found.
func FindGroupContainingLocation(root *models.GameStructure, locationID string) *models.GameStructure {
	if root == nil {
		return nil
	}

	// Check if this group contains the location
	for _, id := range root.LocationIDs {
		if id == locationID {
			return root
		}
	}

	// Recursively search subgroups
	for i := range root.SubGroups {
		if found := FindGroupContainingLocation(&root.SubGroups[i], locationID); found != nil {
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
