package templates

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The themes live in CSS, but the contract is the templates': every colour
// class they reach for (bg-primary/5, border-primary/40, text-warning) resolves
// through these tokens, and a token with no contrast against its own ground
// renders the class invisible rather than wrong. The light and dark themes were
// written with identical brand colours, which can only suit one of them.

var themeBlockPattern = regexp.MustCompile(`(?s)@plugin "daisyui/theme" \{(.*?)\n\}`)

var colourPattern = regexp.MustCompile(`--color-([a-z0-9-]+):\s*oklch\(\s*([0-9.]+)%`)

// brandTokens are the accents a page draws with. The base and neutral ramps are
// surfaces, and are expected to sit close to the ground rather than against it.
var brandTokens = []string{"primary", "secondary", "accent", "info", "success", "error", "warning"}

// minLightnessGap is how far an accent has to sit from the surface it is drawn
// on, in oklch lightness points. Below this a 1.5px rail or a tinted border
// stops being a colour and becomes a rumour.
const minLightnessGap = 25.0

type theme struct {
	name      string
	light     bool
	lightness map[string]float64
}

func parseThemes(t *testing.T) []theme {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "static", "css", "main.css"))
	require.NoError(t, err)

	var themes []theme
	for _, block := range themeBlockPattern.FindAllStringSubmatch(string(raw), -1) {
		body := block[1]
		parsed := theme{light: strings.Contains(body, `color-scheme: "light"`), lightness: map[string]float64{}}
		if name := regexp.MustCompile(`name:\s*"([^"]+)"`).FindStringSubmatch(body); name != nil {
			parsed.name = name[1]
		}
		for _, decl := range colourPattern.FindAllStringSubmatch(body, -1) {
			value, err := strconv.ParseFloat(decl[2], 64)
			require.NoError(t, err)
			parsed.lightness[decl[1]] = value
		}
		themes = append(themes, parsed)
	}
	require.NotEmpty(t, themes, "no daisyUI theme blocks found in main.css")
	return themes
}

// An accent has to contrast with the surface its own theme puts behind it. A
// light theme needs dark accents and a dark theme needs light ones, so the two
// cannot share a palette however much the hues are meant to match.
func TestTheme_AccentsContrastWithTheirOwnBase(t *testing.T) {
	for _, theme := range parseThemes(t) {
		base, ok := theme.lightness["base-100"]
		require.True(t, ok, "theme %q declares no base-100", theme.name)

		for _, token := range brandTokens {
			accent, ok := theme.lightness[token]
			require.True(t, ok, "theme %q declares no %s", theme.name, token)

			gap := accent - base
			if theme.light {
				assert.LessOrEqual(t, gap, -minLightnessGap,
					"%s: --color-%s is L%.1f on an L%.1f ground, which is too pale to see",
					theme.name, token, accent, base)
				continue
			}
			assert.GreaterOrEqual(t, gap, minLightnessGap,
				"%s: --color-%s is L%.1f on an L%.1f ground, which is too dark to see",
				theme.name, token, accent, base)
		}
	}
}

// Text drawn on a filled accent has to clear the accent, not the page. Getting
// the accent right and leaving its content behind swaps one unreadable pairing
// for another.
func TestTheme_AccentContentContrastsWithItsAccent(t *testing.T) {
	for _, theme := range parseThemes(t) {
		for _, token := range brandTokens {
			accent, ok := theme.lightness[token]
			require.True(t, ok, "theme %q declares no %s", theme.name, token)
			content, ok := theme.lightness[token+"-content"]
			require.True(t, ok, "theme %q declares no %s-content", theme.name, token)

			gap := content - accent
			if gap < 0 {
				gap = -gap
			}
			assert.GreaterOrEqual(t, gap, minLightnessGap+15,
				"%s: --color-%s-content is L%.1f on an L%.1f fill",
				theme.name, token, content, accent)
		}
	}
}

// A settings picker must not carry a Tailwind display utility, because .hidden
// and .inline-block are single-class utilities in one layer and Tailwind emits
// .inline-block second: the element then cannot be hidden, by markup or by
// classList.toggle. The stylesheet owns the picker's display instead, in a pair
// scoped to the picker so nothing else in the app is affected.
func TestSettingsPicker_DisplayIsOwnedByTheStylesheet(t *testing.T) {
	markup, err := os.ReadFile("objective_tree.templ")
	require.NoError(t, err)
	for _, line := range strings.Split(string(markup), "\n") {
		if strings.Contains(line, "settings-picker") {
			assert.NotContains(t, line, "inline-block",
				"a display utility here outranks .hidden and the picker stops hiding")
		}
	}

	css, err := os.ReadFile(filepath.Join("..", "..", "..", "static", "css", "main.css"))
	require.NoError(t, err)
	assert.Contains(t, string(css), ".settings-picker.hidden",
		"the hidden state has to be compound, or the picker's own rule outranks it")
}
