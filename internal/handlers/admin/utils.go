package admin

import (
	"context"
	"log/slog"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/flash"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/internal/sessions"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
)

// htmxHeaderTrue is the string value "true" used in HTMX header comparisons.
const htmxHeaderTrue = "true"

type AccessService interface {
	CanAdminAccessBlock(ctx context.Context, userID, blockID string) (bool, error)
	CanAdminAccessQuest(ctx context.Context, userID, instanceID string) (bool, error)
	CanAdminAccessLocation(ctx context.Context, userID, locationID string) (bool, error)
	CanAdminAccessMarker(ctx context.Context, userID, markerID string) (bool, error)
	CanAdminAccessBlockOwner(
		ctx context.Context,
		userID, ownerID string,
		blockContext blocks.BlockContext,
	) (bool, error)
}

type BlockService interface {
	// NewBlockWithOwnerAndContext creates a new content block for the given owner and context
	NewBlockWithOwnerAndContext(
		ctx context.Context,
		ownerID string,
		blockContext blocks.BlockContext,
		blockType string,
	) (blocks.Block, error)
	// NewBlockState creates a new player state for the given block and team
	NewBlockState(ctx context.Context, blockID, runCode, questID string) (blocks.PlayerState, error)
	// NewMockBlockState creates a mock player state (for testing/demo scenarios)
	NewMockBlockState(ctx context.Context, blockID, runCode, questID string) (blocks.PlayerState, error)

	// GetByBlockID fetches a content block by its ID
	GetByBlockID(ctx context.Context, blockID string) (blocks.Block, error)
	// GetBlockWithStateByBlockIDAndRunCode fetches a block + its state
	// for the given block ID and team
	GetBlockWithStateByBlockIDAndRunCode(
		ctx context.Context,
		blockID, runCode, questID string,
	) (blocks.Block, blocks.PlayerState, error)
	// FindByOwnerIDAndContext fetches all content blocks for an owner with specific context
	FindByOwnerIDAndContext(
		ctx context.Context,
		ownerID string,
		blockContext blocks.BlockContext,
	) (blocks.Blocks, error)
	// FindByOwnerID fetches all content blocks for an owner
	FindByOwnerID(ctx context.Context, ownerID string) (blocks.Blocks, error)
	// FindByOwnerIDAndRunCodeWithState fetches all blocks and their states
	// for the given owner and team
	FindByOwnerIDAndRunCodeWithState(
		ctx context.Context,
		ownerID, runCode, questID string,
	) ([]blocks.Block, map[string]blocks.PlayerState, error)
	// FindByOwnerIDAndRunCodeWithStateAndContext fetches all blocks and their states
	// for the given owner, team, and context
	FindByOwnerIDAndRunCodeWithStateAndContext(
		ctx context.Context,
		ownerID, runCode, questID string,
		blockContext blocks.BlockContext,
	) ([]blocks.Block, map[string]blocks.PlayerState, error)

	// UpdateBlock updates the data for the given block
	UpdateBlock(ctx context.Context, block blocks.Block, data map[string][]string) (blocks.Block, error)
	// UpdateState updates the player state for a block
	UpdateState(ctx context.Context, state blocks.PlayerState) (blocks.PlayerState, error)
	// ReorderBlocks changes the display/order of blocks at a location
	ReorderBlocks(ctx context.Context, blockIDs []string) error

	// CheckValidationRequiredForLocation checks if any blocks in a location require validation
	CheckValidationRequiredForLocation(ctx context.Context, locationID string) (bool, error)
	// CheckValidationRequiredForCheckIn checks if any blocks still require validation for a check-in
	CheckValidationRequiredForCheckIn(ctx context.Context, locationID, runCode, questID string) (bool, error)
}

type CreditService interface {
	GetCreditAdjustments(
		ctx context.Context,
		filter services.CreditAdjustmentFilter,
	) ([]models.CreditAdjustments, error)
	GetRunStartLogsSummary(
		ctx context.Context,
		filter services.RunStartLogFilter,
	) ([]services.RunStartSummary, error)
}

type DeleteService interface {
	DeleteBlock(ctx context.Context, blockID string) error
	DeleteQuest(ctx context.Context, userID, instanceID string) error
	DeleteLocation(ctx context.Context, locationID string) error
	ResetTeams(ctx context.Context, instanceID string, runCodes []string) error
	DeleteTeams(ctx context.Context, instanceID string, teamIDs []string) error
	DeleteUser(ctx context.Context, userID string) error
}

type DuplicationService interface {
	DuplicateQuest(
		ctx context.Context,
		user *models.User,
		sourceInstanceID string,
		name string,
	) (*models.Quest, error)
	CreateTemplateFromQuest(
		ctx context.Context,
		user *models.User,
		sourceInstanceID string,
		name string,
	) (*models.Quest, error)
	CreateQuestFromTemplate(
		ctx context.Context,
		user *models.User,
		templateID string,
		name string,
	) (*models.Quest, error)
	CreateQuestFromSharedTemplate(
		ctx context.Context,
		user *models.User,
		templateID string,
		name string,
	) (*models.Quest, error)
	DuplicateLocation(
		ctx context.Context,
		sourceLocation models.Location,
		newInstanceID string,
	) (*models.Location, error)
}

