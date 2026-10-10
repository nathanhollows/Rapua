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

func renderRow(t *testing.T, node *ObjectiveTreeNode) string {
	t.Helper()
	var out strings.Builder
	require.NoError(t, objectiveNode(node, false, 0).Render(context.Background(), &out))
	return out.String()
}

// These settings govern children, so a leaf showing them conflates the
// objective with its container.
func TestObjectiveTree_ChildSettingsAreOnParentsOnly(t *testing.T) {
	leaf := &ObjectiveTreeNode{Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"}}
	section := &ObjectiveTreeNode{
		Objective: models.Objective{ID: "section", Slug: "section", Title: "Section"},
		Children:  []*ObjectiveTreeNode{leaf},
	}

	sectionHTML := renderRow(t, section)
	assert.Contains(t, sectionHTML, "settings-routing-trigger")
	assert.Contains(t, sectionHTML, "settings-band-summary")
	assert.Contains(t, sectionHTML, "settings-finish-label")

	leafHTML := renderRow(t, leaf)
	assert.NotContains(t, leafHTML, "settings-routing-trigger")
	assert.NotContains(t, leafHTML, "settings-band-summary")
	assert.NotContains(t, leafHTML, "settings-finish-label")
}

func TestObjectiveTree_FinishLabelHiddenWhenTheBandAutoCompletes(t *testing.T) {
	one := 1
	leaf := &ObjectiveTreeNode{Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"}}
	auto := &ObjectiveTreeNode{
		Objective: models.Objective{
			ID: "auto", Slug: "auto", Title: "Auto",
			ChildrenMin: &one, ChildrenMax: &one,
		},
		Children:          []*ObjectiveTreeNode{leaf},
		PublishedChildren: 1,
	}

	html := renderRow(t, auto)
	require.Contains(t, html, "settings-finish-label")
	assert.Contains(t, html, "settings-finish-label mt-3 pt-3 border-t border-base-300 hidden",
		"min equals max, so no button is ever shown and the label is pointless")
}

// renderTree renders the whole page, for controls that belong to the quest.
func renderTree(t *testing.T, root models.Objective, nodes []*ObjectiveTreeNode) string {
	t.Helper()
	var out strings.Builder
	require.NoError(t, ObjectiveTree(services.QuestLint{}, root, nodes, models.QuestSettings{}).
		Render(context.Background(), &out))
	return out.String()
}

// The quest routes its own top level, so it needs a band like any section.
func TestObjectiveTree_RootOffersItsOwnCompletionBand(t *testing.T) {
	kids := []*ObjectiveTreeNode{
		{Objective: models.Objective{ID: "a", Slug: "a", Title: "A"}},
		{Objective: models.Objective{ID: "b", Slug: "b", Title: "B"}},
	}

	html := renderTree(t, models.Objective{
		ID: "root", Slug: "root", Routing: models.RouteStrategyFreeRoam,
	}, kids)

	assert.Contains(t, html, "settings-band-summary")
	assert.Contains(t, html, "settings-min")
	assert.Contains(t, html, "settings-max")
	assert.Contains(t, html, `placeholder="2"`,
		"the band counts the published top-level rows, not every row in the quest")
}

// Hidden rather than absent: the toolbar is never re-rendered, so the picker
// must exist when the author leaves guided routing.
func TestObjectiveTree_GuidedHidesTheBandPicker(t *testing.T) {
	leaf := &ObjectiveTreeNode{Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"}}
	section := func(routing models.RouteStrategy) *ObjectiveTreeNode {
		return &ObjectiveTreeNode{
			Objective:         models.Objective{ID: "s", Slug: "s", Title: "S", Routing: routing},
			Children:          []*ObjectiveTreeNode{leaf},
			PublishedChildren: 1,
		}
	}

	guided := renderRow(t, section(models.RouteStrategyOrdered))
	require.Contains(t, guided, "settings-band-picker")
	assert.Contains(t, guided, "settings-band-picker relative hidden")

	open := renderRow(t, section(models.RouteStrategyFreeRoam))
	assert.Contains(t, open, "settings-band-picker relative\"")
}

// Randomised with no window is open exploration, so the script needs a default
// to fill in.
func TestObjectiveRoutingPicker_WindowCarriesItsDefault(t *testing.T) {
	leaf := &ObjectiveTreeNode{Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"}}
	section := &ObjectiveTreeNode{
		Objective: models.Objective{ID: "s", Slug: "s", Title: "S"},
		Children:  []*ObjectiveTreeNode{leaf},
	}

	html := renderRow(t, section)
	assert.Contains(t, html, `data-default="3"`)
}

// The quest's picker is outside the swapped tree, so the script repaints it and
// needs hooks on the trigger and on each option's label.
func TestObjectiveRoutingPicker_TriggerCanBeRepaintedFromAnOption(t *testing.T) {
	kids := []*ObjectiveTreeNode{
		{Objective: models.Objective{ID: "a", Slug: "a", Title: "A"}},
	}

	html := renderTree(t, models.Objective{
		ID: "root", Slug: "root", Routing: models.RouteStrategyOrdered,
	}, kids)

	assert.Contains(t, html, "settings-routing-label",
		"the trigger's text has to be findable to be rewritten")
	assert.Contains(t, html, "settings-routing-icon",
		"and so does its icon, which otherwise keeps naming the old strategy")
	assert.Contains(t, html, "settings-routing-option-label",
		"the new text is read off the option, not duplicated into the script")
}

// A menu anchored toward its own trigger opens back across the row and past the
// card's edge.
func TestObjectiveTree_MenusOpenAwayFromTheirTrigger(t *testing.T) {
	leaf := &ObjectiveTreeNode{Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"}}
	section := &ObjectiveTreeNode{
		Objective: models.Objective{
			ID: "s", Slug: "s", Title: "S", Routing: models.RouteStrategyFreeRoam,
		},
		Children:          []*ObjectiveTreeNode{leaf},
		PublishedChildren: 1,
	}

	row := renderRow(t, section)
	assert.Equal(t, 2, strings.Count(row, "right-0"),
		"both pickers sit at the right of the header, so both menus open leftward")
	assert.NotContains(t, row, "left-0")

	// Settings live together at the right, past the title: the title is what
	// you scan a tree for, and a control before it pushes every name out of
	// line with the one above.
	assert.Less(t, strings.Index(row, "Section"), strings.Index(row, "settings-routing-trigger"))
	assert.Less(t, strings.Index(row, "settings-routing-trigger"), strings.Index(row, "settings-band-summary"),
		"routing first: it is the rule the band then qualifies")

	// Both toolbar pickers sit at the left, so both open rightward.
	tree := renderTree(t, models.Objective{
		ID: "root", Slug: "root", Routing: models.RouteStrategyFreeRoam,
	}, []*ObjectiveTreeNode{leaf})
	toolbar := tree[strings.Index(tree, `id="quest-toolbar"`):strings.Index(tree, `id="quest-builder"`)]
	assert.Equal(t, 2, strings.Count(toolbar, "left-0"))
	assert.NotContains(t, toolbar, "right-0")
}

// classContaining returns the class attribute holding token, so assertions do
// not catch classes on nested icons.
func classContaining(t *testing.T, html, token string) string {
	t.Helper()
	at := strings.Index(html, token)
	require.GreaterOrEqual(t, at, 0, "no element carries %q", token)
	start := strings.LastIndex(html[:at], `class="`) + len(`class="`)
	end := strings.Index(html[start:], `"`)
	require.GreaterOrEqual(t, end, 0, "unterminated class attribute around %q", token)
	return html[start : start+end]
}

// Opacity on an ancestor takes the row's tooltips down with it.
func TestObjectiveTree_ParkedRowsAreMarkedWithoutOpacity(t *testing.T) {
	parked := &ObjectiveTreeNode{
		Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"},
		Draft:     true, OutOfPlay: true,
	}

	html := renderRow(t, parked)
	assert.NotContains(t, classContaining(t, html, "objective-parked"), "opacity",
		"the row holds tooltips, so it is drained by colour and border instead")
	assert.Contains(t, html, "text-warning",
		"the eye keeps its colour: it is the one live thing on a parked row")
}

// The lock badge is appended at runtime; as a third child of the
// justify-between row it strands the primary action mid-page.
func TestObjectiveTree_HeaderHasATitleGroupForTheLockBadge(t *testing.T) {
	html := renderTree(t, models.Objective{ID: "root", Slug: "root"}, nil)

	assert.Contains(t, html, `id="quest-title-group"`)
	// Rows above the tree share its left edge; the header padding matches the
	// other pages'.
	assert.Contains(t, html, `items-center w-full px-6 py-5`)
	assert.Contains(t, html, `class="px-6 pb-4 flex flex-wrap`)
	assert.Contains(t, html, `id="objective-tree" class="px-6"`)
}
