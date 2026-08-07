package middlewares

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/sessions"
)

// RunMiddleware extracts the run code from the session and finds the matching quest.
func RunMiddleware(logger *slog.Logger, runService runService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Preview requests should pass through
		if r.Context().Value(contextkeys.PreviewKey) != nil {
			next.ServeHTTP(w, r)
			return
		}

		// Extract the session
		session, err := sessions.Get(r, "scanscout")
		if err != nil {
			logger.ErrorContext(r.Context(), "getting session: ", "err", err, "ctx", r.Context())
			next.ServeHTTP(w, r)
			return
		}

		// Extract run code from session
		runCode, ok := session.Values["run"].(string)
		if !ok || runCode == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Find the matching run
		run, err := runService.GetRunByCode(r.Context(), runCode)
		if err != nil {
			logger.ErrorContext(r.Context(), "finding run by code: ", "err", err, "runCode", runCode)
			next.ServeHTTP(w, r)
			return
		}

		err = runService.LoadQuest(r.Context(), run)
		if err != nil {
			logger.ErrorContext(r.Context(), "loading relations: ", "err", err)
			next.ServeHTTP(w, r)
			return
		}

		// Add run to context
		ctx := context.WithValue(r.Context(), contextkeys.RunKey, run)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
