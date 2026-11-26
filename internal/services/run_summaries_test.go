package services_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A run's line reads its state from its own stream: a finish outranks the
// clock, a thin run borrows the quest's pace, and progress is the leaves proved
// against the leaves owed.
func TestGetQuestRunSummaries_Statuses(t *testing.T) {
	runService, dbc, cleanup := setupRunsService(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	leaf := &models.Objective{
		ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "the-leaf", Title: "The leaf",
	}
	_, err := dbc.NewInsert().Model(leaf).Exec(ctx)
	require.NoError(t, err)

	eventRepo := repositories.NewRunEventRepository(dbc)
	completionRepo := repositories.NewObjectiveContextCompletionRepository(dbc)
	now := time.Now().UTC()

	record := func(runCode string, eventType models.EventType, when time.Time) {
		t.Helper()
		require.NoError(t, eventRepo.Record(ctx, &models.RunEvent{
			RunCode: runCode, QuestID: parents.QuestID, Type: eventType, CreatedAt: when,
		}))
	}

	// Playing: a steady minute apiece, with the last one just now.
	insertTestTeam(t, dbc, "P001", parents.QuestID)
	for i := range 6 {
		record("P001", models.EventObjectiveViewed, now.Add(-time.Duration(6-i)*time.Minute))
	}
	_, err = completionRepo.Insert(ctx, "P001", leaf.ID, game.ContextObjectiveProof)
	require.NoError(t, err)

	// Finished, however long it has been quiet since.
	insertTestTeam(t, dbc, "P002", parents.QuestID)
	for i := range 5 {
		record("P002", models.EventObjectiveViewed, now.Add(-time.Duration(30+i)*time.Minute))
	}
	record("P002", models.EventRunCompleted, now.Add(-30*time.Minute))

	// Too thin to read: two events and nothing since.
	insertTestTeam(t, dbc, "P003", parents.QuestID)
	record("P003", models.EventRunStarted, now.Add(-40*time.Minute))
	record("P003", models.EventObjectiveViewed, now.Add(-39*time.Minute))

	// A run with a stream has started; the status ladder only reads from there.
	_, err = dbc.NewUpdate().Model((*models.Run)(nil)).
		Set("has_started = ?", true).
		Where("quest_id = ?", parents.QuestID).
		Exec(ctx)
	require.NoError(t, err)

	summaries, err := runService.GetQuestRunSummaries(ctx, parents.QuestID)
	require.NoError(t, err)

	assert.Equal(t, services.StatusPlaying, summaries["P001"].Status)
	assert.Equal(t, 1, summaries["P001"].Done)
	assert.Equal(t, 1, summaries["P001"].Total)
	assert.Equal(t, services.StatusFinished, summaries["P002"].Status)
	assert.Equal(t, services.StatusStalled, summaries["P003"].Status,
		"a run with no pace of its own borrows the quest's, and reads its quiet against it")
	assert.Zero(t, summaries["P003"].Done)
}

// A run recorded before there was a stream still gets a line: its progress is
// a state question, and only its status needs the stream, which such a run does
// not have. The map is keyed off the quest's runs for that reason, not off the
// events.
func TestGetQuestRunSummaries_RunsWithoutAStream(t *testing.T) {
	runService, dbc, cleanup := setupRunsService(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)
	leaf := &models.Objective{
		ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "the-leaf", Title: "The leaf",
	}
	_, err := dbc.NewInsert().Model(leaf).Exec(ctx)
	require.NoError(t, err)

	// Played, and finished as far as the state tables can say: completions and
	// no events at all.
	old := &models.Run{
		ID: gofakeit.UUID(), Code: "OLD1", Name: "Before the stream",
		QuestID: parents.QuestID, HasStarted: true,
	}
	_, err = dbc.NewInsert().Model(old).Exec(ctx)
	require.NoError(t, err)
	_, err = repositories.NewObjectiveContextCompletionRepository(dbc).
		Insert(ctx, old.Code, leaf.ID, game.ContextObjectiveProof)
	require.NoError(t, err)

	insertTestTeam(t, dbc, "NEW1", parents.QuestID)

	summaries, err := runService.GetQuestRunSummaries(ctx, parents.QuestID)
	require.NoError(t, err)

	require.Contains(t, summaries, "OLD1", "a run with no stream still has a line")
	assert.Equal(t, 1, summaries["OLD1"].Done)
	assert.Equal(t, 1, summaries["OLD1"].Total)
	assert.Equal(t, services.StatusStarted, summaries["OLD1"].Status,
		"with no stream there is no pace to read, and the run has begun")

	require.Contains(t, summaries, "NEW1")
	assert.Equal(t, services.StatusNotStarted, summaries["NEW1"].Status)
	assert.Zero(t, summaries["NEW1"].Done)
}

// Another quest's runs, completions and finishes stay out of this quest's map.
// The read joins to reach the quest, and a join that fans out or filters loose
// hands back plausible rows rather than an error.
func TestGetQuestRunSummaries_KeepsOtherQuestsOut(t *testing.T) {
	runService, dbc, cleanup := setupRunsService(t)
	defer cleanup()
	ctx := context.Background()

	mine := createTestParents(t, dbc)
	theirs := createTestParents(t, dbc)

	leaf := func(questID string) models.Objective {
		return models.Objective{
			ID: gofakeit.UUID(), QuestID: questID, Slug: gofakeit.UUID(), Title: "The leaf",
		}
	}
	ourLeaf, theirLeaf := leaf(mine.QuestID), leaf(theirs.QuestID)
	_, err := dbc.NewInsert().Model(&[]models.Objective{ourLeaf, theirLeaf}).Exec(ctx)
	require.NoError(t, err)

	eventRepo := repositories.NewRunEventRepository(dbc)
	completionRepo := repositories.NewObjectiveContextCompletionRepository(dbc)
	finishRepo := repositories.NewSectionFinishRepository(dbc)
	now := time.Now().UTC()

	insertTestTeam(t, dbc, "MINE", mine.QuestID)
	insertTestTeam(t, dbc, "THEIRS", theirs.QuestID)
	for _, run := range []struct {
		code, questID, objectiveID string
	}{
		{"MINE", mine.QuestID, ourLeaf.ID},
		{"THEIRS", theirs.QuestID, theirLeaf.ID},
	} {
		for i := range 5 {
			require.NoError(t, eventRepo.Record(ctx, &models.RunEvent{
				RunCode: run.code, QuestID: run.questID, Type: models.EventObjectiveViewed,
				CreatedAt: now.Add(-time.Duration(5-i) * time.Minute),
			}))
		}
		_, err = completionRepo.Insert(ctx, run.code, run.objectiveID, game.ContextObjectiveProof)
		require.NoError(t, err)
		_, err = finishRepo.Insert(ctx, run.code, run.objectiveID)
		require.NoError(t, err)
	}
	require.NoError(t, eventRepo.Record(ctx, &models.RunEvent{
		RunCode: "THEIRS", QuestID: theirs.QuestID, Type: models.EventRunCompleted,
		CreatedAt: now.Add(-time.Minute),
	}))

	summaries, err := runService.GetQuestRunSummaries(ctx, mine.QuestID)
	require.NoError(t, err)

	assert.Len(t, summaries, 1)
	assert.Equal(t, 1, summaries["MINE"].Done)
	assert.NotContains(t, summaries, "THEIRS")
}

// TestGetQuestRunSummaries_ScalesToFiftyRuns seeds a large game's worth of
// stream, fifty runs of a hundred events each, and measures the whole-quest
// read the runs page makes. The bound is generous on purpose: this test is the
// evidence for the page's cost, and it fails loudly if an N+1 ever creeps in.
func TestGetQuestRunSummaries_ScalesToFiftyRuns(t *testing.T) {
	runService, dbc, cleanup := setupRunsService(t)
	defer cleanup()
	ctx := context.Background()

	parents := createTestParents(t, dbc)

	// A tree of five sections with four leaves each, plus its root.
	objectives := []models.Objective{
		{ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: gofakeit.UUID(), Title: "Quest"},
	}
	root := objectives[0].ID
	for section := range 5 {
		sectionID := gofakeit.UUID()
		objectives = append(objectives, models.Objective{
			ID: sectionID, QuestID: parents.QuestID, ParentID: root,
			Position: section, Slug: gofakeit.UUID(), Title: fmt.Sprintf("Section %d", section),
		})
		for leaf := range 4 {
			objectives = append(objectives, models.Objective{
				ID: gofakeit.UUID(), QuestID: parents.QuestID, ParentID: sectionID,
				Position: leaf, Slug: gofakeit.UUID(), Title: fmt.Sprintf("Leaf %d.%d", section, leaf),
			})
		}
	}
	_, err := dbc.NewInsert().Model(&objectives).Exec(ctx)
	require.NoError(t, err)

	leaves := objectives[1:]
	completionRepo := repositories.NewObjectiveContextCompletionRepository(dbc)
	finishRepo := repositories.NewSectionFinishRepository(dbc)

	const runCount, eventsPerRun = 50, 200
	base := time.Now().UTC().Add(-4 * time.Hour)
	events := make([]models.RunEvent, 0, runCount*eventsPerRun)
	for run := range runCount {
		runCode := fmt.Sprintf("R%03d", run)
		insertTestTeam(t, dbc, runCode, parents.QuestID)

		// A long game's stream apiece: a view or an answer every minute, at a
		// pace any game would read as play.
		for i := range eventsPerRun {
			event := models.RunEvent{
				RunCode:   runCode,
				QuestID:   parents.QuestID,
				Type:      models.EventObjectiveViewed,
				CreatedAt: base.Add(time.Duration(i)*time.Minute + time.Duration(run)*time.Second),
			}
			if i%4 == 0 {
				event.Type = models.EventBlockAnswered
				event.ObjectiveID = leaves[(run+i)%len(leaves)].ID
				event.BlockID = gofakeit.UUID()
			}
			if i == 0 {
				event.Type = models.EventRunStarted
			}
			events = append(events, event)
		}

		// Some progress and a section close, so the summary's maths is the
		// real thing rather than an empty branch.
		for leaf := range 8 + run%4 {
			_, err = completionRepo.Insert(
				ctx, runCode, leaves[leaf%len(leaves)].ID, game.ContextObjectiveProof,
			)
			require.NoError(t, err)
		}
		_, err = finishRepo.Insert(ctx, runCode, objectives[1].ID)
		require.NoError(t, err)
	}
	_, err = dbc.NewInsert().Model(&events).Exec(ctx)
	require.NoError(t, err)

	// Started, so the cadence ladder applies rather than the not-started floor.
	_, err = dbc.NewUpdate().Model((*models.Run)(nil)).
		Set("has_started = ?", true).
		Where("quest_id = ?", parents.QuestID).
		Exec(ctx)
	require.NoError(t, err)

	start := time.Now()
	summaries, err := runService.GetQuestRunSummaries(ctx, parents.QuestID)
	took := time.Since(start)

	require.NoError(t, err)
	assert.Len(t, summaries, runCount)
	assert.NotEqual(t, services.StatusNotStarted, summaries["R001"].Status)
	t.Logf("summaries for %d runs, %d events: %v", runCount, len(events), took)
	assert.Less(t, took, 500*time.Millisecond, "the runs page's read must stay small")
}

// The runs list and the activity table say the same thing about when a run was
// last about: its latest block interaction, and only failing that when it began.
func TestGetQuestRunSummaries_LastSeen(t *testing.T) {
	runService, dbc, cleanup := setupRunsService(t)
	defer cleanup()
	ctx := context.Background()
	parents := createTestParents(t, dbc)

	insertTestTeam(t, dbc, "P001", parents.QuestID)
	insertTestTeam(t, dbc, "P002", parents.QuestID)
	_, err := dbc.NewUpdate().Model((*models.Run)(nil)).
		Set("has_started = ?", true).
		Where("quest_id = ?", parents.QuestID).
		Exec(ctx)
	require.NoError(t, err)

	older := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	newer := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)
	for _, at := range []time.Time{older, newer} {
		block := &models.Block{ID: gofakeit.UUID(), OwnerID: parents.QuestID, Type: "text", Context: "objective_proof"}
		_, blockErr := dbc.NewInsert().Model(block).Exec(ctx)
		require.NoError(t, blockErr)
		state := &models.RunBlockState{RunCode: "P001", BlockID: block.ID, QuestID: parents.QuestID}
		state.UpdatedAt = at
		_, insertErr := dbc.NewInsert().Model(state).Exec(ctx)
		require.NoError(t, insertErr)
	}

	summaries, err := runService.GetQuestRunSummaries(ctx, parents.QuestID)
	require.NoError(t, err)

	assert.WithinDuration(t, newer, summaries["P001"].LastSeen, time.Second, "the newest interaction")
	assert.True(t, summaries["P002"].LastSeen.IsZero(), "a run that has done nothing has no last seen")
}

