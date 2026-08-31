package services

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
)

var (
	ErrAllObjectivesVisited = errors.New("all objectives visited")
	ErrInstanceNotFound     = errors.New("instance not found")
)

type NavigationService struct {
	objectiveRepo                  repositories.ObjectiveRepository
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository
	teamRepo                       repositories.RunRepository
	varStateRepo                   repositories.RunVarStateRepository
	gameStructureService           *GameStructureService
	blockService                   *BlockService
	logger                         *slog.Logger
}

// PlayerObjectiveView contains all data needed to render the /objectives list.
// There is no check-out mechanic for objectives, and no Blocks/BlockStates: the
// list renders objective titles only, not block content.
type PlayerObjectiveView struct {
	Settings models.QuestSettings

	CurrentGroup    *models.GameStructure
	CanAdvanceEarly bool

	NextObjectives []models.Objective
}

// NewNavigationService creates a NavigationService.
func NewNavigationService(
	objectiveRepo repositories.ObjectiveRepository,
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository,
	teamRepo repositories.RunRepository,
	varStateRepo repositories.RunVarStateRepository,
	gameStructureService *GameStructureService,
	blockService *BlockService,
	logger *slog.Logger,
) *NavigationService {
	return &NavigationService{
		objectiveRepo:                  objectiveRepo,
		objectiveContextCompletionRepo: objectiveContextCompletionRepo,
		teamRepo:                       teamRepo,
		varStateRepo:                   varStateRepo,
		gameStructureService:           gameStructureService,
		blockService:                   blockService,
		logger:                         logger,
	}
}

// filterGameStructureForObjectives returns a copy of gs with hidden objective
// IDs stripped from ObjectiveIDs.
func filterGameStructureForObjectives(
	gs models.GameStructure,
	hiddenObjectiveIDs map[string]bool,
) models.GameStructure {
	filtered := gs

	if len(hiddenObjectiveIDs) > 0 && len(gs.ObjectiveIDs) > 0 {
		filteredIDs := make([]string, 0, len(gs.ObjectiveIDs))
		for _, id := range gs.ObjectiveIDs {
			if !hiddenObjectiveIDs[id] {
				filteredIDs = append(filteredIDs, id)
			}
		}
		filtered.ObjectiveIDs = filteredIDs
	}

	filtered.SubGroups = nil
	for _, sub := range gs.SubGroups {
		filtered.SubGroups = append(
			filtered.SubGroups, filterGameStructureForObjectives(sub, hiddenObjectiveIDs),
		)
	}
	return filtered
}

// filterObjectivesByDepends returns objs with any entry whose depends list is
// unmet removed.
func filterObjectivesByDepends(objs []models.Objective, resolver game.VarResolver) []models.Objective {
	out := make([]models.Objective, 0, len(objs))
	for _, obj := range objs {
		if game.EvaluateDepends(obj.Depends, resolver) {
			out = append(out, obj)
		}
	}
	return out
}

// GetPlayerObjectiveView returns a complete view of navigation data for the player objectives UI.
func (s *NavigationService) GetPlayerObjectiveView(
	ctx context.Context,
	team *models.Run,
) (*PlayerObjectiveView, error) {
	if err := s.ensureObjectiveTeamRelationsLoaded(ctx, team); err != nil {
		return nil, fmt.Errorf("loading team relations: %w", err)
	}

	view := &PlayerObjectiveView{
		Settings: team.Quest.Settings,
	}

	objectives, err := s.objectiveRepo.FindByQuestID(ctx, team.QuestID)
	if err != nil {
		return nil, fmt.Errorf("loading quest objectives: %w", err)
	}

	// Loaded before the resolver rather than inside the game-structure branch
	// below: a depends list can name objective.<slug>, so completion facts are
	// an input to reachability, not just to the current-group walk.
	completedIDs, err := s.getCompletedObjectiveIDs(ctx, team.Code)
	if err != nil {
		return nil, fmt.Errorf("getting completed objective ids: %w", err)
	}
	resolver := NewPlayerVarResolver(team.VarStates, completedObjectiveSlugs(objectives, completedIDs))

	hiddenObjectiveIDs := make(map[string]bool)
	for _, obj := range objectives {
		if !game.EvaluateDepends(obj.Depends, resolver) {
			hiddenObjectiveIDs[obj.ID] = true
		}
	}

	// Build a local copy of the team whose Quest.GameStructure has been filtered
	// for reachability, so the caller's team is never modified.
	navTeam := team
	if team.Quest.GameStructure.ID != "" {
		navInstance := team.Quest
		navInstance.GameStructure = filterGameStructureForObjectives(
			team.Quest.GameStructure, hiddenObjectiveIDs,
		)
		navTeam = &models.Run{}
		*navTeam = *team
		navTeam.Quest = navInstance
	}

	var currentGroup *models.GameStructure
	var objectiveIDs []string
	if navTeam.Quest.GameStructure.ID != "" {
		currentGroupID := navigation.ComputeCurrentGroupForObjectives(
			&navTeam.Quest.GameStructure,
			completedIDs,
			navTeam.SkippedGroupIDs,
		)

		if currentGroupID != "" {
			currentGroup = navigation.FindGroupByID(&navTeam.Quest.GameStructure, currentGroupID)
		}
		view.CurrentGroup = currentGroup

		view.CanAdvanceEarly = s.computeCanAdvanceEarlyForObjectives(currentGroup, completedIDs)

		// Same navTeam (depends-filtered) and completedIDs used above, not a second
		// unfiltered fetch: see getValidObjectiveIDsFromGameStructure's doc comment.
		objectiveIDs, err = s.getValidObjectiveIDsFromGameStructure(navTeam, completedIDs)
		if err != nil {
			return nil, err
		}
	}

	byID := make(map[string]models.Objective, len(objectives))
	for _, obj := range objectives {
		byID[obj.ID] = obj
	}
	nextObjectives := make([]models.Objective, 0, len(objectiveIDs))
	for _, id := range objectiveIDs {
		if obj, ok := byID[id]; ok {
			nextObjectives = append(nextObjectives, obj)
		}
	}
	view.NextObjectives = filterObjectivesByDepends(nextObjectives, resolver)

	return view, nil
}

