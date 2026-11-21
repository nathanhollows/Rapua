package templates

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Nothing may change size mid-drag or the drop target flips between child and
// sibling. These pin what keeps the geometry still.
func TestObjectiveTree_DragKeepsItsGeometryStill(t *testing.T) {
	leaf := &ObjectiveTreeNode{Objective: models.Objective{ID: "leaf", Slug: "leaf", Title: "Leaf"}}
	html := renderTree(t, models.Objective{ID: "root", Slug: "root"}, []*ObjectiveTreeNode{leaf})

	assert.Contains(t, html, "emptyInsertThreshold: 0", "a margin around the empty list made every row a nest target")
	assert.NotContains(t, html, "emptyInsertThreshold: 56")
	assert.Contains(t, html, "tree-dragging")
	assert.NotContains(t, html, "evt.stopPropagation()", "stopping dragover starves Sortable's auto-scroll")
	assert.Contains(t, html, "target.dataset.objectiveId",
		"nesting is a decision, posted at the drop, not a second thing moving the list")
	assert.Contains(t, html, "dragScrollTick", "the page scrolls itself while dragging, the same in every browser")
	assert.Contains(t, html, "scroll: false")
	assert.Contains(t, html, "setDragImage",
		"the default picture is as wide as the row and covers the placeholder labels")
	assert.Contains(t, html, "steerNesting", "nesting is decided by horizontal intent")
	assert.Contains(t, html, "rowMidpoint", "the row's own midpoint is the boundary, so it means the same at any depth")

	css, err := os.ReadFile(filepath.Join("..", "..", "..", "static", "css", "src", "editor.css"))
	require.NoError(t, err)
	placeholder := `#objective-tree .objective-rail-item.sortable-ghost-preview {
  height: 0;`
	assert.Contains(t, string(css), placeholder, "a placeholder with height moves everything under the pointer")
	assert.Contains(t, string(css), "li.nest-target > .objective-row",
		"the row that would become the parent is highlighted")
	assert.Contains(t, string(css), "attr(data-drag-hint)", "the gesture is explained while dragging")
	assert.Contains(t, html, "right half of a row to nest")
	assert.Contains(t, html, "Drop here", "the left half of the split line says where a drop lands")
	assert.Contains(t, html, "or nest inside", "and the right half says what it does instead")
	assert.Contains(t, string(css), ".drag-cue-right")
}