func TestLastSeenOrStarted_FallsBackToWhenTheRunBegan(t *testing.T) {
	began := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	seen := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	run := models.Run{StartedAt: began}

	assert.Equal(t, seen, services.LastSeenOrStarted(seen, run))
	assert.Equal(t, began, services.LastSeenOrStarted(time.Time{}, run))
}

// ReadStations is what a facilitator's page stands on, read against a real quest:
// who has cleared a station, who has just opened it, who can visit and has not,
// and who is left out. Each team's own position decides, through the same loader
// the game itself uses.
func TestReadStations_ReadsEachTeamAgainstTheQuest(t *testing.T) {
	runService, dbc, cleanup := setupRunsService(t)
	defer cleanup()
	ctx := context.Background()
	parents := createTestParents(t, dbc)

	root := &models.Objective{
		ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "root", Title: "The quest",
		Routing: game.RouteStrategyFreeRoam,
	}
	mail := &models.Objective{
		ID: gofakeit.UUID(), QuestID: parents.QuestID, ParentID: root.ID, Slug: "mail", Title: "Mailboxes",
		Routing: game.RouteStrategyFreeRoam,
	}
	bbq := &models.Objective{
		ID: gofakeit.UUID(), QuestID: parents.QuestID, ParentID: root.ID, Slug: "bbq", Title: "BBQ Area",
		Routing: game.RouteStrategyFreeRoam, Position: 1,
	}
	for _, objective := range []*models.Objective{root, mail, bbq} {
		_, err := dbc.NewInsert().Model(objective).Exec(ctx)
		require.NoError(t, err)
	}

	eventRepo := repositories.NewRunEventRepository(dbc)
	completionRepo := repositories.NewObjectiveContextCompletionRepository(dbc)
	now := time.Now().UTC()

	for _, code := range []string{"FOXE", "TUIS", "KEAS", "NEWB"} {
		insertTestTeam(t, dbc, code, parents.QuestID)
	}
	_, err := dbc.NewUpdate().Model((*models.Run)(nil)).
		Set("has_started = ?", true).
		Where("quest_id = ? AND code != ?", parents.QuestID, "NEWB").
		Exec(ctx)
	require.NoError(t, err)

	// FOXE has cleared Mailboxes. TUIS opened it two minutes ago. KEAS has done neither.
	_, err = completionRepo.Insert(ctx, "FOXE", mail.ID, game.ContextObjectiveProof)
	require.NoError(t, err)
	require.NoError(t, eventRepo.Record(ctx, &models.RunEvent{
		RunCode: "TUIS", QuestID: parents.QuestID, Type: models.EventObjectiveViewed,
		ObjectiveID: mail.ID, CreatedAt: now.Add(-2 * time.Minute),
	}))
	for _, code := range []string{"FOXE", "TUIS", "KEAS"} {
		require.NoError(t, eventRepo.Record(ctx, &models.RunEvent{
			RunCode: code, QuestID: parents.QuestID, Type: models.EventRunStarted,
			CreatedAt: now.Add(-10 * time.Minute),
		}))
	}

	report, err := runService.ReadStations(ctx, parents.QuestID, now)
	require.NoError(t, err)

	assert.Equal(t, 3, report.Playing)
	assert.Equal(t, 1, report.NotStarted, "a code nobody has used")
	require.Len(t, report.Stations, 2, "the quest's own root is not a station")

	byTitle := map[string]services.StationReading{}
	for _, station := range report.Stations {
		byTitle[station.Objective.Title] = station
	}
	mailboxes, barbecue := byTitle["Mailboxes"], byTitle["BBQ Area"]

	assert.Len(t, mailboxes.Been, 1)
	assert.Equal(t, "FOXE", mailboxes.Been[0].Code)
	require.Len(t, mailboxes.Here, 1)
	assert.Equal(t, "TUIS", mailboxes.Here[0].Code)
	assert.WithinDuration(t, now.Add(-2*time.Minute), mailboxes.Here[0].Arrived, time.Second)
	assert.Len(t, mailboxes.ToCome, 1, "KEAS can visit and has not")

	assert.Len(t, barbecue.ToCome, 3, "free roam: every playing team can go to the barbecue")
	assert.Empty(t, barbecue.Been)
	open := services.GameClock{Kind: services.ClockOpen}
	assert.Equal(t, services.VerdictStay, mailboxes.Verdict(open, report.Playing))
}

// The cache shares one read between every request waiting on it, so the read must
// not die with the request that happened to start it.
func TestCachedStations_IsNotCancelledByTheRequestThatStartedIt(t *testing.T) {
	runService, dbc, cleanup := setupRunsService(t)
	defer cleanup()
	parents := createTestParents(t, dbc)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := runService.CachedStations(ctx, parents.QuestID)

	require.NoError(t, err, "one caller giving up must not fail the read others are waiting for")
}
