package services

import (
	"context"
	"encoding/json"
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
	runVariableRepo                repositories.RunVariableRepository
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
	runVariableRepo repositories.RunVariableRepository,
) *CheckInService {
	return &CheckInService{
		transactor:                     transactor,
		teamRepo:                       teamRepo,
		blockService:                   blockService,
		objectiveRepo:                  objectiveRepo,
		objectiveContextCompletionRepo: objectiveContextCompletionRepo,
		runEventRepo:                   runEventRepo,
		runVariableRepo:                runVariableRepo,
		loader: runStateLoader{
			objectiveRepo:                  objectiveRepo,
			objectiveContextCompletionRepo: objectiveContextCompletionRepo,
			sectionFinishRepo:              sectionFinishRepo,
			blockRepo:                      blockRepo,
			runVariableRepo:                runVariableRepo,
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

	won, err := s.applyAnswer(ctx, tx, &team, block, blockContext, state, data, plan)
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
	input map[string][]string,
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

	setNow, err := s.setVarsWithTx(ctx, tx, team, block, state, plan)
	if err != nil {
		return false, err
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

	answerEvent := answerEventFor(team, block, blockContext, state, input, setNow)
	// The payload carries what the block computed; the column carries what the
	// score moved by.
	answerEvent.Points = credited
	if err := s.runEventRepo.RecordWithTx(ctx, tx, answerEvent); err != nil {
		return false, fmt.Errorf("recording answer: %w", err)
	}

	if completedContext {
		// The completion's column carries no points: the award rode the answer
		// that triggered it, and writing it again there would double the sum.
		if err := s.runEventRepo.RecordWithTx(ctx, tx, objectiveContextCompletedEvent(
			team, block.GetOwnerID(), blockContext, credited,
		)); err != nil {
			return false, fmt.Errorf("recording objective completion: %w", err)
		}
		if plan.completesRun {
			if err := recordRunCompleted(ctx, tx, s.runEventRepo, team); err != nil {
				return false, fmt.Errorf("recording run completion: %w", err)
			}
		}
	}

	return true, nil
}

// answerEventFor builds the event recording one submission. Safe inside the
// transaction: it is pure, reading the state and the input already in hand,
// never the database.
func answerEventFor(
	team *models.Run, block blocks.Block, blockContext game.BlockContext, state blocks.PlayerState,
	input map[string][]string, setNow []string,
) *models.RunEvent {
	// objectiveID is empty for a block the quest owns rather than an objective, so
	// the column never carries a quest ID.
	objectiveID := ""
	if blockContext == game.ContextObjectiveProof || blockContext == game.ContextObjectiveReveal {
		objectiveID = block.GetOwnerID()
	}

	var submitted json.RawMessage
	if reporter, ok := block.(game.SubmissionReporter); ok {
		submitted = reporter.Submission(input, state)
	}

	return answeredEvent(
		team, block, objectiveID, state.IsComplete(), state.GetPointsAwarded(), submitted, setNow,
	)
}

// setVarsWithTx sets what a completed block writes and returns the names it
// set for the first time. It runs under the same gate as the points: a block
// the run could not reach saves its state but earns nothing, and a gate it
// opened would let the player skip whatever stood in front of it.
func (s *CheckInService) setVarsWithTx(
	ctx context.Context, tx *bun.Tx, team *models.Run, block blocks.Block, state blocks.PlayerState,
	plan completionPlan,
) ([]string, error) {
	if !plan.awardPoints {
		return nil, nil
	}
	set, err := s.runVariableRepo.InsertWithTx(ctx, tx, team.Code, block.GetID(), game.VarsSetBy(block, state))
	if err != nil {
		return nil, fmt.Errorf("setting variables: %w", err)
	}
	return set, nil
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

// completionPlan is what a completing submission is entitled to: awardPoints,
// which also sets the block's variables, for a start or finish block, which the
// quest owns and which has no gate to pass, or for a block the run can reach;
// and objective once the context is complete.
type completionPlan struct {
	awardPoints bool
	objective   *models.Objective
	// completesRun is true when this submission's completion is what tips the
	// root over. Derived before the transaction from state already in hand: the
	// post-state is the pre-state plus this one proof, and the derivation is
	// pure, so the answer costs no reads inside the lock.
	completesRun bool
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

	objectives, state, completion, err := s.loader.load(ctx, team)
	if err != nil {
		return completionPlan{}, fmt.Errorf("loading run state: %w", err)
	}
	if !reachableIn(navigation.ComputeFrontier(objectives, state, completion), objective) {
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
		// Only a proof landing changes what the derivation counts: a reveal
		// completing leaves completion exactly where it was.
		if blockContext == game.ContextObjectiveProof {
			state.ProofCompleted[objective.ID] = true
			plan.completesRun = rootsComplete(objectives, navigation.ComputeCompleted(objectives, state))
		}
	}
	return plan, nil
}

// CompleteObjectiveContext logs the completion once every block in the context
// is done. Exported because a content-only context has nothing a player can
// POST to, so it needs a direct caller outside the POST/validate path. Safe to call unconditionally: the idempotency guard
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
	// directly and complete it, opening whatever waits on it out of order.
	objectives, state, complete, err := s.loader.load(ctx, team)
	if err != nil {
		return fmt.Errorf("loading run state: %w", err)
	}
	if !reachableIn(navigation.ComputeFrontier(objectives, state, complete), objective) {
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
		if err := s.recordContextCompleted(
			ctx, tx, team, objective, blockContext, objectives, state,
		); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// recordContextCompleted writes the completion's event and, when the proof is
// what tips the root over, the finish.
func (s *CheckInService) recordContextCompleted(
	ctx context.Context, tx *bun.Tx, team *models.Run, objective *models.Objective,
	blockContext game.BlockContext, objectives []models.Objective, state navigation.RunState,
) error {
	// Content-only contexts complete through this path, where no submission
	// earned anything: zero by definition, not by omission.
	if err := s.runEventRepo.RecordWithTx(ctx, tx, objectiveContextCompletedEvent(
		team, objective.ID, blockContext, 0,
	)); err != nil {
		return fmt.Errorf("recording objective completion: %w", err)
	}

	// Only a proof landing can tip the root; the derivation is pure, so the
	// answer comes from the state already in hand plus this one completion.
	if blockContext != game.ContextObjectiveProof {
		return nil
	}
	state.ProofCompleted[objective.ID] = true
	if !rootsComplete(objectives, navigation.ComputeCompleted(objectives, state)) {
		return nil
	}
	if err := recordRunCompleted(ctx, tx, s.runEventRepo, team); err != nil {
		return fmt.Errorf("recording run completion: %w", err)
	}
	return nil
}

// RunVars returns the variables the run has set, for deciding which content
// blocks it is shown. Preview has no run, so it has set nothing.
func (s *CheckInService) RunVars(ctx context.Context, team *models.Run) (map[string]bool, error) {
	if ctx.Value(contextkeys.PreviewKey) != nil {
		return map[string]bool{}, nil
	}
	names, err := s.runVariableRepo.FindNamesByRunCode(ctx, team.Code)
	if err != nil {
		return nil, fmt.Errorf("loading run variables: %w", err)
	}
	return setOf(names), nil
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
// it, and every variable it waits on is set.
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

	return reachableIn(navigation.ComputeFrontier(objectives, state, complete), objective), nil
}

// reachableIn reports whether the frontier leaves the objective open to the
// run. Complete counts: revisiting somewhere already finished is reading it
// again, not reaching somewhere out of bounds.
func reachableIn(frontier navigation.Frontier, objective *models.Objective) bool {
	switch frontier.StatusOf(objective.ID) {
	case navigation.StatusAvailable, navigation.StatusFinishable, navigation.StatusComplete:
		return true
	case navigation.StatusLocked:
		return false
	}
	return false
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
