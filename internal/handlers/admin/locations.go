package admin

import (
	"encoding/json"
	"net/http"

	"github.com/nathanhollows/Rapua/v8/blocks"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
)

// Locations shows admin the quest builder: the game structure with its
// objectives and their blocks loaded.
func (h *Handler) Locations(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := h.gameStructureService.Load(
		r.Context(),
		user.CurrentQuestID,
		&user.CurrentQuest.GameStructure,
		true, // recursive
	)
	if err != nil {
		h.handleError(
			w,
			r,
			"Locations: loading game structure",
			"Error loading game structure",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	// Needed for the clue indicators.
	err = h.gameStructureService.LoadBlocksForStructure(
		r.Context(),
		&user.CurrentQuest.GameStructure,
		true, // recursive
	)
	if err != nil {
		h.handleError(
			w,
			r,
			"Locations: loading blocks",
			"Error loading blocks",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	c := templates.LocationGroupList(user.CurrentQuest.Settings, user.CurrentQuest.GameStructure)
	err = templates.Layout(c, *user, "Quest", "Quest").Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Locations: rendering template", "error", err)
	}
}

// SaveGameStructure handles saving the game structure from the browser.
func (h *Handler) SaveGameStructure(w http.ResponseWriter, r *http.Request) {
	// Check HTMX headers
	if r.Header.Get("Hx-Request") != htmxHeaderTrue {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	user := h.UserFromContext(r.Context())

	// Parse form data
	if err := r.ParseForm(); err != nil {
		h.handleError(
			w,
			r,
			"SaveGameStructure: parsing form",
			"Error parsing form",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	// Get the structure JSON from form
	structureJSON := r.FormValue("structure")
	if structureJSON == "" {
		h.handleError(w, r, "SaveGameStructure: missing structure", "Missing structure data")
		return
	}

	// Parse the JSON
	var structure models.GameStructure
	if err := json.Unmarshal([]byte(structureJSON), &structure); err != nil {
		h.handleError(
			w,
			r,
			"SaveGameStructure: decoding JSON",
			"Invalid JSON: "+err.Error(),
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	// Validate and save using the service
	if err := h.gameStructureService.Save(r.Context(), user.CurrentQuestID, &structure); err != nil {
		h.handleError(
			w,
			r,
			"SaveGameStructure: saving structure",
			"Validation failed: "+err.Error(),
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		return
	}

	h.handleSuccess(w, r, "Game structure saved")
}

// StartPageEdit shows the start page editor.
func (h *Handler) StartPageEdit(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// Get blocks for the start page.
	pageBlocks, err := h.blockService.FindByOwnerIDAndContext(
		r.Context(),
		user.CurrentQuestID,
		blocks.ContextStart,
	)
	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"StartPageEdit: getting blocks",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		h.redirect(w, r, "/admin/quest")
		return
	}

	data := templates.EditPageData{
		Settings:   user.CurrentQuest.Settings,
		PageBlocks: pageBlocks,
		PageTitle:  "Start",
		PageType:   "start",
	}

	c := templates.EditPage(data)
	err = templates.Layout(c, *user, "Quest", "Edit Start Page").Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "StartPageEdit: rendering template", "Error rendering template", "error", err)
	}
}

// CompletePageEdit shows the complete page editor.
func (h *Handler) CompletePageEdit(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// Get blocks for the complete page
	pageBlocks, err := h.blockService.FindByOwnerIDAndContext(
		r.Context(),
		user.CurrentQuestID,
		blocks.ContextFinish,
	)
	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"completePageEdit: getting blocks",
			"error",
			err,
			"quest_id",
			user.CurrentQuestID,
		)
		h.redirect(w, r, "/admin/quest")
		return
	}

	data := templates.EditPageData{
		Settings:   user.CurrentQuest.Settings,
		PageBlocks: pageBlocks,
		PageTitle:  "Complete",
		PageType:   "complete",
	}

	c := templates.EditPage(data)
	err = templates.Layout(c, *user, "Quest", "Edit Complete Page").Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "CompletePageEdit: rendering template", "Error rendering template", "error", err)
	}
}
