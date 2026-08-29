package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// setupGameStructureServiceForObjectives exposes dbc directly: objectives have
// no dedicated admin-facing service, so tests insert them straight into the DB,
// the same way objective_context_completion_test.go does.
func setupGameStructureServiceForObjectives(
	t *testing.T,
) (*services.GameStructureService, services.QuestService, services.UserService, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	instanceRepo := repositories.NewQuestRepository(dbc)
	instanceSettingsRepo := repositories.NewQuestSettingsRepository(dbc)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	userRepo := repositories.NewUserRepository(dbc)

	gameStructureService := services.NewGameStructureService(objectiveRepo, instanceRepo)
	questService := services.NewQuestService(instanceRepo, instanceSettingsRepo, blockRepo)
	userService := services.NewUserService(userRepo, instanceRepo)

	return gameStructureService, *questService, *userService, dbc, cleanup
}

func createTestObjective(t *testing.T, dbc *bun.DB, questID, title string) *models.Objective {
	t.Helper()
	obj := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: questID,
		Slug:    gofakeit.LetterN(8),
		Title:   title,
	}
	_, err := dbc.NewInsert().Model(obj).Exec(context.Background())
	require.NoError(t, err)
	return obj
}

func TestGameStructureService_Load_Objectives(t *testing.T) {
	service, questService, userService, dbc, cleanup := setupGameStructureServiceForObjectives(t)
	defer cleanup()
	ctx := context.Background()

	user := &models.User{Email: gofakeit.Email(), Password: "password"}
	require.NoError(t, userService.CreateUser(ctx, user, "password"))
	instance, err := questService.CreateQuest(ctx, "Test Instance", user)
	require.NoError(t, err)

	obj1 := createTestObjective(t, dbc, instance.ID, "Objective 1")
	obj2 := createTestObjective(t, dbc, instance.ID, "Objective 2")

	structure := models.GameStructure{
		ID:           "root",
		IsRoot:       true,
		ObjectiveIDs: []string{obj1.ID, obj2.ID},
	}

	err = service.Load(ctx, instance.ID, &structure, false)
	require.NoError(t, err)
	require.Len(t, structure.Objectives, 2)
	assert.Equal(t, obj1.ID, structure.Objectives[0].ID, "order should be preserved")
	assert.Equal(t, obj2.ID, structure.Objectives[1].ID, "order should be preserved")
}

func TestGameStructureService_LoadBlocksForStructure_Objectives(t *testing.T) {
	service, questService, userService, dbc, cleanup := setupGameStructureServiceForObjectives(t)
	defer cleanup()
	ctx := context.Background()

	user := &models.User{Email: gofakeit.Email(), Password: "password"}
	require.NoError(t, userService.CreateUser(ctx, user, "password"))
	instance, err := questService.CreateQuest(ctx, "Test Instance", user)
	require.NoError(t, err)

	obj := createTestObjective(t, dbc, instance.ID, "Objective 1")
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockService := services.NewBlockService(repositories.NewBlockRepository(dbc, blockStateRepo), blockStateRepo)
	_, err = blockService.NewBlockWithOwnerAndContext(ctx, obj.ID, blocks.ContextObjectiveProof, "text")
	require.NoError(t, err)

	structure := models.GameStructure{ID: "root", IsRoot: true, ObjectiveIDs: []string{obj.ID}}
	require.NoError(t, service.Load(ctx, instance.ID, &structure, false))
	require.Empty(t, structure.Objectives[0].Blocks, "blocks should not be loaded yet")

	require.NoError(t, service.LoadBlocksForStructure(ctx, &structure, false))
	require.Len(t, structure.Objectives[0].Blocks, 1)
	assert.Equal(t, blocks.ContextObjectiveProof, structure.Objectives[0].Blocks[0].Context)
}

