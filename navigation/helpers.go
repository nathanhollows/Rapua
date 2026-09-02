package navigation

import "math/rand"

// deterministicShuffleIDs shuffles ALL objective IDs deterministically based on team code,
// then filters to unvisited, then returns up to maxNext.
//
// This ensures:
// - Same random order for a team across all requests
// - Order doesn't change as objectives are completed (random with replacement)
// - Consistent "next N" regardless of what's been visited
//
// Example:
//
//	All objective IDs: [A, B, C, D, E]
//	Team shuffle:      [C, A, E, B, D]  (deterministic per team)
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
	allObjectiveIDs []string,
	completedIDs []string,
	runCode string,
	maxNext int,
) []string {
	if len(allObjectiveIDs) == 0 {
		return []string{}
	}

	// Create deterministic seed from team code
	seed := int64(0)
	for _, c := range runCode {
		seed += int64(c)
	}

	// Shuffle ALL objective IDs (not just unvisited) to maintain consistent order
	rng := rand.New(rand.NewSource(seed))
	shuffled := make([]string, len(allObjectiveIDs))
	copy(shuffled, allObjectiveIDs)
	rng.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	// Filter shuffled list to only unvisited objective IDs
	completed := make(map[string]bool, len(completedIDs))
	for _, id := range completedIDs {
		completed[id] = true
	}
	unvisited := make([]string, 0, len(shuffled))
	for _, id := range shuffled {
		if !completed[id] {
			unvisited = append(unvisited, id)
		}
	}

	// Return up to maxNext unvisited objective IDs (in shuffled order)
	if maxNext > 0 && maxNext < len(unvisited) {
		return unvisited[:maxNext]
	}
	return unvisited
}
