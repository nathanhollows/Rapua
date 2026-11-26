package repositories_test

import (
	"context"
	"testing"

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
// quest, whose quest has settings, an objective with a block, and a run with a
// notification.
type relationGraph struct {
	UserID      string
	QuestID     string
	BlockID     string
	RunCode     string
	ObjectiveID string
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

	blockID := createTestBlock(t, dbc, gofakeit.UUID())
	runCode := createTestTeam(t, dbc, parents.QuestID)

	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: parents.QuestID,
		Slug:    gofakeit.Word() + "-objective",
		Title:   gofakeit.Sentence(3),
	}
	_, err = dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err, "seed objective")

	notification := &models.Notification{
		ID:      gofakeit.UUID(),
		Content: "test notification",
		RunCode: runCode,
	}
	_, err = dbc.NewInsert().Model(notification).Exec(ctx)
	require.NoError(t, err, "seed notification")

	return relationGraph{
		UserID:      parents.UserID,
		QuestID:     parents.QuestID,
		BlockID:     blockID,
		RunCode:     runCode,
		ObjectiveID: objective.ID,
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
	})

	t.Run("LoadMessages", func(t *testing.T) {
		run, err := repo.GetByCode(ctx, graph.RunCode)
		require.NoError(t, err)

		require.NoError(t, repo.LoadMessages(ctx, run))
		require.Len(t, run.Messages, 1, "messages should be loaded")
		assert.Equal(t, "test notification", run.Messages[0].Content)
	})

	t.Run("LoadRelations loads quest and messages", func(t *testing.T) {
		run, err := repo.GetByCode(ctx, graph.RunCode)
		require.NoError(t, err)

		require.NoError(t, repo.LoadRelations(ctx, run))
		assert.Equal(t, graph.QuestID, run.Quest.ID)
		assert.Len(t, run.Messages, 1)
	})

	t.Run("FindAll resolves its Quest relation", func(t *testing.T) {
		runs, err := repo.FindAll(ctx, graph.QuestID)
		require.NoError(t, err)
		require.NotEmpty(t, runs)
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
	require.Len(t, quest.Runs, 1, "runs should be loaded")
	assert.Equal(t, graph.RunCode, quest.Runs[0].Code)
	require.Len(t, quest.Objectives, 1, "objectives should be loaded")
	assert.Equal(t, graph.ObjectiveID, quest.Objectives[0].ID)
}

func TestQuestRepository_GetByID_LoadsObjectives(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()
	ctx := context.Background()

	repo := repositories.NewQuestRepository(dbc)
	graph := seedRelationGraph(t, dbc)

	quest, err := repo.GetByID(ctx, graph.QuestID)
	require.NoError(t, err)

	require.Len(t, quest.Objectives, 1, "objectives should be loaded")
	assert.Equal(t, graph.ObjectiveID, quest.Objectives[0].ID)
}
