package navigation

import (
	"sort"

	"github.com/nathanhollows/Rapua/v8/models"
)

// Progress is how much of a run is behind it and how much is still to come,
// counted in objective completions.
//
// There is no authored total to divide by: a band cuts its siblings the moment
// a choice settles, seeded windows decide which children a team meets first,
// and both make the length of a run a property of that run. Remaining is
// derived from the tree as it stands, and the pair has two properties a reader
// can lean on: the fraction never falls as work lands, and it reaches one
// exactly when the root completes.
type Progress struct {
	Done      int
	Remaining int
}

// Total is what the run's length looks like from here: everything done, plus
// the fewest completions still needed.
func (p Progress) Total() int { return p.Done + p.Remaining }

// ComputeProgress counts a run's completions and the fewest completions it
// still needs.
//
// Done counts every row given to it, drafts included: what a run did, it did.
// Remaining walks what is in play, because a parked node is not work anyone
// still owes. Routing does not enter at all: a window slide is not progress,
// and any child can come up while its parent is unfinished, so no run code and
// no shuffle belong in this signature.
func ComputeProgress(objectives []models.Objective, complete map[string]bool) Progress {
	t := newTree(InPlay(objectives))

	p := Progress{}
	for _, obj := range objectives {
		if complete[obj.ID] {
			p.Done++
		}
	}
	for _, root := range t.roots {
		p.Remaining += remaining(t, root, complete)
	}
	return p
}

// remaining is the fewest completions still owed beneath and including obj,
// assuming every open choice is taken the cheap way: the player may pick any
// children, so the honest reading of an unmade choice is its least cost.
func remaining(t tree, obj models.Objective, complete map[string]bool) int {
	if complete[obj.ID] {
		return 0
	}

	children := t.children[obj.ID]
	completed := 0
	rems := make([]int, 0, len(children))
	for _, child := range children {
		if complete[child.ID] {
			completed++
			continue
		}
		rems = append(rems, remaining(t, child, complete))
	}

	// Min, not Max, because the finish press is the cheap way out: reaching the
	// minimum offers the button, and the button completes the node. A node with
	// nothing left to count, a leaf included, needs only its own completion.
	need := bandOf(obj, len(children)).Min - completed
	if need <= 0 {
		return 1
	}

	sort.Ints(rems)
	total := 1
	for i := 0; i < need && i < len(rems); i++ {
		total += rems[i]
	}
	return total
}
