package players

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/internal/contextkeys"
	blockstemplates "github.com/nathanhollows/Rapua/v6/internal/templates/blocks"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/players"
)

const (
	teamNameMaxLength = 50
)

// Lobby is where teams wait for the game to begin.
func (h *PlayerHandler) Lobby(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	err = h.teamService.LoadRelations(r.Context(), team)
	if err != nil {
		h.logger.Error("loading check ins", "error", err.Error())
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		return
	}

	// Get blocks for the start page (lobby context)
	pageBlocks, blockStates, err := h.blockService.FindByOwnerIDAndTeamCodeWithStateAndContext(
		r.Context(),
		team.InstanceID,
		team.Code,
		blocks.ContextLobby,
	)
	if err != nil {
		h.logger.Error("getting lobby blocks", "error", err.Error())
		http.Redirect(w, r, r.Header.Get("Referer"), http.StatusFound)
		return
	}

	// If the user is in preview mode, only render the template, not the full layout.
	template := templates.Lobby(*team, pageBlocks, blockStates)
	if r.Context().Value(contextkeys.PreviewKey) == nil {
		template = templates.Layout(template, "Start", team.Messages)
	}

	err = template.Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering lobby", "error", err.Error())
	}
}

// GetTeamNameValue returns just the team name value for auto-population.
func (h *PlayerHandler) GetTeamNameValue(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	_, err = w.Write([]byte(team.Name))
	if err != nil {
		h.logger.Error("writing team name value", "error", err.Error())
	}
}

// GetTeamNameForm returns the team name form fragment.
func (h *PlayerHandler) GetTeamNameForm(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	err = templates.TeamNameForm(*team).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering team name form", "error", err.Error())
	}
}

// SetTeamName sets the team name and returns completion status.
func (h *PlayerHandler) SetTeamName(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
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
		h.handleError(w, r, fmt.Errorf("getting block %s: %w", blockID, err).Error(), "Unable to save team name. The block configuration could not be loaded.")
		return
	}

	teamNameBlock, ok := block.(*blocks.TeamNameChangerBlock)
	if !ok {
		h.handleError(w, r, "invalid block type", "Unable to save team name. This block has an unexpected configuration.")
		return
	}

	// Update team name
	teamName := strings.TrimSpace(r.FormValue("name"))
	if teamName == "" {
		h.handleError(w, r, "Team name cannot be empty", "Team name cannot be empty")
		return
	}
	if len(teamName) > teamNameMaxLength {
		h.handleError(w, r, "Team name too long", fmt.Sprintf("Team name cannot be longer than %d characters.", teamNameMaxLength))
		return
	}

	team.Name = teamName
	err = h.teamService.Update(r.Context(), team)
	if err != nil {
		h.handleError(w, r, "Error updating team", "Error updating team", "error", err)
		return
	}

	// Return the complete block state
	err = blockstemplates.TeamNameChangerComplete(*teamNameBlock, team.Name, teamNameBlock.AllowChanging).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering team name complete", "error", err.Error())
	}
}
