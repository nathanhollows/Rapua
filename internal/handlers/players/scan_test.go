package players_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/handlers/players"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

type scanStub struct {
	players.CheckInService
	found    *models.Objective
	err      error
	value    string
	modality blocks.ScanModality
}

func (s *scanStub) ScanCode(
	_ context.Context, _ *models.Run, value string, modality blocks.ScanModality,
) (*models.Objective, error) {
	s.value, s.modality = value, modality
	return s.found, s.err
}

func withRun(r *http.Request) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), contextkeys.RunKey, &models.Run{Code: "RUN1"}))
}

func scanLink(t *testing.T, stub *scanStub, code string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/s/"+code, nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("code", code)
	req = withRun(req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx)))
	w := httptest.NewRecorder()
	players.NewTestPlayerHandler(players.WithCheckInService(stub)).ScanLink(w, req)
	return w
}

func TestPlayerHandler_ScanLink_GoesToTheObjective(t *testing.T) {
	stub := &scanStub{found: &models.Objective{Slug: "rooftop"}}
	w := scanLink(t, stub, "ROOFS")

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/objective/rooftop", w.Header().Get("Location"))
	assert.Equal(t, "ROOFS", stub.value)
	assert.Equal(t, blocks.ModalityCamera, stub.modality, "a link is the phone's camera")
}

func TestPlayerHandler_ScanLink_NoMatchLandsOnTheList(t *testing.T) {
	w := scanLink(t, &scanStub{err: services.ErrNoScanMatch}, "NOPE1")

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, "/objectives", w.Header().Get("Location"))
}

func TestPlayerHandler_ScanLink_WithoutARunJoinsFirst(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/s/ROOFS", nil)
	w := httptest.NewRecorder()
	players.NewTestPlayerHandler(players.WithCheckInService(&scanStub{})).ScanLink(w, req)

	assert.Equal(t, "/play", w.Header().Get("Location"))
}

func scanPost(t *testing.T, stub *scanStub, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/s/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Hx-Request", "true")
	w := httptest.NewRecorder()
	players.NewTestPlayerHandler(players.WithCheckInService(stub)).ScanPost(w, withRun(req))
	return w
}

func TestPlayerHandler_ScanPost_RedirectsOnAMatch(t *testing.T) {
	stub := &scanStub{found: &models.Objective{Slug: "library"}}
	w := scanPost(t, stub, url.Values{"code": {" LIBRY "}})

	assert.Equal(t, "/objective/library", w.Header().Get("Hx-Redirect"))
	assert.Equal(t, "LIBRY", stub.value)
	assert.Equal(t, blocks.ModalityTyped, stub.modality)
}

func TestPlayerHandler_ScanPost_NoMatchSaysSo(t *testing.T) {
	w := scanPost(t, &scanStub{err: services.ErrNoScanMatch}, url.Values{"scanned": {"NOPE1"}})

	assert.Empty(t, w.Header().Get("Hx-Redirect"))
	assert.Contains(t, w.Body.String(), "doesn&#39;t match anything you can reach yet")
}

func TestPlayerHandler_ScanPost_EmptyAsksForACode(t *testing.T) {
	stub := &scanStub{}
	w := scanPost(t, stub, url.Values{})

	assert.Contains(t, w.Body.String(), "Scan a code or type it in.")
	assert.Empty(t, stub.value, "nothing to match")
}
