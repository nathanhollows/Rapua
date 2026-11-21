package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

func setupNotFound(t *testing.T) (*Handler, *models.User) {
	t.Helper()
	dbc, cleanup := setupObjectiveTestDB(t)
	t.Cleanup(cleanup)
	return newObjectiveTestHandler(t, dbc), objectiveTestQuest(t, dbc)
}

func notFound(h *Handler, user *models.User, boosted bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/admin/nowhere", nil)
	if boosted {
		req.Header.Set("Hx-Boosted", "true")
	}
	if user != nil {
		req = withObjectiveUser(req, user)
	}
	w := httptest.NewRecorder()
	h.NotFound(w, req)
	return w
}

// An address typed or followed from outside is answered with a real 404, in the
// admin's own frame. It used to be a bare fragment with no document and no styles.
func TestNotFound_AFullVisitGetsA404InTheAdminFrame(t *testing.T) {
	h, user := setupNotFound(t)

	w := notFound(h, user, false)

	assert.Equal(t, http.StatusNotFound, w.Code)
	body := w.Body.String()
	assert.Contains(t, body, "<html", "a whole document, not a fragment")
	assert.Contains(t, body, "admin.css", "with its styles")
	assert.Contains(t, body, "404 Not Found")
	assert.Contains(t, body, `id="404-image"`, "the bird that follows the cursor")
}

// The page sits on the same background as the rest of the admin, not on the plain one.
func TestNotFound_SitsOnTheAdminsBackground(t *testing.T) {
	h, user := setupNotFound(t)

	w := notFound(h, user, false)

	assert.Contains(t, w.Body.String(), `id="app" data-v-app="" class="flex-1 bg-base-200"`)
}

// htmx does not swap a 4xx into the page, so a boosted visit has to say 200 or the
// page would stay as it was and nothing would explain why.
func TestNotFound_ABoostedVisitIsSwappedIn(t *testing.T) {
	h, user := setupNotFound(t)

	w := notFound(h, user, true)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "404 Not Found")
}

func TestNotFound_WithNoUserIsAPlain404(t *testing.T) {
	h, _ := setupNotFound(t)

	w := notFound(h, nil, false)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.NotContains(t, w.Body.String(), "admin.css")
}
