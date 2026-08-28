package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
)

type LocationStatsService interface {
	IncrementVisitors(ctx context.Context, location *models.Location) error
	DecrementVisitors(ctx context.Context, location *models.Location) error
}

type CheckInService struct {
	checkInRepo                    repositories.CheckInRepository
	locationRepo                   repositories.LocationRepository
	teamRepo                       repositories.RunRepository
	blockService                   *BlockService
	locationStatsService           LocationStatsService
	navigationService              *NavigationService
	varStateRepo                   repositories.RunVarStateRepository
	objectiveRepo                  repositories.ObjectiveRepository
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository
}

func NewCheckInService(
	checkInRepo repositories.CheckInRepository,
	locationRepo repositories.LocationRepository,
	teamRepo repositories.RunRepository,
	locationStatsService LocationStatsService,
	navigationService *NavigationService,
	blockService *BlockService,
	varStateRepo repositories.RunVarStateRepository,
	objectiveRepo repositories.ObjectiveRepository,
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository,
) *CheckInService {
	return &CheckInService{
		checkInRepo:                    checkInRepo,
		locationRepo:                   locationRepo,
		teamRepo:                       teamRepo,
		locationStatsService:           locationStatsService,
		navigationService:              navigationService,
		blockService:                   blockService,
		varStateRepo:                   varStateRepo,
		objectiveRepo:                  objectiveRepo,
		objectiveContextCompletionRepo: objectiveContextCompletionRepo,
	}
}

func (s *CheckInService) CheckOut(ctx context.Context, team *models.Run, locationCode string) error {
	location, err := s.locationRepo.GetByInstanceAndCode(ctx, team.QuestID, locationCode)
	if err != nil {
		return fmt.Errorf("%w: finding location: %w", ErrLocationNotFound, err)
	}

	err = s.teamRepo.LoadRelations(ctx, team)
	if err != nil {
		return fmt.Errorf("loading relations: %w", err)
	}

	// Check if the team must scan out
	if team.MustCheckOut == "" {
		return ErrUnecessaryCheckOut
	} else if team.MustCheckOut != location.ID {
		return ErrCheckOutAtWrongLocation
	}

	// Check if all visible blocks are completed (reload var states so when-clauses are current).
	varStates, err := s.varStateRepo.GetAll(ctx, team.Code, team.QuestID)
	if err != nil {
		return fmt.Errorf("loading var states: %w", err)
	}
	resolver := NewPlayerVarResolver(team, varStates)
	unfinishedCheckIn, err := s.blockService.checkValidationRequiredForCheckIn(
		ctx, location.ID, team.Code, team.QuestID, game.ContextLocationContent, resolver,
	)
	if err != nil {
		return fmt.Errorf("checking if validation is required: %w", err)
	}

	if unfinishedCheckIn {
		return ErrUnfinishedCheckIn
	}

	// Award base points on checkout completion
	team.Points += location.Points

	// Log the scan out and get the updated CheckIn record
	checkIn, err := s.checkOut(ctx, team, location)
	if err != nil {
		return fmt.Errorf("logging scan out: %w", err)
	}

	// Update the CheckIn record to include the base points
	// This ensures the CheckIn record shows the total points earned from this location
	checkIn.Points += location.Points
	err = s.checkInRepo.Update(ctx, &checkIn)
	if err != nil {
		return fmt.Errorf("updating check in points: %w", err)
	}

	// Update team with the awarded points
	err = s.teamRepo.Update(ctx, team)
	if err != nil {
		return fmt.Errorf("updating team points: %w", err)
	}

	return nil
}

func (s *CheckInService) CompleteBlocks(ctx context.Context, runCode string, locationID string) error {
	// sql.ErrNoRows is passed through rather than absorbed: whether a missing
	// check-in is expected depends on the block's context, which the caller knows
	// and this does not.
	checkIn, err := s.checkInRepo.FindCheckInByTeamAndLocation(ctx, runCode, locationID)
	if err != nil {
		return fmt.Errorf("finding check in: %w", err)
	}

	// If the check in is already complete, return early
	if checkIn.BlocksCompleted {
		return nil
	}

	checkIn.BlocksCompleted = true
	err = s.checkInRepo.Update(ctx, checkIn)
	if err != nil {
		return fmt.Errorf("updating check in: %w", err)
	}

	return nil
}

