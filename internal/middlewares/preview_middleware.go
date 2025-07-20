package middlewares

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nathanhollows/Rapua/v4/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/uptrace/bun/schema"
)

type teamService interface {
	LoadRelation(context.Context, *models.Team, string) error
	GetTeamByCode(context.Context, string) (*models.Team, error)
}

type instanceService interface {
	GetInstanceSettings(context.Context, string) (*models.InstanceSettings, error)
}

// PreviewMiddleware sets up a team instance for previewing the game and sets the Preview flag in the context.
func PreviewMiddleware(teamService teamService, instanceService instanceService, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isPreviewRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		err := r.ParseForm()
		if err != nil {
			slog.Error("preview middleware: unable to parse form", "err", err, "ctx", r.Context(), "url", r.URL)
		}

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

		// Load the instance settings separately to avoid team-related queries
		settings, err := instanceService.GetInstanceSettings(r.Context(), team.InstanceID)
		if err != nil {
			slog.Error("preview middleware: failed to load instance settings", "err", err, "instanceID", team.InstanceID)
			// Fall back to default settings if loading fails
			settings = &models.InstanceSettings{
				InstanceID:   team.InstanceID,
				EnablePoints: true, // Default to enabled for preview
			}
		}

		team.Instance = models.Instance{
			ID:        team.InstanceID,
			StartTime: schema.NullTime{Time: time.Now()},
			EndTime:   schema.NullTime{Time: time.Now().Add(1 * time.Hour)},
			Settings:  *settings,
		}

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
