package middlewares

import (
	"log/slog"
	"net/http"

	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/models"
)

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
func QuestEditableMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
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
		if user.CurrentQuest.GetStatus() != models.Active {
			next.ServeHTTP(w, r)
			return
		}

		logger.WarnContext(r.Context(), "edit refused: quest is running",
			"path", r.URL.Path, "quest_id", user.CurrentQuestID)
		w.Header().Set("Hx-Trigger", `{"showMessage":{"type":"error","message":`+
			`"This game is running. Stop it before making changes, or duplicate it to work on a copy."}}`)
		http.Error(w,
			"This game is running. Stop it before making changes, or duplicate it to work on a copy.",
			http.StatusConflict)
	})
}

func isReadOnlyMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}
