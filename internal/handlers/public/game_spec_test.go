//nolint:testpackage // Tests need access to unexported parseGameDoc
package public

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validSpecDoc() string {
	return `{
		"rapua":"v8",
		"name":"Test Game",
		"settings":{},
		"start":[{"type":"start_button"}],
		"finish":[],
		"structure":{
			"routing":"free_roam",
			"completion":"all",
			"children":[{
				"group":{
					"name":"Stage One",
					"routing":"free_roam",
					"completion":"all",
					"children":[{
						"objective":{
							"slug":"obj-a",
							"title":"Objective A",
							"proof":{},
							"reveal":{}
						}
					}]
				}
			}]
		}
	}`
}

func TestSpecJSON(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodGet, "/api/v7/spec", nil)
	w := httptest.NewRecorder()

	h.SpecJSON(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var result map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	assert.Equal(t, "v8", result["version"])
	assert.NotEmpty(t, result["blocks"])
}

func TestLintDoc_ValidDoc_JSONBody(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/v7/lint", strings.NewReader(validSpecDoc()))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.LintDoc(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var result map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	// LintResult.Errors should be empty (or nil/null)
	errs, _ := result["errors"].([]any)
	assert.Empty(t, errs)
}

func TestLintDoc_InvalidDoc_JSONBody(t *testing.T) {
	h := &Handler{}
	// Missing required "name" field
	body := `{"rapua":"v8","settings":{},"start":[],"finish":[],"structure":{"routing":"free_roam","completion":"all","children":[]}}`
	req := httptest.NewRequest(http.MethodPost, "/api/v7/lint", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.LintDoc(w, req)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	var result map[string]any
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	errs, _ := result["errors"].([]any)
	assert.NotEmpty(t, errs)
}

func TestLintDoc_MalformedJSON(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/api/v7/lint", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	h.LintDoc(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestLintDoc_MultipartUpload_Valid(t *testing.T) {
	h := &Handler{}

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", "game.json")
	require.NoError(t, err)
	_, err = fw.Write([]byte(validSpecDoc()))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v7/lint", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()

	h.LintDoc(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestLintDoc_MultipartUpload_MissingFile(t *testing.T) {
	h := &Handler{}

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/v7/lint", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()

	h.LintDoc(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// --- parseGameDoc unit tests ---

func TestParseGameDoc_JSONBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(validSpecDoc()))
	req.Header.Set("Content-Type", "application/json")

	doc, err := parseGameDoc(req)

	require.NoError(t, err)
	assert.Equal(t, "v8", doc.Rapua)
	assert.Equal(t, "Test Game", doc.Name)
}

func TestParseGameDoc_MultipartFile(t *testing.T) {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	fw, err := mw.CreateFormFile("file", "game.json")
	require.NoError(t, err)
	_, err = fw.Write([]byte(validSpecDoc()))
	require.NoError(t, err)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	doc, err := parseGameDoc(req)

	require.NoError(t, err)
	assert.Equal(t, "v8", doc.Rapua)
	assert.Equal(t, "Test Game", doc.Name)
}

func TestParseGameDoc_InvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")

	doc, err := parseGameDoc(req)

	require.Error(t, err)
	assert.Nil(t, doc)
}

func TestParseGameDoc_MultipartMissingFileField(t *testing.T) {
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "/", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())

	doc, err := parseGameDoc(req)

	require.Error(t, err)
	assert.Nil(t, doc)
}
