package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
	"github.com/uptrace/bun"
)

type CheckInService struct {
	transactor                     db.Transactor
	teamRepo                       repositories.RunRepository
	blockService                   *BlockService
	objectiveRepo                  repositories.ObjectiveRepository
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository
	runEventRepo                   repositories.RunEventRepository
	loader                         runStateLoader
}

func NewCheckInService(
	transactor db.Transactor,
	teamRepo repositories.RunRepository,
	blockService *BlockService,
	objectiveRepo repositories.ObjectiveRepository,
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository,
	sectionFinishRepo repositories.SectionFinishRepository,
	blockRepo repositories.BlockRepository,
	runEventRepo repositories.RunEventRepository,
) *CheckInService {
	return &CheckInService{
		transactor:                     transactor,
		teamRepo:                       teamRepo,
		blockService:                   blockService,
		objectiveRepo:                  objectiveRepo,
		objectiveContextCompletionRepo: objectiveContextCompletionRepo,
		runEventRepo:                   runEventRepo,
		loader: runStateLoader{
			objectiveRepo:                  objectiveRepo,
			objectiveContextCompletionRepo: objectiveContextCompletionRepo,
			sectionFinishRepo:              sectionFinishRepo,
			blockRepo:                      blockRepo,
		},
	}
}

func (s *CheckInService) ValidateAndUpdateBlockState(
	ctx context.Context,
	team models.Run,
	data map[string][]string,
) (blocks.PlayerState, blocks.Block, error) {
	blockID := data["block"][0]
	if blockID == "" {
		return nil, nil, errors.New("blockID must be set")
	}

	// Preview uses fresh mock state and writes nothing, events included. It
	// carries the run code "preview", which has no row in runs, so anything
	// written against it trips a foreign key.
	if ctx.Value(contextkeys.PreviewKey) != nil {
		return s.validateInPreview(ctx, team, blockID, data)
	}

	// Every read happens before the transaction opens, and the transaction holds
	// nothing but writes. Holding the write lock across the reachability and
	// frontier work would serialize every submission in the quest behind a full
	// tree load, and in shared-cache mode a read on another connection while the
	// lock is held does not wait, it deadlocks.
	block, state, err := s.blockService.GetBlockWithStateByBlockIDAndRunCode(
		ctx, blockID, team.Code, team.QuestID,
	)
	if err != nil {
		return nil, nil, fmt.Errorf("getting block with state: %w", err)
	}
	if block == nil {
		return nil, nil, errors.New("block not found")
	}
	if state == nil {
		return nil, nil, errors.New("block state not found")
	}

	// Already complete: a resubmission writes nothing, the event included, so a
	// second click is not a second attempt.
	if state.IsComplete() {
		return state, block, nil
	}

	// Read on every submission, not only on a completing one: both the plan and
	// the event need the context.
	blockContext, err := s.blockService.GetBlockContext(ctx, blockID)
	if err != nil {
		return nil, nil, fmt.Errorf("getting block context: %w", err)
	}

	state, err = block.ValidatePlayerInput(state, data)
	if err != nil {
		return nil, nil, fmt.Errorf("validating block: %w", err)
	}

	plan, err := s.planCompletion(ctx, &team, block, blockContext, state.IsComplete())
	if err != nil {
		return nil, nil, err
	}

	tx, err := s.transactor.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	won, err := s.applyAnswer(ctx, tx, &team, block, blockContext, state, plan)
	if err != nil {
		return nil, nil, err
	}
	if !won {
		return state, block, nil
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("committing block validation: %w", err)
	}

	return state, block, nil
}

// applyAnswer writes everything one submission changes, inside the caller's
// transaction: the block state, the points, the completion row and the events. It
// reports false, with nothing written, when the block was already complete.
func (s *CheckInService) applyAnswer(
	ctx context.Context,
	tx *bun.Tx,
	team *models.Run,
	block blocks.Block,
	blockContext game.BlockContext,
	state blocks.PlayerState,
	plan completionPlan,
) (bool, error) {
	// The read that decided the block was incomplete happened outside this
	// transaction, so the condition is re-checked where it cannot go stale: a
	// second submission that lands after the first commits matches no row.
	won, err := s.blockService.UpdateStateIfIncompleteWithTx(ctx, tx, state)
	if err != nil {
		return false, fmt.Errorf("updating block state: %w", err)
	}
	if !won {
		return false, nil
	}

	// One number feeds the score and the event, so the stream's sum and the run's
	// total cannot drift apart.
	credited := 0
	if plan.awardPoints {
		// Incremented in the database, not by writing a total read before this
		// transaction opened: two submissions landing together would each write
		// their own total and one award would be lost.
		//
		// The block's computed value, not its configured points: a partial score
		// is what the player was shown, and it is what they earn.
		credited = state.GetPointsAwarded()
		if err := s.teamRepo.AddPointsWithTx(ctx, tx, team.Code, credited); err != nil {
			return false, fmt.Errorf("awarding points: %w", err)
		}
	}

	completedContext := false
	if plan.objective != nil {
		inserted, err := s.objectiveContextCompletionRepo.InsertWithTx(
			ctx, tx, team.Code, plan.objective.ID, blockContext,
		)
		if err != nil {
			return false, fmt.Errorf("logging objective context completion: %w", err)
		}
		completedContext = inserted
	}

	// objectiveID is empty for a block the quest owns rather than an objective, so
	// the column never carries a quest ID.
	objectiveID := ""
	if blockContext == game.ContextObjectiveProof || blockContext == game.ContextObjectiveReveal {
		objectiveID = block.GetOwnerID()
	}

	answerEvent := answeredEvent(team, block, objectiveID, state.IsComplete(), state.GetPointsAwarded())
	// The payload carries what the block computed; the column carries what the
	// score moved by.
	answerEvent.Points = credited
	if err := s.runEventRepo.RecordWithTx(ctx, tx, answerEvent); err != nil {
		return false, fmt.Errorf("recording answer: %w", err)
	}

	if completedContext {
		// The completion carries no points of its own: the award rode the answer
		// that triggered it, and writing it again here would double the sum.
		if err := s.runEventRepo.RecordWithTx(ctx, tx, objectiveContextCompletedEvent(
			team, block.GetOwnerID(), blockContext,
		)); err != nil {
			return false, fmt.Errorf("recording objective completion: %w", err)
		}
	}

	return true, nil
}

