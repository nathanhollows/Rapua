package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
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

// TestCheckInService_CompleteObjectiveContext_ContentOnly_CalledDirectly: a
// content-only context has nothing a player can POST to, so this proves a
// direct call, with no prior block validation, logs completion and applies
// sets.
func TestCheckInService_CompleteObjectiveContext_ContentOnly_CalledDirectly(t *testing.T) {
	svc, dbc, cleanup := setupCheckInServiceForObjectives(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	runCode := gofakeit.LetterN(6)
	insertTestTeam(t, dbc, runCode, parents.QuestID)

	objective := &models.Objective{
		ID:         gofakeit.UUID(),
		QuestID:    parents.QuestID,
		Slug:       "flavour-text",
		Title:      "Flavour text",
		RevealSets: game.SetsField{"story_seen": "true"},
	}
	_, err := dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockService := services.NewBlockService(repositories.NewBlockRepository(dbc, blockStateRepo), blockStateRepo)
	// Content block only: RequiresValidation()==false, so no POST ever completes it.
	_, err = blockService.NewBlockWithOwnerAndContext(ctx, objective.ID, blocks.ContextObjectiveReveal, "text")
	require.NoError(t, err)

	team := &models.Run{Code: runCode, QuestID: parents.QuestID}

	// No block validation call anywhere before this: simulates a bare page view.
	err = svc.CompleteObjectiveContext(ctx, team, objective.ID, blocks.ContextObjectiveReveal)
	require.NoError(t, err)

	count, err := dbc.NewSelect().
		Model((*models.ObjectiveContextCompletion)(nil)).
		Where("objective_id = ? AND context = ?", objective.ID, blocks.ContextObjectiveReveal).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "viewing a content-only context must be enough to complete it")

	vars, err := repositories.NewRunVarStateRepository(dbc).GetAll(ctx, runCode, parents.QuestID)
	require.NoError(t, err)
	assert.Equal(t, "true", vars["story_seen"])

	// Calling it again (e.g. a second page view) must not duplicate the log row or re-apply sets.
	err = svc.CompleteObjectiveContext(ctx, team, objective.ID, blocks.ContextObjectiveReveal)
	require.NoError(t, err)
	count, err = dbc.NewSelect().
		Model((*models.ObjectiveContextCompletion)(nil)).
		Where("objective_id = ? AND context = ?", objective.ID, blocks.ContextObjectiveReveal).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "repeat views must not duplicate the completion log")
}

// TestCheckInService_IsObjectiveContextPending exercises the objective view
// handler's decision path: pending while a required block is unvalidated,
// no longer pending once it is.
// TestCheckInService_CompleteObjectiveContext_PreviewIsNoOp proves the preview
// guard: a preview run's code ("preview") has no matching row in runs, so
// logging completion for it would violate objective_context_completions'
// run_code FK. Deliberately does not insert a runs row for "preview", so a
// missing guard would surface as an FK constraint error here.
func TestCheckInService_CompleteObjectiveContext_PreviewIsNoOp(t *testing.T) {
	svc, dbc, cleanup := setupCheckInServiceForObjectives(t)
	defer cleanup()
	ctx := context.WithValue(context.Background(), contextkeys.PreviewKey, true)

	parents := createTestParents(t, dbc)

	objective := &models.Objective{
		ID:         gofakeit.UUID(),
		QuestID:    parents.QuestID,
		Slug:       "flavour-text",
		Title:      "Flavour text",
		RevealSets: game.SetsField{"story_seen": "true"},
	}
	_, err := dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	team := &models.Run{Code: "preview", QuestID: parents.QuestID}

	err = svc.CompleteObjectiveContext(ctx, team, objective.ID, blocks.ContextObjectiveReveal)
	require.NoError(t, err, "preview must no-op, not attempt to write a completion row")

	count, err := dbc.NewSelect().
		Model((*models.ObjectiveContextCompletion)(nil)).
		Where("objective_id = ?", objective.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count, "preview must not log a completion")

	vars, err := repositories.NewRunVarStateRepository(dbc).GetAll(ctx, team.Code, parents.QuestID)
	require.NoError(t, err)
	assert.NotContains(t, vars, "story_seen", "preview must not apply context sets")
}

func TestCheckInService_IsObjectiveContextPending(t *testing.T) {
	svc, dbc, cleanup := setupCheckInServiceForObjectives(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	runCode := gofakeit.LetterN(6)
	insertTestTeam(t, dbc, runCode, parents.QuestID)

	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: parents.QuestID,
		Slug:    "find-the-key",
		Title:   "Find the key",
	}
	_, err := dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockService := services.NewBlockService(repositories.NewBlockRepository(dbc, blockStateRepo), blockStateRepo)
	block, err := blockService.NewBlockWithOwnerAndContext(ctx, objective.ID, blocks.ContextObjectiveProof, "free_text")
	require.NoError(t, err)

	// Seed the block's state row the way a page view would; see the comment on
	// the two-block test above.
	_, _, err = blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		ctx, objective.ID, runCode, parents.QuestID, blocks.ContextObjectiveProof,
	)
	require.NoError(t, err)

	team := &models.Run{Code: runCode, QuestID: parents.QuestID}

	pending, err := svc.IsObjectiveContextPending(ctx, team, objective.ID, blocks.ContextObjectiveProof)
	require.NoError(t, err)
	assert.True(t, pending, "unvalidated required block: still pending")

	_, _, err = svc.ValidateAndUpdateBlockState(ctx, *team, map[string][]string{
		"block": {block.GetID()}, "response": {"answer"},
	})
	require.NoError(t, err)

	pending, err = svc.IsObjectiveContextPending(ctx, team, objective.ID, blocks.ContextObjectiveProof)
	require.NoError(t, err)
	assert.False(t, pending, "block validated: no longer pending")
}

func TestCheckInService_GetObjectiveByQuestIDAndSlug(t *testing.T) {
	svc, dbc, cleanup := setupCheckInServiceForObjectives(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: parents.QuestID,
		Slug:    "find-the-key",
		Title:   "Find the key",
	}
	_, err := dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	got, err := svc.GetObjectiveByQuestIDAndSlug(ctx, parents.QuestID, "find-the-key")
	require.NoError(t, err)
	assert.Equal(t, objective.ID, got.ID)

	_, err = svc.GetObjectiveByQuestIDAndSlug(ctx, parents.QuestID, "does-not-exist")
	assert.Error(t, err)
}
