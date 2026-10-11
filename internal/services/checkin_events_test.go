package services_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// failingEventRepo fails every append, which is how a test forces the event write
// to be the thing that goes wrong.
type failingEventRepo struct{ err error }

func (f failingEventRepo) Record(context.Context, *models.RunEvent) error { return f.err }

func (f failingEventRepo) RecordWithTx(context.Context, *bun.Tx, *models.RunEvent) error {
	return f.err
}

func (f failingEventRepo) ListByRun(context.Context, string) ([]models.RunEvent, error) {
	return nil, f.err
}

func (f failingEventRepo) ListByQuestSince(context.Context, string, time.Time) ([]models.RunEvent, error) {
	return nil, f.err
}

func (f failingEventRepo) LatestByType(
	context.Context, string, []models.EventType,
) (map[string]time.Time, error) {
	return nil, f.err
}

func (f failingEventRepo) TimesByType(
	context.Context, string, []models.EventType, time.Time,
) (map[string][]time.Time, error) {
	return nil, f.err
}

func (f failingEventRepo) CompletedSinceResetWithTx(context.Context, *bun.Tx, string) (bool, error) {
	return false, f.err
}

// completingSubmission builds an objective with one proof block, a run, and the
// seeded state rows a page view would leave behind, then returns a check-in
// service and the wherewithal to submit an answer that completes it.
type completingSubmission struct {
	svc          *services.CheckInService
	blockService *services.BlockService
	team         models.Run
	objective    *models.Objective
	block        blocks.Block
	dbc          *bun.DB
}

func setupCompletingSubmission(
	t *testing.T, blockType string, makeEventRepo func(*bun.DB) repositories.RunEventRepository,
) completingSubmission {
	t.Helper()
	dbc, cleanup := setupDB(t)
	t.Cleanup(cleanup)
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
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	blockService := services.NewBlockService(blockRepo, blockStateRepo)

	block, err := blockService.NewBlockWithOwnerAndContext(
		ctx, objective.ID, blocks.ContextObjectiveProof, blockType,
	)
	require.NoError(t, err)

	// ValidateAndUpdateBlockState only UPDATEs an existing state row, so the row a
	// page view would have created has to exist first.
	_, _, err = blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		ctx, objective.ID, runCode, parents.QuestID, blocks.ContextObjectiveProof,
	)
	require.NoError(t, err)

	svc := services.NewCheckInService(
		db.NewTransactor(dbc),
		repositories.NewRunRepository(dbc),
		blockService,
		repositories.NewObjectiveRepository(dbc),
		repositories.NewObjectiveContextCompletionRepository(dbc),
		repositories.NewSectionFinishRepository(dbc),
		blockRepo,
		makeEventRepo(dbc),
	)

	return completingSubmission{
		svc:          svc,
		blockService: blockService,
		team:         models.Run{Code: runCode, QuestID: parents.QuestID},
		objective:    objective,
		block:        block,
		dbc:          dbc,
	}
}

// A failed event write takes the state change down with it. Without this the
// block would read as complete, the request would report failure, and a retry
// would take the short-circuit and never record the event: a silent hole behind a
// loud error.
func TestCheckInService_EventFailureRollsBackEveryWrite(t *testing.T) {
	f := setupCompletingSubmission(t, "free_text", func(*bun.DB) repositories.RunEventRepository {
		return failingEventRepo{err: errors.New("event write failed")}
	})
	ctx := context.Background()

	_, _, err := f.svc.ValidateAndUpdateBlockState(ctx, f.team, map[string][]string{
		"block": {f.block.GetID()}, "response": {"the answer"},
	})
	require.Error(t, err, "the request fails when its event cannot be recorded")

	// All four write classes, not just the one that is easiest to check.
	var state models.RunBlockState
	err = f.dbc.NewSelect().Model(&state).
		Where("run_code = ?", f.team.Code).
		Where("block_id = ?", f.block.GetID()).
		Scan(ctx)
	if err == nil {
		assert.False(t, state.IsComplete, "block state rolled back")
	}

	var run models.Run
	require.NoError(t, f.dbc.NewSelect().Model(&run).Where("code = ?", f.team.Code).Scan(ctx))
	assert.Zero(t, run.Points, "points rolled back")

	count, err := f.dbc.NewSelect().
		Model((*models.ObjectiveContextCompletion)(nil)).
		Where("objective_id = ?", f.objective.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count, "completion rolled back")

	events, err := repositories.NewRunEventRepository(f.dbc).ListByRun(ctx, f.team.Code)
	require.NoError(t, err)
	assert.Empty(t, events, "nothing was written to the stream either")
}

