package middlewares

import (
	"log/slog"
	"net/http"
)

func TextHTMLMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		next.ServeHTTP(w, r)
	})
}

// HtmxOnlyMiddleware ensures that a handler is only accessible via HTMX requests.
// If the request is not an HTMX request, it will be redirected to the provided path.
// This is useful for handlers that should only be accessed via HTMX, such as partial templates.
func HtmxOnlyMiddleware(logger *slog.Logger, redirectPath string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Hx-Request") != "true" {
			logger.Warn("Handler called without HTMX request", "path", r.URL.Path)
			http.Redirect(w, r, redirectPath, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
