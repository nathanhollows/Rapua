package templates

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func renderHome(t *testing.T, codeLength, legacyLength int) string {
	t.Helper()
	var out strings.Builder
	require.NoError(t, Home(nil, codeLength, legacyLength, false).Render(context.Background(), &out))
	return out.String()
}

// The form draws a tile per letter with a break in the middle, and tells its
// script which shorter length an older run's code still has.
func TestHome_DrawsASixLetterCodeInTwoHalves(t *testing.T) {
	html := renderHome(t, 6, 4)

	assert.Equal(t, 6, strings.Count(html, `class="home-tile"`))
	assert.Equal(t, 1, strings.Count(html, `class="home-dash"`))
	assert.Contains(t, html, `data-length="6"`)
	assert.Contains(t, html, `data-legacy="4"`)
	assert.Contains(t, html, "six-letter")
}

func TestPrettyCode(t *testing.T) {
	assert.Equal(t, "ABC-DEF", prettyCode("abcdef"))
	assert.Equal(t, "ABCD", prettyCode("abcd"))
}
