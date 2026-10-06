package templates

import (
	"context"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/services"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func kids(titles ...string) []*ObjectiveTreeNode {
	nodes := make([]*ObjectiveTreeNode, len(titles))
	for i, t := range titles {
		nodes[i] = &ObjectiveTreeNode{Objective: models.Objective{ID: t, Title: t}}
	}
	return nodes
}

func marks(nodes []*ObjectiveTreeNode) []RailMark {
	out := make([]RailMark, len(nodes))
	for i, n := range nodes {
		out[i] = n.Mark
	}
	return out
}

func TestMarkChildren_GuidedNumbersInOrder(t *testing.T) {
	children := kids("one", "two", "three")
	MarkChildren(models.Objective{Routing: models.RouteStrategyOrdered}, children)

	assert.Equal(t, []RailMark{RailStep, RailStep, RailStep}, marks(children))
	assert.Equal(t, []int{1, 2, 3}, []int{children[0].Step, children[1].Step, children[2].Step})
}

// A player never meets a draft, so numbering it would be wrong.
func TestMarkChildren_DraftsAreNotNumbered(t *testing.T) {
	children := kids("one", "parked", "two")
	children[1].Draft = true
	MarkChildren(models.Objective{Routing: models.RouteStrategyOrdered}, children)

	assert.Equal(t, []RailMark{RailStep, RailParked, RailStep}, marks(children))
	assert.Equal(t, 1, children[0].Step)
	assert.Equal(t, 2, children[2].Step, "numbering closes over the parked row")
}

func TestMarkChildren_OpenGivesEveryRowADot(t *testing.T) {
	children := kids("a", "b", "c")
	MarkChildren(models.Objective{Routing: models.RouteStrategyFreeRoam}, children)

	assert.Equal(t, []RailMark{RailDot, RailDot, RailDot}, marks(children))
}

func TestMarkChildren_RandomisedSplitsWindowFromPool(t *testing.T) {
	children := kids("a", "b", "c", "d")
	MarkChildren(models.Objective{Routing: models.RouteStrategyRandomised, MaxNext: 2}, children)

	assert.Equal(t, []RailMark{RailWindow, RailWindow, RailPool, RailPool}, marks(children))
}

// A run never loads a draft, so it cannot be one of the few on offer.
func TestMarkChildren_RandomisedWindowSkipsDrafts(t *testing.T) {
	children := kids("a", "parked", "b", "c")
	children[1].Draft = true
	MarkChildren(models.Objective{Routing: models.RouteStrategyRandomised, MaxNext: 2}, children)

	assert.Equal(t, []RailMark{RailWindow, RailParked, RailWindow, RailPool}, marks(children))
}

func TestMarkChildren_RandomisedWithoutALimitOffersEverything(t *testing.T) {
	children := kids("a", "b", "c")
	MarkChildren(models.Objective{Routing: models.RouteStrategyRandomised}, children)

	assert.Equal(t, []RailMark{RailWindow, RailWindow, RailWindow}, marks(children))
}