type FacilitatorService interface {
	CreateFacilitatorToken(
		ctx context.Context,
		instanceID string,
		locations []string,
		duration time.Duration,
	) (string, error)
	ValidateToken(ctx context.Context, token string) (*models.FacilitatorToken, error)
	CleanupExpiredTokens(ctx context.Context) error
}

type GameScheduleService interface {
	Start(ctx context.Context, instance *models.Quest) error
	Stop(ctx context.Context, instance *models.Quest) error
	SetStartTime(ctx context.Context, instance *models.Quest, start time.Time) error
	SetEndTime(ctx context.Context, instance *models.Quest, end time.Time) error
	ScheduleGame(ctx context.Context, instance *models.Quest, start, end time.Time) error
}

type QuestService interface {
	// CreateQuest creates a new quest for the given user
	CreateQuest(ctx context.Context, name string, user *models.User) (*models.Quest, error)

	// FindByUserID returns all instances for the given user
	FindByUserID(ctx context.Context, userID string) ([]models.Quest, error)
	// FindQuestIDsForUser returns the IDs of all quests for the given user
	FindQuestIDsForUser(ctx context.Context, userID string) ([]string, error)

	// GetByID finds an instance by ID
	GetByID(ctx context.Context, id string) (*models.Quest, error)
	// Update updates an instance
	Update(ctx context.Context, instance *models.Quest) error
}

type IdentityService interface {
	GetAuthenticatedUser(r *http.Request) (*models.User, error)
}

// QuestLoader loads a quest with all relations needed for the admin panel.
type QuestLoader interface {
	GetByIDWithRelations(ctx context.Context, id string) (*models.Quest, error)
}

type QuestSettingsService interface {
	SaveSettings(ctx context.Context, settings *models.QuestSettings) error
	GetQuestSettings(ctx context.Context, instanceID string) (*models.QuestSettings, error)
}

type MarkerService interface {
	// CreateMarker creates a new marker
	CreateMarker(ctx context.Context, name string, lat, lng float64) (models.Marker, error)
	// DuplicateLocation creates a new location given an existing location and the instance ID of the new location
	// FindMarkersNotInQuest finds all markers that are not in the given quest
	FindMarkersNotInQuest(ctx context.Context, instanceID string, otherInstances []string) ([]models.Marker, error)
}

type NavigationService interface {
	GetNextLocations(ctx context.Context, team *models.Run) ([]models.Location, error)
	GetPlayerNavigationView(ctx context.Context, team *models.Run) (*services.PlayerNavigationView, error)
}

type NotificationService interface {
	SendNotification(ctx context.Context, runCode string, content string) (models.Notification, error)
	SendNotificationToAllTeams(ctx context.Context, instanceID string, content string) error
	GetNotifications(ctx context.Context, runCode string) ([]models.Notification, error)
}

type QuickstartService interface {
	DismissQuickstart(ctx context.Context, instanceID string) error
}

type RunService interface {
	// AddTeams adds teams to the database
	AddTeams(ctx context.Context, instanceID string, count int) ([]models.Run, error)

	// FindAll returns all teams for an instance
	FindAll(ctx context.Context, instanceID string) ([]models.Run, error)
	// GetRunByCode returns a run by code
	GetRunByCode(ctx context.Context, code string) (*models.Run, error)
	// GetRunActivityOverview returns a list of runs and their activity
	GetRunActivityOverview(
		ctx context.Context,
		instanceID string,
		locations []models.Location,
	) ([]services.RunActivity, error)

	LoadCheckIns(ctx context.Context, team *models.Run) error
	// LoadRelations loads all relations for a team
	LoadRelations(ctx context.Context, team *models.Run) error

	// BuildLocationGroupMap creates a map from location ID to group info
	BuildLocationGroupMap(structure *models.GameStructure) map[string]services.LocationGroupInfo
	// BuildGroupOrder creates a map from group name to its order in the game structure
	BuildGroupOrder(structure *models.GameStructure) map[string]int
	// GroupCheckInsByGroup groups check-ins by their location's group and sorts by game structure order
	GroupCheckInsByGroup(
		checkIns []models.CheckIn,
		locationGroups map[string]services.LocationGroupInfo,
		groupOrder map[string]int,
	) []services.GroupedCheckIns
}

type UploadService interface {
	UploadFile(
		ctx context.Context,
		file multipart.File,
		fileHeader *multipart.FileHeader,
		data services.UploadMetadata,
	) (*models.Upload, error)
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
	// SwitchQuest switches the user's current quest
	SwitchQuest(ctx context.Context, user *models.User, instanceID string) error
}

type LeaderBoardService interface {
	// GetLeaderBoardData returns sorted and ranked leaderboard data
	GetLeaderBoardData(
		ctx context.Context,
		teams []models.Run,
		locationCount int,
		rankingScheme string,
		sortField string,
		sortOrder string,
	) ([]services.LeaderBoardTeamData, error)
}

