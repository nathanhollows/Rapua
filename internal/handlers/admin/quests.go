package admin

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v7/internal/flash"
	templates "github.com/nathanhollows/Rapua/v7/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v7/models"
)

// Quests shows admin the quests.
func (h *Handler) Quests(w http.ResponseWriter, r *http.Request) {
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
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	c := templates.Instances(user.Quests, user.CurrentQuest, gameTemplates)
	err = templates.Layout(c, *user, "Games and Templates", "Games and Templates").Render(r.Context(), w)
	if err != nil {
		h.handleError(
			w,
			r,
			"Instances: rendering template",
			"Error rendering template",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
	}
}

// QuestsCreate creates a new quest.
func (h *Handler) QuestsCreate(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(
			w,
			r,
			"InstancesCreate: parsing form",
			"Error parsing form",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	name := r.FormValue("name")
	instance, err := h.questService.CreateQuest(r.Context(), name, user)
	if err != nil {
		h.handleError(
			w,
			r,
			"InstancesCreate: creating instance",
			"Error creating instance",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	// Switch to the new instance
	err = h.userService.SwitchQuest(r.Context(), user, instance.ID)
	if err != nil {
		h.handleError(w, r, "InstancesCreate: switching instance", "Error switching instance", "error", err)
		return
	}
	h.setCurrentQuest(w, r, instance.ID)

	h.redirect(w, r, "/admin/quests")
}

// QuestDuplicate duplicates a quest.
func (h *Handler) QuestDuplicate(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "parsing form", "Error parsing form", "error", err)
		return
	}

	id := r.Form.Get("id")
	name := r.Form.Get("name")

	instance, err := h.duplicationService.DuplicateQuest(r.Context(), user, id, name)
	if err != nil {
		h.handleError(
			w,
			r,
			"InstanceDuplicate: duplicating instance",
			"Error duplicating instance",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	err = h.userService.SwitchQuest(r.Context(), user, instance.ID)
	if err != nil {
		h.handleError(
			w,
			r,
			"InstanceDuplicate: switching instance",
			"Error switching instance",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}
	h.setCurrentQuest(w, r, instance.ID)

	h.redirect(w, r, "/admin/quests")
}

// QuestSwitch switches the current quest.
func (h *Handler) QuestSwitch(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	questID := chi.URLParam(r, "id")
	if questID == "" {
		h.handleError(
			w,
			r,
			"InstanceSwitch: missing instance ID",
			"Could not switch instance",
			"error",
			"Instance ID is required",
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	err := h.userService.SwitchQuest(r.Context(), user, questID)
	if err != nil {
		h.handleError(
			w,
			r,
			"InstanceSwitch: switching instance",
			"Error switching instance",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}
	h.setCurrentQuest(w, r, questID)

	if r.URL.Query().Has("redirect") {
		h.redirect(w, r, r.URL.Query().Get("redirect"))
		return
	}

	h.redirect(w, r, r.Header.Get("Referer"))
}

// QuestDelete deletes a quest.
func (h *Handler) QuestDelete(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(
			w,
			r,
			"InstanceDelete: parsing form",
			"Error parsing form",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
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
			"quest_id",
			user.CurrentQuestID,
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
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	instance, err := h.questService.GetByID(r.Context(), id)
	if err != nil {
		h.handleError(
			w,
			r,
			"InstanceDelete: getting instance",
			"Could not find the instance",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	if confirmName != instance.Name {
		h.handleError(
			w,
			r,
			"InstanceDelete: name mismatch",
			"The game name you entered does not match",
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	if user.CurrentQuestID == id {
		renderErr := templates.Toast(*flash.NewError("You cannot delete the instance you are currently using")).
			Render(r.Context(), w)
		if renderErr != nil {
			h.logger.ErrorContext(r.Context(), "InstanceDelete: rendering template", "error", renderErr)
		}
		return
	}

	err = h.deleteService.DeleteQuest(r.Context(), user.ID, id)
	if err != nil {
		h.handleError(
			w,
			r,
			"InstanceDelete: deleting instance",
			"Error deleting instance",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	h.redirect(w, r, "/admin/quests")
}

// getInstanceByID retrieves an instance by ID from various sources (param, form).
func (h *Handler) getInstanceByID(
	w http.ResponseWriter,
	r *http.Request,
) (*models.Quest, bool) {
	var id string

	// Check form value (for POST requests)
	if err := r.ParseForm(); err == nil {
		id = r.Form.Get("id")
	}

	// Fallback to URL param if not found in form
	if id == "" {
		id = chi.URLParam(r, "id")
	}

	if id == "" {
		h.handleError(w, r, "InstanceName: missing id", "Could not find the instance ID")
		return nil, false
	}

	instance, err := h.questService.GetByID(r.Context(), id)
	if err != nil {
		h.handleError(w, r, "InstanceName: getting instance", "Error getting instance", "error", err)
		return nil, false
	}

	return instance, true
}

// QuestsName retrieves the name of a quest.
func (h *Handler) QuestsName(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	instance, ok := h.getInstanceByID(w, r)
	if !ok {
		return
	}

	if err := templates.InstanceName(*instance, instance.ID == user.CurrentQuestID).Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "InstancesName: rendering template", "Error", err, "user_id", user.ID)
		_ = templates.InstanceName(*instance, instance.ID == user.CurrentQuestID).Render(r.Context(), w)
	}
}

// QuestsNameEdit shows the form to edit the name of a quest.
func (h *Handler) QuestsNameEdit(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	instance, ok := h.getInstanceByID(w, r)
	if !ok {
		return
	}

	if err := templates.InstanceNameEdit(*instance).Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "InstancesNameEdit: rendering template", "Error", err, "user_id", user.ID)
		_ = templates.InstanceNameEdit(*instance).Render(r.Context(), w)
	}
}

// QuestsNameEditPost updates the name of a quest.
func (h *Handler) QuestsNameEditPost(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// Fetch instance, considering form data or URL param
	instance, ok := h.getInstanceByID(w, r)
	if !ok {
		return
	}

	if err := r.ParseForm(); err != nil {
		h.handleError(
			w,
			r,
			"InstancesNameEditPost: parsing form",
			"Error parsing form",
			"error",
			err,
			"user_id",
			user.ID,
		)
		_ = templates.InstanceNameEdit(*instance).Render(r.Context(), w)
		return
	}

	name := r.Form.Get("name")
	if name == "" {
		h.handleError(w, r, "InstancesNameEditPost: missing name", "Please provide a name for the instance")
		_ = templates.InstanceNameEdit(*instance).Render(r.Context(), w)
		return
	}

	instance.Name = name
	if err := h.questService.Update(r.Context(), instance); err != nil {
		h.logger.ErrorContext(r.Context(), "InstancesNameEditPost: updating instance", "Error", err)
		_ = templates.InstanceNameEdit(*instance).Render(r.Context(), w)
		return
	}

	h.handleSuccess(w, r, "Updated instance name")

	if err := templates.InstanceName(*instance, instance.ID == user.CurrentQuestID).Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "InstancesNameEditPost: rendering template", "Error", err)
	}
}
