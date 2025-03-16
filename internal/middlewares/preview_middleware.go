package middlewares

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/uptrace/bun/schema"
)

type loadRelation interface {
	LoadRelation(context.Context, *models.Team, string) error
}

// PreviewMiddleware sets up a team instance for previewing the game and sets the Preview flag in the context.
func PreviewMiddleware(teamService loadRelation, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isPreviewRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		r.ParseForm()

		team := models.Team{
			Code:       "preview",
			Name:       "Preview",
			InstanceID: extractInstanceID(r),
		}

		if team.InstanceID == "" {
			slog.Error("preview middleware: instance ID is empty")
			next.ServeHTTP(w, r)
			return
		}

		team.Instance = models.Instance{
			ID:        team.InstanceID,
			StartTime: schema.NullTime{Time: time.Now()},
			EndTime:   schema.NullTime{Time: time.Now().Add(1 * time.Hour)},
		}

		team.Instance.StartTime = schema.NullTime{Time: time.Now()}
		team.Instance.EndTime = schema.NullTime{Time: time.Now().Add(1 * time.Hour)}

		ctx := context.WithValue(r.Context(), contextkeys.TeamKey, &team)
		ctx = context.WithValue(ctx, contextkeys.PreviewKey, true)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// isPreviewRequest checks if the request is for previewing the game and the originator is HTMX.
func isPreviewRequest(r *http.Request) bool {
	u, err := url.Parse(r.Header.Get("Referer"))
	if err != nil {
		return false
	}

	return r.Header.Get("HX-Request") == "true" &&
		(strings.HasPrefix(u.Path, "/templates") ||
			strings.HasPrefix(u.Path, "/admin"))
}

// extractInstanceID extracts the instance ID from the request.
func extractInstanceID(r *http.Request) string {
	if err := r.ParseForm(); err != nil {
		return ""
	}
	return r.Form.Get("instanceID")
}
