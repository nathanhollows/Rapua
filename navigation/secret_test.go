package navigation_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
	"github.com/stretchr/testify/assert"
)

func secret(slug, parentID string, position int) models.Objective {
	obj := node(slug, parentID, position)
	obj.Secret = true
	return obj
}

// secretTree is a section of two visible children and one secret, the secret
// written first so any routing that counted it would show.
func secretTree(routing game.RouteStrategy, minChildren, maxChildren *int) []models.Objective {
	section := node("section", "root", 0)
	section.Routing = routing
	section.ChildrenMin = minChildren
	section.ChildrenMax = maxChildren
	return []models.Objective{
		node("root", "", 0), section,
		secret("bonus", "section", 0), node("one", "section", 1), node("two", "section", 2),
	}
}

func withDiscovered(state navigation.RunState, ids ...string) navigation.RunState {
	state.Discovered = map[string]bool{}
	for _, id := range ids {
		state.Discovered[id] = true
	}
	return state
}

func TestSecret_HiddenWhileItsParentIsOpen(t *testing.T) {
	objectives := secretTree(game.RouteStrategyFreeRoam, nil, nil)
	f := frontierOf(objectives, withDiscovered(runState(objectives, "root", "section")))

	assert.Equal(t, navigation.StatusHidden, f.StatusOf("bonus"))
	assert.Equal(t, []string{"one", "two"}, availableSlugs(f), "never listed until found")
}

func TestSecret_OutsideOrderedRouting(t *testing.T) {
	objectives := secretTree(game.RouteStrategyOrdered, nil, nil)
	f := frontierOf(objectives, withDiscovered(runState(objectives, "root", "section")))

	assert.Equal(t, []navigation.Status{navigation.StatusHidden, navigation.StatusAvailable, navigation.StatusLocked},
		statuses(f, "bonus", "one", "two"), "the secret neither blocks nor takes the first turn")
}

func TestSecret_OutsideTheRandomisedWindow(t *testing.T) {
	objectives := secretTree(game.RouteStrategyRandomised, nil, nil)
	objectives[1].MaxNext = 1
	f := frontierOf(objectives, withDiscovered(runState(objectives, "root", "section")))

	assert.Equal(t, navigation.StatusHidden, f.StatusOf("bonus"))
	assert.Len(t, availableSlugs(f), 1, "the window of one holds a visible child")
}

func TestSecret_ExcludedOnceItsParentIsDone(t *testing.T) {
	objectives := secretTree(game.RouteStrategyFreeRoam, nil, nil)
	f := frontierOf(objectives, withDiscovered(runState(objectives, "root", "section", "one", "two")))

	assert.Equal(t, navigation.StatusComplete, f.StatusOf("section"), "an unfound secret is never required")
	assert.Equal(t, navigation.StatusLocked, f.StatusOf("bonus"))
}

func TestSecret_CompleteAllRequiresWhatWasFound(t *testing.T) {
	objectives := secretTree(game.RouteStrategyFreeRoam, nil, nil)
	state := withDiscovered(runState(objectives, "root", "section", "one", "two"), "bonus")
	f := frontierOf(objectives, state)

	assert.Equal(t, navigation.StatusAvailable, f.StatusOf("section"), "the find joined the set")
	assert.Equal(t, navigation.StatusAvailable, f.StatusOf("bonus"))
	assert.Contains(t, availableSlugs(f), "bonus", "found but unfinished, so it is listed")

	state.ProofCompleted["bonus"] = true
	assert.Equal(t, navigation.StatusComplete, frontierOf(objectives, state).StatusOf("section"))
}

func TestSecret_CountsTowardAMinimum(t *testing.T) {
	objectives := secretTree(game.RouteStrategyFreeRoam, intPtr(2), nil)
	state := withDiscovered(runState(objectives, "root", "section", "one", "bonus"), "bonus")

	assert.Equal(t, navigation.StatusFinishable, frontierOf(objectives, state).StatusOf("section"))
}

func TestSecret_FindReachingAnAutoBoundCompletes(t *testing.T) {
	objectives := secretTree(game.RouteStrategyFreeRoam, intPtr(2), intPtr(2))
	before := runState(objectives, "root", "section", "one")
	assert.Equal(t, navigation.StatusAvailable, frontierOf(objectives, withDiscovered(before)).StatusOf("section"))

	after := withDiscovered(runState(objectives, "root", "section", "one", "bonus"), "bonus")
	assert.Equal(t, navigation.StatusComplete, frontierOf(objectives, after).StatusOf("section"))
}

func sideQuestTree() []models.Objective {
	side := secret("side", "root", 1)
	side.Routing = game.RouteStrategyOrdered
	return []models.Objective{
		node("root", "", 0), node("main", "root", 0), side,
		node("s1", "side", 0), node("s2", "side", 1),
	}
}

func TestSecret_SideQuestOpensOnceFound(t *testing.T) {
	objectives := sideQuestTree()

	hidden := frontierOf(objectives, withDiscovered(runState(objectives, "root")))
	assert.Equal(t, []navigation.Status{navigation.StatusHidden, navigation.StatusLocked, navigation.StatusLocked},
		statuses(hidden, "side", "s1", "s2"))

	found := frontierOf(objectives, withDiscovered(runState(objectives, "root", "side"), "side"))
	assert.Equal(t, []navigation.Status{navigation.StatusAvailable, navigation.StatusLocked},
		statuses(found, "s1", "s2"), "its children follow its own routing")
	assert.Contains(t, availableSlugs(found), "s1")
}

func TestSecret_SideQuestClosesWithItsParent(t *testing.T) {
	objectives := sideQuestTree()
	objectives[0].ChildrenMin = intPtr(1)
	objectives[0].ChildrenMax = intPtr(1)
	state := withDiscovered(runState(objectives, "root", "side", "main"), "side")

	f := frontierOf(objectives, state)
	assert.Equal(t, navigation.StatusComplete, f.StatusOf("root"))
	assert.Equal(t, navigation.StatusLocked, f.StatusOf("s1"), "the parent decides what renders")
}

func TestSecret_Progress(t *testing.T) {
	objectives := secretTree(game.RouteStrategyFreeRoam, nil, nil)

	unfound := navigation.ComputeProgress(objectives, map[string]bool{"one": true}, nil)
	assert.Equal(t, 1, unfound.Found)
	assert.Equal(t, 3, unfound.Remaining, "two, the section and the root: the secret owes nothing")

	pending := navigation.ComputeProgress(objectives, map[string]bool{"one": true},
		map[string]bool{"bonus": true})
	assert.Equal(t, unfound.Remaining+1, pending.Remaining, "a find under complete-all is owed")

	found := navigation.ComputeProgress(objectives, map[string]bool{"one": true, "bonus": true},
		map[string]bool{"bonus": true})
	assert.Equal(t, 2, found.Found, "a find adds to found")
	assert.Equal(t, unfound.Total()+1, found.Total(), "and to the total")
}