// CheckOut logs a check out for a team at a location.
func (s *CheckInService) checkOut(
	ctx context.Context,
	team *models.Run,
	location *models.Location,
) (models.CheckIn, error) {
	scan, err := s.checkInRepo.LogCheckOut(ctx, team, location)
	if err != nil {
		return models.CheckIn{}, fmt.Errorf("checking out: %w", err)
	}

	// Update location statistics
	// TotalVisits was already incremented on check-in, so we need to account for completed visits
	// completedVisitsBefore = TotalVisits - CurrentCount (teams still checked in)
	// newAverage = (oldAverage * completedVisitsBefore + newDuration) / (completedVisitsBefore + 1)
	completedVisitsBefore := location.TotalVisits - location.CurrentCount
	location.AvgDuration =
		(location.AvgDuration*float64(completedVisitsBefore) +
			scan.TimeOut.Sub(scan.TimeIn).Seconds()) /
			float64(completedVisitsBefore+1)
	location.CurrentCount--
	err = s.locationRepo.Update(ctx, location)
	if err != nil {
		return models.CheckIn{}, fmt.Errorf("updating location: %w", err)
	}

	// Update team
	team.MustCheckOut = ""
	err = s.teamRepo.Update(ctx, team)
	if err != nil {
		return models.CheckIn{}, fmt.Errorf("updating team: %w", err)
	}

	return scan, nil
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

	// Check if we're in preview mode - preview mode should use fresh mock state
	isPreview := ctx.Value(contextkeys.PreviewKey) != nil

	var block blocks.Block
	var state blocks.PlayerState
	var err error

	if isPreview {
		// In preview mode, always get a fresh block and create a new mock state
		block, err = s.blockService.GetByBlockID(ctx, blockID)
		if err != nil {
			return nil, nil, fmt.Errorf("getting block in preview mode: %w", err)
		}

		state, err = s.blockService.NewMockBlockState(ctx, blockID, team.Code, team.QuestID)
		if err != nil {
			return nil, nil, fmt.Errorf("creating mock state in preview mode: %w", err)
		}
	} else {
		// In regular mode, get the existing block and state
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

	// In regular mode, return early if already complete to prevent duplicate points
	// Preview mode always uses fresh state so this check is not needed
	if !isPreview && state.IsComplete() {
		return state, block, nil
	}

	// Validate the block
	state, err = block.ValidatePlayerInput(state, data)
	if err != nil {
		return nil, nil, fmt.Errorf("validating block: %w", err)
	}

	// Only persist state changes in regular mode, not in preview mode
	if !isPreview {
		state, err = s.blockService.UpdateState(ctx, state)
		if err != nil {
			return nil, nil, fmt.Errorf("updating block state: %w", err)
		}
		if err = s.writeSetsVars(ctx, team, block, state); err != nil {
			return nil, nil, err
		}
	}

	// Only award points and update check-ins in regular mode, not preview mode
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

// writeSetsVars writes block sets variables to the var-state store after block completion.
func (s *CheckInService) writeSetsVars(
	ctx context.Context,
	team models.Run,
	block blocks.Block,
	state blocks.PlayerState,
) error {
	if setter, ok := block.(blocks.ChoiceVarSetter); ok {
		// GetTriggeredVars already filters to the options the player chose, so
		// every returned value is written: matching the GetSets path below.
		for varName, val := range setter.GetTriggeredVars(state) {
			if game.IsReservedVarName(varName) {
				continue
			}
			if err := s.varStateRepo.Upsert(ctx, team.Code, team.QuestID, varName, val); err != nil {
				return fmt.Errorf("writing sets var %q: %w", varName, err)
			}
		}
		return nil
	}
	if state.IsComplete() {
		for varName, val := range block.GetSets() {
			if game.IsReservedVarName(varName) {
				continue
			}
			if err := s.varStateRepo.Upsert(ctx, team.Code, team.QuestID, varName, val); err != nil {
				return fmt.Errorf("writing sets var %q: %w", varName, err)
			}
		}
	}
	return nil
}

// awardPointsAndComplete awards points, then dispatches to whichever owner-kind's completion
// logic applies: objective context completion for proof/reveal blocks, or the existing
// location check-in completion for everything else.
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

	varStates, err := s.varStateRepo.GetAll(ctx, team.Code, team.QuestID)
	if err != nil {
		return fmt.Errorf("loading var states: %w", err)
	}
	blockResolver := NewPlayerVarResolver(team, varStates)
	unfinished, err := s.blockService.checkValidationRequiredForCheckIn(
		ctx, block.GetOwnerID(), team.Code, team.QuestID, game.ContextLocationContent, blockResolver,
	)
	if err != nil {
		return fmt.Errorf("checking if validation is required: %w", err)
	}
	if !unfinished {
		// A navigation block completes before the team reaches the location it
		// belongs to, so there is no check-in row to mark yet. Anything else
		// missing one is a real fault.
		if err = s.CompleteBlocks(ctx, team.Code, block.GetOwnerID()); err != nil &&
			!errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("completing blocks: %w", err)
		}
	}
	return nil
}

