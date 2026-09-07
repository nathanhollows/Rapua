package templates

import (
	"context"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Rendering has to terminate whatever shape the rows are in. The tree builder
// cuts the edges that close a loop, but the renderer is what a future caller
// hands a graph to, and a cycle there does not draw badly: it never returns
// and takes the process with it.
func TestObjectiveNode_CycleDoesNotRecurseForever(t *testing.T) {
	first := &ObjectiveTreeNode{Objective: models.Objective{ID: "first", Slug: "first", Title: "First"}}
	second := &ObjectiveTreeNode{Objective: models.Objective{ID: "second", Slug: "second", Title: "Second"}}
	first.Children = []*ObjectiveTreeNode{second}
	second.Children = []*ObjectiveTreeNode{first}

	var out strings.Builder
	require.NoError(t, objectiveNode(first, false, 0).Render(context.Background(), &out))
	rendered := out.String()
	assert.Contains(t, rendered, "past what this view draws",
		"the depth cap is what stops it, and it says so rather than truncating silently")
	assert.Contains(t, rendered, "/admin/objective/",
		"and the row at the boundary stays reachable, so it can be dragged back out")
}

// The cap is reachable by dragging: lint warns past four levels but does not
// refuse, so the boundary row belongs to an author who over-nested rather than
// to a broken structure. Blaming a loop would send them hunting for one that
// is not there.
func TestObjectiveNode_TooDeepRowIsNamedAndLinked(t *testing.T) {
	deepest := &ObjectiveTreeNode{
		Objective: models.Objective{ID: "deepest", Slug: "deepest", Title: "Deepest Objective"},
	}
	node := deepest
	for range maxRenderDepth + 2 {
		node = &ObjectiveTreeNode{
			Objective: models.Objective{ID: "mid", Slug: "mid", Title: "Middle"},
			Children:  []*ObjectiveTreeNode{node},
		}
	}

	var out strings.Builder
	require.NoError(t, objectiveNode(node, false, 0).Render(context.Background(), &out))

	rendered := out.String()
	assert.NotContains(t, rendered, "loop",
		"an over-nested quest is not a broken one, and the message must not say it is")
	assert.Contains(t, rendered, "past what this view draws")
}
