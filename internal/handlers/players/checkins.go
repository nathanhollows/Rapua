package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/internal/flash"
	"github.com/nathanhollows/Rapua/internal/handlers"
	"github.com/nathanhollows/Rapua/internal/models"
	"github.com/nathanhollows/Rapua/internal/sessions"
	templates "github.com/nathanhollows/Rapua/internal/templates/players"
)

// CheckIn handles the GET request for scanning a location
func (h *PlayerHandler) CheckIn(w http.ResponseWriter, r *http.Request) {
	data := handlers.TemplateData(r)
	code := chi.URLParam(r, "code")
	code = strings.ToUpper(code)
	data["code"] = code

	team, err := h.getTeamFromContext(r.Context())
	if err == nil {
		if team.MustScanOut != "" {
			err := team.LoadBlockingLocation(r.Context())
			if err != nil {
				h.Logger.Error("CheckIn: loading blocking location", "err", err)
				flash.NewError("Something went wrong. Please try again.").Save(w, r)
				data["blocked"] = true
				http.Redirect(w, r, r.Header.Get("/next"), http.StatusFound)
				return
			}

			if team.BlockingLocation.MarkerID == code {
				flash.NewDefault("Would you like to scan out instead?").Save(w, r)
				http.Redirect(w, r, "/o/"+code, http.StatusFound)
				return
			}

			flash.NewWarning(fmt.Sprintf("You need to scan out at %s.", team.BlockingLocation.Name)).
				SetTitle("You are already scanned in.").
				Save(w, r)
			data["blocked"] = true
		}
	}

	response := h.GameplayService.GetMarkerByCode(r.Context(), code)
	if response.Error != nil {
		flash.NewWarning("Please double check the code and try again.").
			SetTitle("Location not found").Save(w, r)
		http.Redirect(w, r, "/play", http.StatusFound)
		return
	}
	data["marker"] = response.Data["marker"].(*models.Marker)

	data["team"] = team
	data["notifications"], _ = h.NotificationService.GetNotifications(r.Context(), team.Code)
	data["messages"] = flash.Get(w, r)
	handlers.Render(w, data, handlers.PlayerDir, "scan")
}

// CheckInPost handles the POST request for scanning in
func (h *PlayerHandler) CheckInPost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	locationCode := chi.URLParam(r, "code")
	locationCode = strings.ToUpper(locationCode)

	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		team, err = h.GameplayService.GetTeamByCode(r.Context(), r.FormValue("team"))
		if err != nil {
			flash.NewWarning("Please double check the team code and try again.").
				Save(w, r)
			h.Logger.Error(`CheckInPost: getting team by code (post)`, "err", err)
			http.Redirect(w, r, r.Header.Get("referer"), http.StatusFound)
			return
		}
	}

	response := h.GameplayService.CheckIn(r.Context(), team, locationCode)
	for _, msg := range response.FlashMessages {
		msg.Save(w, r)
	}
	if response.Error != nil {
		h.Logger.Error("checking in team", "err", response.Error.Error(), "team", team.Code, "location", locationCode)
		http.Redirect(w, r, r.Header.Get("referer"), http.StatusFound)
		return
	}

	location := response.Data["location"].(*models.Location)

	http.Redirect(w, r, "/checkins/"+location.MarkerID, http.StatusFound)
}

func (h *PlayerHandler) CheckOut(w http.ResponseWriter, r *http.Request) {
	data := handlers.TemplateData(r)
	code := chi.URLParam(r, "code")
	code = strings.ToUpper(code)

	teamCode := ""
	session, _ := sessions.Get(r, "scanscout")
	tcode := session.Values["team"]
	if tcode != nil {
		teamCode = strings.ToUpper(tcode.(string))
	}

	team, err := h.GameplayService.GetTeamByCode(r.Context(), teamCode)
	if err != nil {
		flash.NewWarning("Please double check the code and try again.").
			SetTitle("Team code not found").Save(w, r)
		http.Redirect(w, r, "/play", http.StatusFound)
		return
	}
	team.LoadBlockingLocation(r.Context())

	data["team"] = team

	if team.MustScanOut == "" {
		flash.NewDefault("You don't need to scan out.").
			SetTitle("You're all set!").Save(w, r)
		data["blocked"] = true
	} else if team.BlockingLocation.MarkerID != code {
		flash.NewWarning(fmt.Sprintf("You need to scan out at %s.", team.BlockingLocation.Name)).
			SetTitle("You are scanned in elsewhere.").Save(w, r)
		data["blocked"] = true
	}

	data["messages"] = flash.Get(w, r)
	handlers.Render(w, data, handlers.PlayerDir, "scanout")
}

