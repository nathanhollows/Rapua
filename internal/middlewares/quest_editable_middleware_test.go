package middlewares_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/middlewares"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

// newTestLogger sends the middleware's warnings to the test log rather than
// stderr, so a refused edit is visible where the assertion that expected it is.
func newTestLogger(t *testing.T) *slog.Logger {
	t.Helper()
	return slog.New(slog.NewTextHandler(testWriter{t}, nil))
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Logf("%s", p)
	return len(p), nil
}

func questWithStatus(start, end time.Time) models.Quest {
	return models.Quest{
		StartTime: bun.NullTime{Time: start},
		EndTime:   bun.NullTime{Time: end},
	}
}

// A running game is changed only by stopping it first, so the risk is a
// decision an author takes rather than a side effect they discover.
func TestQuestEditableMiddleware(t *testing.T) {
	past := time.Now().UTC().Add(-time.Hour)
	future := time.Now().UTC().Add(time.Hour)

	for _, tt := range []struct {
		name       string
		method     string
		quest      models.Quest
		wantCalled bool
	}{
		{"reading a running game is allowed", http.MethodGet, questWithStatus(past, time.Time{}), true},
		{"editing a running game is refused", http.MethodPost, questWithStatus(past, time.Time{}), false},
		{"deleting from a running game is refused", http.MethodDelete, questWithStatus(past, time.Time{}), false},
		{"editing a scheduled game is allowed", http.MethodPost, questWithStatus(future, time.Time{}), true},
		{"editing a stopped game is allowed", http.MethodPost, questWithStatus(past, past), true},
		{"editing a game that never ran is allowed", http.MethodPost, models.Quest{}, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })

			user := &models.User{CurrentQuest: tt.quest}
			req := httptest.NewRequest(tt.method, "/admin/objective/some-slug", nil)
			req = req.WithContext(context.WithValue(req.Context(), contextkeys.UserKey, user))

			w := httptest.NewRecorder()
			middlewares.QuestEditableMiddleware(newTestLogger(t), next).ServeHTTP(w, req)

			assert.Equal(t, tt.wantCalled, called)
			if !tt.wantCalled {
				assert.Equal(t, http.StatusConflict, w.Code)
				assert.Contains(t, w.Body.String(), "Stop it before making changes")
			}
		})
	}
}
