package admin

import (
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v4/helpers"
	"github.com/nathanhollows/Rapua/v4/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/admin"
	public "github.com/nathanhollows/Rapua/v4/internal/templates/public"
	"github.com/nathanhollows/Rapua/v4/models"
)

// FacilitatorShowModal renders the modal for creating a facilitator token.
func (h *AdminHandler) FacilitatorShowModal(w http.ResponseWriter, r *http.Request) {
	err := templates.FacilitatorLinkModal().Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "rendering template", "Error rendering template", "error", err)
	}
}

// FacilitatorCreateTokenLink creates a new one-click login link for a facilitators.
func (h *AdminHandler) FacilitatorCreateTokenLink(w http.ResponseWriter, r *http.Request) {
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
		duration = 24 * time.Hour
	case "week":
		duration = 7 * 24 * time.Hour
	case "month":
		duration = 30 * 24 * time.Hour
	default:
		duration = 24 * time.Hour
	}

	var locations []string
	if r.Form.Get("locations") != "" {
		locations = append(locations, r.Form.Get("locations"))
	}

	token, err := h.facilitatorService.CreateFacilitatorToken(r.Context(), user.CurrentInstanceID, locations, duration)
	if err != nil {
		h.handleError(w, r, "creating facilitator token", "Error creating facilitator token")
		return
	}

	url := helpers.URL("/facilitator/login/" + token)

	err = templates.FacilitatorLinkCopyModal(url).Render(r.Context(), w)
	if err != nil {
		h.handleError(w, r, "rendering template", "Error rendering template", "error", err)
	}
}

const facilitatorSessionCookie = "rapua_facilitator"

// FacilitatorLogin accepts a token and creates a session cookie.
func (h *AdminHandler) FacilitatorLogin(w http.ResponseWriter, r *http.Request) {
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
func (h *AdminHandler) FacilitatorDashboard(w http.ResponseWriter, r *http.Request) {
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

	// Fetch locations
	locations, err := h.locationService.FindByInstance(r.Context(), facToken.InstanceID)
	if err != nil {
		h.handleError(w, r, "fetching locations", "Error fetching locations", "error", err)
		return
	}

	var filteredLocations []models.Location
	if len(facToken.Locations) == 0 {
		filteredLocations = locations
	} else {
		for _, loc := range facToken.Locations {
			for _, l := range locations {
				if l.ID == loc {
					filteredLocations = append(filteredLocations, l)
					break
				}
			}
		}
	}

	// Team activity overview
	overview, err := h.teamService.GetTeamActivityOverview(r.Context(), facToken.InstanceID, filteredLocations)
	if err != nil {
		h.handleError(w, r, "fetching team activity overview", "Error fetching team activity overview", "error", err)
		return
	}

	// Create a single simple struct that passes []:
	// - Number of visited teams
	// - Number of teams currently visiting
	// - Average time spent (where applicable)
	// Is addressable by location name
	type LocationOverview struct {
		VisitedTeams     int
		VisitingTeams    int
		AverageTimeSpent float64
	}

	locs := make(map[string]LocationOverview)
	for _, loc := range filteredLocations {
		visited := 0
		visiting := 0
		averageTime := 0.0
		locs[loc.Name] = LocationOverview{VisitedTeams: visited, VisitingTeams: visiting, AverageTimeSpent: averageTime}
	}
	for _, team := range overview {
		for _, loc := range team.Locations {
			if loc.Visited {
				locs[loc.Location.Name] = LocationOverview{
					VisitedTeams:     locs[loc.Location.Name].VisitedTeams + 1,
					VisitingTeams:    locs[loc.Location.Name].VisitingTeams,
					AverageTimeSpent: locs[loc.Location.Name].AverageTimeSpent + loc.Duration,
				}
			}
			if loc.Visiting {
				locs[loc.Location.Name] = LocationOverview{
					VisitedTeams:     locs[loc.Location.Name].VisitedTeams,
					VisitingTeams:    locs[loc.Location.Name].VisitingTeams + 1,
					AverageTimeSpent: locs[loc.Location.Name].AverageTimeSpent,
				}
			}
		}
	}

	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	c := templates.FacilitatorDashboard(locations, overview)
	err = public.AuthLayout(c, "Facilitator Dashboard", authed).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("Activity: rendering template", "error", err)
	}
}
