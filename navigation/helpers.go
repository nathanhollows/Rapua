package navigation

import (
	"crypto/sha256"
	"encoding/binary"
	"math/rand/v2"
)

// shuffleSeed derives the generator's seed from a run and the container being
// shuffled. Both go in because the run alone would give every container of the
// same size the same order, so a team's pick from one pool would predict its
// pick from the next. The separator keeps "ab"+"c" from colliding with "a"+"bc".
func shuffleSeed(runCode, parentID string) (uint64, uint64) {
	sum := sha256.Sum256([]byte(runCode + "\x00" + parentID))
	return binary.BigEndian.Uint64(sum[:8]), binary.BigEndian.Uint64(sum[8:16])
}

// deterministicShuffleIDs shuffles ALL objective IDs deterministically based on
// the team code and the parent they belong to, then filters to unvisited, then
// returns up to maxNext.
//
// This ensures:
// - Same random order for a team across all requests
// - A different order in each container, for the same team
// - Order doesn't change as objectives are completed (random with replacement)
// - Consistent "next N" regardless of what's been visited
//
// The order is a function of the seed and of the order the pool arrives in, so
// a caller must build it the same way each time. The tree query orders by
// position, and a pool gathered from a map would not.
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
	parentID string,
	maxNext int,
) []string {
	if len(allObjectiveIDs) == 0 {
		return []string{}
	}

	// Determinism is the point: a team must see the same order on every request.
	rng := rand.New(rand.NewPCG(shuffleSeed(runCode, parentID))) //nolint:gosec // Not cryptographic.
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