// Handler provides admin functionality for managing game instances.
type Handler struct {
	logger                  *slog.Logger
	accessService           AccessService
	assetGenerator          services.AssetGenerator
	identityService         IdentityService
	blockService            BlockService
	creditService           CreditService
	creditPurchaseRepo      CreditPurchaseRepository
	deleteService           DeleteService
	duplicationService      DuplicationService
	exportService           *services.ExportService
	importService           *services.ImportService
	facilitatorService      FacilitatorService
	gameScheduleService     GameScheduleService
	gameStructureService    *services.GameStructureService
	questLoader             QuestLoader
	questService            QuestService
	instanceSettingsService QuestSettingsService
	locationService         services.LocationService
	markerService           MarkerService
	navigationService       NavigationService
	notificationService     NotificationService
	runService              RunService
	templateService         services.TemplateService
	uploadService           UploadService
	userService             UserService
	quickstartService       QuickstartService
	leaderBoardService      LeaderBoardService
	stripeService           StripeService
}

func NewAdminHandler(
	logger *slog.Logger,
	accessService AccessService,
	assetGenerator services.AssetGenerator,
	identityService IdentityService,
	blockService BlockService,
	creditService CreditService,
	creditPurchaseRepo CreditPurchaseRepository,
	deleteService DeleteService,
	duplicationService DuplicationService,
	exportService *services.ExportService,
	importService *services.ImportService,
	facilitatorService FacilitatorService,
	gameScheduleService GameScheduleService,
	gameStructureService *services.GameStructureService,
	questLoader QuestLoader,
	questService QuestService,
	instanceSettingsService QuestSettingsService,
	locationService services.LocationService,
	markerService MarkerService,
	navigationService NavigationService,
	notificationService NotificationService,
	runService RunService,
	templateService services.TemplateService,
	uploadService UploadService,
	userService UserService,
	quickstartService QuickstartService,
	leaderBoardService LeaderBoardService,
	stripeService StripeService,
) *Handler {
	return &Handler{
		logger:                  logger,
		accessService:           accessService,
		assetGenerator:          assetGenerator,
		identityService:         identityService,
		blockService:            blockService,
		creditService:           creditService,
		creditPurchaseRepo:      creditPurchaseRepo,
		deleteService:           deleteService,
		duplicationService:      duplicationService,
		exportService:           exportService,
		importService:           importService,
		facilitatorService:      facilitatorService,
		gameScheduleService:     gameScheduleService,
		gameStructureService:    gameStructureService,
		questLoader:             questLoader,
		questService:            questService,
		instanceSettingsService: instanceSettingsService,
		locationService:         locationService,
		markerService:           markerService,
		navigationService:       navigationService,
		notificationService:     notificationService,
		runService:              runService,
		templateService:         templateService,
		uploadService:           uploadService,
		userService:             userService,
		quickstartService:       quickstartService,
		leaderBoardService:      leaderBoardService,
		stripeService:           stripeService,
	}
}

// GetIdentityService returns the IdentityService used by the handler.
func (h *Handler) GetIdentityService() IdentityService {
	return h.identityService
}

// GetQuestLoader returns the QuestLoader used by the handler.
func (h *Handler) GetQuestLoader() QuestLoader {
	return h.questLoader
}

// UserFromContext retrieves the user from the context.
// User will always be in the context because of the middleware.
func (h *Handler) UserFromContext(ctx context.Context) *models.User {
	user, ok := ctx.Value(contextkeys.UserKey).(*models.User)
	if !ok {
		return nil
	}
	return user
}

func (h *Handler) handleError(
	w http.ResponseWriter,
	r *http.Request,
	logMsg string,
	flashMsg string,
	params ...any,
) {
	h.logger.ErrorContext(r.Context(), logMsg, params...)
	err := templates.Toast(*flash.NewError(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), logMsg+" - rendering template", "error", err)
	}
}

func (h *Handler) handleSuccess(w http.ResponseWriter, r *http.Request, flashMsg string) {
	err := templates.Toast(*flash.NewSuccess(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), "rendering success template", "error", err)
	}
}

// setCurrentQuest stores the current quest ID in the admin session.
func (h *Handler) setCurrentQuest(w http.ResponseWriter, r *http.Request, instanceID string) {
	session, err := sessions.Get(r, "admin")
	if err != nil {
		h.logger.ErrorContext(r.Context(), "setCurrentQuest: getting session", "error", err)
		return
	}
	session.Values["current_instance"] = instanceID
	if err := session.Save(r, w); err != nil {
		h.logger.ErrorContext(r.Context(), "setCurrentQuest: saving session", "error", err)
	}
}

// redirect is a helper function to redirect the user to a new page.
// It accounts for htmx requests and redirects the user to the referer.
func (h *Handler) redirect(w http.ResponseWriter, r *http.Request, path string) {
	if r.Header.Get("Hx-Request") == htmxHeaderTrue {
		w.Header().Set("Hx-Redirect", path)
		return
	}
	http.Redirect(w, r, path, http.StatusFound)
}
