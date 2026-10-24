package templates

import (
	"context"
	"regexp"
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
	assert.Contains(t, classContaining(t, html, "settings-finish-label "), "hidden",
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
	assert.Contains(t, html, `all 2</span>`,
		"the band counts the published top-level rows, not every row in the quest")
}

// Hidden rather than absent: the script shows the picker the moment the author
// leaves guided routing, before the refresh arrives.
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
	assert.Contains(t, classContaining(t, guided, "settings-band-picker"), "hidden")

	open := renderRow(t, section(models.RouteStrategyFreeRoam))
	assert.NotContains(t, classContaining(t, open, "settings-band-picker"), "hidden")
}

// A guided path takes every step, so its menu keeps the sentence locked at
// all rather than collapsing, and its bounds are written blank whatever was
// stored: the next save leaves no band behind to end the section early.
func TestBandColumn_GuidedIsLockedAtAll(t *testing.T) {
	one := 1
	section := bandSection(3, &one, &one)
	section.Objective.Routing = models.RouteStrategyOrdered

	html := renderRow(t, section)
	assert.NotContains(t, classContaining(t, html, "settings-band-fields"), "hidden")
	assert.Contains(t, html, `all 3</span>`)
	for _, step := range regexp.MustCompile(`<button[^>]*data-kind="need"[^>]*>`).FindAllString(html, -1) {
		assert.Contains(t, step, "disabled")
	}
	assert.Contains(t, html, `class="settings-min" value=""`)
	assert.Contains(t, html, `class="settings-max" value=""`)
	assert.Contains(t, classContaining(t, html, "band-then"), "hidden",
		"with every one required there is nothing to close")
}

// The second half of the sentence names the count it follows, and the cap
// reads as how many players can complete, all of them when there is none.
func TestBandColumn_ReadsAsOneSentence(t *testing.T) {
	two, three := 2, 3
	html := renderRow(t, bandSection(4, &two, nil))
	assert.Contains(t, html, "Once they have completed 2,")
	assert.Contains(t, html, "They can complete up to")
	assert.Contains(t, html, `data-kind="cap" aria-live="polite">all 4</span>`)

	capped := renderRow(t, bandSection(4, &two, &three))
	assert.Contains(t, capped, `data-kind="cap" aria-live="polite">3</span>`)
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

// The script repaints a pick before the refresh lands, so it needs hooks on the
// trigger and on each option's label.
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
	assert.Equal(t, 1, strings.Count(row, "right-0"),
		"the rule sits at the right of the header, so its one menu opens leftward")
	assert.NotContains(t, row, "left-0")

	// Settings live together at the right, past the title: the title is what
	// you scan a tree for, and a control before it pushes every name out of
	// line with the one above.
	assert.Less(t, strings.Index(row, "Section"), strings.Index(row, "settings-routing-trigger"))
	assert.Less(t, strings.Index(row, "settings-routing-trigger"), strings.Index(row, "settings-band-summary"),
		"routing first: it is the rule the band then qualifies")

	// The quest's own rule ends its row as a section's does, so its menu opens
	// leftward too.
	tree := renderTree(t, models.Objective{
		ID: "root", Slug: "root", Routing: models.RouteStrategyFreeRoam,
	}, []*ObjectiveTreeNode{leaf})
	rootSettings := tree[strings.Index(tree, `id="quest-root-settings"`):strings.Index(tree, `id="objective-tree"`)]
	assert.Equal(t, 1, strings.Count(rootSettings, "right-0"))
	assert.NotContains(t, rootSettings, "left-0")
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
	assert.Contains(t, html, `id="quest-lint" class="px-6"`)
	assert.Contains(t, html, `class="quest-plane mx-6`, "the quest's plane shares the header's edge")
	assert.Contains(t, html, `id="objective-tree" class="px-6"`)
}

// The quest gets a plane of its own, apart from the page's controls, headed
// by its rule and the search: the header and the tree no longer share a
// surface.
func TestObjectiveTree_TopLevelHasItsOwnPlane(t *testing.T) {
	html := renderTree(t, models.Objective{ID: "root", Slug: "root"}, nil)
	planeAt := strings.Index(html, `class="quest-plane`)
	require.Positive(t, planeAt)
	plane := html[planeAt:]
	rootAt := strings.Index(plane, `id="quest-root-settings"`)
	treeAt := strings.Index(plane, `id="objective-tree"`)
	require.Positive(t, rootAt)
	require.Positive(t, treeAt)
	assert.Less(t, rootAt, treeAt, "the plane's header heads the tree")
	assert.Less(t, strings.Index(html, `id="quest-scoring"`), planeAt, "the page's controls stay off the plane")

	guided := renderTree(t, models.Objective{ID: "root", Slug: "root", Routing: models.RouteStrategyOrdered}, nil)
	assert.Contains(t, guided, `border-primary/40" data-routing="ordered"`, "framed in the top level's routing hue")
	assert.Contains(t, classContaining(t, guided, "rounded-t-2xl bg-primary/10"), "border-b border-primary/25")
}

