package navigation_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mapResolver resolves depends names from a plain map.
type mapResolver map[string]string

func (m mapResolver) ResolveVar(name string) (string, bool) {
	v, ok := m[name]
	return v, ok
}

// node builds one objective. Tests name objectives by slug and use the slug as
// the id too, so assertions read as the tree does.
func node(slug, parentID string, position int) models.Objective {
	return models.Objective{
		ID: slug, QuestID: "quest", ParentID: parentID, Position: position,
		Slug: slug, Title: slug, Routing: game.RouteStrategyFreeRoam,
	}
}

func intPtr(n int) *int { return &n }

// runState builds a state where every objective has a proof to clear, and the
// named ones have cleared it. That is the ordinary case: an objective with
// nothing to prove is the exception a few tests set up deliberately.
func runState(all []models.Objective, proofCompleted ...string) navigation.RunState {
	state := navigation.RunState{
		ProofCompleted:  map[string]bool{},
		HasProofBlocks:  map[string]bool{},
		SectionFinished: map[string]bool{},
		Vars:            mapResolver{},
		RunCode:         "RUN1",
	}
	for _, obj := range all {
		state.HasProofBlocks[obj.ID] = true
	}
	for _, id := range proofCompleted {
		state.ProofCompleted[id] = true
	}
	return state
}

// withoutProof marks objectives as pure containers: nothing to prove, so their
// children are reachable without any row being written for them.
func withoutProof(state navigation.RunState, ids ...string) navigation.RunState {
	for _, id := range ids {
		state.HasProofBlocks[id] = false
	}
	return state
}

func statuses(f navigation.Frontier, ids ...string) []navigation.Status {
	out := make([]navigation.Status, len(ids))
	for i, id := range ids {
		out[i] = f.StatusOf(id)
	}
	return out
}

func availableSlugs(f navigation.Frontier) []string {
	slugs := make([]string, len(f.Available))
	for i, obj := range f.Available {
		slugs[i] = obj.Slug
	}
	return slugs
}

// --- The perfumers shape ---
//
// intro, then three categories each requiring exactly one of their three
// plants, then an outro gated on all three categories. This is the shape the
// whole design exists to express: the unwritable AND-of-ORs dissolves because
// OR lives in the tree (a min=max=1 parent) and AND is the depends list.
func perfumers() []models.Objective {
	objectives := []models.Objective{
		node("root", "", 0),
		node("intro", "root", 0),
	}

	for i, category := range []string{"top", "heart", "base"} {
		section := node(category, "root", i+1)
		section.ChildrenMin = intPtr(1)
		section.ChildrenMax = intPtr(1)
		objectives = append(objectives, section)
		for j, plant := range []string{"a", "b", "c"} {
			objectives = append(objectives, node(category+"-"+plant, category, j))
		}
	}

	outro := node("outro", "root", 4)
	outro.Depends = game.DependsField{"objective.top", "objective.heart", "objective.base"}
	return append(objectives, outro)
}

func TestComputeFrontier_Perfumers_OutroWaitsForEveryCategory(t *testing.T) {
	objectives := perfumers()

	containers := []string{"root", "top", "heart", "base"}

	// Nothing done yet: every category is open, the outro is not.
	frontier := navigation.ComputeFrontier(objectives, withoutProof(runState(objectives), containers...))
	assert.Equal(t, navigation.StatusLocked, frontier.StatusOf("outro"))
	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("top-a"))

	// One plant proves its category, and closes its siblings with it.
	state := withoutProof(runState(objectives, "top-a"), containers...)
	state.Vars = mapResolver{"objective.top": "done"}
	frontier = navigation.ComputeFrontier(objectives, state)

	assert.Equal(t, navigation.StatusComplete, frontier.StatusOf("top"),
		"one plant of three completes a min=max=1 category")
	assert.Equal(t,
		[]navigation.Status{navigation.StatusLocked, navigation.StatusLocked},
		statuses(frontier, "top-b", "top-c"),
		"completion closes the branch in the same step")
	assert.Equal(t, navigation.StatusLocked, frontier.StatusOf("outro"),
		"two categories still outstanding")

	// All three categories proved.
	state = withoutProof(runState(objectives, "top-a", "heart-b", "base-c"), containers...)
	state.Vars = mapResolver{
		"objective.top": "done", "objective.heart": "done", "objective.base": "done",
	}
	frontier = navigation.ComputeFrontier(objectives, state)

	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("outro"),
		"the depends list is the AND the tree cannot express")
	assert.Equal(t, []string{"intro", "outro"}, availableSlugs(frontier),
		"closed categories leave the frontier entirely")
}

