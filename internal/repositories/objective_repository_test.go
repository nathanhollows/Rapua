package repositories_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestObjectiveRepository_GetByID(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	objective := &models.Objective{
		ID:         gofakeit.UUID(),
		QuestID:    parents.QuestID,
		Slug:       "find-the-key",
		Title:      "Find the key",
		ProofSets:  game.SetsField{"door_unlocked": "true"},
		RevealSets: game.SetsField{"story_advanced": "true"},
	}
	_, err := dbc.NewInsert().Model(objective).Exec(context.Background())
	require.NoError(t, err)

	repo := repositories.NewObjectiveRepository(dbc)
	got, err := repo.GetByID(context.Background(), objective.ID)
	require.NoError(t, err)
	assert.Equal(t, objective.Slug, got.Slug)
	assert.Equal(t, objective.Title, got.Title)
	assert.Equal(t, "true", got.ProofSets["door_unlocked"])
	assert.Equal(t, "true", got.RevealSets["story_advanced"])
}

func TestObjectiveRepository_GetByID_NotFound(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	repo := repositories.NewObjectiveRepository(dbc)
	_, err := repo.GetByID(context.Background(), gofakeit.UUID())
	require.Error(t, err)
}

func TestObjectiveRepository_GetByQuestIDAndSlug(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: parents.QuestID,
		Slug:    "find-the-key",
		Title:   "Find the key",
	}
	_, err := dbc.NewInsert().Model(objective).Exec(context.Background())
	require.NoError(t, err)

	repo := repositories.NewObjectiveRepository(dbc)
	got, err := repo.GetByQuestIDAndSlug(context.Background(), parents.QuestID, "find-the-key")
	require.NoError(t, err)
	assert.Equal(t, objective.ID, got.ID)
	assert.Equal(t, objective.Title, got.Title)
}

func TestObjectiveRepository_GetByQuestIDAndSlug_NotFound(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	parents := createTestParents(t, dbc)

	repo := repositories.NewObjectiveRepository(dbc)
	_, err := repo.GetByQuestIDAndSlug(context.Background(), parents.QuestID, "does-not-exist")
	require.Error(t, err)
}

func TestObjectiveRepository_GetByQuestIDAndSlug_WrongQuest(t *testing.T) {
	dbc, cleanup := setupDB(t)
	defer cleanup()

	parents := createTestParents(t, dbc)
	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: parents.QuestID,
		Slug:    "find-the-key",
		Title:   "Find the key",
	}
	_, err := dbc.NewInsert().Model(objective).Exec(context.Background())
	require.NoError(t, err)

	otherParents := createTestParents(t, dbc)

	repo := repositories.NewObjectiveRepository(dbc)
	_, err = repo.GetByQuestIDAndSlug(context.Background(), otherParents.QuestID, "find-the-key")
	require.Error(t, err, "same slug under a different quest must not match")
}
