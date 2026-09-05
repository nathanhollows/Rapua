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

// frontierOf derives completion first and hands both it and the whole tree to
// the frontier, which is the order and the set the loader uses.
func frontierOf(objectives []models.Objective, state navigation.RunState) navigation.Frontier {
	complete := navigation.ComputeCompleted(objectives, state)
	return navigation.ComputeFrontier(objectives, state, complete)
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
	frontier := frontierOf(objectives, withoutProof(runState(objectives), containers...))
	assert.Equal(t, navigation.StatusLocked, frontier.StatusOf("outro"))
	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("top-a"))

	// One plant proves its category, and closes its siblings with it.
	state := withoutProof(runState(objectives, "top-a"), containers...)
	state.Vars = mapResolver{"objective.top": "done"}
	frontier = frontierOf(objectives, state)

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
	frontier = frontierOf(objectives, state)

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

			frontier := frontierOf(objectives, state)
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

	frontier := frontierOf(objectives, state)
	assert.Equal(t, []string{"section", "two", "three"}, availableSlugs(frontier))
}

// --- Proof gates children ---

// A section's own proof gates everything beneath it. That is what lets a
// chapter give real context before a player goes deeper.
func TestComputeFrontier_SectionProofGatesItsChildren(t *testing.T) {
	objectives := bandTree(nil, nil)
	state := runState(objectives)
	state.ProofCompleted["root"] = true

	frontier := frontierOf(objectives, state)
	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("section"))
	assert.Equal(t,
		[]navigation.Status{navigation.StatusLocked, navigation.StatusLocked, navigation.StatusLocked},
		statuses(frontier, "one", "two", "three"),
		"nothing below a section is reachable until its proof clears")

	state.ProofCompleted["section"] = true
	frontier = frontierOf(objectives, state)
	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("one"))
}

// A section whose proof gates its children must be reachable itself, or the
// subtree is unreachable: its children are locked behind a proof the player has
// no listed way to clear.
func TestComputeFrontier_SectionAwaitingItsOwnProofIsListed(t *testing.T) {
	objectives := bandTree(nil, nil)
	state := runState(objectives)
	state.ProofCompleted["root"] = true

	frontier := frontierOf(objectives, state)
	assert.Equal(t, []string{"section"}, availableSlugs(frontier),
		"the section is the only thing the player can act on")

	// Once its proof clears it becomes navigation rather than an action, and
	// its children take its place in the list.
	state.ProofCompleted["section"] = true
	frontier = frontierOf(objectives, state)
	assert.Equal(t, []string{"one", "two", "three"}, availableSlugs(frontier))
}

// A section with nothing to prove passes straight through to its content: its
// reveal is the intro card, and no row is ever written for it.
func TestComputeFrontier_SectionWithoutProofPassesThrough(t *testing.T) {
	objectives := bandTree(nil, nil)
	state := runState(objectives)
	state.ProofCompleted["root"] = true
	state.HasProofBlocks["section"] = false

	frontier := frontierOf(objectives, state)
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

	frontier := frontierOf(objectives, state)
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

	frontier := frontierOf(objectives, runState(objectives, "root"))
	assert.Equal(t,
		[]navigation.Status{navigation.StatusAvailable, navigation.StatusLocked, navigation.StatusLocked},
		statuses(frontier, "one", "two", "three"))

	frontier = frontierOf(objectives, runState(objectives, "root", "one"))
	assert.Equal(t,
		[]navigation.Status{navigation.StatusComplete, navigation.StatusAvailable, navigation.StatusLocked},
		statuses(frontier, "one", "two", "three"))
}

func TestComputeFrontier_FreeRoamOffersEveryChild(t *testing.T) {
	objectives := bandTree(nil, nil)
	state := runState(objectives, "root", "section")

	frontier := frontierOf(objectives, state)
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

	first := availableSlugs(frontierOf(objectives, state))
	second := availableSlugs(frontierOf(objectives, state))
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
		frontierOf(objectives, state).StatusOf("locked"))

	state.Vars = mapResolver{"found_key": "true"}
	assert.Equal(t, navigation.StatusAvailable,
		frontierOf(objectives, state).StatusOf("locked"))
}

