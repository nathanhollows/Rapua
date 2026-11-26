package navigation_test

import (
	"fmt"
	"testing"

	"github.com/nathanhollows/Rapua/v8/navigation"
	"github.com/stretchr/testify/assert"
)

func poolIDs(prefix string, n int) []string {
	ids := make([]string, n)
	for i := range ids {
		ids[i] = fmt.Sprintf("%s-%d", prefix, i)
	}
	return ids
}

func TestDeterministicShuffleIDs_RepeatsForSameRunAndParent(t *testing.T) {
	ids := poolIDs("bird", 8)
	first := navigation.DeterministicShuffleIDs(ids, nil, "ABCDE", "birds", 0)
	for range 5 {
		assert.Equal(t, first, navigation.DeterministicShuffleIDs(ids, nil, "ABCDE", "birds", 0))
	}
}

func TestDeterministicShuffleIDs_OrderOfRunCodeMatters(t *testing.T) {
	ids := poolIDs("bird", 8)
	assert.NotEqual(t,
		navigation.DeterministicShuffleIDs(ids, nil, "ABCDE", "birds", 0),
		navigation.DeterministicShuffleIDs(ids, nil, "EDCBA", "birds", 0),
		"an anagram of a run code is a different run")
}

func TestDeterministicShuffleIDs_ContainersDoNotShareAnOrder(t *testing.T) {
	// Pools of one size must not pick the same position for one team, or a
	// team's bird would predict its fish.
	positions := map[int]bool{}
	for _, parent := range []string{"birds", "fish", "mammals", "reptiles", "insects"} {
		ids := poolIDs("x", 10)
		first := navigation.DeterministicShuffleIDs(ids, nil, "ABCDE", parent, 1)[0]
		for i, id := range ids {
			if id == first {
				positions[i] = true
			}
		}
	}
	assert.Greater(t, len(positions), 1, "five containers all picked the same position")
}

// The seed used to be the sum of the run code's bytes, so codes that are
// anagrams of one another shared an order. Every code below sums to the same
// value, so this fails against that seed and only passes when the whole code is
// part of the hash.
func TestDeterministicShuffleIDs_AnagramCodesDoNotShareAnOrder(t *testing.T) {
	ids := poolIDs("bird", 10)
	firsts := map[string]bool{}
	for _, code := range []string{"AAAAB", "AAABA", "AABAA", "ABAAA", "BAAAA"} {
		firsts[navigation.DeterministicShuffleIDs(ids, nil, code, "birds", 1)[0]] = true
	}
	assert.Greater(t, len(firsts), 1, "five anagram codes all drew the same first bird")
}

func TestDeterministicShuffleIDs_SeparatorKeepsFieldsApart(t *testing.T) {
	ids := poolIDs("bird", 10)
	assert.NotEqual(t,
		navigation.DeterministicShuffleIDs(ids, nil, "ab", "c", 0),
		navigation.DeterministicShuffleIDs(ids, nil, "a", "bc", 0))
}

func TestDeterministicShuffleIDs_CompletionDoesNotReshuffle(t *testing.T) {
	ids := poolIDs("bird", 6)
	full := navigation.DeterministicShuffleIDs(ids, nil, "ABCDE", "birds", 0)
	rest := navigation.DeterministicShuffleIDs(ids, []string{full[0]}, "ABCDE", "birds", 0)
	assert.Equal(t, full[1:], rest)
}
