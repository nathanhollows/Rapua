package middlewares

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v5/internal/contextkeys"
)

// Interface for IsUserAuthenticated.
type AuthChecker interface {
	IsUserAuthenticated(r *http.Request) bool
}

// AuthStatusMiddleware determines and sets application-wide status.
func AuthStatusMiddleware(authService AuthChecker, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := contextkeys.UserStatus{
			IsAdminLoggedIn: authService.IsUserAuthenticated(r),
		}

		// Add status to context
		ctx := contextkeys.WithUserStatus(r.Context(), status)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
