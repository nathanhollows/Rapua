package handlers

import (
	"net/http"
	"strconv"

	admin "github.com/nathanhollows/Rapua/v3/internal/templates/admin"
)

func (h *AdminHandler) Teams(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	c := admin.Teams(user.CurrentInstance.Teams)
	err := admin.Layout(c, *user, "Teams", "Teams").Render(r.Context(), w)

	if err != nil {
		h.logger.Error("rendering teams page", "error", err.Error())
	}
}

func (h *AdminHandler) TeamsAdd(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	if err := r.ParseForm(); err != nil {
		h.handleError(w, r, "TeamsAdd parsing form", "Error adding teams", "error", err, "instance_id", user.CurrentInstanceID)
		return
	}

	countStr := r.FormValue("count")
	count, err := strconv.Atoi(countStr)
	if err != nil {
		h.handleError(w, r, "TeamsAdd parsing count", "Error adding teams", "error", err, "instance_id", user.CurrentInstanceID)
		return
	}

	// Add the teams
	teams, err := h.teamService.AddTeams(r.Context(), user.CurrentInstanceID, count)
	if err != nil {
		h.handleError(w, r, "TeamsAdd adding teams", "Error adding teams", "error", err, "instance_id", user.CurrentInstanceID)
		return
	}

	err = admin.TeamsList(teams).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("TeamsAdd rendering teams list", "error", err.Error(), "instance_id", user.CurrentInstanceID)
	}
}

func (h *AdminHandler) TeamsDelete(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "parsing form", "Error parsing form", "error", err)
		return
	}

	teamID := r.Form["team-checkbox"]
	if len(teamID) == 0 {
		h.handleError(w, r, "TeamsDelete no team_id", "Error deleting team", "error", nil, "instance_id", user.CurrentInstanceID)
		return
	}

	for _, id := range teamID {
		err := h.deleteService.DeleteTeams(r.Context(), user.CurrentInstanceID, []string{id})
		if err != nil {
			h.handleError(w, r, "TeamsDelete deleting team", "Error deleting team", "error", err, "instance_id", user.CurrentInstanceID, "team_id", teamID)
			return
		}
	}

	teams, err := h.teamService.FindAll(r.Context(), user.CurrentInstanceID)
	if err != nil {
		h.handleError(w, r, "TeamsReset finding teams", "Error finding teams", "error", err, "instance_id", user.CurrentInstanceID)
		return
	}

	err = admin.TeamsTable(teams).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("TeamsReset rendering teams list", "error", err.Error(), "instance_id", user.CurrentInstanceID)
	}
	h.handleSuccess(w, r, "Deleted team(s)")
}

func (h *AdminHandler) TeamsReset(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())
	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "parsing form", "Error parsing form", "error", err)
		return
	}

	teamIDs := r.Form["team-checkbox"]
	if len(teamIDs) == 0 {
		h.handleError(w, r, "TeamsReset no team_id", "No teams selected", "error", nil, "instance_id", user.CurrentInstanceID)
		return
	}

	err = h.deleteService.ResetTeams(r.Context(), user.CurrentInstanceID, teamIDs)
	if err != nil {
		h.handleError(w, r, "TeamsReset deleting team", "Error resetting teams", "error", err, "instance_id", user.CurrentInstanceID, "team_id", teamIDs)
		return
	}

	teams, err := h.teamService.FindAll(r.Context(), user.CurrentInstanceID)
	if err != nil {
		h.handleError(w, r, "TeamsReset finding teams", "Error finding teams", "error", err, "instance_id", user.CurrentInstanceID)
		return
	}

	err = admin.TeamsTable(teams).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("TeamsReset rendering teams list", "error", err.Error(), "instance_id", user.CurrentInstanceID)
	}
	h.handleSuccess(w, r, "Reset team(s)")
}
