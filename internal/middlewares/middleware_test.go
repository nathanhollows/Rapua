package middlewares

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

// discardWriter implements io.Writer but discards all writes.
type discardWriter struct{}

func (dw *discardWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func TestHtmxOnlyMiddleware(t *testing.T) {
	// Test cases
	testCases := []struct {
		name           string
		isHtmxRequest  bool
		expectedStatus int
		expectedPath   string
	}{
		{
			name:           "HTMX Request",
			isHtmxRequest:  true,
			expectedStatus: http.StatusOK,
			expectedPath:   "",
		},
		{
			name:           "Non-HTMX Request",
			isHtmxRequest:  false,
			expectedStatus: http.StatusSeeOther,
			expectedPath:   "/redirect-path",
		},
	}

	// Create a test logger that discards output
	logger := slog.New(slog.NewTextHandler(&discardWriter{}, nil))

	// Set up a next handler that just returns 200 OK
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a request
			req, err := http.NewRequest(http.MethodGet, "/test", nil)
			if err != nil {
				t.Fatal(err)
			}

			// Add HX-Request header if this is an HTMX request
			if tc.isHtmxRequest {
				req.Header.Set("Hx-Request", "true")
			}

			// Create a ResponseRecorder to record the response
			rr := httptest.NewRecorder()

			// Create the middleware
			middleware := HtmxOnlyMiddleware(logger, "/redirect-path", nextHandler)

			// Call the middleware
			middleware.ServeHTTP(rr, req)

			// Check the status code
			if status := rr.Code; status != tc.expectedStatus {
				t.Errorf("Handler returned wrong status code: got %v want %v", status, tc.expectedStatus)
			}

			// For non-HTMX requests, check that we got redirected to the right path
			if !tc.isHtmxRequest {
				location := rr.Header().Get("Location")
				if location != tc.expectedPath {
					t.Errorf("Handler redirected to wrong path: got %v want %v", location, tc.expectedPath)
				}
			}
		})
	}
}
