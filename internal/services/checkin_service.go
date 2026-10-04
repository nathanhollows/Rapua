package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
)

type CheckInService struct {
	teamRepo                       repositories.RunRepository
	blockService                   *BlockService
	objectiveRepo                  repositories.ObjectiveRepository
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository
	loader                         runStateLoader
}

func NewCheckInService(
	teamRepo repositories.RunRepository,
	blockService *BlockService,
	objectiveRepo repositories.ObjectiveRepository,
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository,
	sectionFinishRepo repositories.SectionFinishRepository,
	blockRepo repositories.BlockRepository,
) *CheckInService {
	return &CheckInService{
		teamRepo:                       teamRepo,
		blockService:                   blockService,
		objectiveRepo:                  objectiveRepo,
		objectiveContextCompletionRepo: objectiveContextCompletionRepo,
		loader: runStateLoader{
			objectiveRepo:                  objectiveRepo,
			objectiveContextCompletionRepo: objectiveContextCompletionRepo,
			sectionFinishRepo:              sectionFinishRepo,
			blockRepo:                      blockRepo,
		},
	}
}

func (s *CheckInService) ValidateAndUpdateBlockState( //nolint:gocognit
	ctx context.Context,
	team models.Run,
	data map[string][]string,
) (blocks.PlayerState, blocks.Block, error) {
	blockID := data["block"][0]
	if blockID == "" {
		return nil, nil, errors.New("blockID must be set")
	}

	// Preview mode should use fresh mock state.
	isPreview := ctx.Value(contextkeys.PreviewKey) != nil

	var block blocks.Block
	var state blocks.PlayerState
	var err error

	if isPreview {
		block, err = s.blockService.GetByBlockID(ctx, blockID)
		if err != nil {
			return nil, nil, fmt.Errorf("getting block in preview mode: %w", err)
		}

		state, err = s.blockService.NewMockBlockState(ctx, blockID, team.Code, team.QuestID)
		if err != nil {
			return nil, nil, fmt.Errorf("creating mock state in preview mode: %w", err)
		}
	} else {
		block, state, err = s.blockService.GetBlockWithStateByBlockIDAndRunCode(ctx, blockID, team.Code, team.QuestID)
		if err != nil {
			return nil, nil, fmt.Errorf("getting block with state: %w", err)
		}
	}

	if block == nil {
		return nil, nil, errors.New("block not found")
	}

	if state == nil {
		return nil, nil, errors.New("block state not found")
	}

	// Prevent duplicate points for completed blocks in regular play.
	if !isPreview && state.IsComplete() {
		return state, block, nil
	}

	state, err = block.ValidatePlayerInput(state, data)
	if err != nil {
		return nil, nil, fmt.Errorf("validating block: %w", err)
	}

	// Preview never persists state.
	if !isPreview {
		state, err = s.blockService.UpdateState(ctx, state)
		if err != nil {
			return nil, nil, fmt.Errorf("updating block state: %w", err)
		}
	}

	// Preview never awards points.
	if !isPreview && state.IsComplete() {
		blockContext, err := s.blockService.GetBlockContext(ctx, blockID)
		if err != nil {
			return nil, nil, fmt.Errorf("getting block context: %w", err)
		}
		if err = s.awardPointsAndComplete(ctx, &team, block, blockContext); err != nil {
			return nil, nil, err
		}
	}

	return state, block, nil
}

// awardPointsAndComplete awards points and, for objective contexts, logs
// completion once every block in the context is done.
func (s *CheckInService) awardPointsAndComplete(
	ctx context.Context, team *models.Run, block blocks.Block, blockContext game.BlockContext,
) error {
	if blockContext != game.ContextObjectiveProof && blockContext != game.ContextObjectiveReveal {
		team.Points += block.GetPoints()
		if err := s.teamRepo.Update(ctx, team); err != nil {
			return fmt.Errorf("awarding points: %w", err)
		}
		return nil
	}

	// Reachability first, and once. Points used to be credited before the gate
	// ran, so a team could be paid for finishing an objective the gate then
	// turned away without a completion row: paid for work the game does not
	// record. The answer is passed down rather than asked for again, since each
	// ask is a full tree load and a frontier.
	objective, err := s.objectiveRepo.GetByID(ctx, block.GetOwnerID())
	if err != nil {
		return fmt.Errorf("loading objective: %w", err)
	}
	reachable, err := s.ObjectiveIsReachable(ctx, team, objective)
	if err != nil {
		return err
	}
	if !reachable {
		return nil
	}

	team.Points += block.GetPoints()
	if err := s.teamRepo.Update(ctx, team); err != nil {
		return fmt.Errorf("awarding points: %w", err)
	}

	return s.completeReachableObjectiveContext(ctx, team, objective, blockContext)
}

