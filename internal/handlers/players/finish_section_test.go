package players_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/handlers/players"
	"github.com/stretchr/testify/assert"
)

// The press is a boosted form, so no exit may be an Hx-Redirect, which reloads
// the whole document.
func TestPlayerHandler_FinishSection_RedirectsAreNeverHxRedirects(t *testing.T) {
	handler := players.NewTestPlayerHandler()

	req := httptest.NewRequest(http.MethodPost, "/objective/still/finish", nil)
	req.Header.Set("Hx-Request", "true")
	w := httptest.NewRecorder()

	handler.FinishSection(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/play", w.Header().Get("Location"))
	assert.Empty(t, w.Header().Get("Hx-Redirect"))
}
