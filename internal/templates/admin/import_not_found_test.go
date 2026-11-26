package templates

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

// The import dialog says what will happen before anything does.
func TestImportModal_SaysNothingChangesUntilConfirmed(t *testing.T) {
	html := renderComponent(t, ImportModal())

	assert.Contains(t, html, "Import a game")
	assert.Contains(t, html, "nothing changes until you confirm")
	assert.Contains(t, html, `accept=".json,application/json"`)
	assert.Contains(t, html, `hx-post="/admin/quests/import/check"`)
}

func TestImportCheckResult_RefusesAFileWithProblems(t *testing.T) {
	result := game.LintResult{Errors: []game.LintDiag{
		{Code: "SLUG_DUPLICATE", Message: "two objectives share a slug", Path: "structure.children[1]"},
		{Code: "NAME_MISSING", Message: "the game has no name", Path: "name"},
	}}

	html := renderComponent(t, ImportCheckResult(result, nil, "Everton"))

	assert.Contains(t, html, "2 problems, so this can’t be imported")
	assert.Contains(t, html, "SLUG_DUPLICATE")
	assert.Contains(t, html, "structure.children[1]", "where in the file")
	assert.NotContains(t, html, "Import as a new game", "no way to import it")

	one := renderComponent(t, ImportCheckResult(game.LintResult{Errors: result.Errors[:1]}, nil, "Everton"))
	assert.Contains(t, one, "1 problem, so", "not 1 problems")
}

func TestImportCheckResult_OffersToImportAReadyFile(t *testing.T) {
	warnings := game.LintResult{Warnings: []game.LintDiag{
		{Code: "NO_START", Message: "no start button", Path: "start"},
	}}

	fresh := renderComponent(t, ImportCheckResult(warnings, nil, "Everton"))
	assert.Contains(t, fresh, "Ready to import")
	assert.Contains(t, fresh, "Everton")
	assert.Contains(t, fresh, "1 thing to look at")
	assert.Contains(t, fresh, "These won’t stop the import.")
	assert.Contains(t, fresh, "Import as a new game")
	assert.NotContains(t, fresh, "Overwrite", "nothing to overwrite")

	match := &models.Quest{ID: "q1", Name: "Everton"}
	existing := renderComponent(t, ImportCheckResult(game.LintResult{}, match, "Everton"))
	assert.Contains(t, existing, "Import as a new game")
	assert.Contains(t, existing, "Overwrite “Everton”")
	assert.Contains(t, existing, `data-url="/admin/quests/q1/import"`)
	assert.NotContains(t, existing, "thing to look at")
}

// The admin's 404 is the site's own, with the bird that follows the cursor.
func TestNotFound_IsTheSitesOwnWithTheBird(t *testing.T) {
	html := renderComponent(t, NotFound())

	assert.Contains(t, html, "404 Not Found")
	assert.Contains(t, html, `id="404-image"`)
	assert.Contains(t, html, "removeEventListener",
		"a page change keeps the window, so the listener lets go of a bird that has left")
}