// Routing remaps --color-primary, so the card's primary classes take its hue.
func TestObjectiveNode_SectionCarriesItsRouting(t *testing.T) {
	leaf := &ObjectiveTreeNode{Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"}}
	section := &ObjectiveTreeNode{
		Objective: models.Objective{
			ID: "sect", Slug: "sect", Title: "Section", Routing: models.RouteStrategyRandomised,
		},
		Children: []*ObjectiveTreeNode{leaf},
	}

	html := renderRow(t, section)
	assert.Contains(t, html, `data-routing="randomised"`)
	require.NotContains(t, html, "data-group-color", "the author's palette picker is gone")
}

// The line is drawn on the row and reaches below it, so spacing cannot cut it.
func TestObjectiveNode_RailIsDrawnOnTheRowNotBesideIt(t *testing.T) {
	leaf := &ObjectiveTreeNode{
		Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"},
		Mark:      RailDot,
	}

	html := renderRow(t, leaf)
	assert.Contains(t, html, "objective-rail-item",
		"the row itself carries the rail, so its line can cross the gap below")
	assert.Contains(t, html, `class="objective-mark" data-mark="dot"`)
	assert.NotContains(t, html, "objective-rail-cell",
		"a separate rail column could not reach into the gap")
}

// Spacing is padding on the row, not a list gap, because a gap would cut the
// rail into segments.
func TestObjectiveTree_EveryChildListSharesTheSameSpacing(t *testing.T) {
	leaf := &ObjectiveTreeNode{Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"}}
	section := &ObjectiveTreeNode{
		Objective: models.Objective{ID: "sect", Slug: "sect", Title: "Section"},
		Children:  []*ObjectiveTreeNode{leaf},
	}

	var out strings.Builder
	require.NoError(t, ObjectiveTree(
		services.QuestLint{}, models.Objective{ID: "root", Slug: "root"},
		[]*ObjectiveTreeNode{section}, false,
	).Render(context.Background(), &out))

	rendered := out.String()
	assert.Equal(t, 2, strings.Count(rendered, `objective-children flex flex-col w-full`),
		"the root list and the section's list lay their rows out alike")
	assert.NotContains(t, rendered, "gap-1.5",
		"the list sets no gap: the step is padding on the row, so the rail can cross it")
	assert.NotContains(t, rendered, "join join-vertical",
		"the join vocabulary went with the rail: rows are separate now")
}

// Top-level routing does not govern the system pages, so no rail joins them.
func TestSystemPageLink_HangsItsPadlockInTheGutter(t *testing.T) {
	var out strings.Builder
	require.NoError(t, objectiveSystemPageLink("Start", "/admin/quest/start").
		Render(context.Background(), &out))

	html := out.String()
	assert.Contains(t, html, "objective-outside-mark",
		"the padlock takes a marker's place in the gutter")
	assert.NotContains(t, html, "objective-rail-item",
		"but it is not on the rail: these pages are outside the quest on purpose")
}

// Without it the top level keeps the theme's primary whatever the root's routing.
func TestObjectiveTree_RootListCarriesTheQuestRouting(t *testing.T) {
	leaf := &ObjectiveTreeNode{Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"}}

	var out strings.Builder
	require.NoError(t, ObjectiveTree(
		services.QuestLint{},
		models.Objective{ID: "root", Slug: "root", Routing: models.RouteStrategyFreeRoam},
		[]*ObjectiveTreeNode{leaf}, false,
	).Render(context.Background(), &out))

	rendered := out.String()
	assert.Contains(t, rendered, `data-parent-id="root" data-routing="free_roam"`,
		"the top-level list carries the quest's routing, so its rows' rails take that hue")
	assert.Contains(t, rendered, `data-child-count="1" data-routing="free_roam"`,
		"and so does the toolbar, which holds the quest's own picker outside the swapped tree")
}

// The shuffle finds markers as direct children of a list's rows; deeper nesting
// would let an ancestor's shuffle take a nested group's diamonds.
func TestObjectiveTree_WindowMarkersSitDirectlyOnTheirRows(t *testing.T) {
	children := kids("a", "b", "c")
	parent := models.Objective{
		ID: "p", Slug: "p", Title: "P",
		Routing: models.RouteStrategyRandomised, MaxNext: 2,
	}
	MarkChildren(parent, children)

	html := renderRow(t, &ObjectiveTreeNode{
		Objective: parent, Children: children, PublishedChildren: 3,
	})

	for _, mark := range []string{"window", "pool"} {
		assert.Contains(t, html, `<span class="objective-mark" data-mark="`+mark+`">`,
			"the %s marker is a span the script can retarget", mark)
	}
	assert.Equal(t, 2, strings.Count(html, `class="objective-mark" data-mark="window"`))
	assert.Equal(t, 1, strings.Count(html, `class="objective-mark" data-mark="pool"`))
}

func TestRoutingLabel_RandomisedNamesItsWindow(t *testing.T) {
	assert.Equal(t, "Random 3", routingLabel(models.RouteStrategyRandomised, 3))
	assert.Equal(t, "Guided Path", routingLabel(models.RouteStrategyOrdered, 0))
	assert.Equal(t, "Choose routing", routingLabel("", 0))
	assert.Equal(t, "Random order", routingLabel(models.RouteStrategyRandomised, 0),
		"no cap offers every child, so the shuffle is all that is left to name")
}

func TestRoutingOptionLabel_NamesTheWindowAClickWouldApply(t *testing.T) {
	guided := models.Objective{Routing: models.RouteStrategyOrdered}
	assert.Equal(t, "Random 3", routingOptionLabel(models.RouteStrategyRandomised, guided),
		"not randomised yet, so the menu names the window the picker fills in")

	windowed := models.Objective{Routing: models.RouteStrategyRandomised, MaxNext: 5}
	assert.Equal(t, "Random 5", routingOptionLabel(models.RouteStrategyRandomised, windowed),
		"already randomised, so the menu names the window in force")

	uncapped := models.Objective{Routing: models.RouteStrategyRandomised}
	assert.Equal(t, "Random order", routingOptionLabel(models.RouteStrategyRandomised, uncapped))

	assert.Equal(t, "Open Exploration", routingOptionLabel(models.RouteStrategyFreeRoam, guided),
		"the other two carry no setting, so they are named as they always were")
}
