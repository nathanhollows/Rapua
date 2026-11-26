package blocks_test

import (
	"bytes"
	"context"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression test: the active start button must be a plain form (no
// hx-post/hx-swap, which silently swallowed the redirect to /objectives)
// and must carry the CSRF token as a hidden field, since a plain form gets
// no htmx-injected X-CSRF-TOKEN header at all.
func TestStartGameButtonActive_PlainFormWithCSRFToken(t *testing.T) {
	block := blocks.StartGameButtonBlock{}

	ctx := context.WithValue(context.Background(), "gorilla.csrf.Token", "test-token-value") //nolint:staticcheck // matches the app's own layout.templ lookup key

	var buf bytes.Buffer
	require.NoError(t, templates.StartGameButtonActive(block).Render(ctx, &buf))
	html := buf.String()

	assert.Contains(t, html, `action="/start"`)
	assert.Contains(t, html, `method="post"`)
	assert.NotContains(t, html, `hx-post=`, "a plain form is required: hx-post here silently swallows the redirect")
	assert.Contains(t, html, `name="csrf"`)
	assert.Contains(t, html, "test-token-value", "the real CSRF token must be rendered into the hidden field")
}
