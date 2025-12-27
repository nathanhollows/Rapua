package players

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v6/internal/flash"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/internal/sessions"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/players"
	"github.com/nathanhollows/Rapua/v6/models"
)

type BlockService interface {
	// GetByBlockID fetches a content block by its ID
	GetByBlockID(ctx context.Context, blockID string) (blocks.Block, error)
	// NewMockBlockState creates a mock player state (for testing/demo scenarios)
	NewMockBlockState(ctx context.Context, blockID, teamCode string) (blocks.PlayerState, error)
	// FindByOwnerIDAndContext fetches all content blocks for an owner with specific context
	FindByOwnerIDAndContext(
		ctx context.Context,
		ownerID string,
		blockContext blocks.BlockContext,
	) (blocks.Blocks, error)
	// FindByOwnerIDAndTeamCodeWithStateAndContext fetches all blocks and their states
	// for the given owner, team, and context
	FindByOwnerIDAndTeamCodeWithStateAndContext(
		ctx context.Context,
		ownerID, teamCode string,
		blockContext blocks.BlockContext,
	) ([]blocks.Block, map[string]blocks.PlayerState, error)
}

type CheckInService interface {
	CheckIn(ctx context.Context, team *models.Team, locationCode string) error
	CheckOut(ctx context.Context, team *models.Team, locationCode string) error
	ValidateAndUpdateBlockState(
		ctx context.Context,
		team models.Team,
		data map[string][]string,
	) (blocks.PlayerState, blocks.Block, error)
}

type InstanceService interface {
	GetInstanceSettings(ctx context.Context, instanceID string) (*models.InstanceSettings, error)
	GetByID(ctx context.Context, instanceID string) (*models.Instance, error)
}

type MarkerService interface {
	GetMarkerByCode(ctx context.Context, locationCode string) (models.Marker, error)
}

type NavigationService interface {
	// IsValidLocation(ctx context.Context, team *models.Team, markerID string) (bool, error)
	GetNextLocations(ctx context.Context, team *models.Team) ([]models.Location, error)
	GetPlayerNavigationView(ctx context.Context, team *models.Team) (*services.PlayerNavigationView, error)
	GetPreviewNavigationView(
		ctx context.Context,
		team *models.Team,
		locationID string,
	) (*services.PlayerNavigationView, error)
	// HasVisited(checkins []models.CheckIn, locationID string) bool
}

type LocationService interface {
	GetByID(ctx context.Context, locationID string) (*models.Location, error)
	LoadBlocks(ctx context.Context, location *models.Location) error
}

type NotificationService interface {
	GetNotifications(ctx context.Context, teamCode string) ([]models.Notification, error)
	DismissNotification(ctx context.Context, notificationID string) error
}

type TeamService interface {
	// GetTeamByCode returns a team by code
	GetTeamByCode(ctx context.Context, code string) (*models.Team, error)
	// Update updates a team in the database
	Update(ctx context.Context, team *models.Team) error
	// LoadRelation loads relations for a team
	LoadRelation(ctx context.Context, team *models.Team, relation string) error
	// LoadRelations loads all relations for a team
	LoadRelations(ctx context.Context, team *models.Team) error
	// StartPlaying starts a team playing the game
	StartPlaying(ctx context.Context, teamCod string) error
}

type UploadService interface {
	// UploadFile uploads a file with metadata
	UploadFile(
		ctx context.Context,
		file multipart.File,
		fileHeader *multipart.FileHeader,
		data services.UploadMetadata,
	) (*models.Upload, error)
}

type PlayerHandler struct {
	logger              *slog.Logger
	blockService        BlockService
	checkInService      CheckInService
	instanceService     InstanceService
	locationService     LocationService
	markerService       MarkerService
	navigationService   NavigationService
	notificationService NotificationService
	teamService         TeamService
	uploadService       UploadService
}

func NewPlayerHandler(
	logger *slog.Logger,
	blockService BlockService,
	checkInService CheckInService,
	instanceService InstanceService,
	locationService LocationService,
	markerService MarkerService,
	navigationService NavigationService,
	notificationService NotificationService,
	teamService TeamService,
	uploadService UploadService,
) *PlayerHandler {
	return &PlayerHandler{
		logger:              logger,
		blockService:        blockService,
		checkInService:      checkInService,
		instanceService:     instanceService,
		locationService:     locationService,
		markerService:       markerService,
		navigationService:   navigationService,
		notificationService: notificationService,
		teamService:         teamService,
		uploadService:       uploadService,
	}
}

func (h PlayerHandler) GetInstanceService() InstanceService {
	return h.instanceService
}

func (h PlayerHandler) GetTeamService() TeamService {
	return h.teamService
}

// GetTeamFromContext retrieves the team from the context.
// Team will always be in the context because the middleware.
// However the Team could be nil if the team was not found.
func (h PlayerHandler) getTeamFromContext(ctx context.Context) (*models.Team, error) {
	val := ctx.Value(contextkeys.TeamKey)
	if val == nil {
		return nil, errors.New("team not found")
	}
	team, ok := val.(*models.Team)
	if !ok || team == nil {
		return nil, errors.New("team not found")
	}
	return team, nil
}

// redirect is a helper function to redirect the user to a new page.
// It accounts for htmx requests.
func (h PlayerHandler) redirect(w http.ResponseWriter, r *http.Request, path string) {
	if r.Header.Get("Hx-Request") == "true" {
		w.Header().Set("Hx-Redirect", path)
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
	session.Options.HttpOnly = true
	session.Options.SameSite = http.SameSiteLaxMode
	session.Options.Secure = true
	err = session.Save(r, w)
	if err != nil {
		return fmt.Errorf("saving session: %w", err)
	}

	return nil
}

func (h *PlayerHandler) handleError(
	w http.ResponseWriter,
	r *http.Request,
	logMsg string,
	flashMsg string,
	params ...any,
) {
	h.logger.Error(logMsg, params...)
	err := templates.Toast(*flash.NewError(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.logger.Error(logMsg+" - rendering template", "error", err)
	}
}
