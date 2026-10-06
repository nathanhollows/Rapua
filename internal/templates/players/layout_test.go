package templates

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func render(t *testing.T, c templ.Component) string {
	t.Helper()
	var out strings.Builder
	require.NoError(t, c.Render(context.Background(), &out))
	return out.String()
}

// The dock shows even on a page that is none of its three destinations.
func TestAppLayout_AlwaysCarriesTheDock(t *testing.T) {
	body := templ.Raw("")
	run := models.Run{Quest: models.Quest{Name: "Trace"}}

	for _, active := range []string{"quest", "journal", "team", ""} {
		html := render(t, AppLayout(body, AppChrome{Run: run, Active: active}, "T", nil))
		assert.Contains(t, html, `class="dock`, "active %q still gets a dock", active)
		assert.Contains(t, html, `href="/objectives"`)
		assert.Contains(t, html, `href="/journal"`)
		assert.Contains(t, html, `href="/team"`)
	}
}

// A page on none of the tabs lights nothing rather than guessing.
func TestAppLayout_LightsOnlyTheActiveTab(t *testing.T) {
	run := models.Run{Quest: models.Quest{Name: "Trace"}}

	html := render(t, AppLayout(templ.Raw(""), AppChrome{Run: run, Active: "journal"}, "T", nil))
	assert.Equal(t, 1, strings.Count(html, "dock-active"))

	html = render(t, AppLayout(templ.Raw(""), AppChrome{Run: run, Active: ""}, "T", nil))
	assert.NotContains(t, html, "dock-active")
}

// Before a run exists there is nowhere to navigate to.
func TestLayout_HasNoDockBeforeTheRunStarts(t *testing.T) {
	html := render(t, Layout(templ.Raw(""), "Join", nil))
	assert.NotContains(t, html, `class="dock`)
}
