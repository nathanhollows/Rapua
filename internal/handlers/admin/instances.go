package admin

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v4/internal/flash"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/admin"
)

// Instances shows admin the instances.
func (h *Handler) Instances(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// We need to show both the instances and the templates
	gameTemplates, err := h.templateService.Find(r.Context(), user.ID)
	if err != nil {
		h.handleError(
			w,
			r,
			"Instances: finding templates",
			"Error finding templates",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	c := templates.Instances(user.Instances, user.CurrentInstance, gameTemplates)
	err = templates.Layout(c, *user, "Games and Templates", "Games and Templates").Render(r.Context(), w)
	if err != nil {
		h.handleError(
			w,
			r,
			"Instances: rendering template",
			"Error rendering template",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
	}
}

// InstancesCreate creates a new instance.
func (h *Handler) InstancesCreate(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(
			w,
			r,
			"InstancesCreate: parsing form",
			"Error parsing form",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	name := r.FormValue("name")
	instance, err := h.instanceService.CreateInstance(r.Context(), name, user)
	if err != nil {
		h.handleError(
			w,
			r,
			"InstancesCreate: creating instance",
			"Error creating instance",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	// Switch to the new instance
	err = h.userService.SwitchInstance(r.Context(), user, instance.ID)
	if err != nil {
		h.handleError(w, r, "InstancesCreate: switching instance", "Error switching instance", "error", err)
		return
	}

	h.redirect(w, r, "/admin/instances")
}

// InstanceDuplicate duplicates an instance.
func (h *Handler) InstanceDuplicate(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "parsing form", "Error parsing form", "error", err)
		return
	}

	id := r.Form.Get("id")
	name := r.Form.Get("name")

	instance, err := h.duplicationService.DuplicateInstance(r.Context(), user, id, name)
	if err != nil {
		h.handleError(
			w,
			r,
			"InstanceDuplicate: duplicating instance",
			"Error duplicating instance",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	err = h.userService.SwitchInstance(r.Context(), user, instance.ID)
	if err != nil {
		h.handleError(
			w,
			r,
			"InstanceDuplicate: switching instance",
			"Error switching instance",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	h.redirect(w, r, "/admin/instances")
}

// InstanceSwitch switches the current instance.
func (h *Handler) InstanceSwitch(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	instanceID := chi.URLParam(r, "id")
	if instanceID == "" {
		h.handleError(
			w,
			r,
			"InstanceSwitch: missing instance ID",
			"Could not switch instance",
			"error",
			"Instance ID is required",
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	err := h.userService.SwitchInstance(r.Context(), user, instanceID)
	if err != nil {
		h.handleError(
			w,
			r,
			"InstanceSwitch: switching instance",
			"Error switching instance",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	if r.URL.Query().Has("redirect") {
		h.redirect(w, r, r.URL.Query().Get("redirect"))
		return
	}

	h.redirect(w, r, r.Header.Get("Referer"))
}

// InstanceDelete deletes an instance.
func (h *Handler) InstanceDelete(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(
			w,
			r,
			"InstanceDelete: parsing form",
			"Error parsing form",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	id := r.Form.Get("id")
	if id == "" {
		h.handleError(
			w,
			r,
			"InstanceDelete: missing instance ID",
			"Could not find the instance ID",
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	confirmName := r.Form.Get("confirmname")
	if confirmName == "" {
		h.handleError(
			w,
			r,
			"InstanceDelete: missing name",
			"Please type the game name to confirm",
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	// TODO: Check if the confirmName matches the instance name

	if user.CurrentInstanceID == id {
		err := templates.Toast(*flash.NewError("You cannot delete the instance you are currently using")).
			Render(r.Context(), w)
		if err != nil {
			h.logger.Error("InstanceDelete: rendering template", "error", err)
		}
		return
	}

	err := h.deleteService.DeleteInstance(r.Context(), user.ID, id)
	if err != nil {
		h.handleError(
			w,
			r,
			"InstanceDelete: deleting instance",
			"Error deleting instance",
			"error",
			err,
			"instance_id",
			user.CurrentInstanceID,
		)
		return
	}

	h.redirect(w, r, "/admin/instances")
}
