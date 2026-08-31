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
)

type CheckInService struct {
	teamRepo                       repositories.RunRepository
	blockService                   *BlockService
	varStateRepo                   repositories.RunVarStateRepository
	objectiveRepo                  repositories.ObjectiveRepository
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository
}

func NewCheckInService(
	teamRepo repositories.RunRepository,
	blockService *BlockService,
	varStateRepo repositories.RunVarStateRepository,
	objectiveRepo repositories.ObjectiveRepository,
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository,
) *CheckInService {
	return &CheckInService{
		teamRepo:                       teamRepo,
		blockService:                   blockService,
		varStateRepo:                   varStateRepo,
		objectiveRepo:                  objectiveRepo,
		objectiveContextCompletionRepo: objectiveContextCompletionRepo,
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
		if err = s.writeSetsVars(ctx, team, block, state); err != nil {
			return nil, nil, err
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

func (s *CheckInService) writeSetsVars(
	ctx context.Context,
	team models.Run,
	block blocks.Block,
	state blocks.PlayerState,
) error {
	if setter, ok := block.(blocks.ChoiceVarSetter); ok {
		// GetTriggeredVars already filters to the options the player chose, so
		// every returned value is written: matching the GetSets path below.
		for _, varName := range setter.GetTriggeredVars(state) {
			if game.IsReservedVarName(varName) {
				continue
			}
			if err := s.varStateRepo.Upsert(ctx, team.Code, team.QuestID, varName, game.SetsValueTrue); err != nil {
				return fmt.Errorf("writing sets var %q: %w", varName, err)
			}
		}
		return nil
	}
	if state.IsComplete() {
		for _, varName := range block.GetSets() {
			if game.IsReservedVarName(varName) {
				continue
			}
			if err := s.varStateRepo.Upsert(ctx, team.Code, team.QuestID, varName, game.SetsValueTrue); err != nil {
				return fmt.Errorf("writing sets var %q: %w", varName, err)
			}
		}
	}
	return nil
}

// awardPointsAndComplete awards points and, for objective contexts, logs
// completion once every block in the context is done.
func (s *CheckInService) awardPointsAndComplete(
	ctx context.Context, team *models.Run, block blocks.Block, blockContext game.BlockContext,
) error {
	team.Points += block.GetPoints()
	if err := s.teamRepo.Update(ctx, team); err != nil {
		return fmt.Errorf("awarding points: %w", err)
	}

	if blockContext == game.ContextObjectiveProof || blockContext == game.ContextObjectiveReveal {
		return s.CompleteObjectiveContext(ctx, team, block.GetOwnerID(), blockContext)
	}

	return nil
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

	inserted, err := s.objectiveContextCompletionRepo.Insert(ctx, team.Code, objectiveID, blockContext)
	if err != nil {
		return fmt.Errorf("logging objective context completion: %w", err)
	}
	if !inserted {
		// Already logged by an earlier call; its sets were applied then.
		return nil
	}

	sets := objective.ProofSets
	if blockContext == game.ContextObjectiveReveal {
		sets = objective.RevealSets
	}
	for _, varName := range sets {
		if game.IsReservedVarName(varName) {
			continue
		}
		if err := s.varStateRepo.Upsert(ctx, team.Code, team.QuestID, varName, game.SetsValueTrue); err != nil {
			return fmt.Errorf("writing context sets var %q: %w", varName, err)
		}
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

// ObjectiveIsReachable reports whether a run has met an objective's depends
// list. An objective with no depends is always reachable and costs no queries,
// which is the overwhelmingly common case.
//
// This duplicates what the navigation service computes for the objectives
// list, because a list that merely hides an objective is not a gate: a guessed
// slug reaches it anyway.
func (s *CheckInService) ObjectiveIsReachable(
	ctx context.Context, team *models.Run, objective *models.Objective,
) (bool, error) {
	if len(objective.Depends) == 0 {
		return true, nil
	}

	varStates, err := s.varStateRepo.GetAll(ctx, team.Code, team.QuestID)
	if err != nil {
		return false, fmt.Errorf("loading var states: %w", err)
	}
	completedIDs, err := s.objectiveContextCompletionRepo.FindCompletedObjectiveIDs(
		ctx, team.Code, game.ContextObjectiveReveal,
	)
	if err != nil {
		return false, fmt.Errorf("loading completed objectives: %w", err)
	}
	objectives, err := s.objectiveRepo.FindByQuestID(ctx, team.QuestID)
	if err != nil {
		return false, fmt.Errorf("loading quest objectives: %w", err)
	}

	resolver := NewPlayerVarResolver(varStates, completedObjectiveSlugs(objectives, completedIDs))
	return game.EvaluateDepends(objective.Depends, resolver), nil
}

func (s *CheckInService) GetObjectiveByQuestIDAndSlug(
	ctx context.Context, questID, slug string,
) (*models.Objective, error) {
	return s.objectiveRepo.GetByQuestIDAndSlug(ctx, questID, slug)
}
