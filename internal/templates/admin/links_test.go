package templates

import (
	"regexp"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

var anchor = regexp.MustCompile(`(?s)<a\b[^>]*>`)
var anchorHref = regexp.MustCompile(`href="(/[^"]*)"`)

// outbound is the links in a page that leave the admin for the public site or the
// player's pages, which have their own stylesheet and scripts. A boosted link swaps
// the response into the admin's own page, so it would land unstyled and half-working.
func outbound(html string) []string {
	var found []string
	for _, tag := range anchor.FindAllString(html, -1) {
		m := anchorHref.FindStringSubmatch(tag)
		if m == nil {
			continue
		}
		path := m[1]
		if strings.HasPrefix(path, "/admin") || strings.HasPrefix(path, "/static") {
			continue
		}
		// A link that opens a new window is never boosted, and one that says no is safe.
		if strings.Contains(tag, `hx-boost="false"`) || strings.Contains(tag, "target=") {
			continue
		}
		found = append(found, tag)
	}
	return found
}

// Boosting is on for the admin's nav bar, so a link out of the admin has to say no.
func TestAdminLinksOutOfTheAdminAreNotBoosted(t *testing.T) {
	signedIn := models.User{ID: "u", CurrentQuestID: "q", CurrentQuest: models.Quest{ID: "q", Name: "Everton"}}
	pages := map[string]templ.Component{
		"the page frame, with a quest":    Layout(templ.NopComponent, signedIn, "Activity", "Activity"),
		"the page frame, with no quest":   Layout(templ.NopComponent, models.User{ID: "u"}, "Quests", "Quests"),
		"the facilitator frame":           FacilitatorLayout(templ.NopComponent, "Facilitator Dashboard", false),
		"the facilitator frame for admin": FacilitatorLayout(templ.NopComponent, "Facilitator Dashboard", true),
	}

	for name, page := range pages {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, outbound(renderComponent(t, page)), "links that leave the admin must not be boosted")
		})
	}
}

// The rule has to see what it is looking for: the docs and sign-out links are really
// on the page, in the menu and again in the phone drawer.
func TestAdminLinksOutOfTheAdmin_AreFoundAtAll(t *testing.T) {
	signedIn := models.User{ID: "u", CurrentQuestID: "q", CurrentQuest: models.Quest{ID: "q", Name: "Everton"}}

	html := renderComponent(t, Layout(templ.NopComponent, signedIn, "Activity", "Activity"))

	assert.Equal(t, 2, strings.Count(html, `href="/docs/user" hx-boost="false"`))
	assert.Equal(t, 2, strings.Count(html, `href="/logout" hx-boost="false"`))
	assert.NotEmpty(t, outbound(`<a href="/logout">Sign out</a>`), "and a link that does not opt out is caught")
	assert.Empty(t, outbound(`<a href="/admin/runs">Runs</a><a href="https://example.test/x">x</a>`))
}
