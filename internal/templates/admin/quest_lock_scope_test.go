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

// The lock outlives a page under hx-boost, so it needs a marker to tell editor
// pages from the rest.
func TestLockedEditor_MarksItsOwnScope(t *testing.T) {
	var out strings.Builder
	require.NoError(t, LockedEditor(models.Quest{}, templ.Raw("<p>the page</p>")).
		Render(context.Background(), &out))

	assert.Contains(t, out.String(), "data-quest-lock-scope")
}

// Without the check, navigating to the runs page would disable its controls.
func TestQuestLockScript_ReleasesOffItsOwnPages(t *testing.T) {
	var out strings.Builder
	require.NoError(t, LockedEditor(models.Quest{}, templ.Raw("")).
		Render(context.Background(), &out))

	// Asserted against the script's source, the only form a template test can
	// reach. That is weaker than running it, so both halves are named: the
	// check that this is still an editor, and the release when it is not.
	// Asserting only the selector passed on the bug this commit fixes.
	at := strings.Index(out.String(), "htmx:afterSwap")
	require.GreaterOrEqual(t, at, 0, "no afterSwap handler to check")
	release := out.String()[at:]
	assert.Contains(t, release, "!document.querySelector('[data-quest-lock-scope]')",
		"the handler asks whether this is still an editor")
	assert.Contains(t, release, "window.questLock.set(false)",
		"and lets the lock go when it is not")
	assert.Less(t,
		strings.Index(release, "window.questLock.set(false)"),
		strings.Index(release, "window.questLock.reapply()"),
		"the release runs instead of the reapply, not after it")
}
