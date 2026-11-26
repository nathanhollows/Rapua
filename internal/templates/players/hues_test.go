package templates

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHueFor_WrapsAndIsDeterministic(t *testing.T) {
	assert.Equal(t, questHues[0], HueFor(0))
	assert.Equal(t, questHues[0], HueFor(len(questHues)))
	assert.Equal(t, HueFor(3), HueFor(3))
	assert.Equal(t, questHues[len(questHues)-1], HueFor(-1), "a negative position still lands in the palette")
}

func TestQuestHues_AreDistinct(t *testing.T) {
	for i, a := range questHues {
		for _, b := range questHues[i+1:] {
			diff := math.Abs(float64(a - b))
			diff = math.Min(diff, 360-diff)
			assert.GreaterOrEqual(t, diff, 40.0, "hues %d and %d are too close to tell apart", a, b)
		}
	}
}

// oklch to linear sRGB, clipped to gamut as a browser does.
func oklchToRGB(l, c, hDeg float64) [3]float64 {
	a := c * math.Cos(hDeg*math.Pi/180)
	b := c * math.Sin(hDeg*math.Pi/180)
	l_ := math.Pow(l+0.3963377774*a+0.2158037573*b, 3)
	m_ := math.Pow(l-0.1055613458*a-0.0638541728*b, 3)
	s_ := math.Pow(l-0.0894841775*a-1.291485548*b, 3)
	rgb := [3]float64{
		4.0767416621*l_ - 3.3077115913*m_ + 0.2309699292*s_,
		-1.2684380046*l_ + 2.6097574011*m_ - 0.3413193965*s_,
		-0.0041960863*l_ + 0.7034186147*m_ + 1.707614701*s_,
	}
	for i, v := range rgb {
		rgb[i] = math.Max(0, math.Min(1, v))
	}
	return rgb
}

func luminance(rgb [3]float64) float64 {
	return 0.2126*rgb[0] + 0.7152*rgb[1] + 0.0722*rgb[2]
}

func contrast(a, b [3]float64) float64 {
	la, lb := luminance(a), luminance(b)
	if la < lb {
		la, lb = lb, la
	}
	return (la + 0.05) / (lb + 0.05)
}

// The first .player block is light and the one in the dark media query is dark.
func playerTokens(t *testing.T) (light, dark map[string]float64) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "static", "css", "src", "player.css"))
	require.NoError(t, err)
	css := string(raw)

	block := regexp.MustCompile(`(?s)\n\.player \{(.*?)\n\s*\}`)
	token := regexp.MustCompile(`--(tl|tc|el|ec|il|ic|iol):\s*([0-9.]+)%?`)
	read := func(body string) map[string]float64 {
		out := map[string]float64{}
		for _, m := range token.FindAllStringSubmatch(body, -1) {
			v, err := strconv.ParseFloat(m[2], 64)
			require.NoError(t, err)
			out[m[1]] = v
		}
		return out
	}
	matches := block.FindAllStringSubmatch(css, -1)
	require.Len(t, matches, 1, "expected one top-level .player block")
	light = read(matches[0][1])

	darkBlock := regexp.MustCompile(`(?s)prefers-color-scheme: dark\) \{\s*\.player \{(.*?)\n\s*\}`)
	dm := darkBlock.FindStringSubmatch(css)
	require.NotNil(t, dm, "no dark .player block in player.css")
	merged := map[string]float64{}
	for k, v := range light {
		merged[k] = v
	}
	for k, v := range read(dm[1]) {
		merged[k] = v
	}
	return light, merged
}

// The admin picks its theme by attribute: a mismatch would preview colours the
// game never shows.
func TestPlayerTokens_AttributeBlocksMatchTheirMediaQuery(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "static", "css", "src", "player.css"))
	require.NoError(t, err)
	css := string(raw)
	token := regexp.MustCompile(`--(?:tl|tc|el|ec|il|ic|iol|dl|al-[iwse]-bg|al-on|color-base-[123]):\s*[^;]+;`)

	media := regexp.MustCompile(`(?s)prefers-color-scheme: dark\) \{\s*\.player \{(.*?)\n\s*\}`).FindStringSubmatch(css)
	attr := regexp.MustCompile(`(?s)\[data-theme="dracula"\] \.player \{(.*?)\n\}`).FindStringSubmatch(css)
	require.NotNil(t, media)
	require.NotNil(t, attr)

	want := token.FindAllString(media[1], -1)
	assert.NotEmpty(t, want)
	assert.ElementsMatch(t, want, token.FindAllString(attr[1], -1))
}

// 7:1 because a glyph is small and drawn thin.
func TestQuestHues_MeetContrastFloors(t *testing.T) {
	light, dark := playerTokens(t)
	inkLight := oklchToRGB(0.23574, 0.066, 313.189)
	inkDark := oklchToRGB(0.97747, 0.007, 106.545)

	cases := []struct {
		name string
		tok  map[string]float64
		ink  [3]float64
	}{{"light", light, inkLight}, {"dark", dark, inkDark}}

	for _, c := range cases {
		for _, hue := range questHues {
			h := float64(hue)
			tile := oklchToRGB(c.tok["tl"]/100, c.tok["tc"], h)
			assert.GreaterOrEqual(t, contrast(tile, c.ink), 7.0, "%s hue %d: text on its tile", c.name, hue)

			icon := oklchToRGB(c.tok["il"]/100, c.tok["ic"], h)
			glyph := oklchToRGB(c.tok["iol"]/100, 0.06, h)
			assert.GreaterOrEqual(t, contrast(icon, glyph), 7.0, "%s hue %d: icon glyph on its tile", c.name, hue)
		}
	}
}

// Read from the CSS so it cannot drift from what ships.
func TestPlayerAlerts_MeetContrastInBothThemes(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "static", "css", "src", "player.css"))
	require.NoError(t, err)
	css := string(raw)

	oklch := regexp.MustCompile(`--(al-[iwse]-bg|al-on):\s*oklch\(\s*([0-9.]+)%\s+([0-9.]+)\s+([0-9.]+)\)`)
	read := func(block string) map[string][3]float64 {
		out := map[string][3]float64{}
		for _, m := range oklch.FindAllStringSubmatch(block, -1) {
			l, _ := strconv.ParseFloat(m[2], 64)
			c, _ := strconv.ParseFloat(m[3], 64)
			h, _ := strconv.ParseFloat(m[4], 64)
			out[m[1]] = [3]float64{l / 100, c, h}
		}
		return out
	}

	base := regexp.MustCompile(`(?s)\n\.player \{(.*?)\n\}`).FindStringSubmatch(css)
	dark := regexp.MustCompile(`(?s)prefers-color-scheme: dark\) \{\s*\.player \{(.*?)\n\s*\}`).FindStringSubmatch(css)
	require.NotNil(t, base)
	require.NotNil(t, dark)

	for name, tokens := range map[string]map[string][3]float64{"light": read(base[1]), "dark": read(dark[1])} {
		require.Len(t, tokens, 5, "%s: four faces and the text colour", name)
		on := tokens["al-on"]
		text := oklchToRGB(on[0], on[1], on[2])
		for _, face := range []string{"al-i-bg", "al-w-bg", "al-s-bg", "al-e-bg"} {
			f := tokens[face]
			assert.GreaterOrEqual(t, contrast(oklchToRGB(f[0], f[1], f[2]), text), 7.0, "%s %s", name, face)
		}
	}
}
