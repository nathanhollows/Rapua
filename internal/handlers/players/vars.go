package players

import (
	"context"
	"net/http"
	"slices"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	playerTemplates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
	"github.com/nathanhollows/Rapua/v8/models"
)

func isPreview(ctx context.Context) bool {
	return ctx.Value(contextkeys.PreviewKey) != nil
}

// hiddenBlocks names the blocks still waiting on a variable the run has not
// set. Preview hides nothing: an author previewing has no run to unlock with.
//
// A failure to read the run's variables hides every gated block rather than
// showing them, since what a gate holds back is usually the answer to
// something.
func (h *PlayerHandler) hiddenBlocks(ctx context.Context, team *models.Run, list blocks.Blocks) map[string]bool {
	if isPreview(ctx) || !slices.ContainsFunc(list, func(b blocks.Block) bool { return len(b.GetDepends()) > 0 }) {
		return nil
	}
	vars, err := h.checkInService.RunVars(ctx, team)
	if err != nil {
		h.logger.ErrorContext(ctx, "loading run variables", "error", err.Error(), "team", team.Code)
	}
	hidden := make(map[string]bool)
	for _, block := range list {
		if !game.DependsMet(block.GetDepends(), vars) {
			hidden[block.GetID()] = true
		}
	}
	return hidden
}

// renderUnlockedBlocks swaps in the blocks on this page that the answer just
// opened, so a player sees what their answer earned without reloading. Blocks
// on other pages appear the next time those pages are drawn.
func (h *PlayerHandler) renderUnlockedBlocks(
	w http.ResponseWriter, r *http.Request, team *models.Run, answered blocks.Block, sets []string,
) {
	ctx := r.Context()
	blockContext, err := h.blockService.GetBlockContext(ctx, answered.GetID())
	if err != nil {
		h.logger.ErrorContext(ctx, "validateBlock: loading block context", "error", err.Error())
		return
	}
	siblings, states, err := h.blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		ctx, answered.GetOwnerID(), team.Code, team.QuestID, blockContext,
	)
	if err != nil {
		h.logger.ErrorContext(ctx, "validateBlock: loading sibling blocks", "error", err.Error())
		return
	}
	vars, err := h.checkInService.RunVars(ctx, team)
	if err != nil {
		h.logger.ErrorContext(ctx, "validateBlock: loading run variables", "error", err.Error())
		return
	}
	for _, block := range siblings {
		deps := block.GetDepends()
		// Only blocks waiting on something this answer sets: the rest of the
		// page is unchanged, and redrawing it would reset what the player had
		// open.
		if !slices.ContainsFunc(deps, func(name string) bool { return slices.Contains(sets, name) }) {
			continue
		}
		if !game.DependsMet(deps, vars) {
			continue
		}
		err := playerTemplates.PlayerBlockSlotOOB(team.Quest.Settings, block, states[block.GetID()]).Render(ctx, w)
		if err != nil {
			h.logger.ErrorContext(ctx, "validateBlock: rendering unlocked block", "error", err.Error())
			return
		}
	}
}
