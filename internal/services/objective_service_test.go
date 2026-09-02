package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupObjectiveService(t *testing.T) (services.ObjectiveService, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)
	objectiveService := services.NewObjectiveService(transactor, objectiveRepo)
	return objectiveService, dbc, cleanup
}

func TestObjectiveService_CreateObjective(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()

	t.Run("Create objective", func(t *testing.T) {
		objective, err := service.CreateObjective(context.Background(), validQuestID(t, dbc), "", gofakeit.Sentence(3))
		require.NoError(t, err)
		assert.NotEmpty(t, objective.ID)
	})

	t.Run("Create objective with invalid instance ID", func(t *testing.T) {
		_, err := service.CreateObjective(context.Background(), "", "", gofakeit.Sentence(3))
		require.Error(t, err)
	})

	t.Run("Create objective with invalid title", func(t *testing.T) {
		_, err := service.CreateObjective(context.Background(), validQuestID(t, dbc), "", "")
		require.Error(t, err)
	})
}

// A real failure while checking slug availability must propagate, not be
// silently treated as "slug is available" the way a genuine not-found is.
func TestObjectiveService_CreateObjective_RealSlugCheckErrorPropagates(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	questID := validQuestID(t, dbc)

	cleanup() // close the DB so the slug-availability check fails with something other than sql.ErrNoRows.

	_, err := service.CreateObjective(context.Background(), questID, "", "Find the key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checking slug availability")
}

func TestObjectiveService_CreateObjective_Slug(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("slug generated from title", func(t *testing.T) {
		objective, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Citrus Collection")
		require.NoError(t, err)
		assert.Equal(t, "citrus-collection", objective.Slug)
	})

	t.Run("duplicate title in same instance gets unique slug", func(t *testing.T) {
		questID := validQuestID(t, dbc)
		obj1, err := service.CreateObjective(ctx, questID, "", "Garden Walk")
		require.NoError(t, err)
		obj2, err := service.CreateObjective(ctx, questID, "", "Garden Walk")
		require.NoError(t, err)
		assert.NotEqual(t, obj1.Slug, obj2.Slug)
	})

	t.Run("same title in different instances can share slug", func(t *testing.T) {
		obj1, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Garden Walk")
		require.NoError(t, err)
		obj2, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Garden Walk")
		require.NoError(t, err)
		assert.Equal(t, obj1.Slug, obj2.Slug)
	})
}

func TestObjectiveService_GetByQuestIDAndSlug(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	questID := validQuestID(t, dbc)
	objective, err := service.CreateObjective(ctx, questID, "", gofakeit.Sentence(3))
	require.NoError(t, err)
	require.NotEmpty(t, objective.Slug)

	t.Run("found by slug", func(t *testing.T) {
		found, findErr := service.GetByQuestIDAndSlug(ctx, questID, objective.Slug)
		require.NoError(t, findErr)
		assert.Equal(t, objective.ID, found.ID)
	})

	t.Run("not found with wrong slug", func(t *testing.T) {
		_, findErr := service.GetByQuestIDAndSlug(ctx, questID, gofakeit.Word())
		require.Error(t, findErr)
	})

	t.Run("not found in wrong instance", func(t *testing.T) {
		_, findErr := service.GetByQuestIDAndSlug(ctx, gofakeit.UUID(), objective.Slug)
		require.Error(t, findErr)
	})
}

func TestObjectiveService_UpdateObjective(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("slug updates when title changes", func(t *testing.T) {
		objective, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Original Title")
		require.NoError(t, err)
		assert.Equal(t, "original-title", objective.Slug)

		err = service.UpdateObjective(ctx, &objective, services.ObjectiveUpdateData{Title: "New Title"})
		require.NoError(t, err)
		assert.Equal(t, "new-title", objective.Slug)
		assert.Equal(t, "New Title", objective.Title)
	})

	t.Run("slug unchanged when title not provided", func(t *testing.T) {
		objective, err := service.CreateObjective(ctx, validQuestID(t, dbc), "", "Original Title")
		require.NoError(t, err)
		originalSlug := objective.Slug

		err = service.UpdateObjective(ctx, &objective, services.ObjectiveUpdateData{})
		require.NoError(t, err)
		assert.Equal(t, originalSlug, objective.Slug)
	})

	t.Run("duplicate title in same instance gets unique slug on rename", func(t *testing.T) {
		questID := validQuestID(t, dbc)
		_, err := service.CreateObjective(ctx, questID, "", "Garden Walk")
		require.NoError(t, err)
		obj2, err := service.CreateObjective(ctx, questID, "", gofakeit.Sentence(3))
		require.NoError(t, err)

		err = service.UpdateObjective(ctx, &obj2, services.ObjectiveUpdateData{Title: "Garden Walk"})
		require.NoError(t, err)
		assert.NotEqual(t, "garden-walk", obj2.Slug) // collision resolved with suffix.
		assert.Contains(t, obj2.Slug, "garden-walk")
	})

	t.Run("update persists to storage", func(t *testing.T) {
		questID := validQuestID(t, dbc)
		objective, err := service.CreateObjective(ctx, questID, "", "Original Title")
		require.NoError(t, err)

		err = service.UpdateObjective(ctx, &objective, services.ObjectiveUpdateData{Title: "New Title"})
		require.NoError(t, err)

		reloaded, err := service.GetByQuestIDAndSlug(ctx, questID, "new-title")
		require.NoError(t, err)
		assert.Equal(t, "New Title", reloaded.Title)
	})
}

func TestObjectiveService_FindByQuestID(t *testing.T) {
	service, dbc, cleanup := setupObjectiveService(t)
	defer cleanup()
	ctx := context.Background()

	questID := validQuestID(t, dbc)
	obj1, err := service.CreateObjective(ctx, questID, "", "Find the key")
	require.NoError(t, err)
	obj2, err := service.CreateObjective(ctx, questID, "", "Open the door")
	require.NoError(t, err)

	t.Run("returns all objectives for the quest", func(t *testing.T) {
		objectives, findErr := service.FindByQuestID(ctx, questID)
		require.NoError(t, findErr)
		require.Len(t, objectives, 2)
		ids := []string{objectives[0].ID, objectives[1].ID}
		assert.ElementsMatch(t, []string{obj1.ID, obj2.ID}, ids)
	})

	t.Run("returns empty for a quest with no objectives", func(t *testing.T) {
		objectives, findErr := service.FindByQuestID(ctx, validQuestID(t, dbc))
		require.NoError(t, findErr)
		assert.Empty(t, objectives)
	})
}
