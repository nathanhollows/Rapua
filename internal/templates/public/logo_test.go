package templates

import (
	"context"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/nathanhollows/Rapua/v8/internal/templates/brand"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderPublic(t *testing.T, c templ.Component) string {
	t.Helper()
	var out strings.Builder
	require.NoError(t, c.Render(context.Background(), &out))
	return out.String()
}

// The public site wears the same logo as the admin: the chip and the name from the
// shared brand package, not a generic icon of its own.
func TestSiteHeader_WearsTheRealLogo(t *testing.T) {
	html := renderPublic(t, siteHeader(false, "/about"))

	assert.Contains(t, html, renderPublic(t, brand.Mark()), "the very same markup as the admin bar")
	assert.Contains(t, html, `class="logo" aria-label="Rapua home"`)
	assert.NotContains(t, html, `stroke-width="2.6"`, "the generic map-pin icon it replaced")
}

// Away from the home page the logo is a link home, and a link that navigates has no
// use for a hop, so neither the animation script nor a button is on the page.
func TestSiteHeader_ElsewhereTheLogoIsAPlainLink(t *testing.T) {
	for _, page := range []string{"/about", "/pricing", "/docs", "/creators", "/contact", ""} {
		html := renderPublic(t, siteHeader(false, page))

		assert.Contains(t, html, `<a href="/" class="logo" aria-label="Rapua home"`, page)
		assert.NotContains(t, html, "__logoHop", page)
		assert.NotContains(t, html, `<button type="button" class="logo"`, page)
	}
}

// On the home page there is nowhere to go, so the logo is a button that hops.
func TestSiteHeader_OnTheHomePageTheLogoHops(t *testing.T) {
	html := renderPublic(t, siteHeader(false, "/"))

	assert.Contains(t, html, `<button type="button" class="logo" aria-label="Rapua">`)
	assert.Contains(t, html, "window.__logoHop")
	assert.NotContains(t, html, `<a href="/" class="logo"`, "not a link to the page it is on")
	assert.Contains(t, html, renderPublic(t, brand.Mark()), "still the very same logo")
}

// The hop script outlives a boosted page change, so it cannot decide by the page it
// was loaded on: it must only play for a button, never for the logo link on another page.
func TestLogoHop_OnlyPlaysForAButton(t *testing.T) {
	html := renderPublic(t, brand.HopScript())

	assert.Contains(t, html, "link.tagName !== 'BUTTON'")
}
