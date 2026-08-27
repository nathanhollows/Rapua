package repositories_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectiveContextCompletionRepository_Insert_Idempotent(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	objective := &models.Objective{ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "find-the-key", Title: "Find the key"}
	_, err := dbc.NewInsert().Model(objective).Exec(context.Background())
	require.NoError(t, err)

	runCode := gofakeit.LetterN(6)
	run := &models.Run{ID: gofakeit.UUID(), Code: runCode, Name: "test-run", QuestID: parents.QuestID}
	_, err = dbc.NewInsert().Model(run).Exec(context.Background())
	require.NoError(t, err)

	repo := repositories.NewObjectiveContextCompletionRepository(dbc)

	inserted, err := repo.Insert(context.Background(), runCode, objective.ID, blocks.ContextObjectiveProof)
	require.NoError(t, err)
	assert.True(t, inserted, "first insert should win the race and report inserted")

	insertedAgain, err := repo.Insert(context.Background(), runCode, objective.ID, blocks.ContextObjectiveProof)
	require.NoError(t, err)
	assert.False(t, insertedAgain, "second insert must conflict, not duplicate the log row")

	count, err := dbc.NewSelect().
		Model((*models.ObjectiveContextCompletion)(nil)).
		Where("run_code = ? AND objective_id = ? AND context = ?", runCode, objective.ID, blocks.ContextObjectiveProof).
		Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count, "exactly one completion row, regardless of how many times Insert is called")
}

func TestObjectiveContextCompletionRepository_Insert_DistinctContextsDoNotCollide(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	objective := &models.Objective{ID: gofakeit.UUID(), QuestID: parents.QuestID, Slug: "find-the-key", Title: "Find the key"}
	_, err := dbc.NewInsert().Model(objective).Exec(context.Background())
	require.NoError(t, err)

	runCode := gofakeit.LetterN(6)
	run := &models.Run{ID: gofakeit.UUID(), Code: runCode, Name: "test-run", QuestID: parents.QuestID}
	_, err = dbc.NewInsert().Model(run).Exec(context.Background())
	require.NoError(t, err)

	repo := repositories.NewObjectiveContextCompletionRepository(dbc)

	proofInserted, err := repo.Insert(context.Background(), runCode, objective.ID, blocks.ContextObjectiveProof)
	require.NoError(t, err)
	assert.True(t, proofInserted)

	revealInserted, err := repo.Insert(context.Background(), runCode, objective.ID, blocks.ContextObjectiveReveal)
	require.NoError(t, err)
	assert.True(t, revealInserted, "reveal completing is independent of proof completing, same objective and run")
}
