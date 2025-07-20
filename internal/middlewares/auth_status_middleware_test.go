package middlewares_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nathanhollows/Rapua/v4/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v4/internal/middlewares"
)

// mockAuthChecker is a test implementation of AuthChecker
type mockAuthChecker struct {
	isAuthenticated bool
}

func (m *mockAuthChecker) IsUserAuthenticated(r *http.Request) bool {
	return m.isAuthenticated
}

func TestAuthStatusMiddleware(t *testing.T) {
	testCases := []struct {
		name                string
		isAuthenticated     bool
		expectedAdminStatus bool
	}{
		{
			name:                "Authenticated Admin",
			isAuthenticated:     true,
			expectedAdminStatus: true,
		},
		{
			name:                "Unauthenticated User",
			isAuthenticated:     false,
			expectedAdminStatus: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a mock AuthChecker
			mockAuthService := &mockAuthChecker{
				isAuthenticated: tc.isAuthenticated,
			}

			// Create a test handler that will verify the context
			var capturedCtx context.Context
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedCtx = r.Context()
			})

			// Create the middleware
			middleware := middlewares.AuthStatusMiddleware(mockAuthService, testHandler)

			// Create a test request
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			w := httptest.NewRecorder()

			// Serve the request through the middleware
			middleware.ServeHTTP(w, req)

			// Verify the context
			if capturedCtx == nil {
				t.Fatal("Context was not passed through the middleware")
			}

			// Retrieve the user status from the context
			status := contextkeys.GetUserStatus(capturedCtx)

			// Check if the admin login status matches the expected value
			if status.IsAdminLoggedIn != tc.expectedAdminStatus {
				t.Errorf("Expected IsAdminLoggedIn to be %v, got %v",
					tc.expectedAdminStatus,
					status.IsAdminLoggedIn)
			}
		})
	}
}

func TestAuthStatusMiddleware_NilNext(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Did not expect panic when next handler is nil")
		}
	}()

	mockAuthService := &mockAuthChecker{isAuthenticated: false}

	// This should panic
	middlewares.AuthStatusMiddleware(mockAuthService, nil)
}

func TestAuthStatusMiddleware_NilAuthService(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Did not expect panic when auth service is nil")
		}
	}()

	// This should panic
	middlewares.AuthStatusMiddleware(nil, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
}
