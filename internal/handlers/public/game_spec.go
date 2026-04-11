package public

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/game"
	"github.com/nathanhollows/Rapua/v7/internal/specgen"
)

// SpecJSON serves the generated game spec as JSON.
//
//	GET /api/v7/spec
func (h *Handler) SpecJSON(w http.ResponseWriter, r *http.Request) {
	data, err := specgen.GenerateJSON()
	if err != nil {
		http.Error(w, fmt.Sprintf("error generating spec: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(data)
}

// LintDoc validates a v7 JSON document without importing it.
//
//	POST /api/v7/lint
func (h *Handler) LintDoc(w http.ResponseWriter, r *http.Request) {
	doc, err := parseGameDoc(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid document: %v", err), http.StatusBadRequest)
		return
	}

	result := game.Lint(doc, blocks.Registry())

	w.Header().Set("Content-Type", "application/json")
	if !result.IsValid() {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}
	_ = json.NewEncoder(w).Encode(result)
}

// parseGameDoc reads a GameDoc from a multipart file upload (field "file") or raw JSON body.
func parseGameDoc(r *http.Request) (*game.GameDoc, error) {
	contentType := r.Header.Get("Content-Type")

	var data []byte
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			return nil, fmt.Errorf("parse form: %w", err)
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			return nil, fmt.Errorf("read file field: %w", err)
		}
		defer f.Close()
		data, err = io.ReadAll(f)
		if err != nil {
			return nil, fmt.Errorf("read file: %w", err)
		}
	} else {
		var err error
		data, err = io.ReadAll(io.LimitReader(r.Body, 4<<20))
		if err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}
	}

	var doc game.GameDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &doc, nil
}