// Scoring is the quest's, so it heads the page beside New objective; the
// search shares the Top level row, inside the swapped builder, and is kept
// across the swap so a term survives every edit. The quest's rule ends that
// row, where every section's ends its header.
func TestObjectiveTree_ScoringHeadsThePageAndSearchSharesTheTopLevelRow(t *testing.T) {
	html := renderTree(t, models.Objective{ID: "root", Slug: "root"}, nil)
	scoringAt := strings.Index(html, `id="quest-scoring"`)
	builderAt := strings.Index(html, `id="quest-builder"`)
	require.Positive(t, scoringAt)
	require.Positive(t, builderAt)
	assert.Less(t, scoringAt, strings.Index(html, `href="/admin/objective/new"`), "scoring sits before New objective")
	assert.Less(t, scoringAt, builderAt)

	rootAt := strings.Index(html, `id="quest-root-settings"`)
	require.Positive(t, rootAt)
	rootRow := html[rootAt:]
	startAt := strings.Index(rootRow, `/admin/quest/start`)
	require.Positive(t, startAt)
	assert.Contains(t, rootRow[:startAt], `id="search-objectives"`, "the search is on the Top level row")
	assert.Contains(t, html, `id="quest-search" hx-preserve="true"`)
	pillAt := strings.Index(rootRow, "settings-rule-pill")
	require.Positive(t, pillAt)
	assert.Less(t, strings.Index(rootRow, `id="search-objectives"`), pillAt, "the rule ends the row")
}

// bandSection builds a section with n children in play and the given band.
func bandSection(n int, minChildren, maxChildren *int) *ObjectiveTreeNode {
	section := &ObjectiveTreeNode{
		Objective: models.Objective{
			ID: "s", Slug: "s", Title: "S", Routing: models.RouteStrategyFreeRoam,
			ChildrenMin: minChildren, ChildrenMax: maxChildren,
		},
		PublishedChildren: n,
	}
	for range n {
		leaf := &ObjectiveTreeNode{Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"}}
		section.Children = append(section.Children, leaf)
	}
	return section
}

// Solid up to the minimum, a ring up to the maximum, a faint ring past it: the
// row says "2 or more of 4, done at 3" without a word.
func TestBandPicker_PipsShowTheBand(t *testing.T) {
	two, three := 2, 3
	html := renderRow(t, bandSection(4, &two, &three))

	assert.Equal(t, 2, strings.Count(html, `data-state="required"`))
	assert.Equal(t, 1, strings.Count(html, `data-state="optional"`))
	assert.Equal(t, 1, strings.Count(html, `data-state="closed"`))
	assert.Contains(t, classContaining(t, html, "settings-band-summary"), "sr-only",
		"the words stay for screen readers and the tooltip, but the pips are the label")
}

// A blank band is every child, so every pip is required.
func TestBandPicker_BlankBandRequiresEveryPip(t *testing.T) {
	html := renderRow(t, bandSection(3, nil, nil))

	assert.Equal(t, 3, strings.Count(html, `data-state="required"`))
}

// Past the limit a row of pips is no longer countable at a glance.
func TestBandPicker_ManyChildrenFallBackToWords(t *testing.T) {
	html := renderRow(t, bandSection(maxBandPips+1, nil, nil))

	assert.NotContains(t, html, "band-pip")
	assert.NotContains(t, classContaining(t, html, "settings-band-summary"), "sr-only")
}

// A blank band needs every child and closes on the last, so there is never a
// finish button for its label to name.
func TestBandOffersFinish_BlankBandHasNoButton(t *testing.T) {
	assert.False(t, bandOffersFinish(nil, nil, 3))

	two := 2
	assert.True(t, bandOffersFinish(&two, nil, 3))
}

// The band is asked as two questions in words, never as two bare numbers
// where blank, zero and equal each mean something different.
func TestBandColumn_AsksInWords(t *testing.T) {
	one, two := 1, 2
	cases := []struct {
		name     string
		min, max *int
		count    string
		carry    bool
	}{
		{"every one", nil, nil, "all 4", false},
		{"one closes the rest", &one, &one, "1 of 4", false},
		{"two, then players finish", &two, nil, "2 of 4", true},
		{"none required", zeroBand(), nil, "none", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			html := renderRow(t, bandSection(4, c.min, c.max))

			assert.Contains(t, html, c.count+"</span>")
			assert.Contains(t, html, `type="hidden" class="settings-min"`, "the bounds are not for typing into")
			carryRadio := regexp.MustCompile(`<input[^>]*value="carry"[^>]*>`).FindString(html)
			assert.Equal(t, c.carry, strings.Contains(carryRadio, "checked"))
		})
	}
}

func zeroBand() *int {
	zero := 0
	return &zero
}

// A section that can only be ended by its players shows a copy of their finish
// button on the pill, label and all, beside the pips that say how many.
func TestRulePill_ShowsTheFinishButton(t *testing.T) {
	two := 2
	carry := bandSection(4, &two, nil)
	carry.Objective.FinishLabel = "That's enough distilling"
	html := renderRow(t, carry)
	assert.NotContains(t, classContaining(t, html, "band-finish-chip"), "hidden")
	assert.Contains(t, html, "That&#39;s enough distilling</span>")

	assert.Contains(t, renderRow(t, bandSection(4, &two, nil)), "Finish</span>",
		"an unnamed button says what players will see")
	assert.Contains(t, classContaining(t, renderRow(t, bandSection(4, &two, &two)), "band-finish-chip"), "hidden")
}

// The menu is a live control, so it marks the routing in force.
func TestRuleMenu_MarksTheRoutingInForce(t *testing.T) {
	section := bandSection(2, nil, nil)
	section.Objective.Routing = models.RouteStrategyRandomised

	html := renderRow(t, section)
	current := regexp.MustCompile(`<a[^>]*aria-current="true"[^>]*>`).FindString(html)
	assert.Contains(t, current, `data-value="randomised"`)
	assert.Equal(t, 1, strings.Count(html, `aria-current="true"`))
}