// --- The completion band ---

// bandTree is one section of three children, banded as the test wants.
func bandTree(minChildren, maxChildren *int) []models.Objective {
	section := node("section", "root", 0)
	section.ChildrenMin = minChildren
	section.ChildrenMax = maxChildren
	return []models.Objective{
		node("root", "", 0), section,
		node("one", "section", 0), node("two", "section", 1), node("three", "section", 2),
	}
}

func TestComputeFrontier_Band(t *testing.T) {
	tests := []struct {
		name            string
		minChildren     *int
		maxChildren     *int
		doneChildren    []string
		sectionFinished bool
		want            navigation.Status
	}{
		{
			name:         "no band requires every child",
			doneChildren: []string{"one", "two"}, want: navigation.StatusAvailable,
		},
		{
			name:         "no band completes once every child is done",
			doneChildren: []string{"one", "two", "three"}, want: navigation.StatusComplete,
		},
		{
			name:        "min equal to max auto-completes at that count",
			minChildren: intPtr(2), maxChildren: intPtr(2),
			doneChildren: []string{"one", "two"}, want: navigation.StatusComplete,
		},
		{
			name:        "a range does not complete at its minimum",
			minChildren: intPtr(1), maxChildren: intPtr(3),
			doneChildren: []string{"one"}, want: navigation.StatusFinishable,
		},
		{
			name:        "a range completes when the player presses finish",
			minChildren: intPtr(1), maxChildren: intPtr(3),
			doneChildren: []string{"one"}, sectionFinished: true, want: navigation.StatusComplete,
		},
		{
			name:        "a range completes on its own at its maximum",
			minChildren: intPtr(1), maxChildren: intPtr(3),
			doneChildren: []string{"one", "two", "three"}, want: navigation.StatusComplete,
		},
		{
			name:         "min only offers the button and waits",
			minChildren:  intPtr(1),
			doneChildren: []string{"one", "two"}, want: navigation.StatusFinishable,
		},
		{
			name:         "min only completes by exhausting every child",
			minChildren:  intPtr(1),
			doneChildren: []string{"one", "two", "three"}, want: navigation.StatusComplete,
		},
		{
			name:        "max only offers the button from the start",
			maxChildren: intPtr(2),
			want:        navigation.StatusFinishable,
		},
		{
			name:         "max only auto-completes at its cap",
			maxChildren:  intPtr(2),
			doneChildren: []string{"one", "two"}, want: navigation.StatusComplete,
		},
		{
			name:        "an explicit zero minimum is open from the start",
			minChildren: intPtr(0),
			want:        navigation.StatusFinishable,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			objectives := bandTree(tt.minChildren, tt.maxChildren)
			state := runState(objectives, tt.doneChildren...)
			state.ProofCompleted["root"] = true
			state.ProofCompleted["section"] = true
			state.SectionFinished["section"] = tt.sectionFinished

			frontier := navigation.ComputeFrontier(objectives, state)
			assert.Equal(t, tt.want, frontier.StatusOf("section"))
		})
	}
}

// A section offering its button is something the player can act on, so it is
// listed; one that is merely open is somewhere to navigate to.
func TestComputeFrontier_AvailableListsLeavesAndFinishableSections(t *testing.T) {
	objectives := bandTree(intPtr(1), intPtr(3))
	state := runState(objectives, "one")
	state.ProofCompleted["root"] = true
	state.ProofCompleted["section"] = true

	frontier := navigation.ComputeFrontier(objectives, state)
	assert.Equal(t, []string{"section", "two", "three"}, availableSlugs(frontier))
}

// --- Proof gates children ---

