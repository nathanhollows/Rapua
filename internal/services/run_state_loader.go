package services

import (
	"context"
	"fmt"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
)

// runStateLoader gathers a quest's objectives and the facts one run has
// recorded against them. Every consumer of "is this complete" reads from here,
// so completion has one definition: the band derivation in navigation. Answers
// drawn from the completion log alone disagree with it, because a section
// completes through its children and never earns a row of its own.
type runStateLoader struct {
	objectiveRepo                  repositories.ObjectiveRepository
	objectiveContextCompletionRepo repositories.ObjectiveContextCompletionRepository
	sectionFinishRepo              repositories.SectionFinishRepository
	blockRepo                      repositories.BlockRepository
	varStateRepo                   repositories.RunVarStateRepository
}

// load returns a quest's objectives and one run's state against them.
//
// The resolver is built in two steps because a depends list can name a section
// by slug, and a section has no completion row of its own: completion has to be
// derived before anything can answer whether objective.<slug> is true. Deriving
// it does not read the resolver, so the order is well founded.
func (l runStateLoader) load(
	ctx context.Context, team *models.Run,
) ([]models.Objective, navigation.RunState, error) {
	objectives, err := l.objectiveRepo.FindTreeByQuestID(ctx, team.QuestID)
	if err != nil {
		return nil, navigation.RunState{}, fmt.Errorf("loading objective tree: %w", err)
	}

	state := navigation.RunState{RunCode: team.Code}

	state.HasProofBlocks, err = l.proofBlockOwners(ctx, objectives)
	if err != nil {
		return nil, navigation.RunState{}, err
	}

	proofCompleted, err := l.objectiveContextCompletionRepo.
		FindCompletedObjectiveIDs(ctx, team.Code, game.ContextObjectiveProof)
	if err != nil {
		return nil, navigation.RunState{}, fmt.Errorf("loading completed proof contexts: %w", err)
	}
	state.ProofCompleted = setOf(proofCompleted)

	finished, err := l.sectionFinishRepo.FindFinishedObjectiveIDs(ctx, team.Code)
	if err != nil {
		return nil, navigation.RunState{}, fmt.Errorf("loading finished sections: %w", err)
	}
	state.SectionFinished = setOf(finished)

	varStates, err := l.varStateRepo.GetAll(ctx, team.Code, team.QuestID)
	if err != nil {
		return nil, navigation.RunState{}, fmt.Errorf("loading var states: %w", err)
	}

	completed := navigation.ComputeCompleted(objectives, state)
	state.Vars = NewPlayerVarResolver(varStates, completedSlugsFrom(objectives, completed))

	return objectives, state, nil
}

// proofBlockOwners returns the objectives with at least one proof block. An
// objective with none has nothing to prove.
func (l runStateLoader) proofBlockOwners(
	ctx context.Context, objectives []models.Objective,
) (map[string]bool, error) {
	ownerIDs := make([]string, 0, len(objectives))
	for _, obj := range objectives {
		ownerIDs = append(ownerIDs, obj.ID)
	}

	owners, err := l.blockRepo.FindOwnerIDsWithContext(ctx, ownerIDs, game.ContextObjectiveProof)
	if err != nil {
		return nil, fmt.Errorf("loading proof block owners: %w", err)
	}
	return setOf(owners), nil
}

// completedSlugsFrom names the completed objectives by slug, which is how a
// depends list refers to them.
func completedSlugsFrom(objectives []models.Objective, completed map[string]bool) map[string]bool {
	slugs := make(map[string]bool, len(completed))
	for _, obj := range objectives {
		if completed[obj.ID] && obj.Slug != "" {
			slugs[obj.Slug] = true
		}
	}
	return slugs
}

func setOf(ids []string) map[string]bool {
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}
