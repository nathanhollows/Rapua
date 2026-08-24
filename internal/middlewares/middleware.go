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

func HtmxOnlyMiddleware(logger *slog.Logger, redirectPath string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Hx-Request") != "true" {
			logger.WarnContext(r.Context(), "Handler called without HTMX request", "path", r.URL.Path)
			http.Redirect(w, r, redirectPath, http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
