package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func limited(limit int) http.Handler {
	onLimit := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusTooManyRequests) }
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	return RateLimitByClient(limit, time.Minute, onLimit)(ok)
}

func hit(h http.Handler, remote, realIP string) int {
	req := httptest.NewRequest(http.MethodPost, "/play/check", nil)
	req.RemoteAddr = remote
	if realIP != "" {
		req.Header.Set("X-Real-IP", realIP)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code
}

func TestRateLimitByClientBlocksAfterLimit(t *testing.T) {
	h := limited(10)
	for i := 1; i <= 10; i++ {
		if got := hit(h, "203.0.113.5:4000", ""); got != http.StatusOK {
			t.Fatalf("request %d: got %d, want 200", i, got)
		}
	}
	if got := hit(h, "203.0.113.5:4001", ""); got != http.StatusTooManyRequests {
		t.Fatalf("request 11: got %d, want 429", got)
	}
}

func TestRateLimitByClientKeepsClientsApart(t *testing.T) {
	h := limited(1)
	hit(h, "203.0.113.5:4000", "")
	if got := hit(h, "203.0.113.6:4000", ""); got != http.StatusOK {
		t.Fatalf("second client got %d, want 200", got)
	}
}

func TestRateLimitByClientIgnoresSpoofedHeader(t *testing.T) {
	h := limited(1)
	hit(h, "203.0.113.5:4000", "198.51.100.1")
	if got := hit(h, "203.0.113.5:4000", "198.51.100.2"); got != http.StatusTooManyRequests {
		t.Fatalf("spoofed X-Real-IP escaped the limit: got %d, want 429", got)
	}
}

func TestRateLimitByClientTrustsProxyHeader(t *testing.T) {
	h := limited(1)
	hit(h, "127.0.0.1:4000", "198.51.100.1")
	if got := hit(h, "127.0.0.1:4001", "198.51.100.2"); got != http.StatusOK {
		t.Fatalf("different client behind the proxy got %d, want 200", got)
	}
	if got := hit(h, "127.0.0.1:4002", "198.51.100.1"); got != http.StatusTooManyRequests {
		t.Fatalf("same client behind the proxy got %d, want 429", got)
	}
}
