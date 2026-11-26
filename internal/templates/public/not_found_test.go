package templates

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/templates/brand"
	"github.com/stretchr/testify/assert"
)

// The public 404 and the admin's are one page: the bird is drawn once, in the shared
// package, and each site only frames it.
func TestNotFound_DrawsTheSharedBird(t *testing.T) {
	html := renderPublic(t, NotFound())

	assert.Contains(t, html, renderPublic(t, brand.NotFound()))
	assert.Contains(t, html, `<main id="main"`)
}
