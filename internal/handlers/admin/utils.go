package admin

import (
	"context"
	"log/slog"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/nathanhollows/Rapua/v4/blocks"
	"github.com/nathanhollows/Rapua/v4/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v4/internal/flash"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v4/models"
)

type AccessService interface {
	CanAdminAccessBlock(ctx context.Context, userID, blockID string) (bool, error)
	CanAdminAccessInstance(ctx context.Context, userID, instanceID string) (bool, error)
	CanAdminAccessLocation(ctx context.Context, userID, locationID string) (bool, error)
	CanAdminAccessMarker(ctx context.Context, userID, markerID string) (bool, error)
}

type BlockService interface {
	// NewBlock creates a new content block of the specified type for the given location
	NewBlock(ctx context.Context, locationID string, blockType string) (blocks.Block, error)
	// NewBlockState creates a new player state for the given block and team
	NewBlockState(ctx context.Context, blockID, teamCode string) (blocks.PlayerState, error)
	// NewMockBlockState creates a mock player state (for testing/demo scenarios)
	NewMockBlockState(ctx context.Context, blockID, teamCode string) (blocks.PlayerState, error)

	// GetByBlockID fetches a content block by its ID
	GetByBlockID(ctx context.Context, blockID string) (blocks.Block, error)
	// GetBlockWithStateByBlockIDAndTeamCode fetches a block + its state
	// for the given block ID and team
	GetBlockWithStateByBlockIDAndTeamCode(ctx context.Context, blockID, teamCode string) (blocks.Block, blocks.PlayerState, error)
	// FindByLocationID fetches all content blocks for a location
	FindByLocationID(ctx context.Context, locationID string) (blocks.Blocks, error)
	// FindByLocationIDAndTeamCodeWithState fetches all blocks and their states
	// for the given location and team
	FindByLocationIDAndTeamCodeWithState(ctx context.Context, locationID, teamCode string) ([]blocks.Block, map[string]blocks.PlayerState, error)

	// UpdateBlock updates the data for the given block
	UpdateBlock(ctx context.Context, block blocks.Block, data map[string][]string) (blocks.Block, error)
	// UpdateState updates the player state for a block
	UpdateState(ctx context.Context, state blocks.PlayerState) (blocks.PlayerState, error)
	// ReorderBlocks changes the display/order of blocks at a location
	ReorderBlocks(ctx context.Context, blockIDs []string) error

	// CheckValidationRequiredForLocation checks if any blocks in a location require validation
	CheckValidationRequiredForLocation(ctx context.Context, locationID string) (bool, error)
	// CheckValidationRequiredForCheckIn checks if any blocks still require validation for a check-in
	CheckValidationRequiredForCheckIn(ctx context.Context, locationID, teamCode string) (bool, error)
}

type ClueService interface {
	UpdateClues(ctx context.Context, location *models.Location, clues []string, clueIDs []string) error
}

type DeleteService interface {
	DeleteBlock(ctx context.Context, blockID string) error
	DeleteInstance(ctx context.Context, userID, instanceID string) error
	DeleteLocation(ctx context.Context, locationID string) error
	ResetTeams(ctx context.Context, instanceID string, teamCodes []string) error
	DeleteTeams(ctx context.Context, instanceID string, teamIDs []string) error
	DeleteUser(ctx context.Context, userID string) error
}

type FacilitatorService interface {
	CreateFacilitatorToken(ctx context.Context, instanceID string, locations []string, duration time.Duration) (string, error)
	ValidateToken(ctx context.Context, token string) (*models.FacilitatorToken, error)
	CleanupExpiredTokens(ctx context.Context) error
}

type GameScheduleService interface {
	Start(ctx context.Context, instance *models.Instance) error
	Stop(ctx context.Context, instance *models.Instance) error
	SetStartTime(ctx context.Context, instance *models.Instance, start time.Time) error
	SetEndTime(ctx context.Context, instance *models.Instance, end time.Time) error
	ScheduleGame(ctx context.Context, instance *models.Instance, start, end time.Time) error
}

type IdentityService interface {
	GetAuthenticatedUser(r *http.Request) (*models.User, error)
}

type InstanceSettingsService interface {
	SaveSettings(ctx context.Context, settings *models.InstanceSettings) error
	GetInstanceSettings(ctx context.Context, instanceID string) (*models.InstanceSettings, error)
}

type MarkerService interface {
	// CreateMarker creates a new marker
	CreateMarker(ctx context.Context, name string, lat, lng float64) (models.Marker, error)
	// DuplicateLocation creates a new location given an existing location and the instance ID of the new location
	// FindMarkersNotInInstance finds all markers that are not in the given instance
	FindMarkersNotInInstance(ctx context.Context, instanceID string, otherInstances []string) ([]models.Marker, error)
}

type NavigationService interface {
	GetNextLocations(ctx context.Context, team *models.Team) ([]models.Location, error)
}

