package middlewares

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/nathanhollows/Rapua/v7/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v7/internal/sessions"
	"github.com/nathanhollows/Rapua/v7/models"
)

func TestMain(m *testing.M) {
	os.Setenv("SESSION_KEY", "test-session-key-for-middleware!")
	os.Setenv("SESSION_SECRET", "test-session-secret-middleware!")
	sessions.Start()
	os.Exit(m.Run())
}

// MockAuthService is a mock implementation of AuthenticatedUserGetter.
type MockAuthService struct {
	user *models.User
	err  error
}

func (m *MockAuthService) GetAuthenticatedUser(_ *http.Request) (*models.User, error) {
	return m.user, m.err
}

// MockInstanceLoader is a mock implementation of InstanceLoader.
type MockInstanceLoader struct {
	instance *models.Instance
	err      error
}

func (m *MockInstanceLoader) GetByIDWithRelations(_ context.Context, _ string) (*models.Instance, error) {
	return m.instance, m.err
}

// Dummy handler to simulate the next handler in the middleware chain.
func dummyHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
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
			mockInstanceLoader := &MockInstanceLoader{}
			handler := AdminAuthMiddleware(mockAuthService, mockInstanceLoader, dummyHandler())

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

func TestAdminAuthMiddleware_InstanceFromSession(t *testing.T) {
	userID := "user-123"
	instanceID := "instance-456"

	testCases := []struct {
		name               string
		sessionInstanceID  string
		instance           *models.Instance
		instanceErr        error
		expectedInstanceID string
	}{
		{
			name:              "Valid instance in session",
			sessionInstanceID: instanceID,
			instance:          &models.Instance{ID: instanceID, UserID: userID},
			expectedInstanceID: instanceID,
		},
		{
			name:              "Instance belongs to different user",
			sessionInstanceID: instanceID,
			instance:          &models.Instance{ID: instanceID, UserID: "other-user"},
			expectedInstanceID: "",
		},
		{
			name:              "Instance not found",
			sessionInstanceID: "nonexistent",
			instanceErr:       http.ErrNoCookie, // any error
			expectedInstanceID: "",
		},
		{
			name:              "No instance in session",
			sessionInstanceID: "",
			expectedInstanceID: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			mockAuth := &MockAuthService{
				user: &models.User{
					ID:            userID,
					EmailVerified: true,
				},
			}
			mockLoader := &MockInstanceLoader{
				instance: tc.instance,
				err:      tc.instanceErr,
			}

			var capturedUser *models.User
			inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				capturedUser = r.Context().Value(contextkeys.UserKey).(*models.User)
			})

			handler := AdminAuthMiddleware(mockAuth, mockLoader, inner)

			req := httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil)
			w := httptest.NewRecorder()

			// Set instance ID in session before the request
			if tc.sessionInstanceID != "" {
				session, err := sessions.Get(req, "admin")
				if err != nil {
					t.Fatalf("getting session: %v", err)
				}
				session.Values["current_instance"] = tc.sessionInstanceID
				if err := session.Save(req, w); err != nil {
					t.Fatalf("saving session: %v", err)
				}

				// Rebuild request with the session cookie from the response
				for _, c := range w.Result().Cookies() {
					req.AddCookie(c)
				}
				w = httptest.NewRecorder()
			}

			handler.ServeHTTP(w, req)

			if capturedUser == nil {
				t.Fatal("expected user in context")
			}
			if capturedUser.CurrentInstanceID != tc.expectedInstanceID {
				t.Errorf("expected CurrentInstanceID %q, got %q",
					tc.expectedInstanceID, capturedUser.CurrentInstanceID)
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