func TestGameStructureService_EnsureAllObjectivesIncluded(t *testing.T) {
	service, questService, userService, dbc, cleanup := setupGameStructureServiceForObjectives(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("Orphaned objectives are added to root group", func(t *testing.T) {
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		require.NoError(t, userService.CreateUser(ctx, user, "password"))
		instance, err := questService.CreateQuest(ctx, "Test Instance", user)
		require.NoError(t, err)

		obj1 := createTestObjective(t, dbc, instance.ID, "Objective 1")
		obj2 := createTestObjective(t, dbc, instance.ID, "Objective 2")

		structure := models.GameStructure{ID: "root", IsRoot: true, ObjectiveIDs: []string{obj1.ID}}
		err = service.Save(ctx, instance.ID, &structure)
		require.NoError(t, err)

		reloaded, err := questService.GetByID(ctx, instance.ID)
		require.NoError(t, err)
		assert.Len(t, reloaded.GameStructure.ObjectiveIDs, 2)
		assert.Contains(t, reloaded.GameStructure.ObjectiveIDs, obj2.ID)
	})

	t.Run("No orphans when all objectives are included", func(t *testing.T) {
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		require.NoError(t, userService.CreateUser(ctx, user, "password"))
		instance, err := questService.CreateQuest(ctx, "Test Instance 2", user)
		require.NoError(t, err)

		obj1 := createTestObjective(t, dbc, instance.ID, "Objective 1")
		obj2 := createTestObjective(t, dbc, instance.ID, "Objective 2")

		structure := models.GameStructure{ID: "root", IsRoot: true, ObjectiveIDs: []string{obj1.ID, obj2.ID}}
		err = service.Save(ctx, instance.ID, &structure)
		require.NoError(t, err)

		reloaded, err := questService.GetByID(ctx, instance.ID)
		require.NoError(t, err)
		assert.Equal(t, []string{obj1.ID, obj2.ID}, reloaded.GameStructure.ObjectiveIDs, "order preserved, no duplicates")
	})
}

func TestGameStructureService_InsertObjectiveIntoGroup(t *testing.T) {
	service, questService, userService, dbc, cleanup := setupGameStructureServiceForObjectives(t)
	defer cleanup()
	ctx := context.Background()

	user := &models.User{Email: gofakeit.Email(), Password: "password"}
	require.NoError(t, userService.CreateUser(ctx, user, "password"))
	instance, err := questService.CreateQuest(ctx, "Test Instance", user)
	require.NoError(t, err)

	// obj2/obj3 are deliberately created only once needed, not upfront: Save's
	// orphan sweep (ensureAllObjectivesIncluded) would otherwise auto-include
	// any already-existing objective on the first Save below, before this test
	// gets to insert it explicitly, producing a false duplicate-ID failure.
	obj1 := createTestObjective(t, dbc, instance.ID, "Objective 1")

	structure := models.GameStructure{ID: "root", IsRoot: true, ObjectiveIDs: []string{obj1.ID}}
	require.NoError(t, service.Save(ctx, instance.ID, &structure))

	// Append (no before/after).
	obj2 := createTestObjective(t, dbc, instance.ID, "Objective 2")
	require.NoError(t, service.InsertObjectiveIntoGroup(ctx, instance.ID, obj2.ID, "root", "", ""))
	reloaded, err := questService.GetByID(ctx, instance.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{obj1.ID, obj2.ID}, reloaded.GameStructure.ObjectiveIDs)

	// Insert before obj1.
	obj3 := createTestObjective(t, dbc, instance.ID, "Objective 3")
	require.NoError(t, service.InsertObjectiveIntoGroup(ctx, instance.ID, obj3.ID, "root", "", obj1.ID))
	reloaded, err = questService.GetByID(ctx, instance.ID)
	require.NoError(t, err)
	assert.Equal(t, []string{obj3.ID, obj1.ID, obj2.ID}, reloaded.GameStructure.ObjectiveIDs)
}

func TestGameStructureService_Save_PreventsDuplicateObjectiveIDs(t *testing.T) {
	service, questService, userService, dbc, cleanup := setupGameStructureServiceForObjectives(t)
	defer cleanup()
	ctx := context.Background()

	user := &models.User{Email: gofakeit.Email(), Password: "password"}
	require.NoError(t, userService.CreateUser(ctx, user, "password"))
	instance, err := questService.CreateQuest(ctx, "Test Instance", user)
	require.NoError(t, err)

	obj := createTestObjective(t, dbc, instance.ID, "Objective 1")
	structure := models.GameStructure{ID: "root", IsRoot: true, ObjectiveIDs: []string{obj.ID, obj.ID}}

	err = service.Save(ctx, instance.ID, &structure)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate objective ID")
}