// A section's own proof gates everything beneath it. That is what lets a
// chapter give real context before a player goes deeper.
func TestComputeFrontier_SectionProofGatesItsChildren(t *testing.T) {
	objectives := bandTree(nil, nil)
	state := runState(objectives)
	state.ProofCompleted["root"] = true

	frontier := navigation.ComputeFrontier(objectives, state)
	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("section"))
	assert.Equal(t,
		[]navigation.Status{navigation.StatusLocked, navigation.StatusLocked, navigation.StatusLocked},
		statuses(frontier, "one", "two", "three"),
		"nothing below a section is reachable until its proof clears")

	state.ProofCompleted["section"] = true
	frontier = navigation.ComputeFrontier(objectives, state)
	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("one"))
}

// A section with nothing to prove passes straight through to its content: its
// reveal is the intro card, and no row is ever written for it.
func TestComputeFrontier_SectionWithoutProofPassesThrough(t *testing.T) {
	objectives := bandTree(nil, nil)
	state := runState(objectives)
	state.ProofCompleted["root"] = true
	state.HasProofBlocks["section"] = false

	frontier := navigation.ComputeFrontier(objectives, state)
	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("one"))
}

// A leaf has no band beneath it, so its proof context is the whole of its
// completion: having nothing to prove is not the same as being finished.
func TestComputeFrontier_LeafWithoutProofIsNotCompleteUnseen(t *testing.T) {
	objectives := bandTree(nil, nil)
	state := runState(objectives)
	state.ProofCompleted["root"] = true
	state.ProofCompleted["section"] = true
	state.HasProofBlocks["one"] = false

	frontier := navigation.ComputeFrontier(objectives, state)
	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("one"))
}

// --- Routing ---

func orderedTree() []models.Objective {
	root := node("root", "", 0)
	root.Routing = game.RouteStrategyOrdered
	return []models.Objective{
		root, node("one", "root", 0), node("two", "root", 1), node("three", "root", 2),
	}
}

func TestComputeFrontier_OrderedRoutingOffersOneAtATime(t *testing.T) {
	objectives := orderedTree()

	frontier := navigation.ComputeFrontier(objectives, runState(objectives, "root"))
	assert.Equal(t,
		[]navigation.Status{navigation.StatusAvailable, navigation.StatusLocked, navigation.StatusLocked},
		statuses(frontier, "one", "two", "three"))

	frontier = navigation.ComputeFrontier(objectives, runState(objectives, "root", "one"))
	assert.Equal(t,
		[]navigation.Status{navigation.StatusComplete, navigation.StatusAvailable, navigation.StatusLocked},
		statuses(frontier, "one", "two", "three"))
}

func TestComputeFrontier_FreeRoamOffersEveryChild(t *testing.T) {
	objectives := bandTree(nil, nil)
	state := runState(objectives, "root", "section")

	frontier := navigation.ComputeFrontier(objectives, state)
	assert.Equal(t,
		[]navigation.Status{navigation.StatusAvailable, navigation.StatusAvailable, navigation.StatusAvailable},
		statuses(frontier, "one", "two", "three"))
}

// A randomised section offers a window of its children, and the window is
// stable for a run: the same request twice must not reshuffle it.
func TestComputeFrontier_RandomisedRoutingWindowIsStable(t *testing.T) {
	objectives := bandTree(nil, nil)
	for i := range objectives {
		if objectives[i].ID == "section" {
			objectives[i].Routing = game.RouteStrategyRandomised
			objectives[i].MaxNext = 2
		}
	}
	state := runState(objectives, "root", "section")

	first := availableSlugs(navigation.ComputeFrontier(objectives, state))
	second := availableSlugs(navigation.ComputeFrontier(objectives, state))
	assert.Len(t, first, 2, "max_next caps the window")
	assert.Equal(t, first, second, "the window must not move between requests")
}

// --- Depends ---

func TestComputeFrontier_DependsGatesAnObjective(t *testing.T) {
	locked := node("locked", "root", 1)
	locked.Depends = game.DependsField{"found_key"}
	objectives := []models.Objective{node("root", "", 0), node("open", "root", 0), locked}

	state := runState(objectives, "root")
	assert.Equal(t, navigation.StatusLocked,
		navigation.ComputeFrontier(objectives, state).StatusOf("locked"))

	state.Vars = mapResolver{"found_key": "true"}
	assert.Equal(t, navigation.StatusAvailable,
		navigation.ComputeFrontier(objectives, state).StatusOf("locked"))
}

