package players

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime/multipart"
	"net/http"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/flash"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/internal/sessions"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/players"
	"github.com/nathanhollows/Rapua/v8/models"
)

type BlockService interface {
	GetByBlockID(ctx context.Context, blockID string) (blocks.Block, error)
	GetBlockContext(ctx context.Context, blockID string) (blocks.BlockContext, error)
	NewMockBlockState(ctx context.Context, blockID, runCode, questID string) (blocks.PlayerState, error)
	FindByOwnerIDAndContext(
		ctx context.Context,
		ownerID string,
		blockContext blocks.BlockContext,
	) (blocks.Blocks, error)
	FindByOwnerIDAndRunCodeWithStateAndContext(
		ctx context.Context,
		ownerID, runCode, questID string,
		blockContext blocks.BlockContext,
	) ([]blocks.Block, map[string]blocks.PlayerState, error)
}

type CheckInService interface {
	CheckOut(ctx context.Context, team *models.Run, locationCode string) error
	ValidateAndUpdateBlockState(
		ctx context.Context,
		team models.Run,
		data map[string][]string,
	) (blocks.PlayerState, blocks.Block, error)
	GetObjectiveByQuestIDAndSlug(ctx context.Context, questID, slug string) (*models.Objective, error)
	IsObjectiveContextPending(
		ctx context.Context,
		team *models.Run,
		objectiveID string,
		blockContext blocks.BlockContext,
	) (bool, error)
	CompleteObjectiveContext(
		ctx context.Context,
		team *models.Run,
		objectiveID string,
		blockContext blocks.BlockContext,
	) error
}

type QuestService interface {
	GetQuestSettings(ctx context.Context, questID string) (*models.QuestSettings, error)
	GetByID(ctx context.Context, questID string) (*models.Quest, error)
}

type MarkerService interface {
	GetMarkerByCode(ctx context.Context, locationCode string) (models.Marker, error)
}

type NavigationService interface {
	GetNextLocations(ctx context.Context, team *models.Run) ([]models.Location, error)
	GetPlayerNavigationView(ctx context.Context, team *models.Run) (*services.PlayerNavigationView, error)
	GetPreviewNavigationView(
		ctx context.Context,
		team *models.Run,
		locationID string,
	) (*services.PlayerNavigationView, error)
	GetPlayerObjectiveView(ctx context.Context, team *models.Run) (*services.PlayerObjectiveView, error)
	GetPreviewObjectiveView(
		ctx context.Context,
		team *models.Run,
		objectiveID string,
	) (*services.PlayerObjectiveView, error)
}

type LocationService interface {
	GetByID(ctx context.Context, locationID string) (*models.Location, error)
	LoadBlocks(ctx context.Context, location *models.Location) error
}

type NotificationService interface {
	GetNotifications(ctx context.Context, runCode string) ([]models.Notification, error)
	DismissNotification(ctx context.Context, notificationID string) error
}

type RunService interface {
	GetRunByCode(ctx context.Context, code string) (*models.Run, error)
	Update(ctx context.Context, run *models.Run) error
	LoadRelations(ctx context.Context, run *models.Run) error
	LoadBlockingLocation(ctx context.Context, run *models.Run) error
	LoadQuest(ctx context.Context, run *models.Run) error
	StartPlaying(ctx context.Context, runCode string) error
	GetCompletedObjectives(ctx context.Context, questID, runCode string) ([]models.Objective, error)
}

type UploadService interface {
	UploadFile(
		ctx context.Context,
		file multipart.File,
		fileHeader *multipart.FileHeader,
		data services.UploadMetadata,
	) (*models.Upload, error)
}

//nolint:recvcheck // Read-only methods use value receiver, state-modifying methods use pointer receiver
type PlayerHandler struct {
	logger              *slog.Logger
	blockService        BlockService
	checkInService      CheckInService
	questService        QuestService
	locationService     LocationService
	markerService       MarkerService
	navigationService   NavigationService
	notificationService NotificationService
	runService          RunService
	uploadService       UploadService
}

func NewPlayerHandler(
	logger *slog.Logger,
	blockService BlockService,
	checkInService CheckInService,
	questService QuestService,
	locationService LocationService,
	markerService MarkerService,
	navigationService NavigationService,
	notificationService NotificationService,
	runService RunService,
	uploadService UploadService,
) *PlayerHandler {
	return &PlayerHandler{
		logger:              logger,
		blockService:        blockService,
		checkInService:      checkInService,
		questService:        questService,
		locationService:     locationService,
		markerService:       markerService,
		navigationService:   navigationService,
		notificationService: notificationService,
		runService:          runService,
		uploadService:       uploadService,
	}
}

func (h PlayerHandler) GetQuestService() QuestService {
	return h.questService
}

func (h PlayerHandler) GetRunService() RunService {
	return h.runService
}

// getRunFromContext retrieves the run from the context.
func (h PlayerHandler) getRunFromContext(ctx context.Context) (*models.Run, error) {
	val := ctx.Value(contextkeys.RunKey)
	if val == nil {
		return nil, errors.New("run not found")
	}
	run, ok := val.(*models.Run)
	if !ok || run == nil {
		return nil, errors.New("run not found")
	}
	return run, nil
}

// isObjectiveBuiltQuest reports whether a quest's GameStructure is objective-built
// rather than location-built. A quest doc is always fully one kind or the other,
// never mixed (enforced at the doc level), so the first populated ID list found
// anywhere in the tree, depth-first, settles it for the whole structure. This
// reads team.Quest.GameStructure directly rather than the Locations/Objectives
// relations, which are only conditionally loaded and unreliable as a signal.
// An unconfigured/empty structure (no groups yet) defaults to objective-built.
func isObjectiveBuiltQuest(gs models.GameStructure) bool {
	if found, isObjective := questTypeOf(gs); found {
		return isObjective
	}
	return true
}

func questTypeOf(gs models.GameStructure) (found, isObjective bool) {
	if len(gs.LocationIDs) > 0 {
		return true, false
	}
	if len(gs.ObjectiveIDs) > 0 {
		return true, true
	}
	for _, sub := range gs.SubGroups {
		if f, obj := questTypeOf(sub); f {
			return true, obj
		}
	}
	return false, false
}

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
	session.Values["run"] = teamCode
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
	h.logger.ErrorContext(r.Context(), logMsg, params...)
	err := templates.Toast(*flash.NewError(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.logger.ErrorContext(r.Context(), logMsg+" - rendering template", "error", err)
	}
}
