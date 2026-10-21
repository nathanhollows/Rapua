package navigation_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
	"github.com/stretchr/testify/assert"
)

func gated(obj models.Objective, deps ...string) models.Objective {
	obj.Depends = deps
	return obj
}

func TestFrontier_GatedLeafOpensWhenItsVariablesAreSet(t *testing.T) {
	tree := []models.Objective{
		node("root", "", 0),
		node("gate", "root", 0),
		gated(node("door", "root", 1), "key", "lamp"),
	}
	state := runState(tree, "root")

	assert.Equal(t, navigation.StatusLocked, frontierOf(tree, state).StatusOf("door"))

	state.Vars = map[string]bool{"key": true}
	assert.Equal(t, navigation.StatusLocked, frontierOf(tree, state).StatusOf("door"), "every variable is needed")

	state.Vars["lamp"] = true
	assert.Equal(t, navigation.StatusAvailable, frontierOf(tree, state).StatusOf("door"))
}

func TestFrontier_GatedSectionLocksItsSubtree(t *testing.T) {
	tree := []models.Objective{
		node("root", "", 0),
		gated(node("wing", "root", 0), "key"),
		node("room", "wing", 0),
	}
	state := runState(tree, "root", "wing")

	f := frontierOf(tree, state)
	assert.Equal(t, []navigation.Status{navigation.StatusLocked, navigation.StatusLocked}, statuses(f, "wing", "room"))

	state.Vars = map[string]bool{"key": true}
	f = frontierOf(tree, state)
	assert.Equal(
		t,
		[]navigation.Status{navigation.StatusAvailable, navigation.StatusAvailable},
		statuses(f, "wing", "room"),
	)
}

// A pass-through is still a node in the walk, so its gate holds its children
// however the screen draws them.
func TestFrontier_GatedPassThroughLocksItsChildren(t *testing.T) {
	pool := gated(node("pool", "root", 0), "key")
	pool.PassThrough = true
	tree := []models.Objective{node("root", "", 0), pool, node("bird", "pool", 0)}
	state := runState(tree, "root", "pool")

	assert.Equal(t, navigation.StatusLocked, frontierOf(tree, state).StatusOf("bird"))
	state.Vars = map[string]bool{"key": true}
	assert.Equal(t, navigation.StatusAvailable, frontierOf(tree, state).StatusOf("bird"))
}

func TestFrontier_OrderedSectionWaitsOnAGatedChild(t *testing.T) {
	root := node("root", "", 0)
	root.Routing = game.RouteStrategyOrdered
	tree := []models.Objective{root, gated(node("first", "root", 0), "key"), node("second", "root", 1)}
	state := runState(tree, "root")

	f := frontierOf(tree, state)
	assert.Equal(
		t,
		[]navigation.Status{navigation.StatusLocked, navigation.StatusLocked},
		statuses(f, "first", "second"),
	)
	assert.Empty(t, availableSlugs(f))
}

func TestFrontier_RootIgnoresDepends(t *testing.T) {
	tree := []models.Objective{gated(node("root", "", 0), "key"), node("child", "root", 0)}
	assert.Equal(t, navigation.StatusAvailable, frontierOf(tree, runState(tree, "root")).StatusOf("child"))
}

// A gate decides what a run can reach, never what it has done.
func TestComputeCompleted_IgnoresVariables(t *testing.T) {
	tree := []models.Objective{node("root", "", 0), gated(node("door", "root", 0), "key")}
	state := runState(tree, "door")

	complete := navigation.ComputeCompleted(tree, state)
	assert.True(t, complete["door"])
	assert.Equal(t, navigation.StatusComplete, frontierOf(tree, state).StatusOf("door"))
}
