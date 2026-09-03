package navigation

import (
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
)

// Status is where one objective stands for one run. It is derived from
// completion facts every time it is asked for, never stored: two facts about
// the same run can disagree, but a derivation from them cannot.
type Status string

const (
	// StatusLocked means the run cannot reach this objective. Something above
	// it is unfinished, its parent's routing has not offered it yet, or its
	// depends are unmet.
	StatusLocked Status = "locked"
	// StatusAvailable means the run can work on this objective now.
	StatusAvailable Status = "available"
	// StatusFinishable is StatusAvailable for a section whose band has a range
	// and whose minimum is met: the player may finish it or carry on.
	StatusFinishable Status = "finishable"
	// StatusComplete means the objective is done and its branch is closed.
	StatusComplete Status = "complete"
)

// RunState is everything about one run that the frontier reads. All of it is
// recorded fact: which proof contexts have cleared, which sections the player
// chose to end, and the variables blocks have set.
type RunState struct {
	// ProofCompleted holds the objectives whose proof context has been logged
	// complete, from the append-only completion log.
	ProofCompleted map[string]bool
	// HasProofBlocks holds the objectives that have at least one proof block.
	// An objective with none has nothing to prove, so its proof is cleared
	// without any row ever being written for it.
	//
	// An objective missing from the map therefore reads as having no proof.
	// That direction is deliberate: a gap here opens a section early, where
	// reading it the other way would put a whole subtree out of reach and leave
	// the quest unfinishable.
	HasProofBlocks map[string]bool
	// SectionFinished holds the objectives whose finish button the player has
	// pressed, from the append-only section-finish log.
	SectionFinished map[string]bool
	// Vars resolves the names an objective's depends list can reference. It is
	// required wherever any objective has a depends list, and is not defaulted:
	// a resolver that answers nothing would lock those objectives silently,
	// which is harder to notice than the missing wiring itself.
	Vars game.VarResolver
	// RunCode seeds the shuffle for randomised routing, so a run sees a stable
	// order across requests.
	RunCode string
}

// Frontier is the computed state of every objective in a quest for one run.
type Frontier struct {
	// Status maps objective ID to its state.
	Status map[string]Status
	// Available lists the objectives the run can work on now, in tree order:
	// leaves, sections offering a finish button, and sections whose own proof
	// is still uncleared. A section that is merely open does not appear, since
	// its children are listed in its place.
	Available []models.Objective
}

// StatusOf returns an objective's status, or StatusLocked for one the frontier
// does not know about.
func (f Frontier) StatusOf(objectiveID string) Status {
	if status, ok := f.Status[objectiveID]; ok {
		return status
	}
	return StatusLocked
}

// tree is the quest's objectives arranged for walking, built once per
// computation. Reachability descends from the root and completion rises from
// the leaves, so both directions need to be cheap.
type tree struct {
	byID     map[string]models.Objective
	children map[string][]models.Objective
	roots    []models.Objective
	// hasParkedChildren names the objectives whose children are all out of
	// play. The pruned tree cannot tell those from a leaf, and the difference
	// matters: a leaf is cleared by being seen, while one of these still has a
	// band counting children no run can reach.
	hasParkedChildren map[string]bool
}

func newTree(objectives []models.Objective) tree {
	t := tree{
		byID:     make(map[string]models.Objective, len(objectives)),
		children: make(map[string][]models.Objective, len(objectives)),
	}
	for _, obj := range objectives {
		t.byID[obj.ID] = obj
	}
	for _, obj := range objectives {
		if obj.ParentID == "" {
			t.roots = append(t.roots, obj)
			continue
		}
		// A row naming a parent that is not here has nowhere to hang, and
		// filing it under that id would leave a key in children that no walk
		// ever visits. It is stranded either way; this keeps the map honest.
		if _, ok := t.byID[obj.ParentID]; !ok {
			continue
		}
		t.children[obj.ParentID] = append(t.children[obj.ParentID], obj)
	}
	sortByPosition(t.roots)
	for parentID := range t.children {
		sortByPosition(t.children[parentID])
	}
	return t
}

func sortByPosition(objectives []models.Objective) {
	// Insertion sort: sibling lists are short, and this keeps the comparison
	// beside the thing it orders.
	for i := 1; i < len(objectives); i++ {
		for j := i; j > 0 && less(objectives[j], objectives[j-1]); j-- {
			objectives[j], objectives[j-1] = objectives[j-1], objectives[j]
		}
	}
}

func less(a, b models.Objective) bool {
	if a.Position != b.Position {
		return a.Position < b.Position
	}
	return a.ID < b.ID
}

// parkedParents names the objectives that have children, none of them in play.
// It is derived from the full set, which is the only place the distinction
// exists: after pruning they look childless.
func parkedParents(objectives []models.Objective) map[string]bool {
	inPlay := make(map[string]bool, len(objectives))
	for _, obj := range InPlay(objectives) {
		inPlay[obj.ID] = true
	}

	hasChildren := make(map[string]bool, len(objectives))
	hasChildInPlay := make(map[string]bool, len(objectives))
	for _, obj := range objectives {
		if obj.ParentID == "" {
			continue
		}
		hasChildren[obj.ParentID] = true
		if inPlay[obj.ID] {
			hasChildInPlay[obj.ParentID] = true
		}
	}

	parked := make(map[string]bool, len(objectives))
	for parentID := range hasChildren {
		if !hasChildInPlay[parentID] {
			parked[parentID] = true
		}
	}
	return parked
}