// A negated depends is met until the thing it names happens.
func TestComputeFrontier_NegatedDependsClosesOnceMet(t *testing.T) {
	shortcut := node("shortcut", "root", 1)
	shortcut.Depends = game.DependsField{"not took_long_way"}
	objectives := []models.Objective{node("root", "", 0), node("open", "root", 0), shortcut}

	state := runState(objectives, "root")
	assert.Equal(t, navigation.StatusAvailable,
		frontierOf(objectives, state).StatusOf("shortcut"))

	state.Vars = mapResolver{"took_long_way": "true"}
	assert.Equal(t, navigation.StatusLocked,
		frontierOf(objectives, state).StatusOf("shortcut"))
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
		frontier = frontierOf(objectives, runState(objectives, "root"))
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

	frontier := frontierOf(objectives, runState(objectives, "root"))
	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("open"))
	assert.Equal(t, navigation.StatusLocked, frontier.StatusOf("stray"))
}

func TestComputeFrontier_EmptyQuest(t *testing.T) {
	frontier := frontierOf(nil, runState(nil))
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

	frontier := frontierOf(objectives, state)

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
	frontier := frontierOf(objectives, runState(objectives))
	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("root"))

	// Everything below done: the root completes with its only child.
	state := runState(objectives, "root", "section", "one", "two", "three")
	frontier = frontierOf(objectives, state)
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

	frontier := frontierOf(objectives, runState(objectives, "root"))
	assert.Equal(t, navigation.StatusFinishable, frontier.StatusOf("root"))
}

// Drafting gates a node and everything under it without marking any of them,
// so what is in play is a question about ancestry rather than about a column.
func TestInPlay(t *testing.T) {
	objectives := []models.Objective{
		node("root", "", 0),
		node("live", "root", 0),
		node("parked", "root", 1),
		node("buried", "parked", 0),
		node("deeper", "buried", 0),
	}
	objectives[2].Draft = true

	slugs := []string{}
	for _, obj := range navigation.InPlay(objectives) {
		slugs = append(slugs, obj.Slug)
	}
	assert.Equal(t, []string{"root", "live"}, slugs,
		"the drafted section and everything under it drops out, however deep")

	// Nothing beneath it was marked, so publishing restores the lot.
	objectives[2].Draft = false
	slugs = nil
	for _, obj := range navigation.InPlay(objectives) {
		slugs = append(slugs, obj.Slug)
	}
	assert.ElementsMatch(t, []string{"root", "live", "parked", "buried", "deeper"}, slugs)
}

// Parking a child removes a requirement without manufacturing completion. The
// band counts what is in play on both sides, so the section still needs
// whatever is left, and only finishes when a run has done it.
//
// This reverses the mid-run protection that once lived here. It had to: with
// the parked child still counted, an author who parked the last piece of
// content left the section unfinishable and every run behind it dead-ended,
// with nothing said at edit time. What now stops a run being surprised is that
// a running game cannot be edited at all.
func TestComputeFrontier_ParkingRemovesARequirement(t *testing.T) {
	objectives := []models.Objective{
		node("root", "", 0),
		node("one", "root", 0),
		node("two", "root", 1),
	}
	state := runState(objectives, "root")

	require.NotEqual(t, navigation.StatusComplete, frontierOf(objectives, state).StatusOf("root"),
		"neither child is done")

	objectives[2].Draft = true
	assert.NotEqual(t, navigation.StatusComplete, frontierOf(objectives, state).StatusOf("root"),
		"parking one does not finish the other")

	state = runState(objectives, "root", "one")
	assert.Equal(t, navigation.StatusComplete, frontierOf(objectives, state).StatusOf("root"),
		"but once the child still in play is done, nothing is left to require")
}

// The other direction of the same rule: what a run cleared before a section was
// parked still counts, so the gates it opened stay open.
func TestComputeFrontier_CompletionOutlivesDrafting(t *testing.T) {
	objectives := []models.Objective{
		node("root", "", 0),
		node("gateway", "root", 0),
		node("gated", "root", 1),
	}
	objectives[2].Depends = game.DependsField{"objective.gateway"}

	state := runState(objectives, "root", "gateway")
	state.Vars = mapResolver{"objective.gateway": "done"}
	require.Equal(t, navigation.StatusAvailable, frontierOf(objectives, state).StatusOf("gated"))

	objectives[1].Draft = true
	frontier := frontierOf(objectives, state)
	assert.Equal(t, navigation.StatusLocked, frontier.StatusOf("gateway"), "the parked one is gone")
	assert.Equal(t, navigation.StatusAvailable, frontier.StatusOf("gated"),
		"what it unlocked stays unlocked")
}

