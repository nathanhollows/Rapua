package templates //nolint:testpackage // testing unexported template helpers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func secretTree() (models.Objective, []*ObjectiveTreeNode) {
	root := models.Objective{ID: "root", Slug: "root", Routing: models.RouteStrategyOrdered}
	section := &ObjectiveTreeNode{
		Objective: models.Objective{
			ID: "wharf", Slug: "wharf", Title: "The Wharf", Routing: models.RouteStrategyOrdered,
		},
	}
	section.Children = []*ObjectiveTreeNode{
		{Objective: models.Objective{ID: "crane", Slug: "crane", Title: "Crane"}},
		{Objective: models.Objective{ID: "rooftop", Slug: "rooftop", Title: "Rooftop", Secret: true}},
		{Objective: models.Objective{ID: "shed", Slug: "shed", Title: "Boat Shed"}},
	}
	section.PublishedChildren = publishedCount(section.Children)
	MarkChildren(section.Objective, section.Children)
	top := []*ObjectiveTreeNode{section}
	MarkChildren(root, top)
	return root, top
}

func TestObjectiveTree_SecretsSitInAFooterOutsideTheRail(t *testing.T) {
	root, top := secretTree()
	var out strings.Builder
	require.NoError(t, ObjectiveTree(services.QuestLint{}, root, top, models.QuestSettings{}).
		Render(context.Background(), &out))
	html := out.String()

	footer := strings.Index(html, "objective-secret-footer")
	require.Positive(t, footer, "the group grows a footer")
	assert.Greater(t, strings.Index(html, ">Rooftop<"), footer, "the secret is drawn in it")
	assert.Less(t, strings.Index(html, ">Boat Shed<"), footer, "the visible rows stay above it")

	section := top[0]
	assert.Equal(t, 2, section.PublishedChildren, "the band never counts a secret")
	assert.Equal(t, RailSecret, section.Children[1].Mark)
	assert.Equal(t, 2, section.Children[2].Step, "the secret takes no step")
	assert.Contains(t, html, `data-secret-footer="true"`)
	assert.Contains(t, html, "objective-secret-toggle")
}

func TestObjectiveSecretControl(t *testing.T) {
	render := func(obj models.Objective) string {
		var out strings.Builder
		require.NoError(t, objectiveSecretControl(obj).Render(context.Background(), &out))
		return out.String()
	}

	assert.Empty(t, strings.TrimSpace(render(models.Objective{Slug: "root"})), "the root cannot be secret")

	on := render(models.Objective{Slug: "rooftop", ParentID: "root", Secret: true})
	assert.Contains(t, on, `hx-post="/admin/objective/rooftop/secret"`)
	assert.Contains(t, on, `&#34;secret&#34;:&#34;false&#34;`, "a click turns it off")
	assert.Contains(t, on, `aria-pressed="true"`)
}

// TestObjectiveTreeSecretPreview writes a browsable copy of the tree when
// RAPUA_PREVIEW names a path.
func TestObjectiveTreeSecretPreview(t *testing.T) {
	path := os.Getenv("RAPUA_PREVIEW")
	if path == "" {
		t.Skip("set RAPUA_PREVIEW to write a browsable copy of the page")
	}
	root, top := secretTree()
	var out strings.Builder
	require.NoError(t, ObjectiveTree(services.QuestLint{}, root, top, models.QuestSettings{}).
		Render(context.Background(), &out))

	wd, err := os.Getwd()
	require.NoError(t, err)
	css := "file://" + filepath.Join(wd, "..", "..", "..", "static", "css", "admin.css")
	doc := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en" data-theme="light">
<head><meta charset="utf-8"><link rel="stylesheet" href="%s"></head>
<body class="bg-base-200 min-h-screen p-6">%s</body>
</html>`, css, out.String())
	require.NoError(t, os.WriteFile(path, []byte(doc), 0o600))
}