// InPlay returns the objectives a run can reach, which is every objective that
// is not drafted and has no drafted ancestor.
//
// Drafting gates a node and everything under it without marking any of them:
// the flag stays where the author set it, so publishing restores exactly what
// was there. That makes "is this in play" a question about a node's ancestry
// rather than about its own column, and this is the only place that answers it.
func InPlay(objectives []models.Objective) []models.Objective {
	t := newTree(objectives)
	inPlay := make([]models.Objective, 0, len(objectives))

	var descend func(obj models.Objective)
	descend = func(obj models.Objective) {
		if obj.Draft {
			return
		}
		inPlay = append(inPlay, obj)
		for _, child := range t.children[obj.ID] {
			descend(child)
		}
	}
	for _, root := range t.roots {
		descend(root)
	}
	return inPlay
}

// ComputeCompleted returns the objectives that are complete for a run.
//
// It runs over every objective given to it, drafts included, because completion
// records what a run did rather than what it can still reach. An objective that
// was in play when a team cleared it stays complete once it is drafted away, so
// the gates it opened stay open.
//
// It exists separately because a depends list can name a section by slug, and a
// section completes through its band rather than through a row of its own: the
// only way to answer "is objective.<slug> true" is to derive completion first.
// Completion never reads state.Vars, so there is no circularity in computing it
// before the resolver that reachability then needs.
func ComputeCompleted(objectives []models.Objective, state RunState) map[string]bool {
	t := newTree(objectives)
	complete := make(map[string]bool, len(objectives))
	for _, root := range t.roots {
		markComplete(t, root, state, complete)
	}
	return complete
}

// ComputeFrontier derives every objective's status for one run, from the
// completion ComputeCompleted derived first.
//
// The two are separate calls because they run over different sets. Completion
// is derived over every row, drafts included, because it records what a run
// did: parking a section must not shrink an ancestor's band and complete a
// quest nobody finished. Reachability is derived over what is in play, since
// that is what a run can still act on. Deriving completion again from the
// pruned set would answer a different question with the same name.
//
// Completion rises and reachability descends, which is why the completion pass
// has to have run first: an objective is complete once its own proof has
// cleared and enough of its children are complete, while it is reachable only
// if everything above it is open.
//
// Objectives whose parent is missing, and any caught in a parent cycle, are
// simply never reached by the walk and stay locked. A document that could
// produce either is rejected at import; this tolerates one rather than
// crashing on it.
func ComputeFrontier(objectives []models.Objective, state RunState, complete map[string]bool) Frontier {
	// Pruned here rather than by the caller, because both views are needed and
	// only the full set holds the difference: after pruning, a section whose
	// children are all parked is indistinguishable from a leaf.
	t := newTree(InPlay(objectives))
	t.hasParkedChildren = parkedParents(objectives)

	frontier := Frontier{Status: make(map[string]Status, len(objectives))}
	for _, obj := range objectives {
		frontier.Status[obj.ID] = StatusLocked
	}
	for _, root := range t.roots {
		// The root has no parent to admit it and no depends above it, so it is
		// open by definition; everything below is judged against it.
		assignStatuses(t, root, true, state, complete, &frontier)
	}

	for _, root := range t.roots {
		collectAvailable(t, root, state, &frontier)
	}
	return frontier
}

// collectAvailable gathers what a player can act on, in tree order.
//
// A section that is merely open is somewhere to navigate rather than something
// to do, so it is not listed. A section whose own proof is still uncleared is
// the exception: its proof gates its children, so nothing beneath it is listed
// either, and leaving it out too would put its whole subtree beyond reach.
func collectAvailable(t tree, obj models.Objective, state RunState, frontier *Frontier) {
	children := t.children[obj.ID]
	status := frontier.StatusOf(obj.ID)

	// A node whose children are all parked is not a leaf, whatever the pruned
	// tree looks like: its band still counts them, so clearing its proof would
	// not complete it and offering it hands a player a dead end.
	leaf := len(children) == 0 && !t.hasParkedChildren[obj.ID]
	actionable := status == StatusFinishable ||
		(status == StatusAvailable && (leaf || !proofCleared(obj, state)))
	if actionable {
		frontier.Available = append(frontier.Available, obj)
	}
	for _, child := range children {
		collectAvailable(t, child, state, frontier)
	}
}

