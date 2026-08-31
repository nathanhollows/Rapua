package players

import (
	"fmt"
	"maps"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/blocks"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/blocks"
	playerTemplates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
	"github.com/nathanhollows/Rapua/v8/models"
)

// GetTeamNameBlock returns the team name block with proper state.
func (h *PlayerHandler) GetTeamNameBlock(w http.ResponseWriter, r *http.Request) {
	blockID := chi.URLParam(r, "id")

	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"failed to get team from context for team name block",
			"error", err.Error(),
			"block_id", blockID,
		)
		h.handleError(
			w,
			r,
			"getting team from context",
			"Unable to load your team information. Please try refreshing the page.",
		)
		return
	}

	// Get the block
	block, err := h.blockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(
			w,
			r,
			fmt.Errorf("getting block %s: %w", blockID, err).Error(),
			"This block could not be found. It may have been deleted by the game organiser.",
		)
		return
	}

	// Ensure it's a team name block
	teamNameBlock, ok := block.(*blocks.TeamNameChangerBlock)
	if !ok {
		h.handleError(
			w,
			r,
			"invalid block type",
			"This block has an unexpected configuration. Please contact the game organiser.",
		)
		return
	}

	// Determine which template to render based on team state
	if team.Name == "" {
		// No name set - show incomplete form
		err = templates.TeamNameChangerForm(*teamNameBlock, "").Render(r.Context(), w)
	} else {
		// Name is set - show complete state (or editable if AllowChanging is true)
		err = templates.TeamNameChangerComplete(
			*teamNameBlock, team.Name, teamNameBlock.AllowChanging,
		).Render(r.Context(), w)
	}

	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"failed to render team name block template",
			"error", err.Error(),
			"block_id", blockID,
			"team_code", team.Code,
			"has_team_name", team.Name != "",
		)
	}
}

// GetGameStatusAlertBlock returns the game status alert block with proper state.
func (h *PlayerHandler) GetGameStatusAlertBlock(w http.ResponseWriter, r *http.Request) {
	blockID := chi.URLParam(r, "id")

	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"failed to get team from context for game status alert block",
			"error", err.Error(),
			"block_id", blockID,
		)
		h.handleError(w, r, "getting team from context", "Unable to load game status. Please try refreshing the page.")
		return
	}

	// Get the block
	block, err := h.blockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(w, r, fmt.Errorf("getting block %s: %w", blockID, err).Error(), "This block could not be found.")
		return
	}

	// Ensure it's a game status alert block
	gameStatusBlock, ok := block.(*blocks.GameStatusAlertBlock)
	if !ok {
		h.handleError(w, r, "invalid block type", "This block has an unexpected configuration.")
		return
	}

	// Determine which template to render based on instance status
	status := team.Quest.GetStatus()
	switch status {
	case models.Closed:
		err = templates.GameStatusAlertClosed(*gameStatusBlock).Render(r.Context(), w)
	case models.Scheduled:
		err = templates.GameStatusAlertScheduled(*gameStatusBlock, team.Quest.StartTime.Time).Render(r.Context(), w)
	case models.Active:
		err = templates.GameStatusAlertActive(*gameStatusBlock).Render(r.Context(), w)
	default:
		err = templates.GameStatusAlertClosed(*gameStatusBlock).Render(r.Context(), w)
	}

	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"failed to render game status alert block template",
			"error", err.Error(),
			"block_id", blockID,
			"instance_status", status.String(),
		)
	}
}

