package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupRunEventRepo(t *testing.T) (repositories.RunEventRepository, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	return repositories.NewRunEventRepository(dbc), dbc, cleanup
}

func event(runCode, questID string, eventType models.EventType) *models.RunEvent {
	return &models.RunEvent{RunCode: runCode, QuestID: questID, Type: eventType}
}

func TestRunEventRepository_RunCompletedOutsideATransaction(t *testing.T) {
	repo, dbc, cleanup := setupRunEventRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)

	done, err := repo.RunCompleted(ctx, runCode)
	require.NoError(t, err)
	assert.False(t, done)

	require.NoError(t, repo.Record(ctx, event(runCode, parents.QuestID, models.EventRunCompleted)))
	done, err = repo.RunCompleted(ctx, runCode)
	require.NoError(t, err)
	assert.True(t, done)
}

// The finish guard reads inside the caller's transaction, so a second
// completion racing the first sees the row the first committed.
func TestRunEventRepository_RunCompleted(t *testing.T) {
	repo, dbc, cleanup := setupRunEventRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)
	// Seeded before the transaction opens: a write through the pool while the
	// lock is held deadlocks the shared-cache test database.
	other := createTestTeam(t, dbc, parents.QuestID)

	// Committed rather than rolled back, like the other transaction tests here:
	// a rolled-back transaction wedges the next WipeData into a spin.
	tx, err := db.NewTransactor(dbc).BeginTx(ctx, nil)
	require.NoError(t, err)

	recorded, err := repo.RunCompletedWithTx(ctx, tx, runCode)
	require.NoError(t, err)
	assert.False(t, recorded, "nothing recorded yet")

	require.NoError(t, repo.RecordWithTx(ctx, tx, event(runCode, parents.QuestID, models.EventRunCompleted)))
	recorded, err = repo.RunCompletedWithTx(ctx, tx, runCode)
	require.NoError(t, err)
	assert.True(t, recorded, "seen inside the same transaction that wrote it")

	// Another run's finish is its own.
	recorded, err = repo.RunCompletedWithTx(ctx, tx, other)
	require.NoError(t, err)
	assert.False(t, recorded, "the guard reads one run's stream, not the quest's")

	require.NoError(t, tx.Commit())
}

func TestRunEventRepository_RecordAndListInOrder(t *testing.T) {
	repo, dbc, cleanup := setupRunEventRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)

	base := time.Now().UTC().Add(-time.Hour)
	for i, eventType := range []models.EventType{
		models.EventRunStarted,
		models.EventObjectiveViewed,
		models.EventBlockAnswered,
	} {
		e := event(runCode, parents.QuestID, eventType)
		e.CreatedAt = base.Add(time.Duration(i) * time.Minute)
		require.NoError(t, repo.Record(ctx, e))
	}

	events, err := repo.ListByRun(ctx, runCode)
	require.NoError(t, err)
	require.Len(t, events, 3)

	assert.Equal(t, models.EventRunStarted, events[0].Type)
	assert.Equal(t, models.EventObjectiveViewed, events[1].Type)
	assert.Equal(t, models.EventBlockAnswered, events[2].Type)
}

// The autoincrement id is the whole reason this column is not a uuid: one request
// can emit two events in the same microsecond, and their order has to survive.
func TestRunEventRepository_SameInstantKeepsWriteOrder(t *testing.T) {
	repo, dbc, cleanup := setupRunEventRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)

	sameInstant := time.Now().UTC()
	first := event(runCode, parents.QuestID, models.EventBlockAnswered)
	first.CreatedAt = sameInstant
	second := event(runCode, parents.QuestID, models.EventObjectiveContextCompleted)
	second.CreatedAt = sameInstant

	require.NoError(t, repo.Record(ctx, first))
	require.NoError(t, repo.Record(ctx, second))

	events, err := repo.ListByRun(ctx, runCode)
	require.NoError(t, err)
	require.Len(t, events, 2)
	assert.Equal(t, models.EventBlockAnswered, events[0].Type)
	assert.Equal(t, models.EventObjectiveContextCompleted, events[1].Type)
	assert.Less(t, events[0].ID, events[1].ID)
}

