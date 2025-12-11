package players

import (
	"fmt"
	"maps"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v6/blocks"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/blocks"
	"github.com/nathanhollows/Rapua/v6/models"
)

// GetTeamNameBlock returns the team name block with proper state.
func (h *PlayerHandler) GetTeamNameBlock(w http.ResponseWriter, r *http.Request) {
	blockID := chi.URLParam(r, "id")

	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.logger.Error(
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
		err = templates.TeamNameChangerComplete(*teamNameBlock, team.Name, teamNameBlock.AllowChanging).Render(r.Context(), w)
	}

	if err != nil {
		h.logger.Error(
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

	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.logger.Error(
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
	status := team.Instance.GetStatus()
	switch status {
	case models.Closed:
		err = templates.GameStatusAlertClosed(*gameStatusBlock).Render(r.Context(), w)
	case models.Scheduled:
		err = templates.GameStatusAlertScheduled(*gameStatusBlock, team.Instance.StartTime.Time).Render(r.Context(), w)
	case models.Active:
		err = templates.GameStatusAlertActive(*gameStatusBlock).Render(r.Context(), w)
	default:
		err = templates.GameStatusAlertClosed(*gameStatusBlock).Render(r.Context(), w)
	}

	if err != nil {
		h.logger.Error(
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

	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.logger.Error(
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
	status := team.Instance.GetStatus()
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
		h.logger.Error(
			"failed to render start game button block template",
			"error", err.Error(),
			"block_id", blockID,
			"instance_status", status.String(),
		)
	}
}

// ValidateBlock runs input validation on the block.
func (h *PlayerHandler) ValidateBlock(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
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
		h.logger.Error(
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

	err = templates.RenderPlayerUpdate(team.Instance.Settings, block, state).Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, fmt.Errorf("validateBlock: rendering template: %w", err).Error(), "Something went wrong!")
		return
	}
}
