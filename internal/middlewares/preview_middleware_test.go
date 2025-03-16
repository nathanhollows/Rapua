package middlewares_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v3/internal/middlewares"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockTeamService struct {
	mock.Mock
}

func (m *MockTeamService) LoadRelation(ctx context.Context, team *models.Team, relation string) error {
	args := m.Called(ctx, team, relation)
	return args.Error(0)
}

func TestPreviewMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		requestURL     string
		hxRequest      string
		instanceID     string
		loadError      error
		expectsTeam    bool
		expectsPreview bool
	}{
		{
			name:           "Non-preview request should pass through",
			requestURL:     "/user/dashboard",
			hxRequest:      "false",
			expectsTeam:    false,
			expectsPreview: false,
		},
		{
			name:           "Preview request without instance ID should pass through",
			requestURL:     "/admin/settings",
			hxRequest:      "true",
			instanceID:     "",
			expectsTeam:    false,
			expectsPreview: false,
		},
		{
			name:           "Valid preview request with instance ID",
			requestURL:     "/templates/preview",
			hxRequest:      "true",
			instanceID:     "123",
			expectsTeam:    true,
			expectsPreview: true,
		},
		{
			name:           "Preview request but LoadRelation fails",
			requestURL:     "/admin/preview",
			hxRequest:      "true",
			instanceID:     "123",
			loadError:      errors.New("database error"),
			expectsTeam:    false,
			expectsPreview: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockService := new(MockTeamService)
			if tt.instanceID != "" {
				mockService.On("LoadRelation", mock.Anything, mock.Anything, "Instance").Return(tt.loadError)
			}

			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				ctx := r.Context()
				team, hasTeam := ctx.Value(contextkeys.TeamKey).(*models.Team)
				_, hasPreview := ctx.Value(contextkeys.PreviewKey).(bool)

				assert.Equal(t, tt.expectsTeam, hasTeam)
				assert.Equal(t, tt.expectsPreview, hasPreview)

				if tt.expectsTeam {
					assert.NotNil(t, team)
					assert.Equal(t, "TEST", team.Code)
					assert.Equal(t, tt.instanceID, team.InstanceID)
				}
			})

			middleware := middlewares.PreviewMiddleware(mockService, handler)
			req := httptest.NewRequest(http.MethodGet, tt.requestURL+"?instance_id="+tt.instanceID, nil)
			if tt.hxRequest == "true" {
				req.Header.Set("HX-Request", "true")
			}
			resp := httptest.NewRecorder()
			middleware.ServeHTTP(resp, req)
		})
	}
}