// A section whose children are all parked has nothing in play below it, so its
// own proof is the whole of its completion: it is a leaf, and is offered and
// cleared like one. Lint says the same about a document (ALL_CHILDREN_DRAFT).
func TestComputeFrontier_SectionWithOnlyParkedChildrenIsALeaf(t *testing.T) {
	objectives := []models.Objective{
		node("root", "", 0),
		node("section", "root", 0),
		node("one", "section", 0),
	}
	require.Equal(t, []string{"one"},
		availableSlugs(frontierOf(objectives, runState(objectives, "root", "section"))))

	objectives[2].Draft = true
	assert.Equal(t, []string{"section"},
		availableSlugs(frontierOf(objectives, runState(objectives, "root"))),
		"with nothing below it in play, the section is the thing to do")

	// Clearing its own proof completes it, where before it was offered nothing
	// and the quest could not be finished at all.
	frontier := frontierOf(objectives, runState(objectives, "root", "section"))
	assert.Equal(t, navigation.StatusComplete, frontier.StatusOf("section"))
	assert.Equal(t, navigation.StatusComplete, frontier.StatusOf("root"))
}

// A section with content of its own is somewhere to go before it is a heading
// over its children. Without this a chapter introduction is authored and never
// read, because the children are listed in its place from the start.
func TestComputeFrontier_SectionWithUnseenContentIsOffered(t *testing.T) {
	objectives := []models.Objective{
		node("root", "", 0),
		node("heart", "root", 0),
		node("rose", "heart", 0),
		node("jasmine", "heart", 1),
	}

	// No gate: the section's proof is empty, so its children are reachable
	// straight away. It has a card to show, though.
	state := runState(objectives)
	state.HasProofBlocks["root"] = false
	state.HasProofBlocks["heart"] = false
	state.HasRevealBlocks = map[string]bool{"heart": true}
	state.RevealSeen = map[string]bool{}

	assert.Equal(t, []string{"heart"}, availableSlugs(frontierOf(objectives, state)),
		"the section is offered while it still has something to say")

	state.RevealSeen["heart"] = true
	assert.Equal(t, []string{"rose", "jasmine"}, availableSlugs(frontierOf(objectives, state)),
		"and steps back to being a heading once it has said it")
}

// A section with nothing of its own stays navigation, so its children are
// listed in its place from the start.
func TestComputeFrontier_SectionWithoutContentStaysAHeading(t *testing.T) {
	objectives := []models.Objective{
		node("root", "", 0),
		node("heart", "root", 0),
		node("rose", "heart", 0),
	}
	state := runState(objectives)
	state.HasProofBlocks["root"] = false
	state.HasProofBlocks["heart"] = false

	assert.Equal(t, []string{"rose"}, availableSlugs(frontierOf(objectives, state)))
}

// The band is the reason ErrParkingBreaksBand exists, so the runtime has to
// agree with what the toggle and lint promise: a section with no explicit
// bounds needs whatever is left in play, not everything ever authored.
func TestComputeFrontier_OmittedBandNeedsWhatRemains(t *testing.T) {
	objectives := []models.Objective{
		node("root", "", 0),
		node("section", "root", 0),
		node("one", "section", 0),
		node("two", "section", 1),
	}
	objectives[3].Draft = true

	// One is the only child left in play, and it is done.
	state := runState(objectives, "root", "section", "one")
	frontier := frontierOf(objectives, state)

	assert.Equal(t, navigation.StatusComplete, frontier.StatusOf("section"),
		"the parked child cannot be required: no run can ever complete it")
	assert.Equal(t, navigation.StatusComplete, frontier.StatusOf("root"))
	assert.Empty(t, availableSlugs(frontier), "and the quest is finished rather than dead-ended")
}
