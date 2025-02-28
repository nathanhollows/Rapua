package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v3/internal/flash"
	"github.com/nathanhollows/Rapua/v3/internal/services"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v3/models"
)

// getTemplateByID retrieves a template by ID from various sources (param, form, direct).
func (h *AdminHandler) getTemplateByID(w http.ResponseWriter, r *http.Request, idOverride ...string) (*models.Instance, bool) {
	var id string

	// Check if an explicit ID was passed
	if len(idOverride) > 0 && idOverride[0] != "" {
		id = idOverride[0]
	} else {
		// Check form value (for POST requests)
		if err := r.ParseForm(); err == nil {
			id = r.Form.Get("id")
		}

		// Fallback to URL param if not found in form
		if id == "" {
			id = chi.URLParam(r, "id")
		}
	}

	if id == "" {
		h.handleError(w, r, "TemplateName: missing id", "Could not find the template ID")
		return nil, false
	}

	template, err := h.TemplateService.GetByID(r.Context(), id)
	if err != nil {
		h.handleError(w, r, "TemplateName: getting template", "Error getting template", "error", err)
		return nil, false
	}

	return template, true
}

// TemplatesCreate creates a new template, which is a type of instance.
func (h *AdminHandler) TemplatesCreate(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "TemplateCreate: parsing form", "Error parsing form", "error", err)
		return
	}

	name := r.FormValue("name")
	id := r.FormValue("id")

	if id == "" {
		h.handleError(w, r, "TemplateCreate: missing id", "Could not find the instance ID")
		return
	}
	if name == "" {
		h.handleError(w, r, "TemplateCreate: missing name", "Please provide a name for the template")
		return
	}

	_, err := h.TemplateService.CreateFromInstance(r.Context(), user.ID, id, name)
	if err != nil {
		h.handleError(w, r, "TemplateCreate: creating instance", "Error creating instance", "error", err, "instance_id", user.CurrentInstanceID)
		return
	}

	err = templates.Toast(*flash.NewSuccess("Template created")).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("InstanceDelete: rendering template", "Error", err)
	}

	gameTemplates, err := h.TemplateService.Find(r.Context(), user.ID)
	err = templates.Templates(gameTemplates).Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "Instances: rendering template", "Error rendering template", "error", err, "instance_id", user.CurrentInstanceID)
	}
}

// TemplatesLaunch launches an instance from a template.
func (h *AdminHandler) TemplatesLaunch(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "TemplateCreate: parsing form", "Error parsing form", "error", err)
		return
	}

	id := r.FormValue("id")
	if id == "" {
		h.handleError(w, r, "TemplateCreate: missing id", "Could not find the instance ID")
		return
	}

	name := r.FormValue("name")
	if name == "" {
		h.handleError(w, r, "TemplateCreate: missing name", "Please provide a name for the template")
		return
	}

	// Regenerate refers to location codes
	regen := r.Form.Has("regenerate")

	// Create a new instance from the template
	newGame, err := h.TemplateService.LaunchInstance(r.Context(), user.ID, id, name, regen)
	if err != nil {
		h.handleError(w, r, "TemplateCreate: creating instance", "Error creating instance", "error", err, "user_id", user.ID)
		return
	}

	// Switch to the new instance
	err = h.UserService.SwitchInstance(r.Context(), user, newGame.ID)
	if err != nil {
		h.handleError(w, r, "InstancesCreate: switching instance", "Error switching instance", "error", err)
		return
	}

	h.redirect(w, r, "/admin/instances")
}

// TemplatesDelete deletes a template.
func (h *AdminHandler) TemplatesDelete(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "TemplateDelete: parsing form", "Error parsing form", "error", err)
		return
	}

	id := r.Form.Get("id")
	if id == "" {
		h.handleError(w, r, "TemplateDelete: missing id", "Could not find the instance ID")
		return
	}

	template, err := h.TemplateService.GetByID(r.Context(), id)
	if err != nil {
		h.handleError(w, r, "TemplateDelete: getting template", "Error getting template", "error", err, "instance_id", user.CurrentInstanceID)
		return
	}

	_, err = h.InstanceService.DeleteInstance(r.Context(), user, template.ID, template.Name)
	if err != nil {
		h.handleError(w, r, "InstanceDelete: deleting instance", "Error deleting instance", "error", err, "instance_id", user.CurrentInstanceID)
	} else {
		err = templates.Toast(*flash.NewSuccess("Template deleted")).Render(r.Context(), w)
		if err != nil {
			h.Logger.Error("InstanceDelete: rendering template", "Error", err)
		}
	}

	gameTemplates, err := h.TemplateService.Find(r.Context(), user.ID)
	err = templates.Templates(gameTemplates).Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "Instances: rendering template", "Error rendering template", "error", err, "instance_id", user.CurrentInstanceID)
	}
}

