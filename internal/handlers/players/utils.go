package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/nathanhollows/Rapua/v3/blocks"
	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v3/internal/flash"
	"github.com/nathanhollows/Rapua/v3/internal/services"
	"github.com/nathanhollows/Rapua/v3/internal/sessions"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/players"
	"github.com/nathanhollows/Rapua/v3/models"
)

type BlockService interface {
	// NewMockBlockState creates a mock player state (for testing/demo scenarios)
	NewMockBlockState(ctx context.Context, blockID, teamCode string) (blocks.PlayerState, error)
	// FindByLocationID fetches all content blocks for a location
	FindByLocationID(ctx context.Context, locationID string) (blocks.Blocks, error)
	// FindByLocationIDAndTeamCodeWithState fetches all blocks and their states
	// for the given location and team
	FindByLocationIDAndTeamCodeWithState(ctx context.Context, locationID, teamCode string) ([]blocks.Block, map[string]blocks.PlayerState, error)
}

type CheckInService interface {
	CheckIn(ctx context.Context, team *models.Team, locationCode string) error
	CheckOut(ctx context.Context, team *models.Team, locationCode string) error
}

type InstanceSettingsService interface {
	GetInstanceSettings(ctx context.Context, instanceID string) (*models.InstanceSettings, error)
}

type NavigationService interface {
	// IsValidLocation(ctx context.Context, team *models.Team, markerID string) (bool, error)
	GetNextLocations(ctx context.Context, team *models.Team) ([]models.Location, error)
	// HasVisited(checkins []models.CheckIn, locationID string) bool
}

type PlayerHandler struct {
	Logger                  *slog.Logger
	blockService            BlockService
	checkInService          CheckInService
	GameplayService         services.GameplayService
	InstanceService         services.InstanceService
	InstanceSettingsService InstanceSettingsService
	NavigationService       NavigationService
	NotificationService     services.NotificationService
	TeamService             services.TeamService
}

func NewPlayerHandler(
	logger *slog.Logger,
	blockService BlockService,
	checkInService CheckInService,
	gameplayService services.GameplayService,
	instanceService services.InstanceService,
	instanceSettingsService InstanceSettingsService,
	navigationService NavigationService,
	notificationService services.NotificationService,
	teamService services.TeamService,
) *PlayerHandler {
	return &PlayerHandler{
		Logger:                  logger,
		blockService:            blockService,
		checkInService:          checkInService,
		GameplayService:         gameplayService,
		InstanceService:         instanceService,
		InstanceSettingsService: instanceSettingsService,
		NavigationService:       navigationService,
		NotificationService:     notificationService,
		TeamService:             teamService,
	}
}

// GetTeamFromContext retrieves the team from the context.
// Team will always be in the context because the middleware.
// However the Team could be nil if the team was not found.
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

// redirect is a helper function to redirect the user to a new page.
// It accounts for htmx requests.
func (h PlayerHandler) redirect(w http.ResponseWriter, r *http.Request, path string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", path)
		return
	}
	http.Redirect(w, r, path, http.StatusFound)
}

func (h *PlayerHandler) startSession(w http.ResponseWriter, r *http.Request, teamCode string) error {
	session, err := sessions.Get(r, "scanscout")
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}
	session.Values["team"] = teamCode
	session.Options.Path = "/"
	err = session.Save(r, w)
	if err != nil {
		return fmt.Errorf("saving session: %w", err)
	}

	return nil
}

// invalidateSession invalidates the current session.
func invalidateSession(r *http.Request, w http.ResponseWriter) error {
	session, err := sessions.Get(r, "scanscout")
	if err != nil {
		return fmt.Errorf("getting session: %w", err)
	}
	session.Options.MaxAge = -1
	err = session.Save(r, w)
	if err != nil {
		return fmt.Errorf("saving session: %w", err)
	}
	return nil
}

func (h *PlayerHandler) handleError(w http.ResponseWriter, r *http.Request, logMsg string, flashMsg string, params ...interface{}) {
	h.Logger.Error(logMsg, params...)
	err := templates.Toast(*flash.NewError(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error(logMsg+" - rendering template", "error", err)
	}
}
