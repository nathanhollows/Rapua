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

// refusedToast stands in for the admin handler's toast, which is what the
// middleware is given so it does not report an error its own way.
func refusedToast(w http.ResponseWriter, _ *http.Request) error {
	_, err := w.Write([]byte("<div>This game is running. Stop it before making changes.</div>"))
	return err
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
			middlewares.QuestEditableMiddleware(newTestLogger(t), refusedToast, next).ServeHTTP(w, req)

			assert.Equal(t, tt.wantCalled, called)
			if !tt.wantCalled {
				// A 200 carrying the message, because htmx discards the body of
				// an error response: a 4xx here is a click that does nothing.
				assert.Equal(t, http.StatusOK, w.Code)
				assert.Contains(t, w.Body.String(), "Stop it before making changes")
			}
		})
	}
}

// One click on the banner unlocks the page, and every request it makes after
// that carries the header. The lock is a rail against editing a live game by
// accident, so an author who says they mean it is taken at their word.
func TestQuestEditableMiddleware_UnlockHeaderPassesThrough(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })

	user := &models.User{CurrentQuest: questWithStatus(time.Now().UTC().Add(-time.Hour), time.Time{})}
	req := httptest.NewRequest(http.MethodPost, "/admin/objective/some-slug", nil)
	req = req.WithContext(context.WithValue(req.Context(), contextkeys.UserKey, user))
	req.Header.Set("X-Quest-Unlock", "true")

	w := httptest.NewRecorder()
	middlewares.QuestEditableMiddleware(newTestLogger(t), refusedToast, next).ServeHTTP(w, req)

	assert.True(t, called, "the edit goes through")
}

// Without the header the same request is refused, so the unlock has to be an
// action somebody took rather than a state they were already in.
func TestQuestEditableMiddleware_UnlockIsNotTheDefault(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })

	user := &models.User{CurrentQuest: questWithStatus(time.Now().UTC().Add(-time.Hour), time.Time{})}
	req := httptest.NewRequest(http.MethodPost, "/admin/objective/some-slug", nil)
	req = req.WithContext(context.WithValue(req.Context(), contextkeys.UserKey, user))

	w := httptest.NewRecorder()
	middlewares.QuestEditableMiddleware(newTestLogger(t), refusedToast, next).ServeHTTP(w, req)

	assert.False(t, called)
	assert.Contains(t, w.Body.String(), "Stop it before making changes")
}