// The stream sums to the score: the credit rides the answered row, and the
// completion it triggered carries none.
func TestCheckInService_EventsSumToTheScore(t *testing.T) {
	f := setupCompletingSubmission(t, "free_text", repositories.NewRunEventRepository)
	ctx := context.Background()

	// The default block is worth zero, which would make the invariant vacuous.
	_, err := f.dbc.NewUpdate().
		Model((*models.Block)(nil)).
		Set("points = ?", 10).
		Where("id = ?", f.block.GetID()).
		Exec(ctx)
	require.NoError(t, err)

	_, _, err = f.svc.ValidateAndUpdateBlockState(ctx, f.team, map[string][]string{
		"block": {f.block.GetID()}, "response": {"the answer"},
	})
	require.NoError(t, err)

	var run models.Run
	require.NoError(t, f.dbc.NewSelect().Model(&run).Where("code = ?", f.team.Code).Scan(ctx))
	require.Equal(t, 10, run.Points, "the completion was credited")

	stored, err := repositories.NewRunEventRepository(f.dbc).ListByRun(ctx, f.team.Code)
	require.NoError(t, err)
	require.NotEmpty(t, stored)

	sum := 0
	for _, e := range stored {
		sum += e.Points
	}
	assert.Equal(t, run.Points, sum, "the events sum to the score they describe")

	// The completion's payload carries the closing credit for reporting reads,
	// while its column stays zero, so the sum never doubles.
	var completion *models.RunEvent
	for i := range stored {
		if stored[i].Type == models.EventObjectiveContextCompleted {
			completion = &stored[i]
		}
	}
	require.NotNil(t, completion)
	var payload models.ObjectiveContextCompletedData
	require.NoError(t, json.Unmarshal(completion.Data, &payload))
	assert.Equal(t, 10, payload.Points)
	assert.Zero(t, completion.Points, "the column stays zero, or the sum would double")
}

// A wrong answer that completes a no-retry block earns nothing: the credit is the
// value the block computed, not the configured points the test sets to 100.
func TestCheckInService_CreditsTheComputedValue(t *testing.T) {
	f := setupCompletingSubmission(t, "quiz", repositories.NewRunEventRepository)
	ctx := context.Background()

	_, err := f.blockService.UpdateBlock(ctx, f.block, map[string][]string{
		"points":         {"100"},
		"question":       {"Which one?"},
		"option_text":    {"A boat", "A train"},
		"option_correct": {"option_1"},
	})
	require.NoError(t, err)

	// option_0 is wrong, and with retries off the submission completes anyway.
	_, _, err = f.svc.ValidateAndUpdateBlockState(ctx, f.team, map[string][]string{
		"block": {f.block.GetID()}, "quiz_option": {"option_0"},
	})
	require.NoError(t, err)

	var run models.Run
	require.NoError(t, f.dbc.NewSelect().Model(&run).Where("code = ?", f.team.Code).Scan(ctx))
	assert.Zero(t, run.Points, "a wrong answer earns nothing, not the configured points")

	stored, err := repositories.NewRunEventRepository(f.dbc).ListByRun(ctx, f.team.Code)
	require.NoError(t, err)

	var answered *models.RunEvent
	for i := range stored {
		if stored[i].Type == models.EventBlockAnswered {
			answered = &stored[i]
		}
	}
	require.NotNil(t, answered, "the wrong answer is still an attempt")
	assert.Zero(t, answered.Points, "and the log records what was credited")
}

