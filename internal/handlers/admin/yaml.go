package admin

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nathanhollows/Rapua/v7/internal/services"
	"github.com/nathanhollows/Rapua/v7/models"
	"gopkg.in/yaml.v3"
)

// YAMLExportService defines the interface for exporting instances as YAML.
type YAMLExportService interface {
	Export(ctx context.Context, instanceID string, mode services.ExportMode) (*models.QuestDef, error)
}

// YAMLImportService defines the interface for importing YAML into instances.
type YAMLImportService interface {
	ImportCreate(ctx context.Context, userID string, def *models.QuestDef, isTemplate bool) (*models.Instance, []services.ImportWarning, error)
	ImportUpdate(ctx context.Context, instanceID string, def *models.QuestDef) (*models.Instance, []services.ImportWarning, error)
}

// ExportTemplateYAML exports the current instance as template YAML (no entity IDs).
func (h *Handler) ExportTemplateYAML(w http.ResponseWriter, r *http.Request) {
	h.exportYAML(w, r, services.ExportTemplate)
}

// ExportInstanceYAML exports the current instance as instance YAML (with entity IDs for round-trip update).
func (h *Handler) ExportInstanceYAML(w http.ResponseWriter, r *http.Request) {
	h.exportYAML(w, r, services.ExportInstance)
}

func (h *Handler) exportYAML(w http.ResponseWriter, r *http.Request, mode services.ExportMode) {
	user := h.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	instanceID := r.URL.Query().Get("id")
	if instanceID == "" {
		instanceID = user.CurrentInstanceID
	}

	// Verify the user owns this instance
	if instanceID != user.CurrentInstanceID {
		instance, err := h.instanceService.GetByID(r.Context(), instanceID)
		if err != nil || instance.UserID != user.ID {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
	}

	def, err := h.yamlExportService.Export(r.Context(), instanceID, mode)
	if err != nil {
		h.logger.Error("exporting YAML", "error", err)
		h.handleError(w, r, "exporting YAML", "Failed to export YAML")
		return
	}

	data, err := yaml.Marshal(def)
	if err != nil {
		h.logger.Error("marshalling YAML", "error", err)
		h.handleError(w, r, "marshalling YAML", "Failed to export YAML")
		return
	}

	filename := fmt.Sprintf("%s.yaml", models.Slugify(def.Name))
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	if _, err := w.Write(data); err != nil {
		h.logger.Error("writing YAML response", "error", err, "instance_id", instanceID)
	}
}

// ValidateImportYAML parses and validates uploaded YAML, returning validation results.
func (h *Handler) ValidateImportYAML(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	def, err := parseYAMLFromRequest(r)
	if err != nil {
		h.handleError(w, r, "parsing YAML", err.Error())
		return
	}

	result := services.ValidateQuestDef(def)

	if !result.Valid {
		msgs := make([]string, len(result.Errors))
		for i, e := range result.Errors {
			msgs[i] = e.Error()
		}
		h.handleError(w, r, "validating YAML", strings.Join(msgs, "; "))
		return
	}

	// Valid — proceed with confirmation message
	msg := fmt.Sprintf("YAML is valid: %q with %d stops", def.Name, len(def.Stops))
	if len(result.Warnings) > 0 {
		warnMsgs := make([]string, len(result.Warnings))
		for i, warn := range result.Warnings {
			warnMsgs[i] = warn.Message
		}
		msg += fmt.Sprintf(" (warnings: %s)", strings.Join(warnMsgs, "; "))
	}
	h.handleSuccess(w, r, msg)
}

// ImportYAML creates a new instance from uploaded YAML.
// The "is_template" form field controls whether the new instance is a template.
func (h *Handler) ImportYAML(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	def, err := parseYAMLFromRequest(r)
	if err != nil {
		h.handleError(w, r, "parsing YAML", err.Error())
		return
	}

	isTemplate := r.FormValue("is_template") == "true"

	instance, warnings, err := h.yamlImportService.ImportCreate(r.Context(), user.ID, def, isTemplate)
	if err != nil {
		h.logger.Error("importing YAML", "error", err)
		h.handleError(w, r, "importing YAML", fmt.Sprintf("Import failed: %s", err.Error()))
		return
	}

	// Switch to the new instance
	if err := h.userService.SwitchInstance(r.Context(), user, instance.ID); err != nil {
		h.logger.Error("switching instance", "error", err)
	}

	if len(warnings) > 0 {
		warnMsgs := make([]string, len(warnings))
		for i, w := range warnings {
			warnMsgs[i] = w.Message
		}
		h.logger.Warn("YAML import warnings", "instance", instance.Name, "warnings", strings.Join(warnMsgs, "; "))
	}

	h.redirect(w, r, "/admin/locations")
}

// UpdateYAML updates the current instance from uploaded YAML.
func (h *Handler) UpdateYAML(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	def, err := parseYAMLFromRequest(r)
	if err != nil {
		h.handleError(w, r, "parsing YAML", err.Error())
		return
	}

	instance, warnings, err := h.yamlImportService.ImportUpdate(r.Context(), user.CurrentInstanceID, def)
	if err != nil {
		h.logger.Error("updating from YAML", "error", err)
		h.handleError(w, r, "updating from YAML", fmt.Sprintf("Update failed: %s", err.Error()))
		return
	}

	if len(warnings) > 0 {
		warnMsgs := make([]string, len(warnings))
		for i, w := range warnings {
			warnMsgs[i] = w.Message
		}
		h.logger.Warn("YAML update warnings", "instance", instance.Name, "warnings", strings.Join(warnMsgs, "; "))
	}

	h.redirect(w, r, "/admin/locations")
}

// parseYAMLFromRequest extracts and parses YAML from a multipart form upload.
func parseYAMLFromRequest(r *http.Request) (*models.QuestDef, error) {
	const maxUploadSize = 250 << 10 // 250KB
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		return nil, fmt.Errorf("parsing form: %w", err)
	}

	file, header, err := r.FormFile("yaml_file")
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}
	defer file.Close()

	// Validate file extension to reject obviously wrong uploads early
	name := header.Filename
	if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
		return nil, fmt.Errorf("file must be a .yaml or .yml file, got %q", name)
	}

	data, err := io.ReadAll(io.LimitReader(file, maxUploadSize))
	if err != nil {
		return nil, fmt.Errorf("reading file contents: %w", err)
	}

	var def models.QuestDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}

	return &def, nil
}
