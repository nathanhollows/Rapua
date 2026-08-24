package middlewares

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun/schema"
)

type runService interface {
	LoadQuest(context.Context, *models.Run) error
	GetRunByCode(context.Context, string) (*models.Run, error)
}

type questService interface {
	GetQuestSettings(context.Context, string) (*models.QuestSettings, error)
	GetByID(context.Context, string) (*models.Quest, error)
}

func PreviewMiddleware(
	logger *slog.Logger,
	_ runService,
	questService questService,
	identityService AuthenticatedUserGetter,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isPreviewRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		err := r.ParseForm()
		if err != nil {
			logger.ErrorContext(
				r.Context(),
				"preview middleware: unable to parse form",
				"err",
				err,
				"ctx",
				r.Context(),
				"url",
				r.URL,
			)
		}

		team := models.Run{
			Code:    "preview",
			Name:    "Preview",
			QuestID: extractInstanceID(r),
		}

		if team.QuestID == "" {
			logger.ErrorContext(r.Context(), "preview middleware: instance ID is empty")
			next.ServeHTTP(w, r)
			return
		}

		instance, err := questService.GetByID(r.Context(), team.QuestID)
		if err != nil {
			logger.ErrorContext(
				r.Context(),
				"preview middleware: failed to get instance",
				"err",
				err,
				"questID",
				team.QuestID,
			)
			http.Error(w, "Instance not found", http.StatusNotFound)
			return
		}

		// Templates are public; regular instances require auth and ownership.
		if !instance.IsTemplate {
			var user *models.User
			user, err = identityService.GetAuthenticatedUser(r)
			if err != nil || user.ID != instance.UserID {
				logger.WarnContext(r.Context(), "preview middleware: unauthorized access attempt",
					"questID", team.QuestID,
					"isAuthenticated", err == nil)
				http.Error(w, "Access denied", http.StatusForbidden)
				return
			}
		}

		// Load the instance settings separately to avoid team-related queries
		settings, err := questService.GetQuestSettings(r.Context(), team.QuestID)
		if err != nil {
			logger.ErrorContext(r.Context(),
				"preview middleware: failed to load instance settings",
				"err",
				err,
				"questID",
				team.QuestID,
			)
			// Fall back to default settings if loading fails
			settings = &models.QuestSettings{
				QuestID:      team.QuestID,
				EnablePoints: true, // Default to enabled for preview
			}
		}

		team.Quest = models.Quest{
			ID:        team.QuestID,
			StartTime: schema.NullTime{Time: time.Now()},
			EndTime:   schema.NullTime{Time: time.Now().Add(1 * time.Hour)},
			Settings:  *settings,
		}

		ctx := context.WithValue(r.Context(), contextkeys.RunKey, &team)
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

	return r.Header.Get("Hx-Request") == "true" &&
		(strings.HasPrefix(u.Path, "/templates") ||
			strings.HasPrefix(u.Path, "/admin"))
}

// extractInstanceID extracts the instance ID from the request.
func extractInstanceID(r *http.Request) string {
	if err := r.ParseForm(); err != nil {
		return ""
	}
	return r.Form.Get("questID")
}