// A negated depends is met until the thing it names happens.
func TestComputeFrontier_NegatedDependsClosesOnceMet(t *testing.T) {
	shortcut := node("shortcut", "root", 1)
	shortcut.Depends = game.DependsField{"not took_long_way"}
	objectives := []models.Objective{node("root", "", 0), node("open", "root", 0), shortcut}

	state := runState(objectives, "root")
	assert.Equal(t, navigation.StatusAvailable,
		navigation.ComputeFrontier(objectives, state).StatusOf("shortcut"))

	state.Vars = mapResolver{"took_long_way": "true"}
	assert.Equal(t, navigation.StatusLocked,
		navigation.ComputeFrontier(objectives, state).StatusOf("shortcut"))
}

// --- Damaged trees ---

// A cycle is rejected at import, but the engine must tolerate one rather than
// run away on it: the rows are simply never reached.
func TestComputeFrontier_ToleratesACycle(t *testing.T) {
	objectives := []models.Objective{
		node("root", "", 0), node("open", "root", 0),
		node("a", "b", 0), node("b", "a", 0),
	}

	var frontier navigation.Frontier
	require.NotPanics(t, func() {
		frontier = navigation.ComputeFrontier(objectives, runState(objectives, "root"))
	})
	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("open"))
	assert.Equal(t,
		[]navigation.Status{navigation.StatusLocked, navigation.StatusLocked},
		statuses(frontier, "a", "b"))
}

func TestComputeFrontier_ToleratesAMissingParent(t *testing.T) {
	objectives := []models.Objective{
		node("root", "", 0), node("open", "root", 0), node("stray", "vanished", 0),
	}

	frontier := navigation.ComputeFrontier(objectives, runState(objectives, "root"))
	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("open"))
	assert.Equal(t, navigation.StatusLocked, frontier.StatusOf("stray"))
}

func TestComputeFrontier_EmptyQuest(t *testing.T) {
	frontier := navigation.ComputeFrontier(nil, runState(nil))
	assert.Empty(t, frontier.Available)
	assert.Equal(t, navigation.StatusLocked, frontier.StatusOf("anything"))
}

// Closing a branch must not rewrite the history under it. An objective already
// finished still reads as complete once its section closes: only the
// unfinished siblings drop out of reach.
func TestComputeFrontier_ClosedBranchKeepsItsCompletions(t *testing.T) {
	objectives := bandTree(intPtr(1), intPtr(1))
	state := runState(objectives, "one")
	state.ProofCompleted["root"] = true
	state.ProofCompleted["section"] = true

	frontier := navigation.ComputeFrontier(objectives, state)

	require.Equal(t, navigation.StatusComplete, frontier.StatusOf("section"),
		"one of three completes a min=max=1 section")
	assert.Equal(t, navigation.StatusComplete, frontier.StatusOf("one"),
		"the objective the player finished stays finished")
	assert.Equal(t,
		[]navigation.Status{navigation.StatusLocked, navigation.StatusLocked},
		statuses(frontier, "two", "three"),
		"only the unfinished siblings leave the frontier")
}

// The root is an objective like any other and gets a status like any other.
// Nothing else in this file reads it, so without this the walk could stop
// assigning it and every other assertion would still pass.
func TestComputeFrontier_RootGetsAStatus(t *testing.T) {
	objectives := bandTree(nil, nil)

	// Its own proof outstanding: open, and nothing below it is reachable.
	frontier := navigation.ComputeFrontier(objectives, runState(objectives))
	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("root"))

	// Everything below done: the root completes with its only child.
	state := runState(objectives, "root", "section", "one", "two", "three")
	frontier = navigation.ComputeFrontier(objectives, state)
	assert.Equal(t, navigation.StatusComplete, frontier.StatusOf("root"))
}

// A root offering a finish button is the whole quest asking to be ended, and it
// reads the same as any other banded section.
func TestComputeFrontier_RootCanBeFinishable(t *testing.T) {
	objectives := bandTree(nil, nil)
	for i := range objectives {
		if objectives[i].ID == "root" {
			objectives[i].ChildrenMin = intPtr(0)
		}
	}

	frontier := navigation.ComputeFrontier(objectives, runState(objectives, "root"))
	assert.Equal(t, navigation.StatusFinishable, frontier.StatusOf("root"))
}
