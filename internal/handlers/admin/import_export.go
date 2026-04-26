package admin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/game"
	templates "github.com/nathanhollows/Rapua/v7/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v7/models"
)

// ExportInstance exports an instance as a v7 JSON document download.
//
//	GET /admin/instances/{id}/export
func (h *Handler) ExportInstance(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	instanceID := chi.URLParam(r, "id")

	ok, err := h.accessService.CanAdminAccessInstance(r.Context(), user.ID, instanceID)
	if err != nil || !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	doc, err := h.exportService.ExportInstance(r.Context(), instanceID)
	if err != nil {
		h.handleError(
			w,
			r,
			"ExportInstance: export",
			"Error exporting instance",
			"error",
			err,
			"instance_id",
			instanceID,
		)
		return
	}

	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		h.handleError(
			w,
			r,
			"ExportInstance: marshal",
			"Error serialising document",
			"error",
			err,
			"instance_id",
			instanceID,
		)
		return
	}

	filename := fmt.Sprintf("rapua-instance-%s.json", instanceID)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ImportCheck validates an uploaded JSON file and returns an HTML fragment.
// If the document ID matches an instance owned by the logged-in user, the
// fragment offers an "overwrite" button alongside "create new".
//
//	POST /admin/instances/import/check
func (h *Handler) ImportCheck(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	doc, err := parseUploadedDoc(r)
	if err != nil {
		_ = templates.ImportErrorResult(fmt.Sprintf("Could not read file: %v", err)).Render(r.Context(), w)
		return
	}

	lintResult := game.Lint(doc, blocks.Registry())

	// Only check ownership when the doc is otherwise valid.
	var matched *models.Instance
	if lintResult.IsValid() && doc.ID != "" {
		inst, err := h.instanceService.GetByID(r.Context(), doc.ID)
		if err == nil && inst.UserID == user.ID {
			matched = inst
		}
	}

	_ = templates.ImportCheckResult(lintResult, matched, doc.Name).Render(r.Context(), w)
}

// ImportInstanceCreate creates a new instance from an uploaded v7 JSON document.
//
//	POST /admin/instances/import
func (h *Handler) ImportInstanceCreate(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	doc, err := parseUploadedDoc(r)
	if err != nil {
		_ = templates.ImportErrorResult(fmt.Sprintf("Could not read file: %v", err)).Render(r.Context(), w)
		return
	}

	_, err = h.importService.ImportCreate(r.Context(), user.ID, doc)
	if err != nil {
		_ = templates.ImportErrorResult(fmt.Sprintf("Import failed: %v", err)).Render(r.Context(), w)
		return
	}

	h.redirect(w, r, "/admin/instances")
}

// ImportInstanceUpdate updates an existing instance from an uploaded v7 JSON document.
//
//	POST /admin/instances/{id}/import
func (h *Handler) ImportInstanceUpdate(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	instanceID := chi.URLParam(r, "id")

	ok, err := h.accessService.CanAdminAccessInstance(r.Context(), user.ID, instanceID)
	if err != nil || !ok {
		_ = templates.ImportErrorResult("You do not have permission to update this game.").Render(r.Context(), w)
		return
	}

	doc, err := parseUploadedDoc(r)
	if err != nil {
		_ = templates.ImportErrorResult(fmt.Sprintf("Could not read file: %v", err)).Render(r.Context(), w)
		return
	}

	_, err = h.importService.ImportUpdate(r.Context(), user.ID, instanceID, doc)
	if err != nil {
		_ = templates.ImportErrorResult(fmt.Sprintf("Import failed: %v", err)).Render(r.Context(), w)
		return
	}

	h.redirect(w, r, "/admin/instances")
}

const maxUploadBytes = 4 << 20 // 4 MB

// parseUploadedDoc reads a GameDoc from a multipart file upload (field "file") or raw JSON body.
func parseUploadedDoc(r *http.Request) (*game.GameDoc, error) { //nolint:nestif // standard multipart-or-body parse pattern
	contentType := r.Header.Get("Content-Type")

	var data []byte
	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
			return nil, fmt.Errorf("parse form: %w", err)
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			return nil, fmt.Errorf("read file field: %w", err)
		}
		defer f.Close()
		var readErr error
		data, readErr = io.ReadAll(io.LimitReader(f, maxUploadBytes))
		if readErr != nil {
			return nil, fmt.Errorf("read file: %w", readErr)
		}
	} else {
		var readErr error
		data, readErr = io.ReadAll(io.LimitReader(r.Body, maxUploadBytes))
		if readErr != nil {
			return nil, fmt.Errorf("read body: %w", readErr)
		}
	}

	var doc game.GameDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return &doc, nil
}