// ensureObjectiveTeamRelationsLoaded loads the quest, messages, and fresh var
// states for an objective-built quest: the player handlers render team.Messages
// for the notification banner, and var states must be fresh for depends evaluation.
func (s *NavigationService) ensureObjectiveTeamRelationsLoaded(ctx context.Context, team *models.Run) error {
	if team.Quest.ID == "" {
		if err := s.teamRepo.LoadQuest(ctx, team); err != nil {
			return err
		}
	}
	if err := s.teamRepo.LoadMessages(ctx, team); err != nil {
		return fmt.Errorf("loading messages: %w", err)
	}
	varStates, err := s.varStateRepo.GetAll(ctx, team.Code, team.QuestID)
	if err != nil {
		return fmt.Errorf("loading var states: %w", err)
	}
	team.VarStates = varStates
	return nil
}

// computeCanAdvanceEarlyForObjectives returns true when the team has met the group minimum
// but not all objectives are complete, and AutoAdvance is disabled.
func (s *NavigationService) computeCanAdvanceEarlyForObjectives(group *models.GameStructure, completedIDs []string) bool {
	if group == nil || group.AutoAdvance || len(group.ObjectiveIDs) == 0 {
		return false
	}
	completedSet := make(map[string]bool, len(completedIDs))
	for _, id := range completedIDs {
		completedSet[id] = true
	}
	completedCount := 0
	for _, objID := range group.ObjectiveIDs {
		if completedSet[objID] {
			completedCount++
		}
	}
	allComplete := completedCount == len(group.ObjectiveIDs)
	var isMinimumMet bool
	switch group.CompletionType {
	case models.CompletionAll:
		isMinimumMet = allComplete
	case models.CompletionMinimum:
		isMinimumMet = completedCount >= group.MinimumRequired
	}
	return isMinimumMet && !allComplete
}

// getCompletedObjectiveIDs returns the completed objective IDs for a run: since
// objective completion has no check-in step, the raw ingredient comes from the
// append-only completion log.
func (s *NavigationService) getCompletedObjectiveIDs(ctx context.Context, runCode string) ([]string, error) {
	return s.objectiveContextCompletionRepo.FindCompletedObjectiveIDs(ctx, runCode, game.ContextObjectiveReveal)
}

// getValidObjectiveIDsFromGameStructure returns objective IDs rather than
// hydrated objects: hydration is a single batch lookup, done once by the caller.
// completedIDs is passed in rather than fetched here: the caller already needs
// it for CurrentGroup/CanAdvanceEarly, and this must run against the same
// when-filtered structure the caller used for those, not a second unfiltered
// fetch. Using two different structures for the two halves of one view is
// how a hidden current-group objective went missing from NextObjectives while
// CurrentGroup had already moved past it.
func (s *NavigationService) getValidObjectiveIDsFromGameStructure(
	team *models.Run,
	completedIDs []string,
) ([]string, error) {
	currentGroupID := navigation.ComputeCurrentGroupForObjectives(
		&team.Quest.GameStructure, completedIDs, team.SkippedGroupIDs,
	)
	if currentGroupID == "" {
		return []string{}, nil
	}

	objectiveIDs := navigation.GetAvailableObjectiveIDs(
		&team.Quest.GameStructure,
		currentGroupID,
		completedIDs,
		team.Code,
	)

	if len(objectiveIDs) == 0 {
		_, shouldAdvance, _ := navigation.GetNextGroup(&team.Quest.GameStructure, currentGroupID, completedIDs)
		if !shouldAdvance {
			return []string{}, ErrAllObjectivesVisited
		}
		return []string{}, nil
	}

	return objectiveIDs, nil
}

// GetPreviewObjectiveView creates a simplified navigation view showing only
// the specified objective within its containing group for preview mode.
// No block loading: the /objectives list renders the objective title only.
func (s *NavigationService) GetPreviewObjectiveView(
	ctx context.Context,
	team *models.Run,
	objectiveID string,
) (*PlayerObjectiveView, error) {
	if err := s.ensureObjectiveTeamRelationsLoaded(ctx, team); err != nil {
		return nil, fmt.Errorf("loading team relations: %w", err)
	}

	group := navigation.FindGroupContainingObjective(&team.Quest.GameStructure, objectiveID)
	if group == nil {
		return nil, errors.New("objective not found in game structure")
	}

	objective, err := s.objectiveRepo.GetByID(ctx, objectiveID)
	if err != nil {
		return nil, fmt.Errorf("loading objective: %w", err)
	}

	view := &PlayerObjectiveView{
		Settings:        team.Quest.Settings,
		CurrentGroup:    group,
		NextObjectives:  []models.Objective{*objective},
		CanAdvanceEarly: false,
	}

	return view, nil
}
