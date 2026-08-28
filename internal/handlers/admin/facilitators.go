package admin

import (
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/v8/internal/config"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	public "github.com/nathanhollows/Rapua/v8/internal/templates/public"
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

	objectives, err := h.objectiveService.FindByQuestID(r.Context(), facToken.QuestID)
	if err != nil {
		h.handleError(w, r, "fetching objectives", "Error fetching objectives", "error", err)
		return
	}

	filteredObjectives := h.filterObjectivesForToken(objectives, facToken)

	teams, err := h.runService.FindAll(r.Context(), facToken.QuestID)
	if err != nil {
		h.handleError(w, r, "fetching teams", "Error fetching teams", "error", err)
		return
	}
	startedTeams := 0
	for _, team := range teams {
		if team.HasStarted {
			startedTeams++
		}
	}

	authed := contextkeys.GetUserStatus(r.Context()).IsAdminLoggedIn
	c := templates.FacilitatorDashboard(filteredObjectives, startedTeams)
	err = public.AuthLayout(c, "Facilitator Dashboard", authed).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "Activity: rendering template", "error", err)
	}
}

func (h *Handler) filterObjectivesForToken(
	objectives []models.Objective,
	token *models.FacilitatorToken,
) []models.Objective {
	if len(token.Objectives) == 0 {
		return objectives
	}

	var filtered []models.Objective
	for _, allowedID := range token.Objectives {
		for _, obj := range objectives {
			if obj.ID == allowedID {
				filtered = append(filtered, obj)
				break
			}
		}
	}
	return filtered
}
