package admin

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/internal/config"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
)

const (
	hoursPerDay  = 24
	daysPerWeek  = 7
	daysPerMonth = 30
)

// FacilitatorShowModal renders the modal for creating a facilitator token.
func (h *Handler) FacilitatorShowModal(w http.ResponseWriter, r *http.Request) {
	err := templates.FacilitatorLinkModal().Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "rendering template", "Error rendering template", "error", err)
	}
}

// FacilitatorCreateTokenLink creates a new one-click login link for a facilitators.
func (h *Handler) FacilitatorCreateTokenLink(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	err := r.ParseForm()
	if err != nil {
		h.handleError(w, r, "parsing form", "Error parsing form", "error", err)
		return
	}

	var duration time.Duration
	switch r.Form.Get("duration") {
	case "hour":
		duration = time.Hour
	case "day":
		duration = hoursPerDay * time.Hour
	case "week":
		duration = daysPerWeek * hoursPerDay * time.Hour
	case "month":
		duration = daysPerMonth * hoursPerDay * time.Hour
	default:
		duration = hoursPerDay * time.Hour
	}

	var objectives []string
	if r.Form.Get("objectives") != "" {
		objectives = append(objectives, r.Form.Get("objectives"))
	}

	token, err := h.facilitatorService.CreateFacilitatorToken(r.Context(), user.CurrentQuestID, objectives, duration)
	if err != nil {
		h.handleError(w, r, "creating facilitator token", "Error creating facilitator token")
		return
	}

	url := config.SiteURL("/facilitator/login/" + token)

	err = templates.FacilitatorLinkCopyModal(url).Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "rendering template", "Error rendering template", "error", err)
	}
}

const facilitatorSessionCookie = "rapua_facilitator"

// FacilitatorLogin accepts a token and creates a session cookie.
func (h *Handler) FacilitatorLogin(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}

	// Validate token
	facToken, err := h.facilitatorService.ValidateToken(r.Context(), token)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	// Set a session cookie with the token
	http.SetCookie(w, &http.Cookie{
		Name:     facilitatorSessionCookie,
		Value:    token,
		Expires:  facToken.ExpiresAt,
		HttpOnly: true, // Prevent JavaScript access
		Secure:   true, // Only send over HTTPS
		Path:     "/facilitator",
	})

	// Redirect to the facilitator dashboard
	http.Redirect(w, r, "/facilitator/dashboard", http.StatusSeeOther)
}

// FacilitatorDashboard renders the facilitator dashboard.
func (h *Handler) FacilitatorDashboard(w http.ResponseWriter, r *http.Request) {
	token, err := r.Cookie(facilitatorSessionCookie)
	if err != nil {
		h.handleError(
			w,
			r,
			"facilitator session expired",
			"Your session has expired. Please ask for another login link.",
		)
		h.redirect(w, r, "/")
		return
	}

	facToken, err := h.facilitatorService.ValidateToken(r.Context(), token.Value)
	if err != nil {
		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}

	quest, err := h.questService.GetByID(r.Context(), facToken.QuestID)
	if err != nil {
		h.handleError(w, r, "fetching quest", "Error fetching the game", "error", err)
		return
	}

	now := time.Now().UTC()
	report, err := h.runService.CachedStations(r.Context(), facToken.QuestID)
	if err != nil {
		h.handleError(w, r, "reading stations", "Error reading the game", "error", err)
		return
	}
	report.Stations = stationsForToken(report.Stations, facToken)

	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	c := templates.FacilitatorDashboard(templates.FacilitatorDashboardData{
		QuestName: quest.Name,
		Now:       now,
		Clock:     services.BuildGameClock(*quest, now),
		Report:    report,
		Watching:  watchedStations(r, report.Stations, facToken),
	})
	err = templates.FacilitatorLayout(c, "Facilitator Dashboard", authed).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Activity: rendering template", "error", err)
	}
}

// facilitatorWatchCookie holds the stations a facilitator chose to follow. The
// page sets it and the server reads it, so what is rendered is already the
// choice and nothing has to be hidden afterwards.
const facilitatorWatchCookie = "rapua_watch"

// stationsForToken keeps the stations a link covers, in the quest's order. A
// link made with no objectives covers all of them.
func stationsForToken(stations []services.StationReading, token *models.FacilitatorToken) []services.StationReading {
	if len(token.Objectives) == 0 {
		return stations
	}
	allowed := make(map[string]bool, len(token.Objectives))
	for _, id := range token.Objectives {
		allowed[id] = true
	}
	var kept []services.StationReading
	for _, station := range stations {
		if allowed[station.Objective.ID] {
			kept = append(kept, station)
		}
	}
	return kept
}

// watchedStations is the stations this facilitator follows, by objective ID. The
// cookie holds short keys, and only a station the link covers can be named by
// one, whatever the cookie says. With no choice made yet, a link made for
// particular stations follows all of them, and a link for the whole game follows
// none until asked.
func watchedStations(
	r *http.Request, stations []services.StationReading, token *models.FacilitatorToken,
) map[string]bool {
	idForKey := make(map[string]string, len(stations))
	for _, station := range stations {
		idForKey[services.StationKey(station.Objective.ID)] = station.Objective.ID
	}

	watching := make(map[string]bool)
	cookie, err := r.Cookie(facilitatorWatchCookie)
	if err != nil {
		if len(token.Objectives) > 0 {
			for _, id := range idForKey {
				watching[id] = true
			}
		}
		return watching
	}
	// The browser writes the commas percent-encoded and Go hands back the raw value.
	value, err := url.QueryUnescape(cookie.Value)
	if err != nil {
		value = cookie.Value
	}
	for _, key := range strings.Split(value, ",") {
		if id, ok := idForKey[key]; ok {
			watching[id] = true
		}
	}
	return watching
}