// GetStartGameButtonBlock returns the start game button block with proper state.
func (h *PlayerHandler) GetStartGameButtonBlock(w http.ResponseWriter, r *http.Request) {
	blockID := chi.URLParam(r, "id")

	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"failed to get team from context for start game button block",
			"error", err.Error(),
			"block_id", blockID,
		)
		h.handleError(w, r, "getting team from context", "Unable to load start button. Please try refreshing the page.")
		return
	}

	// Get the block
	block, err := h.blockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(w, r, fmt.Errorf("getting block %s: %w", blockID, err).Error(), "This block could not be found.")
		return
	}

	// Ensure it's a start game button block
	startButtonBlock, ok := block.(*blocks.StartGameButtonBlock)
	if !ok {
		h.handleError(w, r, "invalid block type", "This block has an unexpected configuration.")
		return
	}

	// Determine which template to render based on instance status
	status := team.Quest.GetStatus()
	switch status {
	case models.Closed:
		err = templates.StartGameButtonClosed(*startButtonBlock).Render(r.Context(), w)
	case models.Scheduled:
		err = templates.StartGameButtonScheduled(*startButtonBlock).Render(r.Context(), w)
	case models.Active:
		err = templates.StartGameButtonActive(*startButtonBlock).Render(r.Context(), w)
	default:
		err = templates.StartGameButtonClosed(*startButtonBlock).Render(r.Context(), w)
	}

	if err != nil {
		h.logger.ErrorContext(r.Context(),
			"failed to render start game button block template",
			"error", err.Error(),
			"block_id", blockID,
			"instance_status", status.String(),
		)
	}
}

// ValidateBlock runs input validation on the block.
func (h *PlayerHandler) ValidateBlock(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.handleError(w, r, "validateBlock: getting team from context", "Something went wrong!")
		return
	}

	err = r.ParseForm()
	if err != nil {
		h.handleError(w, r, fmt.Errorf("validateBlock: parsing form: %w", err).Error(), "Something went wrong!")
		return
	}
	data := make(map[string][]string)
	maps.Copy(data, r.PostForm)

	state, block, err := h.checkInService.ValidateAndUpdateBlockState(r.Context(), *team, data)
	if err != nil {
		blockID := "unknown"
		if block != nil {
			blockID = block.GetID()
		}
		h.logger.ErrorContext(r.Context(),
			"validateBlock: validating and updating block state",
			"Something went wrong. Please try again.",
			err,
			"block",
			blockID,
			"team",
			team.Code,
		)
		return
	}

	// If the block fires sets triggers on completion, notify listening containers
	// so they can re-fetch and re-evaluate block visibility without a full reload.
	if state.IsComplete() && len(block.GetSets()) > 0 {
		w.Header().Set("Hx-Trigger", "varsChanged")
	}

	err = templates.RenderPlayerUpdate(team.Quest.Settings, block, state).Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, fmt.Errorf("validateBlock: rendering template: %w", err).Error(), "Something went wrong!")
		return
	}

	if state.IsComplete() {
		h.renderObjectiveZoneSwapIfProofJustCompleted(w, r, team, block)
	}
}

// renderObjectiveZoneSwapIfProofJustCompleted appends an out-of-band swap of
// an objective's reveal zone to the response, when the block that just
// completed was the last unvalidated block in that objective's proof
// context. Reveal is a content-only zone as often as not (nothing for a
// player to POST), so its completion is triggered here rather than by a
// player action of its own.
func (h *PlayerHandler) renderObjectiveZoneSwapIfProofJustCompleted(
	w http.ResponseWriter, r *http.Request, team *models.Run, block blocks.Block,
) {
	blockContext, err := h.blockService.GetBlockContext(r.Context(), block.GetID())
	if err != nil || blockContext != blocks.ContextObjectiveProof {
		return
	}

	pending, err := h.checkInService.IsObjectiveContextPending(r.Context(), team, block.GetOwnerID(), blocks.ContextObjectiveProof)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "validateBlock: checking proof completion", "error", err.Error())
		return
	}
	if pending {
		return
	}

	if err := h.checkInService.CompleteObjectiveContext(r.Context(), team, block.GetOwnerID(), blocks.ContextObjectiveReveal); err != nil {
		h.logger.ErrorContext(r.Context(), "validateBlock: completing reveal context", "error", err.Error())
		return
	}

	revealBlocks, revealStates, err := h.blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		r.Context(), block.GetOwnerID(), team.Code, team.QuestID, blocks.ContextObjectiveReveal,
	)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "validateBlock: loading reveal blocks", "error", err.Error())
		return
	}

	data := playerTemplates.ObjectiveViewData{
		Settings: team.Quest.Settings,
		Zone:     blocks.ContextObjectiveReveal,
		Blocks:   revealBlocks,
		States:   revealStates,
	}
	if err := playerTemplates.ObjectiveZoneUpdate(data).Render(r.Context(), w); err != nil {
		h.logger.ErrorContext(r.Context(), "validateBlock: rendering reveal zone", "error", err.Error())
	}
}
