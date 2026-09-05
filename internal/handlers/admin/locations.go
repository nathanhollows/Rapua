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

	objectiveIDs := make([]string, len(objectives))
	for i, obj := range objectives {
		objectiveIDs[i] = obj.ID
	}
	points, err := h.blockService.FindPointsByOwnerIDs(r.Context(), objectiveIDs)
	if err != nil {
		h.handleError(
			w, r, "Locations: loading objective points", "Error loading quest",
			"error", err, "quest_id", user.CurrentQuestID,
		)
		return
	}

	root, nodes := buildObjectiveTree(objectives, points)

	c := templates.LockedEditor(
		user.CurrentQuest,
		templates.ObjectiveTree(root, nodes, user.CurrentQuest.Settings.EnablePoints),
	)
	err = templates.Layout(c, *user, "Quest", "Quest").Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Locations: rendering template", "error", err)
	}
}

// buildObjectiveTree nests FindTree's flat, parent-first rows by pointer, so
// a parent built early still picks up children appended later in the same
// pass. Root is returned separately: it's the quest itself, not a row, but
// still needs somewhere to carry its own settings.
//
// Draft is split from OutOfPlay because a row hidden only by an ancestor
// must still look hidden, or edits below a parked section go blind to what
// players can see.
func buildObjectiveTree(
	objectives []models.Objective, points map[string]int,
) (models.Objective, []*templates.ObjectiveTreeNode) {
	nodes := make(map[string]*templates.ObjectiveTreeNode, len(objectives))
	for i := range objectives {
		nodes[objectives[i].ID] = &templates.ObjectiveTreeNode{
			Objective: objectives[i],
			Points:    points[objectives[i].ID],
		}
	}

	var root models.Objective
	for _, obj := range objectives {
		node := nodes[obj.ID]
		if obj.ParentID == "" {
			node.Draft = obj.Draft
			node.OutOfPlay = obj.Draft
			root = obj
			continue
		}
		// An orphan or a cycle is left out of the tree rather than guessed at.
		parent, ok := nodes[obj.ParentID]
		if !ok {
			continue
		}
		node.Draft = obj.Draft
		node.OutOfPlay = obj.Draft || parent.OutOfPlay
		parent.Children = append(parent.Children, node)
	}

	if node, ok := nodes[root.ID]; ok {
		return root, node.Children
	}
	return root, nil
}

// StartPageEdit shows the start page editor.
func (h *Handler) StartPageEdit(w http.ResponseWriter, r *http.Request) {
	h.systemPageEdit(w, r, blocks.ContextStart, "Start", "start")
}

// CompletePageEdit shows the complete page editor.
func (h *Handler) CompletePageEdit(w http.ResponseWriter, r *http.Request) {
	h.systemPageEdit(w, r, blocks.ContextFinish, "Complete", "complete")
}

// systemPageEdit covers Start and Complete: they differ only in block
// context and naming.
func (h *Handler) systemPageEdit(
	w http.ResponseWriter,
	r *http.Request,
	blockContext blocks.BlockContext,
	pageTitle, pageType string,
) {
	user := h.UserFromContext(r.Context())

	pageBlocks, err := h.blockService.FindByOwnerIDAndContext(r.Context(), user.CurrentQuestID, blockContext)
	if err != nil {
		h.logger.ErrorContext(r.Context(), pageType+"PageEdit: getting blocks",
			"error", err, "quest_id", user.CurrentQuestID)
		h.redirect(w, r, "/admin/quest")
		return
	}

	c := templates.LockedEditor(user.CurrentQuest, templates.EditPage(templates.EditPageData{
		Settings:   user.CurrentQuest.Settings,
		PageBlocks: pageBlocks,
		PageTitle:  pageTitle,
		PageType:   pageType,
	}))
	if err := templates.Layout(c, *user, "Quest", "Edit "+pageTitle+" Page").Render(r.Context(), w); err != nil {
		h.handleError(w, r, pageType+"PageEdit: rendering template", "Error rendering template", "error", err)
	}
}
