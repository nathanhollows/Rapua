package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupCheckInServiceForObjectives wires a real CheckInService against an in-memory
// DB. locationStatsService and navigationService are nil: CompleteObjectiveContext
// never touches them, only CheckIn/CheckOut do.
func setupCheckInServiceForObjectives(t *testing.T) (*services.CheckInService, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	blockService := services.NewBlockService(blockRepo, blockStateRepo)

	svc := services.NewCheckInService(
		repositories.NewCheckInRepository(dbc),
		repositories.NewLocationRepository(dbc),
		repositories.NewRunRepository(dbc),
		nil, nil,
		blockService,
		repositories.NewRunVarStateRepository(dbc),
		repositories.NewObjectiveRepository(dbc),
		repositories.NewObjectiveContextCompletionRepository(dbc),
	)
	return svc, dbc, cleanup
}

// TestCheckInService_ObjectiveProofContext_CompletesOnceAllBlocksDone exercises the
// full path: two proof blocks under one objective, validating each in turn. The
// context should only be logged complete, and its sets only applied, once both
// blocks are done: not after the first.
func TestCheckInService_ObjectiveProofContext_CompletesOnceAllBlocksDone(t *testing.T) {
	svc, dbc, cleanup := setupCheckInServiceForObjectives(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	runCode := gofakeit.LetterN(6)
	insertTestTeam(t, dbc, runCode, parents.QuestID)

	objective := &models.Objective{
		ID:        gofakeit.UUID(),
		QuestID:   parents.QuestID,
		Slug:      "find-the-key",
		Title:     "Find the key",
		ProofSets: game.SetsField{"door_unlocked": "true"},
	}
	_, err := dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockService := services.NewBlockService(repositories.NewBlockRepository(dbc, blockStateRepo), blockStateRepo)
	block1, err := blockService.NewBlockWithOwnerAndContext(ctx, objective.ID, blocks.ContextObjectiveProof, "free_text")
	require.NoError(t, err)
	block2, err := blockService.NewBlockWithOwnerAndContext(ctx, objective.ID, blocks.ContextObjectiveProof, "free_text")
	require.NoError(t, err)

	// Seed block state rows the way a page view would (FindByOwnerIDAndRunCodeWithStateAndContext
	// -> populateMissingStates): ValidateAndUpdateBlockState only UPDATEs an existing row, it
	// does not create one, so a block touched for the first time needs this first.
	_, _, err = blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		ctx, objective.ID, runCode, parents.QuestID, blocks.ContextObjectiveProof,
	)
	require.NoError(t, err)

	team := models.Run{Code: runCode, QuestID: parents.QuestID}
	varStateRepo := repositories.NewRunVarStateRepository(dbc)

	countCompletions := func() int {
		count, cErr := dbc.NewSelect().
			Model((*models.ObjectiveContextCompletion)(nil)).
			Where("objective_id = ?", objective.ID).
			Count(ctx)
		require.NoError(t, cErr)
		return count
	}

	// First block only: context must not be complete, sets must not fire yet.
	_, _, err = svc.ValidateAndUpdateBlockState(ctx, team, map[string][]string{
		"block": {block1.GetID()}, "response": {"answer one"},
	})
	require.NoError(t, err)
	assert.Equal(t, 0, countCompletions(), "context should not complete with only one of two blocks done")
	vars, err := varStateRepo.GetAll(ctx, runCode, parents.QuestID)
	require.NoError(t, err)
	assert.NotContains(t, vars, "door_unlocked")

	// Second block completes the context: log it, and apply proof sets.
	_, _, err = svc.ValidateAndUpdateBlockState(ctx, team, map[string][]string{
		"block": {block2.GetID()}, "response": {"answer two"},
	})
	require.NoError(t, err)
	assert.Equal(t, 1, countCompletions(), "context should complete exactly once, all blocks now done")
	vars, err = varStateRepo.GetAll(ctx, runCode, parents.QuestID)
	require.NoError(t, err)
	assert.Equal(t, "true", vars["door_unlocked"])
}

// TestCheckInService_ObjectiveContext_NoSetsDefined_StillLogsCompletion asserts that
// completion logging is unconditional: it happens even when the context defines no
// sets at all, per the system-triggered/admin-authored split.
func TestCheckInService_ObjectiveContext_NoSetsDefined_StillLogsCompletion(t *testing.T) {
	svc, dbc, cleanup := setupCheckInServiceForObjectives(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	runCode := gofakeit.LetterN(6)
	insertTestTeam(t, dbc, runCode, parents.QuestID)

	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: parents.QuestID,
		Slug:    "no-sets-here",
		Title:   "No sets here",
	}
	_, err := dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockService := services.NewBlockService(repositories.NewBlockRepository(dbc, blockStateRepo), blockStateRepo)
	block, err := blockService.NewBlockWithOwnerAndContext(ctx, objective.ID, blocks.ContextObjectiveReveal, "free_text")
	require.NoError(t, err)

	// Seed the block's state row the way a page view would; see the comment in the
	// two-block test above.
	_, _, err = blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		ctx, objective.ID, runCode, parents.QuestID, blocks.ContextObjectiveReveal,
	)
	require.NoError(t, err)

	team := models.Run{Code: runCode, QuestID: parents.QuestID}

	_, _, err = svc.ValidateAndUpdateBlockState(ctx, team, map[string][]string{
		"block": {block.GetID()}, "response": {"answer"},
	})
	require.NoError(t, err)

	count, err := dbc.NewSelect().
		Model((*models.ObjectiveContextCompletion)(nil)).
		Where("objective_id = ? AND context = ?", objective.ID, blocks.ContextObjectiveReveal).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "completion logs regardless of whether the context defines any sets")
}