// A completion the game turned away is never credited: the payload keeps the
// value the block computed, the column records the zero, and nothing claims the
// objective finished. The objective sits behind an ordered parent whose first
// child is unfinished, so the run cannot reach it.
func TestCheckInService_GateDeclinedCreditIsRecordedAsZero(t *testing.T) {
	f := setupCompletingSubmission(t, "free_text", repositories.NewRunEventRepository)
	ctx := context.Background()

	parent := &models.Objective{
		ID: gofakeit.UUID(), QuestID: f.team.QuestID, Slug: "campus", Title: "Campus",
		Routing: game.RouteStrategyOrdered,
	}
	_, err := f.dbc.NewInsert().Model(parent).Exec(ctx)
	require.NoError(t, err)

	first := &models.Objective{
		ID: gofakeit.UUID(), QuestID: f.team.QuestID, ParentID: parent.ID,
		Position: 0, Slug: "first", Title: "First",
	}
	_, err = f.dbc.NewInsert().Model(first).Exec(ctx)
	require.NoError(t, err)

	_, err = f.dbc.NewUpdate().Model((*models.Objective)(nil)).
		Set("parent_id = ?", parent.ID).
		Set("position = ?", 1).
		Where("id = ?", f.objective.ID).
		Exec(ctx)
	require.NoError(t, err)

	// The block is worth 10, so the divergence between the two numbers is real.
	_, err = f.dbc.NewUpdate().Model((*models.Block)(nil)).
		Set("points = ?", 10).
		Where("id = ?", f.block.GetID()).
		Exec(ctx)
	require.NoError(t, err)

	_, _, err = f.svc.ValidateAndUpdateBlockState(ctx, f.team, map[string][]string{
		"block": {f.block.GetID()}, "response": {"the answer"},
	})
	require.NoError(t, err)

	var run models.Run
	require.NoError(t, f.dbc.NewSelect().Model(&run).Where("code = ?", f.team.Code).Scan(ctx))
	assert.Zero(t, run.Points, "an unreachable completion is never credited")

	stored, err := repositories.NewRunEventRepository(f.dbc).ListByRun(ctx, f.team.Code)
	require.NoError(t, err)

	var answered *models.RunEvent
	for i := range stored {
		require.NotEqual(t, models.EventObjectiveContextCompleted, stored[i].Type,
			"a completion the game turned away was recorded as finished")
		if stored[i].Type == models.EventBlockAnswered {
			answered = &stored[i]
		}
	}
	require.NotNil(t, answered)

	assert.Zero(t, answered.Points, "the column records the zero that was credited")

	var payload models.BlockAnsweredData
	require.NoError(t, json.Unmarshal(answered.Data, &payload))
	assert.Equal(t, 10, payload.PointsAwarded, "while the payload keeps what the block computed")
}

// The finish is recorded where it happens. The fixture's objective is the
// quest's root, so completing it is the run completing.
func TestCheckInService_CompletingTheRootRecordsTheFinish(t *testing.T) {
	f := setupCompletingSubmission(t, "free_text", repositories.NewRunEventRepository)
	ctx := context.Background()

	_, _, err := f.svc.ValidateAndUpdateBlockState(ctx, f.team, map[string][]string{
		"block": {f.block.GetID()}, "response": {"the answer"},
	})
	require.NoError(t, err)

	stored, err := repositories.NewRunEventRepository(f.dbc).ListByRun(ctx, f.team.Code)
	require.NoError(t, err)

	finishes := 0
	for i, e := range stored {
		if e.Type == models.EventRunCompleted {
			finishes++
			assert.Equal(t, len(stored)-1, i, "the finish is the last thing recorded")
		}
	}
	assert.Equal(t, 1, finishes)
}

// A resubmission of an answer that already completed its block writes nothing at
// all, so one click does not become two attempts in the record.
func TestCheckInService_ResubmissionRecordsNoSecondEvent(t *testing.T) {
	f := setupCompletingSubmission(t, "free_text", repositories.NewRunEventRepository)
	ctx := context.Background()
	events := repositories.NewRunEventRepository(f.dbc)

	answer := map[string][]string{"block": {f.block.GetID()}, "response": {"the answer"}}

	_, _, err := f.svc.ValidateAndUpdateBlockState(ctx, f.team, answer)
	require.NoError(t, err)

	_, _, err = f.svc.ValidateAndUpdateBlockState(ctx, f.team, answer)
	require.NoError(t, err)

	stored, err := events.ListByRun(ctx, f.team.Code)
	require.NoError(t, err)

	answers := 0
	for _, e := range stored {
		if e.Type == models.EventBlockAnswered {
			answers++
		}
	}
	assert.Equal(t, 1, answers, "a second identical submission is not a second attempt")
}

// Two teammates answering the same block at the same instant, which is one click
// as far as the record is concerned. Which request wins is not the point: that
// only one of them records an attempt is, or a double click reads as two.
//
// Neither request's error is asserted on. Under contention one of them may be
// refused by the database, and that is a legitimate outcome; what must hold is
// what ends up recorded.
func TestCheckInService_ConcurrentCompletionsRecordOneEvent(t *testing.T) {
	f := setupCompletingSubmission(t, "free_text", repositories.NewRunEventRepository)
	ctx := context.Background()
	events := repositories.NewRunEventRepository(f.dbc)

	answer := map[string][]string{"block": {f.block.GetID()}, "response": {"the answer"}}

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _, _ = f.svc.ValidateAndUpdateBlockState(ctx, f.team, answer)
		}()
	}
	wg.Wait()

	stored, err := events.ListByRun(ctx, f.team.Code)
	require.NoError(t, err)

	answers := 0
	for _, e := range stored {
		if e.Type == models.EventBlockAnswered {
			answers++
		}
	}
	assert.Equal(t, 1, answers, "one click is one attempt, however many requests it became")

	count, err := f.dbc.NewSelect().
		Model((*models.ObjectiveContextCompletion)(nil)).
		Where("objective_id = ?", f.objective.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "and one completion, not two")
}