// Fragments //

// TemplatesName retrieves the name of a template.
func (h *AdminHandler) TemplatesName(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	template, ok := h.getTemplateByID(w, r)
	if !ok {
		return
	}

	if err := templates.TemplateName(*template).Render(r.Context(), w); err != nil {
		h.Logger.Error("InstanceDelete: rendering template", "Error", err, "user_id", user.ID)
		_ = templates.TemplateName(*template).Render(r.Context(), w)
	}
}

// TemplatesName shows the form to edit the name of a template.
func (h *AdminHandler) TemplatesNameEdit(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	template, ok := h.getTemplateByID(w, r)
	if !ok {
		return
	}

	if err := templates.TemplateNameEdit(*template).Render(r.Context(), w); err != nil {
		h.Logger.Error("InstanceDelete: rendering template", "Error", err, "user_id", user.ID)
		_ = templates.TemplateNameEdit(*template).Render(r.Context(), w)
	}
}

// TemplatesNameEditPost updates the name of a template.
func (h *AdminHandler) TemplatesNameEditPost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// Fetch template, considering form data or URL param
	template, ok := h.getTemplateByID(w, r)
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "TemplateNameEditPost: parsing form", "Error parsing form", "error", err, "user_id", user.ID)
		_ = templates.TemplateNameEdit(*template).Render(r.Context(), w)
		return
	}

	name := r.Form.Get("name")
	if name == "" {
		h.handleError(w, r, "TemplateNameEditPost: missing name", "Please provide a name for the template")
		_ = templates.TemplateNameEdit(*template).Render(r.Context(), w)
		return
	}

	template.Name = name
	if err := h.TemplateService.Update(r.Context(), template); err != nil {
		h.Logger.Error("InstanceDelete: rendering template", "Error", err)
		_ = templates.TemplateNameEdit(*template).Render(r.Context(), w)
		return
	}

	h.handleSuccess(w, r, "Updated template name")

	if err := templates.TemplateName(*template).Render(r.Context(), w); err != nil {
		h.Logger.Error("InstanceDelete: rendering template", "Error", err)
	}
}

func (h *AdminHandler) TemplatesShare(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	template, ok := h.getTemplateByID(w, r)
	if !ok {
		return
	}

	if err := templates.TemplateShareModal(*template).Render(r.Context(), w); err != nil {
		h.handleError(w, r, "TemplateShare: rendering template", "Error rendering template", "error", err, "user_id", user.ID)
	}
}

func (h *AdminHandler) TemplatesSharePost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "TemplateSharePost: parsing form", "Error parsing form", "error", err, "user_id", user.ID)
		return
	}

	usesStr := r.Form.Get("limit")
	uses := 0
	if usesStr != "" {
		var err error
		uses, err = strconv.Atoi(usesStr)
		if err != nil {
			h.handleError(w, r, "TemplateSharePost: parsing uses", "Error parsing uses", "error", err, "user_id", user.ID)
			return
		}
	}

	data := services.ShareLinkData{
		TemplateID: r.Form.Get("id"),
		Validity:   r.Form.Get("validity"),
		MaxUses:    uses,
		Regenerate: r.Form.Has("regenerate"),
	}

	link, err := h.TemplateService.CreateShareLink(r.Context(), user.ID, data)
	if err != nil {
		h.handleError(w, r, "TemplateSharePost: creating link", "Error creating link", "error", err, "user_id", user.ID)
		_ = templates.TemplateShareModal(models.Instance{}).Render(r.Context(), w)
		return
	}

	err = templates.ShareLinkCopyModal(link).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("TemplateSharePost: rendering template", "Error", err, "user_id", user.ID)
	}

}