// markComplete settles an objective and everything beneath it, depth first.
func markComplete(t tree, obj models.Objective, state RunState, complete map[string]bool) {
	children := t.children[obj.ID]
	for _, child := range children {
		markComplete(t, child, state, complete)
	}

	// Proof gates the objective itself as well as its children: a section that
	// has not proved itself cannot be finished by its children finishing.
	if !proofCleared(obj, state) {
		return
	}
	// An objective with no children has no band to complete it, so its proof
	// context is the whole of its completion and the log has to say so. The
	// trivially-cleared shortcut above is about not gating children behind a
	// proof that does not exist; it is not a way to finish without being seen.
	if len(children) == 0 && !state.ProofCompleted[obj.ID] {
		return
	}

	completedChildren := 0
	for _, child := range children {
		if complete[child.ID] {
			completedChildren++
		}
	}
	if bandMet(bandOf(obj, len(children)), completedChildren, state.SectionFinished[obj.ID]) {
		complete[obj.ID] = true
	}
}

// proofCleared reports whether an objective's proof context is behind the run.
// An objective with no proof blocks has nothing to prove, so it clears without
// a row ever being written.
func proofCleared(obj models.Objective, state RunState) bool {
	if !state.HasProofBlocks[obj.ID] {
		return true
	}
	return state.ProofCompleted[obj.ID]
}

func bandOf(obj models.Objective, childCount int) game.Band {
	return game.FillBand(obj.ChildrenMin, obj.ChildrenMax, childCount)
}

// bandMet reports whether a completion band is satisfied.
//
// A band with no range auto-completes the moment enough children are done. A
// band with a range never completes on child count alone below its maximum:
// reaching the minimum only offers the finish button, and the player's press is
// what completes the objective.
func bandMet(band game.Band, completedChildren int, finished bool) bool {
	if completedChildren >= band.Max {
		return true
	}
	if band.AutoCompletes() {
		return false
	}
	return finished && completedChildren >= band.Min
}

// assignStatuses walks down from the root, giving every objective a status.
//
// open says whether the run can reach this objective. The walk continues
// through closed branches rather than stopping at them, because an objective
// that is already complete still reads as complete once the branch above it
// closes: only the unfinished ones drop to locked.
//
// An objective's children are open only while the objective itself is open,
// unfinished, and past its own proof. Completion and closure are one event.
func assignStatuses(
	t tree, obj models.Objective, open bool, state RunState, complete map[string]bool, frontier *Frontier,
) {
	switch {
	case complete[obj.ID]:
		frontier.Status[obj.ID] = StatusComplete
	case !open:
		frontier.Status[obj.ID] = StatusLocked
	default:
		frontier.Status[obj.ID] = openStatus(t, obj, state, complete)
	}

	children := t.children[obj.ID]
	childrenOpen := open && !complete[obj.ID] && proofCleared(obj, state)
	admitted := admittedChildren(obj, children, complete, state.RunCode)
	for _, child := range children {
		childOpen := childrenOpen &&
			admitted[child.ID] &&
			game.EvaluateDepends(child.Depends, state.Vars)
		assignStatuses(t, child, childOpen, state, complete, frontier)
	}
}

// openStatus reports how an unfinished objective the run has reached presents
// to it: offering a finish button, or simply there to work on.
func openStatus(t tree, obj models.Objective, state RunState, complete map[string]bool) Status {
	children := t.children[obj.ID]
	band := bandOf(obj, len(children))
	// A band with no range has no decision to offer, and a section already
	// finished is waiting on its children rather than on the player.
	if band.AutoCompletes() || state.SectionFinished[obj.ID] {
		return StatusAvailable
	}

	completedChildren := 0
	for _, child := range children {
		if complete[child.ID] {
			completedChildren++
		}
	}
	if completedChildren >= band.Min {
		return StatusFinishable
	}
	return StatusAvailable
}

// admittedChildren applies a parent's routing to decide which of its children
// the run may work on. Routing orders children; it does not gate them on
// anything outside the sibling list.
func admittedChildren(
	parent models.Objective, children []models.Objective, complete map[string]bool, runCode string,
) map[string]bool {
	admitted := make(map[string]bool, len(children))

	switch parent.Routing {
	case game.RouteStrategyOrdered:
		// Every child up to and including the first unfinished one: the run
		// works through them in the order the author wrote.
		for _, child := range children {
			admitted[child.ID] = true
			if !complete[child.ID] {
				return admitted
			}
		}

	case game.RouteStrategyRandomised:
		ids := make([]string, 0, len(children))
		completedIDs := make([]string, 0, len(children))
		for _, child := range children {
			ids = append(ids, child.ID)
			if complete[child.ID] {
				completedIDs = append(completedIDs, child.ID)
			}
		}
		// Completed children stay admitted so their status still reads as
		// complete rather than dropping back to locked.
		for _, id := range completedIDs {
			admitted[id] = true
		}
		for _, id := range deterministicShuffleIDs(ids, completedIDs, runCode, parent.MaxNext) {
			admitted[id] = true
		}

	case game.RouteStrategyFreeRoam:
		fallthrough
	default:
		// Anything unrecognised reads as free roam, including the retired
		// secret value a row written before it was retired can still carry.
		// What that value meant was keeping a group out of the listings, and
		// nothing is kept out of listings now; admitting nothing instead would
		// leave authored content no player could reach.
		for _, child := range children {
			admitted[child.ID] = true
		}
	}

	return admitted
}
