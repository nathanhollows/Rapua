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
	data, err := parseGameBytes(r)
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

const maxUploadBytes = 4 << 20 // 4 MB

// parseGameBytes reads raw JSON bytes from a multipart file upload (field "file") or raw JSON body.
func parseGameBytes(r *http.Request) ([]byte, error) {
	contentType := r.Header.Get("Content-Type")

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
			return nil, fmt.Errorf("parse form: %w", err)
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			return nil, fmt.Errorf("read file field: %w", err)
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return nil, fmt.Errorf("read file: %w", err)
		}
		return data, nil
	}

	data, err := io.ReadAll(io.LimitReader(r.Body, maxUploadBytes))
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return data, nil
}

// parseGameDoc reads a GameDoc from a multipart file upload (field "file") or raw JSON body.
func parseGameDoc(r *http.Request) (*game.GameDoc, error) {
	data, err := parseGameBytes(r)
	if err != nil {
		return nil, err
	}
	var doc game.GameDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &doc, nil
}
