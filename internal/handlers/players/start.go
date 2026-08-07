package players

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	blockstemplates "github.com/nathanhollows/Rapua/v8/internal/templates/blocks"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
)

const (
	teamNameMaxLength = 50
)

// Start is where teams wait for the game to begin.
func (h *PlayerHandler) Start(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	err = h.runService.LoadRelations(r.Context(), team)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "loading check ins", "error", err.Error())
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		return
	}

	// Get blocks for the start page.
	// In preview mode the team has no DB row, so passing its code would cause a
	// FK violation when block states are persisted. Use "" instead so mock
	// (non-persisted) states are created.
	teamCode := team.Code
	if r.Context().Value(contextkeys.PreviewKey) != nil {
		teamCode = ""
	}
	pageBlocks, blockStates, err := h.blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		r.Context(),
		team.QuestID,
		teamCode,
		team.QuestID,
		blocks.ContextStart,
	)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "getting start blocks", "error", err.Error())
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		return
	}

	// If the user is in preview mode, only render the template, not the full layout.
	template := templates.Start(*team, pageBlocks, blockStates)
	if r.Context().Value(contextkeys.PreviewKey) == nil {
		template = templates.Layout(template, "Start", team.Messages)
	}

	err = template.Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "rendering start", "error", err.Error())
	}
}

// GetTeamNameValue returns just the team name value for auto-population.
func (h *PlayerHandler) GetTeamNameValue(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	_, err = w.Write([]byte(team.Name))
	if err != nil {
		h.logger.ErrorContext(r.Context(), "writing team name value", "error", err.Error())
	}
}

// GetTeamNameForm returns the team name form fragment.
func (h *PlayerHandler) GetTeamNameForm(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	err = templates.TeamNameForm(*team).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "rendering team name form", "error", err.Error())
	}
}

// SetTeamName sets the team name and returns completion status.
func (h *PlayerHandler) SetTeamName(w http.ResponseWriter, r *http.Request) {
	team, err := h.getRunFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	if err = r.ParseForm(); err != nil {
		h.handleError(w, r, "Error parsing form", "Error parsing form", "error", err)
		return
	}

	blockID := r.FormValue("block")

	// Get the block BEFORE saving to check current settings
	block, err := h.blockService.GetByBlockID(r.Context(), blockID)
	if err != nil {
		h.handleError(
			w,
			r,
			fmt.Errorf("getting block %s: %w", blockID, err).Error(),
			"Unable to save team name. The block configuration could not be loaded.",
		)
		return
	}

	teamNameBlock, ok := block.(*blocks.TeamNameChangerBlock)
	if !ok {
		h.handleError(
			w,
			r,
			"invalid block type",
			"Unable to save team name. This block has an unexpected configuration.",
		)
		return
	}

	// Update team name
	teamName := strings.TrimSpace(r.FormValue("name"))
	if teamName == "" {
		h.handleError(w, r, "Team name cannot be empty", "Team name cannot be empty")
		return
	}
	if len(teamName) > teamNameMaxLength {
		h.handleError(
			w,
			r,
			"Team name too long",
			fmt.Sprintf("Team name cannot be longer than %d characters.", teamNameMaxLength),
		)
		return
	}

	team.Name = teamName
	err = h.runService.Update(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "Error updating team", "Error updating team", "error", err)
		return
	}

	// Return the complete block state
	err = blockstemplates.TeamNameChangerComplete(*teamNameBlock, team.Name, teamNameBlock.AllowChanging).
		Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "rendering team name complete", "error", err.Error())
	}
}