func (s *CheckInService) validateInPreview(
	ctx context.Context, team models.Run, blockID string, data map[string][]string,
) (blocks.PlayerState, blocks.Block, error) {
	block, err := s.blockService.GetByBlockID(ctx, blockID)
	if err != nil {
		return nil, nil, fmt.Errorf("getting block in preview mode: %w", err)
	}

	state, err := s.blockService.NewMockBlockState(ctx, blockID, team.Code, team.QuestID)
	if err != nil {
		return nil, nil, fmt.Errorf("creating mock state in preview mode: %w", err)
	}

	if block == nil {
		return nil, nil, errors.New("block not found")
	}
	if state == nil {
		return nil, nil, errors.New("block state not found")
	}

	state, err = block.ValidatePlayerInput(state, data)
	if err != nil {
		return nil, nil, fmt.Errorf("validating block: %w", err)
	}

	return state, block, nil
}

// completionPlan is what a completing submission is entitled to: awardPoints for
// a start or finish block, which the quest owns and which has no gate to pass, and
// objective once the context is complete.
type completionPlan struct {
	awardPoints bool
	objective   *models.Objective
}

// planCompletion runs the gates before anything is written, so points are never
// credited for work the game does not record. The gate is asked once, and its
// answer carried into the transaction.
func (s *CheckInService) planCompletion(
	ctx context.Context,
	team *models.Run,
	block blocks.Block,
	blockContext game.BlockContext,
	complete bool,
) (completionPlan, error) {
	if !complete {
		return completionPlan{}, nil
	}

	if blockContext != game.ContextObjectiveProof && blockContext != game.ContextObjectiveReveal {
		return completionPlan{awardPoints: true}, nil
	}

	objective, err := s.objectiveRepo.GetByID(ctx, block.GetOwnerID())
	if err != nil {
		return completionPlan{}, fmt.Errorf("loading objective: %w", err)
	}

	reachable, err := s.ObjectiveIsReachable(ctx, team, objective)
	if err != nil {
		return completionPlan{}, err
	}
	if !reachable {
		return completionPlan{}, nil
	}

	stillRequired, err := s.blockService.checkValidationRequiredExcluding(
		ctx, objective.ID, team.Code, team.QuestID, blockContext, block.GetID(),
	)
	if err != nil {
		return completionPlan{}, fmt.Errorf("checking if objective context is complete: %w", err)
	}

	plan := completionPlan{awardPoints: true}
	if !stillRequired {
		plan.objective = objective
	}
	return plan, nil
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

	// Only the writes are transactional, for the same reason the check-in path
	// keeps its reads outside: the lock is not held across a tree load. Two
	// concurrent completions both get this far; the insert's conflict clause
	// decides which one recorded it.
	tx, err := s.transactor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	inserted, err := s.objectiveContextCompletionRepo.InsertWithTx(
		ctx, tx, team.Code, objective.ID, blockContext,
	)
	if err != nil {
		return fmt.Errorf("logging objective context completion: %w", err)
	}

	if inserted {
		if err := s.runEventRepo.RecordWithTx(ctx, tx, objectiveContextCompletedEvent(
			team, objective.ID, blockContext,
		)); err != nil {
			return fmt.Errorf("recording objective completion: %w", err)
		}
	}

	return tx.Commit()
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

// GetObjectiveByID finds an objective by id, for the places that hold a parent
// reference rather than a slug. Like the slug lookup it asks nothing about
// reachability: the caller has already decided what it is allowed to show.
func (s *CheckInService) GetObjectiveByID(
	ctx context.Context, objectiveID string,
) (*models.Objective, error) {
	return s.objectiveRepo.GetByID(ctx, objectiveID)
}

// GetObjectiveByQuestIDAndSlug finds an objective by slug without asking
// whether a run can reach it. Drafts included: preview is for looking at
// content mid-edit, and ObjectiveIsReachable is what gates a real player.
func (s *CheckInService) GetObjectiveByQuestIDAndSlug(
	ctx context.Context, questID, slug string,
) (*models.Objective, error) {
	return s.objectiveRepo.GetByQuestIDAndSlug(ctx, questID, slug)
}
