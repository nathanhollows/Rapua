package players

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
	"github.com/nathanhollows/Rapua/v8/models"
)

// Journal is /journal's handler: the run's diary, built from its event stream.
// The feed is history; the chrome's bar is the present, so both are loaded
// together and never conflated.
func (h *PlayerHandler) Journal(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil || team == nil {
		http.Redirect(w, r, "/play", http.StatusFound)
		return
	}

	if r.Context().Value(contextkeys.PreviewKey) != nil {
		h.journalPreview(w, r, team)
		return
	}

	err = h.runService.LoadRelations(r.Context(), team)
	if err != nil {
		// A blind Referer redirect loops forever when Referer is empty (direct
		// nav, htmx requests): a toast fails safely instead.
		h.handleError(w, r, "Journal: loading team relations", "Error loading journal", "error", err.Error())
		return
	}

	view, err := h.navigationService.GetJournal(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "Journal: building the diary", "Error loading journal", "error", err.Error())
		return
	}

	data := templates.JournalParams{Run: *team, View: view, Show: r.URL.Query().Get("show")}
	err = templates.AppLayout(
		templates.Journal(data), templates.JournalChrome(*team, view), "Journal", team.Messages,
	).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "rendering journal", "error", err.Error())
	}
}

// journalPreview renders the diary's shell for the editor's phone frame. A
// preview has no run, so there is no history to draw and the empty state is
// the honest picture.
func (h *PlayerHandler) journalPreview(w http.ResponseWriter, r *http.Request, team *models.Run) {
	err := h.runService.LoadQuest(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "JournalPreview: loading quest", "Error loading quest", "error", err)
		return
	}

	err = templates.Journal(templates.JournalParams{Run: *team}).Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "JournalPreview: rendering template", "Error rendering template", "error", err)
	}
}
