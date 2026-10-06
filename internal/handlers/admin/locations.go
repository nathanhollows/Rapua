package admin

import (
	"net/http"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"

	"github.com/nathanhollows/Rapua/v8/internal/services"
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
			w, r, "Locations: loading objective points", "Error loading point badges",
			"error", err, "quest_id", user.CurrentQuestID,
		)
		return
	}

	// A lint failure is not a reason to withhold the builder: an author whose
	// quest is in a state lint cannot read is exactly the author who needs the
	// screen that lets them fix it.
	lintResult, err := h.lintService.LintQuest(r.Context(), user.CurrentQuestID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Locations: linting quest",
			"error", err, "quest_id", user.CurrentQuestID)
		// Said out loud rather than swallowed: the zero value is a clean
		// quest, so a failure here would otherwise tell an author their quest
		// is sound when nothing checked it.
		lintResult = services.QuestLint{Unavailable: true}
	}

	root, nodes := buildObjectiveTree(objectives, points, lintResult)

	c := templates.LockedEditor(
		user.CurrentQuest,
		templates.QuestBuilder(lintResult, root, nodes, user.CurrentQuest.Settings.EnablePoints),
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
	objectives []models.Objective, points map[string]int, lint services.QuestLint,
) (models.Objective, []*templates.ObjectiveTreeNode) {
	nodes := make(map[string]*templates.ObjectiveTreeNode, len(objectives))
	for i := range objectives {
		nodes[objectives[i].ID] = &templates.ObjectiveTreeNode{
			Objective: objectives[i],
			Points:    points[objectives[i].ID],
			Lint:      lint.For(objectives[i].ID),
		}
	}

	var root models.Objective
	hasParent := make(map[string]bool, len(objectives))
	for _, obj := range objectives {
		node := nodes[obj.ID]
		// Every row's own flag, before anything about where it sits: a row
		// whose parent is missing is still drafted or not, and skipping this
		// with the attach step drew a parked orphan as though it were live.
		node.Draft = obj.Draft
		node.OutOfPlay = obj.Draft

		if obj.ParentID == "" {
			root = obj
			continue
		}
		parent, ok := nodes[obj.ParentID]
		if !ok {
			continue
		}
		node.OutOfPlay = obj.Draft || parent.OutOfPlay
		parent.Children = append(parent.Children, node)
		hasParent[obj.ID] = true
	}

	cutCycleEdges(objectives, nodes, hasParent)

	// After assembly: both are facts about a node's children, not the node.
	for _, node := range nodes {
		node.PublishedChildren = 0
		for _, child := range node.Children {
			if !child.Draft {
				node.PublishedChildren++
			}
		}
		templates.MarkChildren(node.Objective, node.Children)
	}

	var topLevel []*templates.ObjectiveTreeNode
	if node, ok := nodes[root.ID]; ok {
		topLevel = node.Children
	}
	// A row nothing reaches would otherwise vanish with its subtree, and
	// damage nobody can see is damage nobody can mend: it is shown at the top
	// level instead, so the operator can drag it back where it belongs.
	//
	// Rows with no parent edge, not rows the root cannot reach: an orphan's
	// children are unreachable too, and appending those as well would draw the
	// same card once here and again nested under its parent.
	for _, obj := range objectives {
		if obj.ID == root.ID || hasParent[obj.ID] {
			continue
		}
		// Propagate down from here, since the attach loop ran in query order
		// and could not: a stranded row's own subtree was assembled before
		// anyone knew the row itself was parked.
		markOutOfPlay(nodes[obj.ID], nodes[obj.ID].Draft)
		topLevel = append(topLevel, nodes[obj.ID])
	}
	// Last, because strays are appended to topLevel after the root's children.
	templates.MarkChildren(root, topLevel)
	return root, topLevel
}

// markOutOfPlay carries a parked ancestor's state down a subtree.
func markOutOfPlay(node *templates.ObjectiveTreeNode, outOfPlay bool) {
	node.OutOfPlay = node.Draft || outOfPlay
	for _, child := range node.Children {
		markOutOfPlay(child, node.OutOfPlay)
	}
}

// cutCycleEdges removes the edges that close a loop, so what reaches the
// renderer is a tree.
//
// objectiveNode caps its own recursion depth, but a cycle would still draw the
// same rows over and over until it hit that cap. Dropping a back edge costs
// one nesting relationship and buys a graph that terminates on its own; lint
// still reports the cycle, and the row it frees becomes draggable at the top
// level.
//
// Walked in row order rather than map order so the edge that gets cut is the
// same one on every render.
func cutCycleEdges(
	objectives []models.Objective,
	nodes map[string]*templates.ObjectiveTreeNode,
	hasParent map[string]bool,
) {
	const (
		unvisited = 0
		onPath    = 1
		done      = 2
	)
	state := make(map[string]int, len(nodes))

	var walk func(node *templates.ObjectiveTreeNode)
	walk = func(node *templates.ObjectiveTreeNode) {
		state[node.Objective.ID] = onPath
		kept := node.Children[:0]
		for _, child := range node.Children {
			if state[child.Objective.ID] == onPath {
				delete(hasParent, child.Objective.ID)
				continue
			}
			kept = append(kept, child)
			if state[child.Objective.ID] == unvisited {
				walk(child)
			}
		}
		node.Children = kept
		state[node.Objective.ID] = done
	}

	for _, obj := range objectives {
		if state[obj.ID] == unvisited {
			walk(nodes[obj.ID])
		}
	}
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

	// NO_START_BUTTON is about this page and belongs to no row, so the builder
	// tree had nowhere to show it and this was the one screen where it
	// mattered.
	lintResult, err := h.lintService.LintQuest(r.Context(), user.CurrentQuestID)
	if err != nil {
		h.logger.ErrorContext(r.Context(), pageType+"PageEdit: linting quest",
			"error", err, "quest_id", user.CurrentQuestID)
		lintResult = services.QuestLint{Unavailable: true}
	}

	c := templates.LockedEditor(user.CurrentQuest, templates.SystemPageEditor(
		lintResult, systemPageLintPrefix(blockContext), templates.EditPageData{
			Settings:   user.CurrentQuest.Settings,
			PageBlocks: pageBlocks,
			PageTitle:  pageTitle,
			PageType:   pageType,
		}))
	if err := templates.Layout(c, *user, "Quest", "Edit "+pageTitle+" Page").Render(r.Context(), w); err != nil {
		h.handleError(w, r, pageType+"PageEdit: rendering template", "Error rendering template", "error", err)
	}
}

// systemPageLintPrefix maps a page's block context to the path the linter
// addresses that page by.
func systemPageLintPrefix(blockContext game.BlockContext) string {
	if blockContext == game.ContextFinish {
		return "finish"
	}
	return "start"
}
