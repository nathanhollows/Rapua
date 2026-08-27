package admin

import (
	"bytes"
	"context"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validImportDocJSON() string {
	return `{"rapua":"v8","name":"Test Game","settings":{},"start":[{"type":"start_button"}],"finish":[],"structure":{"routing":"free_roam","completion":"all","children":[{"group":{"name":"G","routing":"free_roam","completion":"all","children":[{"objective":{"slug":"a","title":"A","proof":{},"reveal":{}}}]}}]}}`
}

func withImportUser(r *http.Request) *http.Request {
	user := &models.User{ID: "user-1"}
	return r.WithContext(context.WithValue(r.Context(), contextkeys.UserKey, user))
}

func newImportHandler() *Handler {
	return &Handler{
		logger: slog.New(slog.DiscardHandler),
	}
}

// --- parseUploadedDoc ---

func TestParseUploadedDoc_JSONBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(validImportDocJSON()))
	req.Header.Set("Content-Type", "application/json")

	doc, err := parseUploadedDoc(req)

	require.NoError(t, err)
	assert.Equal(t, "v8", doc.Rapua)
	assert.Equal(t, "Test Game", doc.Name)
}

func TestParseUploadedDoc_MultipartFile(t *testing.T) {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", "game.json")
	require.NoError(t, err)
	_, err = fw.Write([]byte(validImportDocJSON()))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	doc, err := parseUploadedDoc(req)

	require.NoError(t, err)
	assert.Equal(t, "v8", doc.Rapua)
	assert.Equal(t, "Test Game", doc.Name)
}

func TestParseUploadedDoc_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")

	doc, err := parseUploadedDoc(req)

	require.Error(t, err)
	assert.Nil(t, doc)
}

func TestParseUploadedDoc_MultipartMissingFile(t *testing.T) {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	doc, err := parseUploadedDoc(req)

	require.Error(t, err)
	assert.Nil(t, doc)
}

// --- ImportCheck ---

func TestImportCheck_MalformedUpload(t *testing.T) {
	h := newImportHandler()
	req := httptest.NewRequest(http.MethodPost, "/admin/quests/import/check",
		strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	req = withImportUser(req)
	w := httptest.NewRecorder()

	h.ImportCheck(w, req)

	// Renders ImportErrorResult — 200 with HTML fragment
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Body.String())
}

func TestImportCheck_InvalidDoc_LintFails(t *testing.T) {
	h := newImportHandler()
	// Missing required "name" field → lint error
	body := `{"rapua":"v8","settings":{},"start":[],"finish":[],"structure":{"routing":"free_roam","completion":"all","children":[]}}`
	req := httptest.NewRequest(http.MethodPost, "/admin/quests/import/check",
		strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = withImportUser(req)
	w := httptest.NewRecorder()

	h.ImportCheck(w, req)

	// Renders ImportCheckResult with errors — 200 with HTML fragment
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Body.String())
}

func TestImportCheck_ValidDoc_NoDocID(t *testing.T) {
	// A valid doc without an ID — skips the questService lookup entirely.
	h := newImportHandler()
	req := httptest.NewRequest(http.MethodPost, "/admin/quests/import/check",
		strings.NewReader(validImportDocJSON()))
	req.Header.Set("Content-Type", "application/json")
	req = withImportUser(req)
	w := httptest.NewRecorder()

	h.ImportCheck(w, req)

	// Renders ImportCheckResult with no matched instance — 200 with HTML fragment
	assert.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, w.Body.String())
}
