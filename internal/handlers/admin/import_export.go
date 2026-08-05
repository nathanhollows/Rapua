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

// ExportQuest exports a quest as a v7 JSON document download.
//
//	GET /admin/quests/{id}/export
func (h *Handler) ExportQuest(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	questID := chi.URLParam(r, "id")

	ok, err := h.accessService.CanAdminAccessQuest(r.Context(), user.ID, questID)
	if err != nil || !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	doc, err := h.exportService.ExportInstance(r.Context(), questID)
	if err != nil {
		h.handleError(
			w,
			r,
			"ExportInstance: export",
			"Error exporting instance",
			"error",
			err,
			"quest_id",
			questID,
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
			"quest_id",
			questID,
		)
		return
	}

	filename := fmt.Sprintf("rapua-instance-%s.json", questID)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// ImportCheck validates an uploaded JSON file and returns an HTML fragment.
// If the document ID matches an instance owned by the logged-in user, the
// fragment offers an "overwrite" button alongside "create new".
//
//	POST /admin/quests/import/check
func (h *Handler) ImportCheck(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	doc, err := parseUploadedDoc(r)
	if err != nil {
		_ = templates.ImportErrorResult(fmt.Sprintf("Could not read file: %v", err)).Render(r.Context(), w)
		return
	}

	lintResult := game.Lint(doc, blocks.Registry())

	// Only check ownership when the doc is otherwise valid.
	var matched *models.Quest
	if lintResult.IsValid() && doc.ID != "" {
		inst, err := h.questService.GetByID(r.Context(), doc.ID)
		if err == nil && inst.UserID == user.ID {
			matched = inst
		}
	}

	_ = templates.ImportCheckResult(lintResult, matched, doc.Name).Render(r.Context(), w)
}

// ImportInstanceCreate creates a new instance from an uploaded v7 JSON document.
//
//	POST /admin/quests/import
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

	h.redirect(w, r, "/admin/quests")
}

// ImportInstanceUpdate updates an existing instance from an uploaded v7 JSON document.
//
//	POST /admin/quests/{id}/import
func (h *Handler) ImportInstanceUpdate(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	questID := chi.URLParam(r, "id")

	ok, err := h.accessService.CanAdminAccessQuest(r.Context(), user.ID, questID)
	if err != nil || !ok {
		_ = templates.ImportErrorResult("You do not have permission to update this game.").Render(r.Context(), w)
		return
	}

	doc, err := parseUploadedDoc(r)
	if err != nil {
		_ = templates.ImportErrorResult(fmt.Sprintf("Could not read file: %v", err)).Render(r.Context(), w)
		return
	}

	_, err = h.importService.ImportUpdate(r.Context(), user.ID, questID, doc)
	if err != nil {
		_ = templates.ImportErrorResult(fmt.Sprintf("Import failed: %v", err)).Render(r.Context(), w)
		return
	}

	h.redirect(w, r, "/admin/quests")
}

const maxUploadBytes = 4 << 20 // 4 MB

// parseUploadedDoc reads a GameDoc from a multipart file upload (field "file") or raw JSON body.
func parseUploadedDoc(r *http.Request) (*game.GameDoc, error) {
	contentType := r.Header.Get("Content-Type")

	data, err := readUploadData(r, contentType)
	if err != nil {
		return nil, err
	}

	var doc game.GameDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return &doc, nil
}

// readUploadData reads the raw bytes from either a multipart upload or a raw JSON body.
func readUploadData(r *http.Request, contentType string) ([]byte, error) {
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		data, err := io.ReadAll(io.LimitReader(r.Body, maxUploadBytes))
		if err != nil {
			return nil, fmt.Errorf("read body: %w", err)
		}
		return data, nil
	}
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		return nil, fmt.Errorf("parse form: %w", err)
	}
	f, _, err := r.FormFile("file")
	if err != nil {
		return nil, fmt.Errorf("read file field: %w", err)
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxUploadBytes))
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}
	return data, nil
}
