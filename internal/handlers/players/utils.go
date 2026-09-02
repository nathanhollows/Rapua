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
	ValidateAndUpdateBlockState(
		ctx context.Context,
		team models.Run,
		data map[string][]string,
	) (blocks.PlayerState, blocks.Block, error)
	GetObjectiveByQuestIDAndSlug(ctx context.Context, questID, slug string) (*models.Objective, error)
	ObjectiveIsReachable(ctx context.Context, team *models.Run, objective *models.Objective) (bool, error)
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

type NavigationService interface {
	GetPlayerObjectiveView(ctx context.Context, team *models.Run) (*services.PlayerObjectiveView, error)
	FinishSection(ctx context.Context, team *models.Run, objectiveID string) (bool, error)
	GetPreviewObjectiveView(
		ctx context.Context,
		team *models.Run,
		objectiveID string,
	) (*services.PlayerObjectiveView, error)
}

type NotificationService interface {
	GetNotifications(ctx context.Context, runCode string) ([]models.Notification, error)
	DismissNotification(ctx context.Context, notificationID string) error
}

type RunService interface {
	GetRunByCode(ctx context.Context, code string) (*models.Run, error)
	Update(ctx context.Context, run *models.Run) error
	LoadRelations(ctx context.Context, run *models.Run) error
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
