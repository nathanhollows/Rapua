package middlewares

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v7/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v7/models"
)

// dummyTeamService is a stub implementation of runService.
type dummyTeamService struct{}

func (d *dummyTeamService) LoadQuest(_ context.Context, _ *models.Run) error {
	return nil
}

func (d *dummyTeamService) GetRunByCode(_ context.Context, _ string) (*models.Run, error) {
	return nil, errors.New("team not found")
}

// dummyInstanceService is a stub implementation of questService.
type dummyInstanceService struct {
	isTemplate bool
	userID     string
}

func (d *dummyInstanceService) GetQuestSettings(
	_ context.Context,
	questID string,
) (*models.QuestSettings, error) {
	return &models.QuestSettings{
		QuestID:      questID,
		EnablePoints: true,
	}, nil
}

func (d *dummyInstanceService) GetByID(
	_ context.Context,
	questID string,
) (*models.Quest, error) {
	return &models.Quest{
		ID:         questID,
		IsTemplate: d.isTemplate,
		UserID:     d.userID,
	}, nil
}

// dummyIdentityService is a stub implementation of identityService.
type dummyIdentityService struct {
	user *models.User
	err  error
}

func (d *dummyIdentityService) GetAuthenticatedUser(_ *http.Request) (*models.User, error) {
	return d.user, d.err
}

// TestPreviewMiddleware_NonPreview ensures that when the request is not a preview, the middleware simply passes the request along.
func TestPreviewMiddleware_NonPreview(t *testing.T) {
	nextCalled := false
	var receivedCtx context.Context

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		receivedCtx = r.Context() //nolint:fatcontext // Test needs to capture context
		w.WriteHeader(http.StatusOK)
	})

	dummyTeamService := &dummyTeamService{}
	dummyInstanceService := &dummyInstanceService{}
	dummyIdentityService := &dummyIdentityService{}
	middleware := PreviewMiddleware(
		slog.Default(), dummyTeamService, dummyInstanceService, dummyIdentityService, nextHandler,
	)

	// Create a request without preview headers.
	req := httptest.NewRequest(http.MethodGet, "http://example.com/", nil)
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("expected next handler to be called")
	}
	// Ensure that no preview context keys were added.
	if receivedCtx.Value(contextkeys.PreviewKey) != nil {
		t.Error("expected PreviewKey not set in context")
	}
	if receivedCtx.Value(contextkeys.RunKey) != nil {
		t.Error("expected RunKey not set in context")
	}
}

// TestPreviewMiddleware_PreviewWithoutQuestID verifies that if the preview request
// does not include an questID in its form data, the middleware does not add preview context.
func TestPreviewMiddleware_PreviewWithoutQuestID(t *testing.T) {
	nextCalled := false
	var receivedCtx context.Context

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		receivedCtx = r.Context() //nolint:fatcontext // Test needs to capture context
		w.WriteHeader(http.StatusOK)
	})

	dummyTeamService := &dummyTeamService{}
	dummyInstanceService := &dummyInstanceService{}
	dummyIdentityService := &dummyIdentityService{}
	middleware := PreviewMiddleware(
		slog.Default(), dummyTeamService, dummyInstanceService, dummyIdentityService, nextHandler,
	)

	// Create a preview request (HX-Request header is "true" and referer starts with "/templates")
	// but without an "questID" form value.
	req := httptest.NewRequest(http.MethodPost, "http://example.com/templates/some", strings.NewReader(""))
	req.Header.Set("Hx-Request", "true")
	req.Header.Set("Referer", "http://example.com/templates/some")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("expected next handler to be called")
	}
	// Since questID is empty, no preview context should be set.
	if receivedCtx.Value(contextkeys.PreviewKey) != nil {
		t.Error("expected PreviewKey not set in context due to missing questID")
	}
	if receivedCtx.Value(contextkeys.RunKey) != nil {
		t.Error("expected RunKey not set in context due to missing questID")
	}
}

// TestPreviewMiddleware_PreviewWithQuestID ensures that for a valid preview request with an questID,
// the middleware injects a team with the expected properties into the context.
func TestPreviewMiddleware_PreviewWithQuestID(t *testing.T) {
	nextCalled := false
	var receivedCtx context.Context

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		receivedCtx = r.Context() //nolint:fatcontext // Test needs to capture context
		w.WriteHeader(http.StatusOK)
	})

	dummyTeamService := &dummyTeamService{}
	// Use a template so auth is not required
	dummyInstanceService := &dummyInstanceService{isTemplate: true}
	dummyIdentityService := &dummyIdentityService{}
	middleware := PreviewMiddleware(
		slog.Default(), dummyTeamService, dummyInstanceService, dummyIdentityService, nextHandler,
	)

	// Create a preview request with a valid questID.
	form := url.Values{}
	form.Set("questID", "instance123")
	req := httptest.NewRequest(http.MethodPost, "http://example.com/admin/dashboard", strings.NewReader(form.Encode()))
	req.Header.Set("Hx-Request", "true")
	req.Header.Set("Referer", "http://example.com/admin/dashboard")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("expected next handler to be called")
	}
	// Check that PreviewKey is set in context.
	if previewVal := receivedCtx.Value(contextkeys.PreviewKey); previewVal == nil {
		t.Error("expected PreviewKey to be set in context")
	}

	// Check that RunKey is set and that its data is as expected.
	teamVal := receivedCtx.Value(contextkeys.RunKey)
	if teamVal == nil {
		t.Fatal("expected RunKey to be set in context")
	}
	team, ok := teamVal.(*models.Run)
	if !ok {
		t.Fatal("RunKey value is not of type *models.Run")
	}
	if team.Code != "preview" {
		t.Errorf("expected team.Code to be 'preview', got %q", team.Code)
	}
	if team.Name != "Preview" {
		t.Errorf("expected team.Name to be 'Preview', got %q", team.Name)
	}
	if team.QuestID != "instance123" {
		t.Errorf("expected team.QuestID to be 'instance123', got %q", team.QuestID)
	}

	// Validate instance start and end times are set roughly as expected.
	now := time.Now()
	delta := time.Minute
	start := team.Quest.StartTime.Time
	end := team.Quest.EndTime.Time
	if start.Before(now.Add(-delta)) || start.After(now.Add(delta)) {
		t.Error("team.Quest.StartTime is not within the expected range")
	}
	if end.Before(now.Add(time.Hour-delta)) || end.After(now.Add(time.Hour+delta)) {
		t.Error("team.Quest.EndTime is not within the expected range")
	}
}
