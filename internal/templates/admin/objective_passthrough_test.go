package templates //nolint:testpackage // renderRow is unexported

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sectionNode(kind navigation.SectionKind, passThrough bool) *ObjectiveTreeNode {
	leaf := &ObjectiveTreeNode{Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"}}
	return &ObjectiveTreeNode{
		Objective: models.Objective{ID: "pool", Slug: "pool", Title: "Pool", PassThrough: passThrough},
		Children:  []*ObjectiveTreeNode{leaf},
		Kind:      kind,
	}
}

// A leaf is never a place, so there is nothing for the flag to hide.
func TestObjectiveTree_PassThroughIsOnSectionsOnly(t *testing.T) {
	leaf := &ObjectiveTreeNode{Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"}}

	assert.Contains(t, renderRow(t, sectionNode(navigation.SectionDrawn, false)), "objective-passthrough-toggle")
	assert.NotContains(t, renderRow(t, leaf), "objective-passthrough-toggle")
}

// passThroughSwitch returns the switch's input tag, which says whether it is on.
func passThroughSwitch(t *testing.T, html string) string {
	t.Helper()
	input := regexp.MustCompile(`<input[^>]*objective-passthrough-toggle[^>]*>`).FindString(html)
	require.NotEmpty(t, input, "the section's menu carries the switch")
	return input
}

// Pass-through is uncommon, so an unmarked section shows no ghost at all. The
// switch waits in its menu rather than behind a hover on every header.
func TestObjectiveTree_UnmarkedSectionHasNoGhost(t *testing.T) {
	html := renderRow(t, sectionNode(navigation.SectionDrawn, false))

	assert.NotContains(t, html, "objective-passthrough-mark")
	assert.NotContains(t, passThroughSwitch(t, html), "checked")
}

// A marked section says so at rest, leading the pill whose menu holds its
// switch, and drops the card a place gets.
func TestObjectiveTree_MarkedSectionIsAlwaysShownAndFlat(t *testing.T) {
	html := renderRow(t, sectionNode(navigation.SectionPassThrough, true))

	assert.Contains(t, html, "objective-passthrough-mark")
	assert.Contains(t, html, "text-primary")
	assert.Contains(t, html, "objective-passthrough")
	assert.Contains(t, passThroughSwitch(t, html), "checked")
	ghost := strings.Index(html, "objective-passthrough-mark")
	assert.Less(t, strings.Index(html, "settings-rule-pill"), ghost, "the ghost is part of the pill")
	assert.Less(t, ghost, strings.Index(html, "settings-routing-trigger"),
		"the ghost qualifies the section before the routing says how it shows what is inside")
}

// A level the depth cap flattens looks the same to the player as a marked one,
// so the editor draws it the same way, with its own explanation.
func TestObjectiveTree_ForcedSectionIsShownWithItsReason(t *testing.T) {
	html := renderRow(t, sectionNode(navigation.SectionForced, false))

	assert.Contains(t, html, "objective-passthrough-mark")
	assert.Contains(t, html, "nested more than 2 sections deep")
	assert.NotContains(t, passThroughSwitch(t, html), "checked",
		"flattened is not marked: the switch offers to make it deliberate")
}

// The quest cannot pass through, so its own menu has no switch.
func TestObjectiveTree_QuestMenuHasNoPassThrough(t *testing.T) {
	leaf := &ObjectiveTreeNode{Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"}}
	tree := renderTree(t, models.Objective{ID: "root", Slug: "root"}, []*ObjectiveTreeNode{leaf})
	rootSettings := tree[strings.Index(tree, `id="quest-root-settings"`):strings.Index(tree, `id="objective-tree"`)]

	assert.NotContains(t, rootSettings, "objective-passthrough-toggle")
}

// Adding inside a section is New objective's arrow, one control for the page,
// and never part of a section's rule: adding has nothing to do with routing.
func TestObjectiveTree_NewObjectiveAddsInsideAnySection(t *testing.T) {
	inner := sectionNode(navigation.SectionDrawn, false)
	outer := &ObjectiveTreeNode{
		Objective: models.Objective{ID: "outer", Slug: "outer", Title: "Outer"},
		Children:  []*ObjectiveTreeNode{inner},
	}
	tree := renderTree(t, models.Objective{ID: "root", Slug: "root"}, []*ObjectiveTreeNode{outer})
	builderAt := strings.Index(tree, `id="quest-builder"`)
	require.Positive(t, builderAt)
	header := tree[:builderAt]

	assert.Contains(t, header, "/admin/objective/new?parentId=outer")
	assert.Contains(t, header, "/admin/objective/new?parentId=pool", "nested sections are offered too")
	assert.NotContains(t, header, "parentId=leaf", "a leaf is nested by dragging onto it")
	assert.Less(t, strings.Index(header, "parentId=outer"), strings.Index(header, "parentId=pool"),
		"in tree order, so the list reads like the tree")
	assert.NotContains(t, tree[builderAt:], "objective/new",
		"no section's menu or row carries an add of its own")
}

// A parked section is out of play; its pass-through styling would say two
// things at once.
func TestObjectiveTree_ParkedSectionKeepsItsParkedLook(t *testing.T) {
	node := sectionNode(navigation.SectionPassThrough, true)
	node.Draft, node.OutOfPlay = true, true

	html := renderRow(t, node)

	assert.Contains(t, html, "objective-parked")
	assert.NotRegexp(t, `objective-passthrough[ "]`, html, "the card is not drawn flat")
}

// TestRenderPassThroughPreview writes a browsable copy of a small tree when
// RAPUA_PREVIEW names a path, so the flat look can be seen beside a drawn one.
func TestRenderPassThroughPreview(t *testing.T) {
	path := os.Getenv("RAPUA_PREVIEW")
	if path == "" {
		t.Skip("set RAPUA_PREVIEW to write a browsable copy of the tree")
	}

	leaf := func(id string) *ObjectiveTreeNode {
		return &ObjectiveTreeNode{Objective: models.Objective{ID: id, Slug: id, Title: "Find the " + id}}
	}
	pool := func(id, title string, kind navigation.SectionKind, flagged bool, kids ...string) *ObjectiveTreeNode {
		node := &ObjectiveTreeNode{
			Objective: models.Objective{
				ID: id, Slug: id, Title: title, PassThrough: flagged, Routing: models.RouteStrategyRandomised,
			},
			Kind: kind,
		}
		for _, kid := range kids {
			node.Children = append(node.Children, leaf(kid))
		}
		MarkChildren(node.Objective, node.Children)
		return node
	}
	trip := pool("trip", "The zoo trip", navigation.SectionDrawn, false)
	trip.Objective.Routing = models.RouteStrategyFreeRoam
	birds := pool("birds", "Pick a bird", navigation.SectionPassThrough, true, "owl", "heron")
	birds.Children = append(birds.Children, pool("rare", "A rare one", navigation.SectionDrawn, false, "kea"))
	MarkChildren(birds.Objective, birds.Children)
	trip.Children = []*ObjectiveTreeNode{
		birds,
		pool("fish", "Pick a fish", navigation.SectionDrawn, false, "eel", "trout"),
		pool("deep", "Too deep", navigation.SectionForced, false, "gecko"),
	}
	MarkChildren(trip.Objective, trip.Children)

	page := renderRow(t, trip)
	wd, err := os.Getwd()
	require.NoError(t, err)
	css := "file://" + filepath.Join(wd, "..", "..", "..", "static", "css", "tailwind.css")
	doc := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="dracula"><head><meta charset="utf-8">
<link rel="stylesheet" href="%s"></head>
<body class="bg-base-200 p-6"><ul class="objective-children max-w-3xl">%s</ul></body></html>`, css, page)
	require.NoError(t, os.WriteFile(path, []byte(doc), 0o600))
}
