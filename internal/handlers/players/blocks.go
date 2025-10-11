package players

import (
	"fmt"
	"maps"
	"net/http"

	templates "github.com/nathanhollows/Rapua/v4/internal/templates/blocks"
)

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