// CompleteObjectiveContext logs the completion and applies the context's sets
// once every block in the context is done. Logging is unconditional even when
// the context defines no sets. Exported because a content-only context has
// nothing a player can POST to, so it needs a direct caller outside the
// POST/validate path. Safe to call unconditionally: the idempotency guard
// makes a context that is not done, or already logged, a harmless no-op.
func (s *CheckInService) CompleteObjectiveContext(
	ctx context.Context, team *models.Run, objectiveID string, blockContext game.BlockContext,
) error {
	resolver, err := s.newTeamResolver(ctx, team)
	if err != nil {
		return err
	}

	stillRequired, err := s.blockService.checkValidationRequiredForCheckIn(
		ctx, objectiveID, team.Code, team.QuestID, blockContext, resolver,
	)
	if err != nil {
		return fmt.Errorf("checking if objective context is complete: %w", err)
	}
	if stillRequired {
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

	objective, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return fmt.Errorf("loading objective: %w", err)
	}
	sets := objective.ProofSets
	if blockContext == game.ContextObjectiveReveal {
		sets = objective.RevealSets
	}
	for varName, val := range sets {
		if game.IsReservedVarName(varName) {
			continue
		}
		if err := s.varStateRepo.Upsert(ctx, team.Code, team.QuestID, varName, val); err != nil {
			return fmt.Errorf("writing context sets var %q: %w", varName, err)
		}
	}
	return nil
}

// newTeamResolver: shared by CompleteObjectiveContext and IsObjectiveContextPending
// so both resolve when-clauses against identical, freshly-loaded var state.
func (s *CheckInService) newTeamResolver(ctx context.Context, team *models.Run) (game.VarResolver, error) {
	varStates, err := s.varStateRepo.GetAll(ctx, team.Code, team.QuestID)
	if err != nil {
		return nil, fmt.Errorf("loading var states: %w", err)
	}
	return NewPlayerVarResolver(team, varStates), nil
}

// IsObjectiveContextPending reports whether an objective's proof or reveal
// context still has unvalidated required blocks for this team: the same check
// CompleteObjectiveContext uses internally, exposed for callers (the objective
// view handler) that need to decide which zone to render before completing anything.
func (s *CheckInService) IsObjectiveContextPending(
	ctx context.Context, team *models.Run, objectiveID string, blockContext game.BlockContext,
) (bool, error) {
	resolver, err := s.newTeamResolver(ctx, team)
	if err != nil {
		return false, err
	}
	return s.blockService.checkValidationRequiredForCheckIn(
		ctx, objectiveID, team.Code, team.QuestID, blockContext, resolver,
	)
}

func (s *CheckInService) GetObjectiveByQuestIDAndSlug(
	ctx context.Context, questID, slug string,
) (*models.Objective, error) {
	return s.objectiveRepo.GetByQuestIDAndSlug(ctx, questID, slug)
}
