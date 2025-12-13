package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v6/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/uptrace/bun/schema"
)

// TestStartMiddleware_PreviewRequest ensures preview requests bypass the middleware.
func TestStartMiddleware_PreviewRequest(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	dummyService := &dummyTeamService{}
	middleware := StartMiddleware(dummyService, nextHandler)

	// Create a request with preview context
	req := httptest.NewRequest(http.MethodGet, "http://example.com/next", nil)
	ctx := context.WithValue(req.Context(), contextkeys.PreviewKey, true)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("expected next handler to be called for preview requests")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, rr.Code)
	}
}

// TestStartMiddleware_NoTeamInContext tests redirection when no team is in context.
func TestStartMiddleware_NoTeamInContext(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	dummyService := &dummyTeamService{}
	middleware := StartMiddleware(dummyService, nextHandler)

	// Create a request without team in context
	req := httptest.NewRequest(http.MethodGet, "http://example.com/next", nil)
	rr := httptest.NewRecorder()

	middleware.ServeHTTP(rr, req)

	if nextCalled {
		t.Error("expected next handler not to be called when no team in context")
	}

	if rr.Code != http.StatusFound {
		t.Errorf("expected status code %d, got %d", http.StatusFound, rr.Code)
	}

	if location := rr.Header().Get("Location"); location != "/play" {
		t.Errorf("expected redirect to /play, got %s", location)
	}
}

// TestStartMiddleware_InvalidTeamType tests redirection when team is not of proper type.
func TestStartMiddleware_InvalidTeamType(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	dummyService := &dummyTeamService{}
	middleware := StartMiddleware(dummyService, nextHandler)

	// Create a request with wrong team type in context
	req := httptest.NewRequest(http.MethodGet, "http://example.com/next", nil)
	ctx := context.WithValue(req.Context(), contextkeys.TeamKey, "not a team object")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if nextCalled {
		t.Error("expected next handler not to be called with invalid team type")
	}

	if rr.Code != http.StatusFound {
		t.Errorf("expected status code %d, got %d", http.StatusFound, rr.Code)
	}

	if location := rr.Header().Get("Location"); location != "/play" {
		t.Errorf("expected redirect to /play, got %s", location)
	}
}

// TestStartMiddleware_NilTeamOrEmptyCode tests redirection when team is nil or has empty code.
func TestStartMiddleware_NilTeamOrEmptyCode(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	dummyService := &dummyTeamService{}
	middleware := StartMiddleware(dummyService, nextHandler)

	// Create a request with team having empty code
	req := httptest.NewRequest(http.MethodGet, "http://example.com/next", nil)
	team := &models.Team{Code: ""}
	ctx := context.WithValue(req.Context(), contextkeys.TeamKey, team)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if nextCalled {
		t.Error("expected next handler not to be called with empty team code")
	}

	if rr.Code != http.StatusFound {
		t.Errorf("expected status code %d, got %d", http.StatusFound, rr.Code)
	}

	if location := rr.Header().Get("Location"); location != "/play" {
		t.Errorf("expected redirect to /play, got %s", location)
	}
}

// TestStartMiddleware_ScheduledGameRedirectsToStart tests redirection to Start when game status is not active.
func TestStartMiddleware_ScheduledGameRedirectsToStart(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	dummyService := &dummyTeamService{}
	middleware := StartMiddleware(dummyService, nextHandler)

	// Create a request with team and scheduled game instance
	req := httptest.NewRequest(http.MethodGet, "http://example.com/next", nil)

	// Mock instance with non-active status
	instance := models.Instance{Status: models.Scheduled}
	team := &models.Team{
		Code:     "team123",
		Instance: instance,
	}

	ctx := context.WithValue(req.Context(), contextkeys.TeamKey, team)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if nextCalled {
		t.Error("expected next handler not to be called for scheduled game")
	}

	if rr.Code != http.StatusFound {
		t.Errorf("expected status code %d, got %d", http.StatusFound, rr.Code)
	}

	if location := rr.Header().Get("Location"); location != "/start" {
		t.Errorf("expected redirect to /start, got %s", location)
	}
}

// TestStartMiddleware_AlreadyInStart tests that when already in Start, no redirection happens.
func TestStartMiddleware_AlreadyInStart(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	dummyService := &dummyTeamService{}
	middleware := StartMiddleware(dummyService, nextHandler)

	// Create a request with team and scheduled game instance but already in Start
	req := httptest.NewRequest(http.MethodGet, "http://example.com/start", nil)

	// Mock instance with non-active status
	instance := models.Instance{Status: models.Scheduled}
	team := &models.Team{
		Code:     "team123",
		Instance: instance,
	}

	ctx := context.WithValue(req.Context(), contextkeys.TeamKey, team)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("expected next handler to be called when already in start")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, rr.Code)
	}
}

// TestStartMiddleware_ActiveGameProceedsNormally tests that active games proceed without redirection.
func TestStartMiddleware_ActiveGameProceedsNormally(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	dummyService := &dummyTeamService{}
	middleware := StartMiddleware(dummyService, nextHandler)

	// Create a request with team and active game instance
	req := httptest.NewRequest(http.MethodGet, "http://example.com/next", nil)

	// Mock instance with active status
	instance := models.Instance{
		StartTime: schema.NullTime{Time: time.Now()},
		EndTime:   schema.NullTime{Time: time.Now().Add(1 * time.Hour)},
	}
	team := &models.Team{
		Code:     "team123",
		Instance: instance,
	}

	ctx := context.WithValue(req.Context(), contextkeys.TeamKey, team)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("expected next handler to be called for active game")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, rr.Code)
	}
}
