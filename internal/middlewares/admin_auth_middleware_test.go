package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nathanhollows/Rapua/v4/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v4/models"
)

// MockAuthService is a mock implementation of AuthenticatedUserGetter.
type MockAuthService struct {
	user *models.User
	err  error
}

func (m *MockAuthService) GetAuthenticatedUser(r *http.Request) (*models.User, error) {
	return m.user, m.err
}

// Dummy handler to simulate the next handler in the middleware chain.
func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestAdminAuthMiddleware(t *testing.T) {
	testCases := []struct {
		name               string
		user               *models.User
		authErr            error
		expectedStatusCode int
		expectedLocation   string
	}{
		{
			name:               "Unauthenticated User",
			user:               nil,
			authErr:            http.ErrNoCookie,
			expectedStatusCode: http.StatusSeeOther,
			expectedLocation:   "/login",
		},
		{
			name: "Authenticated User Needs Email Verification",
			user: &models.User{
				EmailVerified: false,
				Provider:      "",
			},
			authErr:            nil,
			expectedStatusCode: http.StatusSeeOther,
			expectedLocation:   "/verify-email",
		},
		{
			name: "Authenticated User with OAuth (No Email Verification Needed)",
			user: &models.User{
				EmailVerified: false,
				Provider:      "google",
			},
			authErr:            nil,
			expectedStatusCode: http.StatusOK,
			expectedLocation:   "",
		},
		{
			name: "Fully Authenticated User",
			user: &models.User{
				EmailVerified: true,
				Provider:      "",
			},
			authErr:            nil,
			expectedStatusCode: http.StatusOK,
			expectedLocation:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock auth service
			mockAuthService := &MockAuthService{
				user: tc.user,
				err:  tc.authErr,
			}

			// Create a test request
			req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
			w := httptest.NewRecorder()

			// Create the middleware handler
			handler := AdminAuthMiddleware(mockAuthService, dummyHandler())

			// Call the middleware
			handler.ServeHTTP(w, req)

			// Check the response
			result := w.Result()
			defer result.Body.Close()

			if result.StatusCode != tc.expectedStatusCode {
				t.Errorf("Expected status code %d, got %d", tc.expectedStatusCode, result.StatusCode)
			}

			if tc.expectedLocation != "" {
				location := result.Header.Get("Location")
				if location != tc.expectedLocation {
					t.Errorf("Expected redirect to %s, got %s", tc.expectedLocation, location)
				}
			}
		})
	}
}

func TestAdminCheckInstanceMiddleware(t *testing.T) {
	testCases := []struct {
		name               string
		path               string
		user               *models.User
		expectedStatusCode int
		expectedLocation   string
	}{
		{
			name: "User with Instance on Non-Instances Page",
			path: "/admin/dashboard",
			user: &models.User{
				CurrentInstanceID: "instance-123",
			},
			expectedStatusCode: http.StatusOK,
			expectedLocation:   "",
		},
		{
			name: "User without Instance on Non-Instances Page",
			path: "/admin/dashboard",
			user: &models.User{
				CurrentInstanceID: "",
			},
			expectedStatusCode: http.StatusSeeOther,
			expectedLocation:   "/admin/instances",
		},
		{
			name: "User on Instances Page",
			path: "/admin/instances",
			user: &models.User{
				CurrentInstanceID: "",
			},
			expectedStatusCode: http.StatusOK,
			expectedLocation:   "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a test request
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)

			// Add user to context
			ctx := context.WithValue(req.Context(), contextkeys.UserKey, tc.user)
			req = req.WithContext(ctx)

			w := httptest.NewRecorder()

			// Create the middleware handler
			handler := AdminCheckInstanceMiddleware(dummyHandler())

			// Call the middleware
			handler.ServeHTTP(w, req)

			// Check the response
			result := w.Result()
			defer result.Body.Close()

			if result.StatusCode != tc.expectedStatusCode {
				t.Errorf("Expected status code %d, got %d", tc.expectedStatusCode, result.StatusCode)
			}

			if tc.expectedLocation != "" {
				location := result.Header.Get("Location")
				if location != tc.expectedLocation {
					t.Errorf("Expected redirect to %s, got %s", tc.expectedLocation, location)
				}
			}
		})
	}
}
