package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v5/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v5/models"
	"github.com/uptrace/bun/schema"
)

// TestLobbyMiddleware_PreviewRequest ensures preview requests bypass the middleware.
func TestLobbyMiddleware_PreviewRequest(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	dummyService := &dummyTeamService{}
	middleware := LobbyMiddleware(dummyService, nextHandler)

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

// TestLobbyMiddleware_NoTeamInContext tests redirection when no team is in context.
func TestLobbyMiddleware_NoTeamInContext(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	dummyService := &dummyTeamService{}
	middleware := LobbyMiddleware(dummyService, nextHandler)

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

// TestLobbyMiddleware_InvalidTeamType tests redirection when team is not of proper type.
func TestLobbyMiddleware_InvalidTeamType(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	dummyService := &dummyTeamService{}
	middleware := LobbyMiddleware(dummyService, nextHandler)

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

// TestLobbyMiddleware_NilTeamOrEmptyCode tests redirection when team is nil or has empty code.
func TestLobbyMiddleware_NilTeamOrEmptyCode(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	dummyService := &dummyTeamService{}
	middleware := LobbyMiddleware(dummyService, nextHandler)

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

// TestLobbyMiddleware_ScheduledGameRedirectsToLobby tests redirection to lobby when game status is not active.
func TestLobbyMiddleware_ScheduledGameRedirectsToLobby(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	dummyService := &dummyTeamService{}
	middleware := LobbyMiddleware(dummyService, nextHandler)

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

	if location := rr.Header().Get("Location"); location != "/lobby" {
		t.Errorf("expected redirect to /lobby, got %s", location)
	}
}

// TestLobbyMiddleware_AlreadyInLobby tests that when already in lobby, no redirection happens.
func TestLobbyMiddleware_AlreadyInLobby(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	dummyService := &dummyTeamService{}
	middleware := LobbyMiddleware(dummyService, nextHandler)

	// Create a request with team and scheduled game instance but already in lobby
	req := httptest.NewRequest(http.MethodGet, "http://example.com/lobby", nil)

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
		t.Error("expected next handler to be called when already in lobby")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("expected status code %d, got %d", http.StatusOK, rr.Code)
	}
}

// TestLobbyMiddleware_ActiveGameProceedsNormally tests that active games proceed without redirection.
func TestLobbyMiddleware_ActiveGameProceedsNormally(t *testing.T) {
	nextCalled := false

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusOK)
	})

	dummyService := &dummyTeamService{}
	middleware := LobbyMiddleware(dummyService, nextHandler)

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
