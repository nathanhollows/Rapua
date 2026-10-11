package navigation_test

import (
	"math/rand"
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
	"github.com/stretchr/testify/assert"
)

// objective builds a tree row for the tests. Untouched bands mean every child
// is required, matching FillBand's nil/nil case.
func objective(id, parent string) models.Objective {
	return models.Objective{ID: id, ParentID: parent}
}

// leaf builds several children under one parent.
func leaves(parent string, ids ...string) []models.Objective {
	rows := make([]models.Objective, 0, len(ids))
	for _, id := range ids {
		rows = append(rows, objective(id, parent))
	}
	return rows
}

// The headline case: a band of one-of-two over a small branch and a large one.
// The large branch must never appear in the total, and finishing the small one
// must land the run at the end.
func TestComputeProgress_TheCutBranchNeverCounts(t *testing.T) {
	root := objective("root", "")
	root.ChildrenMin = intPtr(1)
	root.ChildrenMax = intPtr(2)
	a := objective("A", "root")
	b := objective("B", "root")
	objectives := append([]models.Objective{root, a, b}, append(
		leaves("A", "A1", "A2"), leaves("B", "B1", "B2", "B3", "B4", "B5", "B6", "B7", "B8", "B9", "B10")...,
	)...)

	start := navigation.ComputeProgress(objectives, map[string]bool{})
	assert.Equal(t, 0, start.Done)
	assert.Equal(t, 4, start.Remaining, "the small branch, not the ten")

	afterA := navigation.ComputeProgress(objectives, map[string]bool{
		"A": true, "A1": true, "A2": true,
	})
	assert.Equal(t, 3, afterA.Done)
	assert.Equal(t, 1, afterA.Remaining, "nothing left but the finish press")
	assert.Equal(t, start.Total(), afterA.Total(), "the total held while the press was owed")

	afterPress := navigation.ComputeProgress(objectives, map[string]bool{
		"root": true, "A": true, "A1": true, "A2": true,
	})
	assert.Zero(t, afterPress.Remaining)
	assert.Equal(t, afterPress.Done, afterPress.Total())
}

// An explicit minimum of zero is press-only: the children are never required.
func TestComputeProgress_ExplicitZeroBandIsPressOnly(t *testing.T) {
	root := objective("root", "")
	root.ChildrenMin = intPtr(0)
	root.ChildrenMax = intPtr(2)
	objectives := append([]models.Objective{root}, leaves("root", "C1", "C2")...)

	p := navigation.ComputeProgress(objectives, map[string]bool{})
	assert.Equal(t, 1, p.Remaining, "the press is the whole of the work")

	done := navigation.ComputeProgress(objectives, map[string]bool{"root": true})
	assert.Zero(t, done.Remaining)
}

// A ranged band met at its minimum reads one remaining until the press lands.
func TestComputeProgress_FinishableReadsOneRemaining(t *testing.T) {
	root := objective("root", "")
	root.ChildrenMin = intPtr(2)
	root.ChildrenMax = intPtr(5)
	objectives := append([]models.Objective{root}, leaves("root", "C1", "C2", "C3", "C4", "C5")...)

	p := navigation.ComputeProgress(objectives, map[string]bool{"C1": true, "C2": true})
	assert.Equal(t, 2, p.Done)
	assert.Equal(t, 1, p.Remaining)
}

// Drafted nodes are not work anyone still owes, but a draft the run finished
// before it was parked stays in Done: what a run did, it did.
func TestComputeProgress_Drafts(t *testing.T) {
	root := objective("root", "")
	kept := objective("kept", "root")
	drafted := objective("drafted", "root")
	drafted.Draft = true
	objectives := append([]models.Objective{root, kept, drafted}, append(
		leaves("kept", "K1"), leaves("drafted", "D1", "D2", "D3")...,
	)...)

	p := navigation.ComputeProgress(objectives, map[string]bool{})
	assert.Equal(t, 3, p.Remaining, "root, the one in-play child, and its leaf")

	earned := navigation.ComputeProgress(objectives, map[string]bool{
		"drafted": true, "D1": true, "D2": true, "D3": true,
	})
	assert.Equal(t, 4, earned.Done, "finished before it was parked, so it counts")
	assert.Equal(t, 3, earned.Remaining)
}

func TestComputeProgress_EmptyQuest(t *testing.T) {
	p := navigation.ComputeProgress(nil, map[string]bool{})
	assert.Zero(t, p.Done)
	assert.Zero(t, p.Total())
}

// Work landing never pushes the bar back: Remaining never rises when a
// completion is added, whatever it is, and both ends agree at the finish.
func TestComputeProgress_NeverGoesBackwards(t *testing.T) {
	root := objective("root", "")
	root.ChildrenMin = intPtr(1)
	root.ChildrenMax = intPtr(3)
	mid := objective("mid", "root")
	mid.ChildrenMin = intPtr(1)
	mid.ChildrenMax = intPtr(3)
	objectives := append([]models.Objective{root, mid}, append(
		leaves("root", "R1", "R2", "R3"), leaves("mid", "M1", "M2", "M3")...,
	)...)

	ids := make([]string, 0, len(objectives))
	for _, obj := range objectives {
		ids = append(ids, obj.ID)
	}

	complete := map[string]bool{}
	prev := navigation.ComputeProgress(objectives, complete)
	rng := rand.New(rand.NewSource(1))
	for _, i := range rng.Perm(len(ids)) {
		complete[ids[i]] = true
		next := navigation.ComputeProgress(objectives, complete)

		assert.LessOrEqual(t, next.Remaining, prev.Remaining, "remaining rose after %s", ids[i])
		if prev.Total() > 0 {
			assert.GreaterOrEqualf(t, float64(next.Done)/float64(next.Total()),
				float64(prev.Done)/float64(prev.Total()), "the bar fell after %s", ids[i])
		}
		prev = next
	}
	assert.Zero(t, prev.Remaining, "everything complete leaves nothing owed")
}
