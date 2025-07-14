package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v3/internal/flash"
	"github.com/nathanhollows/Rapua/v3/internal/services"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v3/models"
)

type DeleteService interface {
	DeleteBlock(ctx context.Context, blockID string) error
	DeleteInstance(ctx context.Context, userID, instanceID string) error
	DeleteLocation(ctx context.Context, locationID string) error
	ResetTeams(ctx context.Context, instanceID string, teamCodes []string) error
	DeleteTeams(ctx context.Context, instanceID string, teamIDs []string) error
	DeleteUser(ctx context.Context, userID string) error
}

type GameScheduleService interface {
	Start(ctx context.Context, instance *models.Instance) error
	Stop(ctx context.Context, instance *models.Instance) error
	SetStartTime(ctx context.Context, instance *models.Instance, start time.Time) error
	SetEndTime(ctx context.Context, instance *models.Instance, end time.Time) error
	ScheduleGame(ctx context.Context, instance *models.Instance, start, end time.Time) error
}

type InstanceSettingsService interface {
	SaveSettings(ctx context.Context, settings *models.InstanceSettings) error
	GetInstanceSettings(ctx context.Context, instanceID string) (*models.InstanceSettings, error)
}

type NavigationService interface {
	GetNextLocations(ctx context.Context, team *models.Team) ([]models.Location, error)
}

type QuickstartService interface {
	DismissQuickstart(ctx context.Context, instanceID string) error
}

type MarkerService interface {
	// CreateMarker creates a new marker
	CreateMarker(ctx context.Context, name string, lat, lng float64) (models.Marker, error)
	// DuplicateLocation creates a new location given an existing location and the instance ID of the new location
	// FindMarkersNotInInstance finds all markers that are not in the given instance
	FindMarkersNotInInstance(ctx context.Context, instanceID string, otherInstances []string) ([]models.Marker, error)
}

type AdminHandler struct {
	Logger                  *slog.Logger
	AssetGenerator          services.AssetGenerator
	AuthService             services.AuthService
	BlockService            services.BlockService
	ClueService             services.ClueService
	DeleteService           DeleteService
	FacilitatorService      services.FacilitatorService
	GameManagerService      services.GameManagerService
	GameplayService         services.GameplayService
	GameScheduleService     GameScheduleService
	InstanceService         services.InstanceService
	instanceSettingsService InstanceSettingsService
	LocationService         services.LocationService
	MarkerService           MarkerService
	NavigationService       NavigationService
	NotificationService     services.NotificationService
	TeamService             services.TeamService
	TemplateService         services.TemplateService
	UploadService           services.UploadService
	UserService             services.UserService
	QuickstartService       QuickstartService
}

func NewAdminHandler(
	logger *slog.Logger,
	assetGenerator services.AssetGenerator,
	authService services.AuthService,
	blockService services.BlockService,
	clueService services.ClueService,
	DeleteService DeleteService,
	facilitatorService services.FacilitatorService,
	gameManagerService services.GameManagerService,
	gameplayService services.GameplayService,
	gameScheduleService GameScheduleService,
	instanceService services.InstanceService,
	instanceSettingsService InstanceSettingsService,
	locationService services.LocationService,
	markerService MarkerService,
	navigationService NavigationService,
	notificationService services.NotificationService,
	teamService services.TeamService,
	templateService services.TemplateService,
	uploadService services.UploadService,
	userService services.UserService,
	quickstartService QuickstartService,
) *AdminHandler {
	return &AdminHandler{
		Logger:                  logger,
		AssetGenerator:          assetGenerator,
		AuthService:             authService,
		BlockService:            blockService,
		ClueService:             clueService,
		DeleteService:           DeleteService,
		FacilitatorService:      facilitatorService,
		GameManagerService:      gameManagerService,
		GameplayService:         gameplayService,
		GameScheduleService:     gameScheduleService,
		InstanceService:         instanceService,
		instanceSettingsService: instanceSettingsService,
		LocationService:         locationService,
		MarkerService:           markerService,
		NavigationService:       navigationService,
		NotificationService:     notificationService,
		TeamService:             teamService,
		TemplateService:         templateService,
		UploadService:           uploadService,
		UserService:             userService,
		QuickstartService:       quickstartService,
	}
}

// GetUserFromContext retrieves the user from the context.
// User will always be in the context because the middleware.
func (h AdminHandler) UserFromContext(ctx context.Context) *models.User {
	return ctx.Value(contextkeys.UserKey).(*models.User)
}

func (h *AdminHandler) handleError(w http.ResponseWriter, r *http.Request, logMsg string, flashMsg string, params ...interface{}) {
	h.Logger.Error(logMsg, params...)
	err := templates.Toast(*flash.NewError(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error(logMsg+" - rendering template", "error", err)
	}
}

func (h *AdminHandler) handleSuccess(w http.ResponseWriter, r *http.Request, flashMsg string) {
	err := templates.Toast(*flash.NewSuccess(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering success template", "error", err)
	}
}

// redirect is a helper function to redirect the user to a new page.
// It accounts for htmx requests and redirects the user to the referer.
func (h AdminHandler) redirect(w http.ResponseWriter, r *http.Request, path string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", path)
		return
	}
	http.Redirect(w, r, path, http.StatusFound)
}
