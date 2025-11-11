package navigation

import (
	"github.com/nathanhollows/Rapua/v6/models"
	"golang.org/x/exp/rand"
)

// findParentAndIndex finds the parent of target group and its index in parent's SubGroups.
// Returns (parent, index) or (nil, -1) if not found.
func findParentAndIndex(
	target *models.GameStructure,
	current *models.GameStructure,
) (*models.GameStructure, int) {
	for i, subGroup := range current.SubGroups {
		if subGroup.ID == target.ID {
			return current, i
		}

		// Recursively search in subgroup's children
		if parent, idx := findParentAndIndex(target, &subGroup); parent != nil {
			return parent, idx
		}
	}
	return nil, -1
}

// makeSet converts a slice of strings to a map for O(1) lookup.
func makeSet(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// filterUnvisitedIDs returns location IDs that haven't been completed.
func filterUnvisitedIDs(
	locationIDs []string,
	completedIDs []string,
) []string {
	if len(locationIDs) == 0 {
		return []string{}
	}

	completed := makeSet(completedIDs)
	unvisited := make([]string, 0, len(locationIDs))

	for _, id := range locationIDs {
		if !completed[id] {
			unvisited = append(unvisited, id)
		}
	}

	return unvisited
}

// findMinOrderID returns the first unvisited location ID from the ordered list.
// For ordered routing, the position in LocationIDs slice IS the order.
// Returns empty string if all locations are completed.
func findMinOrderID(locationIDs []string, completedIDs []string) string {
	if len(locationIDs) == 0 {
		return ""
	}

	completed := makeSet(completedIDs)

	// Return first unvisited location (they're already in order)
	for _, id := range locationIDs {
		if !completed[id] {
			return id
		}
	}

	return "" // All completed
}

// deterministicShuffleIDs shuffles ALL location IDs deterministically based on team code,
// then filters to unvisited, then returns up to maxNext.
//
// This ensures:
// - Same random order for a team across all requests
// - Order doesn't change as locations are completed (random with replacement)
// - Consistent "next N" regardless of what's been visited
//
// Example:
//
//	All location IDs: [A, B, C, D, E]
//	Team shuffle:     [C, A, E, B, D]  (deterministic per team)
//	Completed: [C]
//	Unvisited from shuffled: [A, E, B, D]
//	Return first 2: [A, E]
//
// Later, after completing A:
//
//	Completed: [C, A]
//	Unvisited from SAME shuffle: [E, B, D]
//	Return first 2: [E, B]  (consistent order maintained)
func deterministicShuffleIDs(
	allLocationIDs []string,
	completedIDs []string,
	teamCode string,
	maxNext int,
) []string {
	if len(allLocationIDs) == 0 {
		return []string{}
	}

	// Create deterministic seed from team code
	seed := uint64(0)
	for _, c := range teamCode {
		seed += uint64(c)
	}

	// Shuffle ALL location IDs (not just unvisited) to maintain consistent order
	rng := rand.New(rand.NewSource(seed))
	shuffled := make([]string, len(allLocationIDs))
	copy(shuffled, allLocationIDs)
	rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	// Filter shuffled list to only unvisited location IDs
	completed := makeSet(completedIDs)
	unvisited := make([]string, 0, len(shuffled))
	for _, id := range shuffled {
		if !completed[id] {
			unvisited = append(unvisited, id)
		}
	}

	// Return up to maxNext unvisited location IDs (in shuffled order)
	if maxNext > 0 && maxNext < len(unvisited) {
		return unvisited[:maxNext]
	}
	return unvisited
}

// checkDuplicateGroupIDs recursively checks for duplicate group IDs.
func checkDuplicateGroupIDs(group *models.GameStructure, seen map[string]bool) error {
	if group.ID != "" {
		if seen[group.ID] {
			return ErrDuplicateGroupID
		}
		seen[group.ID] = true
	}

	for i := range group.SubGroups {
		if err := checkDuplicateGroupIDs(&group.SubGroups[i], seen); err != nil {
			return err
		}
	}

	return nil
}

// validateCompletionType checks that ordered groups have CompletionAll type.
func validateCompletionType(group *models.GameStructure) error {
	if group.Routing == models.RouteStrategyOrdered && group.CompletionType != models.CompletionAll {
		return ErrInvalidCompletionType
	}

	for i := range group.SubGroups {
		if err := validateCompletionType(&group.SubGroups[i]); err != nil {
			return err
		}
	}

	return nil
}

// checkDuplicateLocationIDs recursively checks for duplicate location IDs.
func checkDuplicateLocationIDs(group *models.GameStructure, seen map[string]bool) error {
	for _, id := range group.LocationIDs {
		if seen[id] {
			return ErrDuplicateLocationID
		}
		seen[id] = true
	}

	for i := range group.SubGroups {
		if err := checkDuplicateLocationIDs(&group.SubGroups[i], seen); err != nil {
			return err
		}
	}

	return nil
}

// validateGroupMetadata checks that visible groups have required metadata.
func validateGroupMetadata(group *models.GameStructure) error {
	// Root group can have empty name and color
	if !group.IsRoot {
		if group.Name == "" {
			return ErrMissingGroupName
		}
		if group.Color == "" {
			return ErrMissingGroupColor
		}
	}

	// Recursively validate subgroups
	for i := range group.SubGroups {
		if err := validateGroupMetadata(&group.SubGroups[i]); err != nil {
			return err
		}
	}

	return nil
}

// validateRootConstraints ensures only the top-level group can be a root.
func validateRootConstraints(group *models.GameStructure) error {
	return checkRootConstraints(group, true)
}

// checkRootConstraints recursively checks that only the top-level group is marked as root.
func checkRootConstraints(group *models.GameStructure, isTopLevel bool) error {
	// Only top-level group can be root
	if !isTopLevel && group.IsRoot {
		return ErrNonRootIsRoot
	}

	// Top-level group should be root (optional check - can be relaxed)
	// if isTopLevel && !group.IsRoot {
	//     return ErrTopLevelNotRoot
	// }

	// Recursively check subgroups (none should be root)
	for i := range group.SubGroups {
		if err := checkRootConstraints(&group.SubGroups[i], false); err != nil {
			return err
		}
	}

	return nil
}

// validateRoutingStrategies checks that routing strategies have valid configuration.
func validateRoutingStrategies(group *models.GameStructure) error {
	// Skip validation for root group (it doesn't use routing)
	if !group.IsRoot {
		// Random routing must have MaxNext > 0 (0 means unlimited, but should be explicit)
		if group.Routing == models.RouteStrategyRandom && group.MaxNext == 0 {
			return ErrInvalidMaxNext
		}
	}

	// Recursively validate subgroups
	for i := range group.SubGroups {
		if err := validateRoutingStrategies(&group.SubGroups[i]); err != nil {
			return err
		}
	}

	return nil
}
