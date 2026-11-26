package middlewares

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
)

type AuthChecker interface {
	IsUserAuthenticated(r *http.Request) bool
}

func AuthStatusMiddleware(authService AuthChecker, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := contextkeys.UserStatus{
			IsAdminLoggedIn: authService.IsUserAuthenticated(r),
		}

		ctx := contextkeys.WithUserStatus(r.Context(), status)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
