package handlers

import (
	"fmt"
	"net/http"

	templates "github.com/nathanhollows/Rapua/v3/internal/templates/blocks"
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
		h.handleError(w, r, fmt.Errorf("validateBlock: parsing form: %v", err).Error(), "Something went wrong!")
		return
	}
	data := make(map[string][]string)
	for key, value := range r.Form {
		data[key] = value
	}

	state, block, err := h.GameplayService.ValidateAndUpdateBlockState(r.Context(), *team, data)
	if err != nil {
		h.Logger.Error("validateBlock: validating and updating block state", "Something went wrong. Please try again.", err, "block", block.GetID(), "team", team.Code)
		return
	}

	err = templates.RenderPlayerUpdate(team.Instance.Settings, block, state).Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, fmt.Errorf("validateBlock: rendering template: %v", err).Error(), "Something went wrong!")
		return
	}
}
