package admin

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v8/blocks"

	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
)

// Locations shows admin the quest builder: the game structure with its
// objectives and their blocks loaded.
func (h *Handler) Locations(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	objectives, err := h.objectiveService.FindTree(r.Context(), user.CurrentQuestID)
	if err != nil {
		h.handleError(
			w, r, "Locations: loading objective tree", "Error loading quest",
			"error", err, "quest_id", user.CurrentQuestID,
		)
		return
	}

	c := templates.ObjectiveTree(objectiveTreeNodes(objectives))
	err = templates.Layout(c, *user, "Quest", "Quest").Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Locations: rendering template", "error", err)
	}
}

// objectiveTreeNodes flattens the tree for rendering. The rows arrive with each
// parent ahead of its children, so depth is the parent's depth plus one, and
// the root is dropped: it holds everything, so listing it says nothing.
func objectiveTreeNodes(objectives []models.Objective) []templates.ObjectiveTreeNode {
	depths := make(map[string]int, len(objectives))
	hasChildren := make(map[string]bool, len(objectives))
	for _, obj := range objectives {
		hasChildren[obj.ParentID] = true
	}

	nodes := make([]templates.ObjectiveTreeNode, 0, len(objectives))
	for _, obj := range objectives {
		if obj.ParentID == "" {
			depths[obj.ID] = 0
			continue
		}
		depth := depths[obj.ParentID] + 1
		depths[obj.ID] = depth
		nodes = append(nodes, templates.ObjectiveTreeNode{
			Objective: obj,
			Depth:     depth - 1,
			IsSection: hasChildren[obj.ID],
		})
	}
	return nodes
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
