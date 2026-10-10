package players

import (
	"net/http"
	"strings"

	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
)

// teamNameLimit is what fits a leaderboard row on a phone without truncating
// every entry. Enforced here because a form's maxlength only binds the browser.
const teamNameLimit = 40

// trimTeamName trims space and cuts to teamNameLimit, so a request that skips
// the form cannot leave a name longer than a row can show.
func trimTeamName(raw string) string {
	name := strings.TrimSpace(raw)
	if len([]rune(name)) > teamNameLimit {
		name = strings.TrimSpace(string([]rune(name)[:teamNameLimit]))
	}
	return name
}

// TeamNameForm swaps the rename button for the field.
func (h *PlayerHandler) TeamNameForm(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	if err := templates.TeamNameField(*team).Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "TeamNameForm: rendering", "error", err)
	}
}

// TeamNamePost renames the run from its own page.
//
// Separate from the start page's handler, which saves through a block a quest
// need not have: a rename failing because the author left that block out would
// be unexplainable from here.
func (h *PlayerHandler) TeamNamePost(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "TeamNamePost: parsing form", "Could not save that name", "error", err)
		return
	}

	// An empty name is a team going back to its code, which is a thing somebody
	// may well want: the code is a name everyone already knows them by.
	team.Name = trimTeamName(r.FormValue("name"))
	if err := h.runService.Update(r.Context(), team); err != nil {
		h.handleError(w, r, "TeamNamePost: saving", "Could not save that name", "error", err)
		return
	}

	if err := templates.TeamNameControl(*team).Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "TeamNamePost: rendering", "error", err)
	}
}
