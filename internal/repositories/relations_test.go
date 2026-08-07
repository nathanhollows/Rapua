package repositories_test

import (
	"context"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Bun resolves the names passed to .Relation("X") at query-build time, against
// the rel: tags on the model. A renamed or deleted relation therefore fails at
// runtime, not compile time. The tests in this file exist to execute every
// relation-bearing repository query at least once, so that a stale relation
// name fails the build instead of a request.

// relationGraph holds the IDs of a fully-populated object graph: a user with a
// quest, whose quest has settings, a marker, a location with a block, and a run
// with a check-in and a notification.
type relationGraph struct {
	UserID     string
	QuestID    string
	MarkerCode string
	LocationID string
	BlockID    string
	RunCode    string
}

// seedRelationGraph inserts one of every row needed to make all relations
// resolve to non-empty results.
func seedRelationGraph(t *testing.T, dbc *bun.DB) relationGraph {
	t.Helper()
	ctx := context.Background()

	parents := createTestParents(t, dbc)

	settings := &models.QuestSettings{QuestID: parents.QuestID, EnablePoints: true}
	_, err := dbc.NewInsert().Model(settings).Exec(ctx)
	require.NoError(t, err, "seed quest settings")

	blockID := createTestBlock(t, dbc, parents.LocationID)
	runCode := createTestTeam(t, dbc, parents.QuestID)

	checkIn := &models.CheckIn{
		QuestID:    parents.QuestID,
		RunID:      runCode,
		LocationID: parents.LocationID,
		TimeIn:     time.Now(),
	}
	_, err = dbc.NewInsert().Model(checkIn).Exec(ctx)
	require.NoError(t, err, "seed check-in")

	notification := &models.Notification{
		ID:      gofakeit.UUID(),
		Content: "test notification",
		RunCode: runCode,
	}
	_, err = dbc.NewInsert().Model(notification).Exec(ctx)
	require.NoError(t, err, "seed notification")

	return relationGraph{
		UserID:     parents.UserID,
		QuestID:    parents.QuestID,
		MarkerCode: parents.MarkerCode,
		LocationID: parents.LocationID,
		BlockID:    blockID,
		RunCode:    runCode,
	}
}

func TestRunRepository_LoadRelations(t *testing.T) {
	repo, _, dbc, cleanup := setupTeamRepo(t)
	defer cleanup()
	ctx := context.Background()

	graph := seedRelationGraph(t, dbc)

	t.Run("LoadQuest", func(t *testing.T) {
		run, err := repo.GetByCode(ctx, graph.RunCode)
		require.NoError(t, err)

		require.NoError(t, repo.LoadQuest(ctx, run))
		assert.Equal(t, graph.QuestID, run.Quest.ID, "quest should be loaded")
		assert.Equal(t, graph.QuestID, run.Quest.Settings.QuestID, "nested settings should be loaded")
		assert.Len(t, run.Quest.Locations, 1, "nested locations should be loaded")
	})

	t.Run("LoadCheckIns", func(t *testing.T) {
		run, err := repo.GetByCode(ctx, graph.RunCode)
		require.NoError(t, err)

		require.NoError(t, repo.LoadCheckIns(ctx, run))
		require.Len(t, run.CheckIns, 1, "check-ins should be loaded")
		assert.Equal(t, graph.LocationID, run.CheckIns[0].Location.ID, "nested location should be loaded")
	})

	t.Run("LoadMessages", func(t *testing.T) {
		run, err := repo.GetByCode(ctx, graph.RunCode)
		require.NoError(t, err)

		require.NoError(t, repo.LoadMessages(ctx, run))
		require.Len(t, run.Messages, 1, "messages should be loaded")
		assert.Equal(t, "test notification", run.Messages[0].Content)
	})

	t.Run("LoadBlockingLocation is a no-op when nothing blocks the run", func(t *testing.T) {
		run, err := repo.GetByCode(ctx, graph.RunCode)
		require.NoError(t, err)

		require.NoError(t, repo.LoadBlockingLocation(ctx, run))
		assert.Empty(t, run.BlockingLocation.ID)
	})

	t.Run("LoadBlockingLocation loads the location the run must check out of", func(t *testing.T) {
		run, err := repo.GetByCode(ctx, graph.RunCode)
		require.NoError(t, err)
		run.MustCheckOut = graph.LocationID
		require.NoError(t, repo.Update(ctx, run))

		require.NoError(t, repo.LoadBlockingLocation(ctx, run))
		assert.Equal(t, graph.LocationID, run.BlockingLocation.ID)

		// Restore so sibling subtests see an unblocked run.
		run.MustCheckOut = ""
		require.NoError(t, repo.Update(ctx, run))
	})

	t.Run("GetByCode resolves its BlockingLocation relation", func(t *testing.T) {
		run, err := repo.GetByCode(ctx, graph.RunCode)
		require.NoError(t, err)
		run.MustCheckOut = graph.LocationID
		require.NoError(t, repo.Update(ctx, run))

		// Re-fetch so the relation, not LoadBlockingLocation, populates the field.
		reloaded, err := repo.GetByCode(ctx, graph.RunCode)
		require.NoError(t, err)
		assert.Equal(t, graph.LocationID, reloaded.BlockingLocation.ID,
			"BlockingLocation relation should join must_scan_out against location id")

		run.MustCheckOut = ""
		require.NoError(t, repo.Update(ctx, run))
	})

	t.Run("LoadRelations loads quest, check-ins, blocking location and messages", func(t *testing.T) {
		run, err := repo.GetByCode(ctx, graph.RunCode)
		require.NoError(t, err)

		require.NoError(t, repo.LoadRelations(ctx, run))
		assert.Equal(t, graph.QuestID, run.Quest.ID)
		assert.Len(t, run.CheckIns, 1)
		assert.Len(t, run.Messages, 1)
	})

	t.Run("FindAll resolves its Quest, BlockingLocation and CheckIns relations", func(t *testing.T) {
		runs, err := repo.FindAll(ctx, graph.QuestID)
		require.NoError(t, err)
		require.NotEmpty(t, runs)
	})
}

func TestLocationRepository_LoadRelations(t *testing.T) {
	repo, _, dbc, cleanup := setupLocationRepo(t)
	defer cleanup()
	ctx := context.Background()

	graph := seedRelationGraph(t, dbc)

	t.Run("LoadRelations", func(t *testing.T) {
		location, err := repo.GetByID(ctx, graph.LocationID)
		require.NoError(t, err)

		require.NoError(t, repo.LoadRelations(ctx, location))
		assert.Equal(t, graph.QuestID, location.Quest.ID, "quest should be loaded")
		assert.Equal(t, graph.QuestID, location.Quest.Settings.QuestID, "nested quest settings should be loaded")
		assert.Equal(t, graph.MarkerCode, location.Marker.Code, "marker should be loaded")
		assert.Len(t, location.Blocks, 1, "blocks should be loaded")
	})

	t.Run("LoadMarker", func(t *testing.T) {
		location, err := repo.GetByID(ctx, graph.LocationID)
		require.NoError(t, err)

		require.NoError(t, repo.LoadMarker(ctx, location))
		assert.Equal(t, graph.MarkerCode, location.Marker.Code)
	})

	t.Run("LoadInstance", func(t *testing.T) {
		location, err := repo.GetByID(ctx, graph.LocationID)
		require.NoError(t, err)

		require.NoError(t, repo.LoadInstance(ctx, location))
		assert.Equal(t, graph.QuestID, location.Quest.ID)
	})

	t.Run("LoadBlocks", func(t *testing.T) {
		location, err := repo.GetByID(ctx, graph.LocationID)
		require.NoError(t, err)

		require.NoError(t, repo.LoadBlocks(ctx, location))
		require.Len(t, location.Blocks, 1)
		assert.Equal(t, graph.BlockID, location.Blocks[0].ID)
	})

	t.Run("FindByInstance resolves its Marker relation", func(t *testing.T) {
		locations, err := repo.FindByInstance(ctx, graph.QuestID)
		require.NoError(t, err)
		require.Len(t, locations, 1)
		assert.Equal(t, graph.MarkerCode, locations[0].Marker.Code)
	})

	t.Run("GetByInstanceAndCode resolves its Marker relation", func(t *testing.T) {
		location, err := repo.GetByInstanceAndCode(ctx, graph.QuestID, graph.MarkerCode)
		require.NoError(t, err)
		assert.Equal(t, graph.LocationID, location.ID)
		assert.Equal(t, graph.MarkerCode, location.Marker.Code)
	})

	t.Run("FindLocationsByMarkerID", func(t *testing.T) {
		locations, err := repo.FindLocationsByMarkerID(ctx, graph.MarkerCode)
		require.NoError(t, err)
		require.Len(t, locations, 1)
		assert.Equal(t, graph.LocationID, locations[0].ID)
	})

	t.Run("FindByIDs", func(t *testing.T) {
		locations, err := repo.FindByIDs(ctx, graph.QuestID, []string{graph.LocationID})
		require.NoError(t, err)
		require.Len(t, locations, 1)
		assert.Equal(t, graph.LocationID, locations[0].ID)
	})
}

func TestQuestRepository_GetByIDWithRelations(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	repo := repositories.NewQuestRepository(dbc)
	graph := seedRelationGraph(t, dbc)

	quest, err := repo.GetByIDWithRelations(ctx, graph.QuestID)
	require.NoError(t, err)

	assert.Equal(t, graph.QuestID, quest.ID)
	assert.Equal(t, graph.QuestID, quest.Settings.QuestID, "settings should be loaded")
	require.Len(t, quest.Locations, 1, "locations should be loaded")
	assert.Equal(t, graph.MarkerCode, quest.Locations[0].Marker.Code, "nested location markers should be loaded")
	require.Len(t, quest.Runs, 1, "runs should be loaded")
	assert.Equal(t, graph.RunCode, quest.Runs[0].Code)
}
