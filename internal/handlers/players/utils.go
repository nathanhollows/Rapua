package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/nathanhollows/Rapua/internal/contextkeys"
	"github.com/nathanhollows/Rapua/internal/flash"
	"github.com/nathanhollows/Rapua/internal/services"
	templates "github.com/nathanhollows/Rapua/internal/templates/players"
	"github.com/nathanhollows/Rapua/models"
)

type PlayerHandler struct {
	Logger              *slog.Logger
	BlockService        services.BlockService
	GameplayService     services.GameplayService
	NotificationService services.NotificationService
	TeamService         services.TeamService
}

func NewPlayerHandler(
	logger *slog.Logger,
	blockService services.BlockService,
	gameplayService services.GameplayService,
	notificationService services.NotificationService,
	teamService services.TeamService,
) *PlayerHandler {
	return &PlayerHandler{
		Logger:              logger,
		BlockService:        blockService,
		GameplayService:     gameplayService,
		NotificationService: notificationService,
		TeamService:         teamService,
	}
}

// getTeamIfExists retrieves a team by its code if present.
func (h *PlayerHandler) getTeamIfExists(ctx context.Context, teamCode interface{}) (*models.Team, error) {
	if teamCode == nil {
		return nil, nil
	}
	return h.GameplayService.GetTeamByCode(ctx, teamCode.(string))
}

// GetTeamFromContext retrieves the team from the context
// Team will always be in the context because the middleware
// However the Team could be nil if the team was not found
func (h PlayerHandler) getTeamFromContext(ctx context.Context) (*models.Team, error) {
	val := ctx.Value(contextkeys.TeamKey)
	if val == nil {
		return nil, errors.New("team not found")
	}
	team := val.(*models.Team)
	if team == nil {
		return nil, errors.New("team not found")
	}
	return team, nil
}

// redirect is a helper function to redirect the user to a new page
// It accounts for htmx requests
func (h PlayerHandler) redirect(w http.ResponseWriter, r *http.Request, path string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", path)
		return
	}
	http.Redirect(w, r, path, http.StatusFound)
}

// invalidateSession invalidates the current session.
func invalidateSession(session *sessions.Session, r *http.Request, w http.ResponseWriter) {
	session.Options.MaxAge = -1
	session.Save(r, w)
}

func (h *PlayerHandler) handleError(w http.ResponseWriter, r *http.Request, logMsg string, flashMsg string, params ...interface{}) {
	h.Logger.Error(logMsg, params...)
	err := templates.Toast(*flash.NewError(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error(logMsg+" - rendering template", "error", err)
	}
}

func (h *PlayerHandler) handleSuccess(w http.ResponseWriter, r *http.Request, flashMsg string) {
	err := templates.Toast(*flash.NewSuccess(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering success template", "error", err)
	}
}
