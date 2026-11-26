package middlewares

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/httprate"
)

// RateLimitByClient allows each client up to limit requests per window and
// answers the rest with 429 from onLimit. The counter belongs to the returned
// middleware, so reuse one value to share a budget across routes.
func RateLimitByClient(limit int, window time.Duration, onLimit http.HandlerFunc) func(http.Handler) http.Handler {
	return httprate.Limit(
		limit,
		window,
		httprate.WithKeyFuncs(clientIPKey),
		httprate.WithLimitHandler(onLimit),
	)
}

// clientIPKey identifies the caller by address. X-Real-IP is believed only when
// the connection comes from a loopback or private address, which is the reverse
// proxy; anywhere else the header is the caller's own claim and would let them
// pick a fresh bucket on every request.
func clientIPKey(r *http.Request) (string, error) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer != nil && (peer.IsLoopback() || peer.IsPrivate()) {
		if real := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); real != nil {
			return real.String(), nil
		}
	}
	return host, nil
}
