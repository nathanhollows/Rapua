package middlewares

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/sessions"
	"github.com/nathanhollows/Rapua/v8/models"
)

type AuthenticatedUserGetter interface {
	GetAuthenticatedUser(r *http.Request) (*models.User, error)
}

// InstanceLoader loads an instance with all relations needed for the admin panel.
type InstanceLoader interface {
	GetByIDWithRelations(ctx context.Context, id string) (*models.Quest, error)
}

func AdminAuthMiddleware(
	logger *slog.Logger,
	authService AuthenticatedUserGetter,
	instanceLoader InstanceLoader,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := authService.GetAuthenticatedUser(r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// OAuth signups skip email verification.
		if !user.EmailVerified && user.Provider == "" {
			http.Redirect(w, r, "/verify-email", http.StatusSeeOther)
			return
		}

		if session, err := sessions.Get(r, "admin"); err != nil {
			logger.ErrorContext(r.Context(), "AdminAuthMiddleware: getting session", "error", err)
		} else {
			if questID, ok := session.Values["current_instance"].(string); ok && questID != "" {
				instance, err := instanceLoader.GetByIDWithRelations(r.Context(), questID)
				if err == nil && instance.UserID == user.ID {
					user.CurrentQuestID = instance.ID
					user.CurrentQuest = *instance
				}
				// Invalid or unauthorized instance IDs are silently ignored;
				// AdminCheckInstanceMiddleware redirects to /admin/quests.
			}
		}

		ctx := context.WithValue(r.Context(), contextkeys.UserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func AdminCheckInstanceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(contextkeys.UserKey).(*models.User)
		if !ok || user == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		reg := regexp.MustCompile(`/admin/quests/?`)
		if reg.MatchString(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		if user.CurrentQuestID == "" {
			http.Redirect(w, r, "/admin/quests", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}
