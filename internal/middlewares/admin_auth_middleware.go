package middlewares

import (
	"context"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/nathanhollows/Rapua/v7/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v7/internal/sessions"
	"github.com/nathanhollows/Rapua/v7/models"
)

type AuthenticatedUserGetter interface {
	GetAuthenticatedUser(r *http.Request) (*models.User, error)
}

// InstanceLoader loads an instance with all relations needed for the admin panel.
type InstanceLoader interface {
	GetByIDWithRelations(ctx context.Context, id string) (*models.Instance, error)
}

// AdminAuthMiddleware ensures the user is authenticated and has verified their email.
// It also loads the current instance from a cookie and populates the user struct.
func AdminAuthMiddleware(authService AuthenticatedUserGetter, instanceLoader InstanceLoader, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Make sure the user is authenticated
		user, err := authService.GetAuthenticatedUser(r)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Redirect to verify email if the user hasn't verified their email
		// and they didn't sign up with OAuth
		if !user.EmailVerified && user.Provider == "" {
			http.Redirect(w, r, "/verify-email", http.StatusSeeOther)
			return
		}

		// Load current instance from session
		if session, err := sessions.Get(r, "admin"); err != nil {
			slog.Error("AdminAuthMiddleware: getting session", "error", err)
		} else {
			if instanceID, ok := session.Values["current_instance"].(string); ok && instanceID != "" {
				instance, err := instanceLoader.GetByIDWithRelations(r.Context(), instanceID)
				if err == nil && instance.UserID == user.ID {
					user.CurrentInstanceID = instance.ID
					user.CurrentInstance = *instance
				}
				// Invalid/unauthorized instance ID is silently ignored;
				// AdminCheckInstanceMiddleware will redirect to /admin/instances
			}
		}

		// Add the user to the context
		ctx := context.WithValue(r.Context(), contextkeys.UserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminCheckInstanceMiddleware ensures the user has an instance selected.
func AdminCheckInstanceMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(contextkeys.UserKey).(*models.User)
		if !ok || user == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		// Check if the route contains /admin/instances
		reg := regexp.MustCompile(`/admin/instances/?`)
		if reg.MatchString(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		if user.CurrentInstanceID == "" {
			// flash.Message{
			// 	Title:   "Error",
			// 	Message: "Please select an instance to continue",
			// 	Style:   flash.Error,
			// }.Save(w, r)
			http.Redirect(w, r, "/admin/instances", http.StatusSeeOther)
			return
		}

		next.ServeHTTP(w, r)
	})
}