// CompleteObjectiveContext logs the completion and applies the context's sets
// once every block in the context is done. Logging is unconditional even when
// the context defines no sets. Exported because a content-only context has
// nothing a player can POST to, so it needs a direct caller outside the
// POST/validate path. Safe to call unconditionally: the idempotency guard
// makes a context that is not done, or already logged, a harmless no-op, and
// an objective the run cannot yet reach is a no-op for the same reason.
func (s *CheckInService) CompleteObjectiveContext(
	ctx context.Context, team *models.Run, objectiveID string, blockContext game.BlockContext,
) error {
	// Preview runs have no matching row in runs, and objective_context_completions.run_code
	// has a real FK to it: logging completion for a preview run would violate that
	// constraint. Preview has no real completion state to log anyway.
	if ctx.Value(contextkeys.PreviewKey) != nil {
		return nil
	}

	stillRequired, err := s.blockService.checkValidationRequiredForCheckIn(
		ctx, objectiveID, team.Code, team.QuestID, blockContext,
	)
	if err != nil {
		return fmt.Errorf("checking if objective context is complete: %w", err)
	}
	if stillRequired {
		return nil
	}

	// Loaded before the insert, not after, so the depends check below runs
	// before anything is written. Only reached once the context is actually
	// complete, so this is not a per-validation cost.
	objective, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return fmt.Errorf("loading objective: %w", err)
	}

	// A slug is guessable and the objective view completes content-only
	// contexts on GET, so without this a player could reach a gated objective
	// directly and fire its sets, opening every downstream gate out of order.
	reachable, err := s.ObjectiveIsReachable(ctx, team, objective)
	if err != nil {
		return err
	}
	if !reachable {
		return nil
	}

	return s.logObjectiveContextCompletion(ctx, team, objective, blockContext)
}

// completeReachableObjectiveContext is CompleteObjectiveContext for a caller
// that has already established the objective is reachable, so the gate is not
// paid for twice: each check loads the quest's tree and derives a frontier.
func (s *CheckInService) completeReachableObjectiveContext(
	ctx context.Context, team *models.Run, objective *models.Objective, blockContext game.BlockContext,
) error {
	if ctx.Value(contextkeys.PreviewKey) != nil {
		return nil
	}

	stillRequired, err := s.blockService.checkValidationRequiredForCheckIn(
		ctx, objective.ID, team.Code, team.QuestID, blockContext,
	)
	if err != nil {
		return fmt.Errorf("checking if objective context is complete: %w", err)
	}
	if stillRequired {
		return nil
	}

	return s.logObjectiveContextCompletion(ctx, team, objective, blockContext)
}

// logCompletionAndApplySets writes the completion row and, if that row is new,
// fires the context's sets. The insert is the idempotency guard: sets belong to
// the call that recorded the completion, not to every call that finds it done.
func (s *CheckInService) logObjectiveContextCompletion(
	ctx context.Context, team *models.Run, objective *models.Objective, blockContext game.BlockContext,
) error {
	if _, err := s.objectiveContextCompletionRepo.Insert(
		ctx, team.Code, objective.ID, blockContext,
	); err != nil {
		return fmt.Errorf("logging objective context completion: %w", err)
	}
	return nil
}

// IsObjectiveContextPending reports whether an objective's proof or reveal
// context still has unvalidated required blocks for this team: the same check
// CompleteObjectiveContext uses internally, exposed for callers (the objective
// view handler) that need to decide which zone to render before completing anything.
func (s *CheckInService) IsObjectiveContextPending(
	ctx context.Context, team *models.Run, objectiveID string, blockContext game.BlockContext,
) (bool, error) {
	return s.blockService.checkValidationRequiredForCheckIn(
		ctx, objectiveID, team.Code, team.QuestID, blockContext,
	)
}

// ObjectiveIsReachable reports whether a run can be on this objective's page at
// all: it is in play, its ancestors are open, its parent's routing has offered
// it, and its depends are met.
//
// This asks the frontier rather than re-deriving a piece of it, because a list
// that merely leaves an objective out is not a gate: a guessed slug, a printed
// QR code or a stale bookmark reaches the page directly. Every reason the list
// would omit an objective has to be a reason the page turns it away, or they
// are two different games.
func (s *CheckInService) ObjectiveIsReachable(
	ctx context.Context, team *models.Run, objective *models.Objective,
) (bool, error) {
	objectives, state, complete, err := s.loader.load(ctx, team)
	if err != nil {
		return false, fmt.Errorf("loading run state: %w", err)
	}

	frontier := navigation.ComputeFrontier(objectives, state, complete)
	switch frontier.StatusOf(objective.ID) {
	case navigation.StatusAvailable, navigation.StatusFinishable, navigation.StatusComplete:
		// Complete counts: revisiting somewhere already finished is reading it
		// again, not reaching somewhere out of bounds.
		return true, nil
	case navigation.StatusLocked:
		return false, nil
	}
	return false, nil
}

// GetObjectiveByQuestIDAndSlug finds an objective by slug without asking
// whether a run can reach it. Drafts included: preview is for looking at
// content mid-edit, and ObjectiveIsReachable is what gates a real player.
func (s *CheckInService) GetObjectiveByQuestIDAndSlug(
	ctx context.Context, questID, slug string,
) (*models.Objective, error) {
	return s.objectiveRepo.GetByQuestIDAndSlug(ctx, questID, slug)
}
