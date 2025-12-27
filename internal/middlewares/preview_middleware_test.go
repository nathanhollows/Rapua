package middlewares

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v6/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v6/models"
)

// dummyTeamService is a stub implementation of teamService.
type dummyTeamService struct{}

func (d *dummyTeamService) LoadRelation(ctx context.Context, t *models.Team, rel string) error {
	return nil
}

func (d *dummyTeamService) GetTeamByCode(ctx context.Context, code string) (*models.Team, error) {
	return nil, errors.New("team not found")
}

// dummyInstanceService is a stub implementation of instanceService.
type dummyInstanceService struct {
	isTemplate bool
	userID     string
}

func (d *dummyInstanceService) GetInstanceSettings(
	ctx context.Context,
	instanceID string,
) (*models.InstanceSettings, error) {
	return &models.InstanceSettings{
		InstanceID:   instanceID,
		EnablePoints: true,
	}, nil
}

func (d *dummyInstanceService) GetByID(
	ctx context.Context,
	instanceID string,
) (*models.Instance, error) {
	return &models.Instance{
		ID:         instanceID,
		IsTemplate: d.isTemplate,
		UserID:     d.userID,
	}, nil
}

// dummyIdentityService is a stub implementation of identityService.
type dummyIdentityService struct {
	user *models.User
	err  error
}

func (d *dummyIdentityService) GetAuthenticatedUser(r *http.Request) (*models.User, error) {
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
	middleware := PreviewMiddleware(dummyTeamService, dummyInstanceService, dummyIdentityService, nextHandler)

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
	if receivedCtx.Value(contextkeys.TeamKey) != nil {
		t.Error("expected TeamKey not set in context")
	}
}

// TestPreviewMiddleware_PreviewWithoutInstanceID verifies that if the preview request
// does not include an instanceID in its form data, the middleware does not add preview context.
func TestPreviewMiddleware_PreviewWithoutInstanceID(t *testing.T) {
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
	middleware := PreviewMiddleware(dummyTeamService, dummyInstanceService, dummyIdentityService, nextHandler)

	// Create a preview request (HX-Request header is "true" and referer starts with "/templates")
	// but without an "instanceID" form value.
	req := httptest.NewRequest(http.MethodPost, "http://example.com/templates/some", strings.NewReader(""))
	req.Header.Set("Hx-Request", "true")
	req.Header.Set("Referer", "http://example.com/templates/some")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("expected next handler to be called")
	}
	// Since instanceID is empty, no preview context should be set.
	if receivedCtx.Value(contextkeys.PreviewKey) != nil {
		t.Error("expected PreviewKey not set in context due to missing instanceID")
	}
	if receivedCtx.Value(contextkeys.TeamKey) != nil {
		t.Error("expected TeamKey not set in context due to missing instanceID")
	}
}

// TestPreviewMiddleware_PreviewWithInstanceID ensures that for a valid preview request with an instanceID,
// the middleware injects a team with the expected properties into the context.
func TestPreviewMiddleware_PreviewWithInstanceID(t *testing.T) {
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
	middleware := PreviewMiddleware(dummyTeamService, dummyInstanceService, dummyIdentityService, nextHandler)

	// Create a preview request with a valid instanceID.
	form := url.Values{}
	form.Set("instanceID", "instance123")
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

	// Check that TeamKey is set and that its data is as expected.
	teamVal := receivedCtx.Value(contextkeys.TeamKey)
	if teamVal == nil {
		t.Fatal("expected TeamKey to be set in context")
	}
	team, ok := teamVal.(*models.Team)
	if !ok {
		t.Fatal("TeamKey value is not of type *models.Team")
	}
	if team.Code != "preview" {
		t.Errorf("expected team.Code to be 'preview', got %q", team.Code)
	}
	if team.Name != "Preview" {
		t.Errorf("expected team.Name to be 'Preview', got %q", team.Name)
	}
	if team.InstanceID != "instance123" {
		t.Errorf("expected team.InstanceID to be 'instance123', got %q", team.InstanceID)
	}

	// Validate instance start and end times are set roughly as expected.
	now := time.Now()
	delta := time.Minute
	start := team.Instance.StartTime.Time
	end := team.Instance.EndTime.Time
	if start.Before(now.Add(-delta)) || start.After(now.Add(delta)) {
		t.Error("team.Instance.StartTime is not within the expected range")
	}
	if end.Before(now.Add(time.Hour-delta)) || end.After(now.Add(time.Hour+delta)) {
		t.Error("team.Instance.EndTime is not within the expected range")
	}
}
