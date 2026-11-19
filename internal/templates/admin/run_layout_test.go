package templates_test

import (
	"strings"
	"testing"

	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

// Media sits below the timeline and only exists when there are uploads.
func TestTeamOverview_ShowsMediaBelowTheTimelineOnlyWhenThereIsSome(t *testing.T) {
	with := runViewFixture()
	with.Uploads = []*models.Upload{{Type: models.MediaTypeImage, OriginalURL: "https://example.com/a.jpg"}}
	html := render(t, templates.TeamOverview(with))

	assert.Contains(t, html, "Uploaded media")
	assert.Contains(t, html, `<span class="badge badge-ghost badge-sm">1</span>`)
	assert.Less(t, strings.Index(html, "Timeline"), strings.Index(html, "Uploaded media"))
	assert.NotContains(t, html, "Media (1)", "no tab for it")

	assert.NotContains(t, render(t, templates.TeamOverview(runViewFixture())), "Uploaded media")
}