// A zero timestamp is stamped, at sub-second resolution.
func TestRunEventRepository_StampsZeroCreatedAt(t *testing.T) {
	repo, dbc, cleanup := setupRunEventRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)

	before := time.Now().UTC().Add(-time.Second)
	require.NoError(t, repo.Record(ctx, event(runCode, parents.QuestID, models.EventRunStarted)))

	events, err := repo.ListByRun(ctx, runCode)
	require.NoError(t, err)
	require.Len(t, events, 1)

	assert.False(t, events[0].CreatedAt.IsZero())
	assert.True(t, events[0].CreatedAt.After(before))
	// The offset, not the Location identity: the driver rebuilds the zone on the
	// way out, so comparing against time.UTC would be testing the driver's
	// allocation rather than the value that was stored.
	_, offset := events[0].CreatedAt.Zone()
	assert.Equal(t, 0, offset, "stored in UTC")
	// Sub-second precision, which current_timestamp would not have given.
	assert.NotZero(t, events[0].CreatedAt.Nanosecond())
}

func TestRunEventRepository_LatestByTypeHonoursTheTypeSet(t *testing.T) {
	repo, dbc, cleanup := setupRunEventRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)

	base := time.Now().UTC().Add(-time.Hour)
	quiet := event(runCode, parents.QuestID, models.EventNotificationSent)
	quiet.CreatedAt = base
	require.NoError(t, repo.Record(ctx, quiet))

	meaningful := event(runCode, parents.QuestID, models.EventBlockAnswered)
	meaningful.CreatedAt = base.Add(30 * time.Minute)
	require.NoError(t, repo.Record(ctx, meaningful))

	latest, err := repo.LatestByType(
		ctx, parents.QuestID, []models.EventType{models.EventBlockAnswered},
	)
	require.NoError(t, err)
	require.Contains(t, latest, runCode)
	assert.WithinDuration(t, meaningful.CreatedAt, latest[runCode], time.Second)

	// A filter that matches nothing leaves the run absent, not zero.
	none, err := repo.LatestByType(ctx, parents.QuestID, []models.EventType{models.EventUploadAdded})
	require.NoError(t, err)
	assert.NotContains(t, none, runCode)
}

func TestRunEventRepository_TimesByTypeWindow(t *testing.T) {
	repo, dbc, cleanup := setupRunEventRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)

	base := time.Now().UTC().Add(-2 * time.Hour)
	for i := range 4 {
		e := event(runCode, parents.QuestID, models.EventObjectiveViewed)
		e.CreatedAt = base.Add(time.Duration(i) * 30 * time.Minute)
		require.NoError(t, repo.Record(ctx, e))
	}

	types := []models.EventType{models.EventObjectiveViewed}

	all, err := repo.TimesByType(ctx, parents.QuestID, types, time.Time{})
	require.NoError(t, err)
	require.Len(t, all[runCode], 4)
	assert.True(t, all[runCode][0].Before(all[runCode][3]), "times come back oldest first")

	// A zero since means the whole history, so this matches the unfiltered read.
	windowed, err := repo.TimesByType(ctx, parents.QuestID, types, base.Add(45*time.Minute))
	require.NoError(t, err)
	assert.Len(t, windowed[runCode], 2, "the window bounds every gap in the baseline")
}

// The constraint under the rollback, exercised rather than stubbed. A stubbed
// repository proves that an error undoes the writes beside it; this proves the
// database will actually raise one, which is the half a stub cannot show.
func TestRunEventRepository_ForeignKeyRejectsAndRollsBack(t *testing.T) {
	repo, dbc, cleanup := setupRunEventRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)

	tx, err := dbc.BeginTx(ctx, nil)
	require.NoError(t, err)

	require.NoError(t, repo.RecordWithTx(ctx, &tx, event(runCode, parents.QuestID, models.EventRunStarted)))

	// A quest that does not exist. quest_id carries a real foreign key, so this is
	// the write that fails.
	orphan := event(runCode, "00000000-0000-0000-0000-000000000000", models.EventRunStarted)
	require.Error(t, repo.RecordWithTx(ctx, &tx, orphan), "an event for a quest that is not there is refused")

	require.NoError(t, tx.Rollback())

	stored, err := repo.ListByRun(ctx, runCode)
	require.NoError(t, err)
	assert.Empty(t, stored, "the event written before the failure went back with it")
}

// A deleted run takes its stream with it, through the cascade on run_code.
func TestRunEventRepository_CascadeOnRunDelete(t *testing.T) {
	repo, dbc, cleanup := setupRunEventRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)
	runCode := createTestTeam(t, dbc, parents.QuestID)

	require.NoError(t, repo.Record(ctx, event(runCode, parents.QuestID, models.EventRunStarted)))

	_, err := dbc.NewDelete().Model((*models.Run)(nil)).Where("code = ?", runCode).Exec(ctx)
	require.NoError(t, err)

	events, err := repo.ListByRun(ctx, runCode)
	require.NoError(t, err)
	assert.Empty(t, events)
}
