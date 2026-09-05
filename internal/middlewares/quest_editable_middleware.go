package middlewares

import (
	"log/slog"
	"net/http"

	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/models"
)

const (
	// QuestUnlockHeader is set by the banner's unlock on every request the page
	// makes afterwards.
	QuestUnlockHeader = "X-Quest-Unlock"

	// questUnlockValue is the header value the banner sets when the author
	// confirms the unlock.
	questUnlockValue = "true"

	// QuestRunningMessage is the one wording, shared by the middleware, the
	// toast and the handlers that guard themselves.
	QuestRunningMessage = "This game is running. Stop it before making changes, " +
		"or duplicate it to work on a copy."
)

// QuestEditable reports whether a quest may be changed: it is not running, or
// the author unlocked the page knowing it is.
func QuestEditable(quest *models.Quest, r *http.Request) bool {
	return quest.GetStatus() != models.Active || r.Header.Get(QuestUnlockHeader) == questUnlockValue
}

// QuestEditableMiddleware refuses edits to a running game.
//
// A change made mid-run reaches players immediately and cannot be taken back:
// parking a section that teams are working through, deleting an objective a
// depends list names, reordering what someone is halfway along. Every one of
// those is recoverable at the desk and unrecoverable in a museum.
//
// So a game has to be stopped before it can be changed, which makes the risk a
// decision somebody takes rather than a side effect they discover. An author
// who wants to keep the running game duplicates it instead.
//
// Reads are untouched: an author may look at a running game freely.
//
// onRefused renders the message. It is passed in because the templates live
// above this package, and the middleware should not be the one place in the app
// that reports an error its own way.
func QuestEditableMiddleware(
	logger *slog.Logger, onRefused func(http.ResponseWriter, *http.Request) error, next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isReadOnlyMethod(r.Method) {
			next.ServeHTTP(w, r)
			return
		}

		user, ok := r.Context().Value(contextkeys.UserKey).(*models.User)
		if !ok || user == nil {
			next.ServeHTTP(w, r)
			return
		}
		if QuestEditable(&user.CurrentQuest, r) {
			next.ServeHTTP(w, r)
			return
		}

		logger.WarnContext(r.Context(), "edit refused: quest is running",
			"path", r.URL.Path, "quest_id", user.CurrentQuestID)

		// The app's own toast, swapped out of band into #alerts, which is how
		// every admin handler reports a refusal. It has to be a 200: htmx
		// discards the body of an error response, so a 4xx here is a click that
		// does nothing at all, which is what this used to be.
		// No status written on failure: the toast has already sent a 200 and
		// part of a body, so http.Error here corrupts it.
		if err := onRefused(w, r); err != nil {
			logger.ErrorContext(r.Context(), "edit refused: rendering the message", "error", err)
		}
	})
}

func isReadOnlyMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}