type NotificationService interface {
	SendNotification(ctx context.Context, teamCode string, content string) (models.Notification, error)
	SendNotificationToAllTeams(ctx context.Context, instanceID string, content string) error
	GetNotifications(ctx context.Context, teamCode string) ([]models.Notification, error)
}

type QuickstartService interface {
	DismissQuickstart(ctx context.Context, instanceID string) error
}

type TeamService interface {
	// AddTeams adds teams to the database
	AddTeams(ctx context.Context, instanceID string, count int) ([]models.Team, error)

	// FindAll returns all teams for an instance
	FindAll(ctx context.Context, instanceID string) ([]models.Team, error)
	// GetTeamByCode returns a team by code
	GetTeamByCode(ctx context.Context, code string) (*models.Team, error)
	// GetTeamActivityOverview returns a list of teams and their activity
	GetTeamActivityOverview(ctx context.Context, instanceID string, locations []models.Location) ([]services.TeamActivity, error)

	// LoadRelation loads relations for a team
	LoadRelation(ctx context.Context, team *models.Team, relation string) error
	// LoadRelations loads all relations for a team
	LoadRelations(ctx context.Context, team *models.Team) error
}

type UploadService interface {
	UploadFile(ctx context.Context, file multipart.File, fileHeader *multipart.FileHeader, data services.UploadMetadata) (*models.Upload, error)
	Search(ctx context.Context, filters map[string]string) ([]*models.Upload, error)
}

type UserService interface {
	// CreateUser creates a new user
	CreateUser(ctx context.Context, user *models.User, passwordConfirm string) error
	// UpdateUser updates a user
	UpdateUser(ctx context.Context, user *models.User) error
	// UpdateUserProfile updates a user's profile with form data
	UpdateUserProfile(ctx context.Context, user *models.User, profile map[string]string) error
	// ChangePassword changes a user's password
	ChangePassword(ctx context.Context, user *models.User, oldPassword, newPassword, confirmPassword string) error
	// SwitchInstance switches the user's current instance
	SwitchInstance(ctx context.Context, user *models.User, instanceID string) error
}

type AdminHandler struct {
	logger                  *slog.Logger
	accessService           AccessService
	assetGenerator          services.AssetGenerator
	identityService         IdentityService
	blockService            BlockService
	clueService             ClueService
	deleteService           DeleteService
	facilitatorService      FacilitatorService
	gameScheduleService     GameScheduleService
	instanceService         services.InstanceService
	instanceSettingsService InstanceSettingsService
	locationService         services.LocationService
	markerService           MarkerService
	navigationService       NavigationService
	notificationService     NotificationService
	teamService             TeamService
	templateService         services.TemplateService
	uploadService           UploadService
	userService             UserService
	quickstartService       QuickstartService
}

func NewAdminHandler(
	logger *slog.Logger,
	accessService AccessService,
	assetGenerator services.AssetGenerator,
	identityService IdentityService,
	blockService BlockService,
	clueService ClueService,
	DeleteService DeleteService,
	facilitatorService FacilitatorService,
	gameScheduleService GameScheduleService,
	instanceService services.InstanceService,
	instanceSettingsService InstanceSettingsService,
	locationService services.LocationService,
	markerService MarkerService,
	navigationService NavigationService,
	notificationService NotificationService,
	teamService TeamService,
	templateService services.TemplateService,
	uploadService UploadService,
	userService UserService,
	quickstartService QuickstartService,
) *AdminHandler {
	return &AdminHandler{
		logger:                  logger,
		accessService:           accessService,
		assetGenerator:          assetGenerator,
		identityService:         identityService,
		blockService:            blockService,
		clueService:             clueService,
		deleteService:           DeleteService,
		facilitatorService:      facilitatorService,
		gameScheduleService:     gameScheduleService,
		instanceService:         instanceService,
		instanceSettingsService: instanceSettingsService,
		locationService:         locationService,
		markerService:           markerService,
		navigationService:       navigationService,
		notificationService:     notificationService,
		teamService:             teamService,
		templateService:         templateService,
		uploadService:           uploadService,
		userService:             userService,
		quickstartService:       quickstartService,
	}
}

// GetIdentityService returns the IdentityService used by the handler.
func (h *AdminHandler) GetIdentityService() IdentityService {
	return h.identityService
}

// GetUserFromContext retrieves the user from the context.
// User will always be in the context because the middleware.
func (h AdminHandler) UserFromContext(ctx context.Context) *models.User {
	return ctx.Value(contextkeys.UserKey).(*models.User)
}

func (h *AdminHandler) handleError(w http.ResponseWriter, r *http.Request, logMsg string, flashMsg string, params ...interface{}) {
	h.logger.Error(logMsg, params...)
	err := templates.Toast(*flash.NewError(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.logger.Error(logMsg+" - rendering template", "error", err)
	}
}

func (h *AdminHandler) handleSuccess(w http.ResponseWriter, r *http.Request, flashMsg string) {
	err := templates.Toast(*flash.NewSuccess(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering success template", "error", err)
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