func (h *PlayerHandler) CheckOutPost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	locationCode := chi.URLParam(r, "code")
	locationCode = strings.ToUpper(locationCode)

	teamCode := r.FormValue("team")
	teamCode = strings.ToUpper(teamCode)

	team, err := h.GameplayService.GetTeamByCode(r.Context(), teamCode)
	if err != nil {
		flash.NewWarning("Please double check the team code and try again.").
			SetTitle("Team code not found").Save(w, r)
		http.Redirect(w, r, "/checkouts/"+locationCode, http.StatusFound)
		return
	}

	response := h.GameplayService.CheckOut(r.Context(), team, locationCode)
	for _, msg := range response.FlashMessages {
		msg.Save(w, r)
	}
	if response.Error != nil {
		h.Logger.Error("checking out team", "err", response.Error.Error(), "team", team.Code, "location", locationCode)
		http.Redirect(w, r, r.Header.Get("referer"), http.StatusFound)
		return
	}

	flash.NewSuccess("You have checked out.").Save(w, r)
	http.Redirect(w, r, "/next", http.StatusFound)
}

// MyCheckins shows the found locations page
func (h *PlayerHandler) MyCheckins(w http.ResponseWriter, r *http.Request) {
	team, err := h.getTeamFromContext(r.Context())
	if err != nil || team == nil {
		http.Redirect(w, r, "/play", http.StatusFound)
		return
	}

	err = team.LoadScans(r.Context())
	if err != nil {
		flash.NewError("Error loading check ins.").Save(w, r)
		h.Logger.Error("loading check ins", "error", err.Error())
		http.Redirect(w, r, r.Header.Get("referer"), http.StatusFound)
		return
	}

	err = team.LoadBlockingLocation(r.Context())
	if err != nil {
		// We don't want to stop the user from seeing their check-ins if the blocking location can't be loaded
		h.Logger.Error("loading blocking location", "error", err.Error())
	}

	if len(team.Scans) == 0 {
		flash.Message{
			Style:   flash.Default,
			Message: "You haven't checked in anywhere yet.",
		}.Save(w, r)
		http.Redirect(w, r, "/next", http.StatusFound)
		return
	}

	// TODO: Handle notifications
	// notifications, _ := h.NotificationService.GetNotifications(r.Context(), team.Code)

	c := templates.Checkins(*team)
	err = templates.Layout(c, "My Check-ins").Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering checkins", "error", err.Error())
	}
}

// CheckInView shows the page for a specific location
func (h *PlayerHandler) CheckInView(w http.ResponseWriter, r *http.Request) {
	locationCode := chi.URLParam(r, "id")

	team, err := h.getTeamFromContext(r.Context())
	if err != nil {
		flash.NewError("Error loading team.").Save(w, r)
		http.Redirect(w, r, "/play", http.StatusFound)
		return
	}

	err = team.LoadBlockingLocation(r.Context())
	if err != nil {
		flash.NewError("Error loading blocking location.").Save(w, r)
		h.Logger.Error("loading blocking location", "error", err.Error())
		http.Redirect(w, r, r.Header.Get("referer"), http.StatusFound)
		return
	}

	if team.MustScanOut != "" {
		if team.BlockingLocation.MarkerID != locationCode {
			flash.NewDefault("You are currently checked into "+team.BlockingLocation.Name).Save(w, r)
		}
	}

	err = team.LoadScans(r.Context())
	if err != nil {
		flash.NewError("Error loading locations.").Save(w, r)
		http.Redirect(w, r, r.Header.Get("referer"), http.StatusFound)
		return
	}

	// Get the index of the location in the team's scans
	index := -1
	for i, scan := range team.Scans {
		if scan.Location.MarkerID == locationCode {
			index = i
			break
		}
	}

	if index == -1 {
		flash.NewWarning("Please double check the code and try again.").Save(w, r)
		http.Redirect(w, r, r.Header.Get("referer"), http.StatusFound)
		return
	}

	c := templates.CheckInView(team.Scans[index])
	err = templates.Layout(c, team.Scans[index].Location.Name).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering checkin view", "error", err.Error())
	}

}
