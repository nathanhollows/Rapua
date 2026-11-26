package public

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/specgen"
)

// SpecJSON serves the generated game spec as JSON.
//
//	GET /api/v8/spec
func (h *Handler) SpecJSON(w http.ResponseWriter, _ *http.Request) {
	data, err := specgen.GenerateJSON()
	if err != nil {
		http.Error(w, fmt.Sprintf("error generating spec: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// LintDoc validates a v8 JSON document without importing it.
//
//	POST /api/v8/lint
func (h *Handler) LintDoc(w http.ResponseWriter, r *http.Request) {
	data, err := parseGameBytes(w, r)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid document: %v", err), http.StatusBadRequest)
		return
	}

	result := game.LintJSON(data, blocks.Registry())

	// INVALID_JSON means the bytes weren't valid JSON at all — return 400 directly.
	if result.HasError("INVALID_JSON") {
		http.Error(w, fmt.Sprintf("invalid document: %s", result.Errors[0].Message), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if !result.IsValid() {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}
	_ = json.NewEncoder(w).Encode(result)
}

// maxDocMB is the document limit in the units the error speaks: a reader
// told "over 4 MB" should be able to match that to the cap.
const maxDocMB = 4

const maxDocBytes = maxDocMB << 20 // 4 MB

// maxFramingMB is the MIME framing and any other fields a request carries
// around the document, so the ceiling sits above the limit rather than on it.
const maxFramingMB = 1

//nolint:mnd // Bytes are the cap; megabytes are how it is said.
const maxRequestBytes = maxDocBytes + (maxFramingMB << 20)

// parseGameBytes reads raw JSON bytes from a multipart file upload (field "file") or raw JSON body.
//
// This route is public, so the body is capped before anything reads it: the
// multipart parse spills what it cannot hold in memory to disk, and an
// unbounded read of the file grows with whatever the caller sent.
func parseGameBytes(w http.ResponseWriter, r *http.Request) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxDocBytes); err != nil {
			return nil, fmt.Errorf("parse form: %w", err)
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			return nil, fmt.Errorf("read file field: %w", err)
		}
		defer f.Close()
		return readDocument(io.LimitReader(f, maxDocBytes+1))
	}

	return readDocument(io.LimitReader(r.Body, maxDocBytes+1))
}

// readDocument reads a document and refuses one past the cap, which a limit reader alone would
// silently shorten into a parse error about something else.
func readDocument(reader io.Reader) ([]byte, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(data) > maxDocBytes {
		return nil, fmt.Errorf("document too large: over %d MB", maxDocMB)
	}
	return data, nil
}
