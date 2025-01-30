package handlers

import (
	"errors"
	"net/http"

	"github.com/nathanhollows/Rapua/internal/services"
	templates "github.com/nathanhollows/Rapua/internal/templates/players"
)

func (h *PlayerHandler) Finish(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		h.redirect(w, r, "/play")
		return
	}

	locations, err := h.GameplayService.SuggestNextLocations(r.Context(), team)
	if err != nil {
		if !errors.Is(err, services.ErrAllLocationsVisited) {
			h.handleError(w, r, "Next: getting next locations", "Error getting next locations", "Could not load data", err)
			return
		}
	}
	if len(locations) > 0 {
		h.redirect(w, r, "/next")
		return
	}

	// data["notifications"], _ = h.NotificationService.GetNotifications(r.Context(), team.Code)
	c := templates.Finish(*team, locations)
	err = templates.Layout(c, "The End", team.Messages).Render(r.Context(), w)
}
