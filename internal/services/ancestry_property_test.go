package services_test

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/require"
)

// The same arbitrary shapes, against the walks that read stored rows. Both
// unbounded walks found in round 4 would hang here on the first cycle.
func TestAncestryWalks_TerminateOnAnyShape(t *testing.T) {
	for seed := range int64(300) {
		rng := rand.New(rand.NewSource(seed))
		count := 1 + rng.Intn(12)

		parentOf := make(map[string]string, count)
		ids := make([]string, count)
		for i := range count {
			ids[i] = fmt.Sprintf("row-%d", i)
		}
		for _, id := range ids {
			switch rng.Intn(4) {
			case 0:
				parentOf[id] = ""
			case 1:
				parentOf[id] = "absent"
			default:
				parentOf[id] = ids[rng.Intn(count)]
			}
		}

		for _, id := range ids {
			seen := 0
			models.WalkAncestors(id, parentOf, func(string) bool {
				seen++
				require.LessOrEqual(t, seen, count+1, "seed %d: walk from %s did not stop", seed, id)
				return true
			})
			models.HasAncestor(id, ids[rng.Intn(count)], parentOf)
		}
	}
}
