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

func setupGameStructureService(
	t *testing.T,
) (*services.GameStructureService, services.LocationService, services.BlockService, services.QuestService, services.UserService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	// Initialize repositories
	instanceRepo := repositories.NewQuestRepository(dbc)
	instanceSettingsRepo := repositories.NewQuestSettingsRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	userRepo := repositories.NewUserRepository(dbc)

	// Initialize services
	gameStructureService := services.NewGameStructureService(locationRepo, objectiveRepo, instanceRepo)
	markerService := services.NewMarkerService(markerRepo)
	locationService := services.NewLocationService(locationRepo, markerRepo, blockRepo, markerService)
	blockService := services.NewBlockService(blockRepo, blockStateRepo)
	questService := services.NewQuestService(instanceRepo, instanceSettingsRepo, blockRepo)
	userService := services.NewUserService(userRepo, instanceRepo)

	// Set up relation loader
	gameStructureService.SetRelationLoader(locationRepo)

	return gameStructureService, locationService, *blockService, *questService, *userService, cleanup
}

func TestGameStructureService_LoadBlocksForStructure(t *testing.T) {
	service, locationService, blockService, questService, userService, cleanup := setupGameStructureService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Load blocks for simple structure", func(t *testing.T) {
		// Create user
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		err := userService.CreateUser(ctx, user, "password")
		require.NoError(t, err)

		// Create instance
		instance, err := questService.CreateQuest(ctx, "Test Instance", user)
		require.NoError(t, err)

		// Create locations with blocks
		loc1, err := locationService.CreateLocation(ctx, instance.ID, "Location 1", 0, 0, 0)
		require.NoError(t, err)

		// Add block to location 1
		_, err = blockService.NewBlockWithOwnerAndContext(ctx, loc1.ID, blocks.ContextNavigation, "text")
		require.NoError(t, err)

		// Create game structure
		structure := models.GameStructure{
			ID:          "root",
			IsRoot:      true,
			LocationIDs: []string{loc1.ID},
		}

		// Load locations first
		err = service.Load(ctx, instance.ID, &structure, false)
		require.NoError(t, err)
		assert.Len(t, structure.Locations, 1)
		assert.Empty(t, structure.Locations[0].Blocks, "Blocks should not be loaded yet")

		// Load blocks
		err = service.LoadBlocksForStructure(ctx, &structure, false)
		require.NoError(t, err)

		// Verify blocks are loaded (default header + manually created markdown)
		assert.Len(t, structure.Locations[0].Blocks, 2)
		// First block should be the default header with location_content context
		assert.Equal(t, blocks.ContextLocationContent, structure.Locations[0].Blocks[0].Context)
		assert.Equal(t, "header", structure.Locations[0].Blocks[0].Type)
		// Second block should be the manually created markdown with location_clues context
		assert.Equal(t, blocks.ContextNavigation, structure.Locations[0].Blocks[1].Context)
		assert.Equal(t, "text", structure.Locations[0].Blocks[1].Type)
	})

	t.Run("Load blocks recursively for nested structure", func(t *testing.T) {
		// Create user
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		err := userService.CreateUser(ctx, user, "password")
		require.NoError(t, err)

		// Create instance
		instance, err := questService.CreateQuest(ctx, "Test Instance 2", user)
		require.NoError(t, err)

		// Create locations
		loc1, err := locationService.CreateLocation(ctx, instance.ID, "Location 1", 0, 0, 0)
		require.NoError(t, err)
		loc2, err := locationService.CreateLocation(ctx, instance.ID, "Location 2", 0, 0, 0)
		require.NoError(t, err)

		// Add blocks
		_, err = blockService.NewBlockWithOwnerAndContext(ctx, loc1.ID, blocks.ContextNavigation, "text")
		require.NoError(t, err)
		_, err = blockService.NewBlockWithOwnerAndContext(ctx, loc2.ID, blocks.ContextLocationContent, "text")
		require.NoError(t, err)

		// Create nested structure
		structure := models.GameStructure{
			ID:          "root",
			IsRoot:      true,
			LocationIDs: []string{loc1.ID},
			SubGroups: []models.GameStructure{
				{
					ID:          "group1",
					Name:        "Group 1",
					Color:       "primary",
					LocationIDs: []string{loc2.ID},
				},
			},
		}

		// Load locations recursively
		err = service.Load(ctx, instance.ID, &structure, true)
		require.NoError(t, err)

		// Load blocks recursively
		err = service.LoadBlocksForStructure(ctx, &structure, true)
		require.NoError(t, err)

		// Verify blocks are loaded at both levels (default header + manually created markdown)
		assert.Len(t, structure.Locations[0].Blocks, 2, "Root location should have blocks")
		assert.Len(t, structure.SubGroups[0].Locations[0].Blocks, 2, "Subgroup location should have blocks")
	})

	t.Run("Non-recursive load only loads current level", func(t *testing.T) {
		// Create user
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		err := userService.CreateUser(ctx, user, "password")
		require.NoError(t, err)

		// Create instance
		instance, err := questService.CreateQuest(ctx, "Test Instance 3", user)
		require.NoError(t, err)

		// Create locations
		loc1, err := locationService.CreateLocation(ctx, instance.ID, "Location 1", 0, 0, 0)
		require.NoError(t, err)
		loc2, err := locationService.CreateLocation(ctx, instance.ID, "Location 2", 0, 0, 0)
		require.NoError(t, err)

		// Add blocks
		_, err = blockService.NewBlockWithOwnerAndContext(ctx, loc1.ID, blocks.ContextNavigation, "text")
		require.NoError(t, err)
		_, err = blockService.NewBlockWithOwnerAndContext(ctx, loc2.ID, blocks.ContextLocationContent, "text")
		require.NoError(t, err)

		// Create nested structure
		structure := models.GameStructure{
			ID:          "root",
			IsRoot:      true,
			LocationIDs: []string{loc1.ID},
			SubGroups: []models.GameStructure{
				{
					ID:          "group1",
					Name:        "Group 1",
					Color:       "primary",
					LocationIDs: []string{loc2.ID},
				},
			},
		}

		// Load locations recursively
		err = service.Load(ctx, instance.ID, &structure, true)
		require.NoError(t, err)

		// Load blocks NON-recursively
		err = service.LoadBlocksForStructure(ctx, &structure, false)
		require.NoError(t, err)

		// Verify only root level has blocks loaded (default header + manually created markdown)
		assert.Len(t, structure.Locations[0].Blocks, 2, "Root location should have blocks")
		assert.Empty(t, structure.SubGroups[0].Locations[0].Blocks, "Subgroup location should NOT have blocks")
	})
}

func TestGameStructureService_EnsureAllLocationsIncluded(t *testing.T) {
	service, locationService, _, questService, userService, cleanup := setupGameStructureService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Orphaned locations are added to root group", func(t *testing.T) {
		// Create user
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		err := userService.CreateUser(ctx, user, "password")
		require.NoError(t, err)

		// Create instance
		instance, err := questService.CreateQuest(ctx, "Test Instance", user)
		require.NoError(t, err)

		// Create locations
		loc1, err := locationService.CreateLocation(ctx, instance.ID, "Location 1", 0, 0, 0)
		require.NoError(t, err)
		loc2, err := locationService.CreateLocation(ctx, instance.ID, "Location 2", 0, 0, 0)
		require.NoError(t, err)
		loc3, err := locationService.CreateLocation(ctx, instance.ID, "Location 3", 0, 0, 0)
		require.NoError(t, err)

		// Create structure with only loc1 - loc2 and loc3 are orphaned
		structure := models.GameStructure{
			ID:          "root",
			IsRoot:      true,
			LocationIDs: []string{loc1.ID},
		}

		// Save should add orphaned locations to root
		err = service.Save(ctx, instance.ID, &structure)
		require.NoError(t, err)

		// Reload instance to verify
		reloaded, err := questService.GetByID(ctx, instance.ID)
		require.NoError(t, err)

		// All locations should be in root group
		assert.Len(t, reloaded.GameStructure.LocationIDs, 3)
		assert.Contains(t, reloaded.GameStructure.LocationIDs, loc1.ID)
		assert.Contains(t, reloaded.GameStructure.LocationIDs, loc2.ID)
		assert.Contains(t, reloaded.GameStructure.LocationIDs, loc3.ID)
	})

	t.Run("Orphaned locations in subgroups are detected", func(t *testing.T) {
		// Create user
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		err := userService.CreateUser(ctx, user, "password")
		require.NoError(t, err)

		// Create instance
		instance, err := questService.CreateQuest(ctx, "Test Instance 2", user)
		require.NoError(t, err)

		// Create locations
		loc1, err := locationService.CreateLocation(ctx, instance.ID, "Location 1", 0, 0, 0)
		require.NoError(t, err)
		loc2, err := locationService.CreateLocation(ctx, instance.ID, "Location 2", 0, 0, 0)
		require.NoError(t, err)
		loc3, err := locationService.CreateLocation(ctx, instance.ID, "Orphan Location", 0, 0, 0)
		require.NoError(t, err)

		// Create structure with subgroup - loc3 is orphaned
		structure := models.GameStructure{
			ID:          "root",
			IsRoot:      true,
			LocationIDs: []string{loc1.ID},
			SubGroups: []models.GameStructure{
				{
					ID:          "group1",
					Name:        "Group 1",
					Color:       "primary",
					LocationIDs: []string{loc2.ID},
				},
			},
		}

		// Save should add orphaned location to root
		err = service.Save(ctx, instance.ID, &structure)
		require.NoError(t, err)

		// Reload instance to verify
		reloaded, err := questService.GetByID(ctx, instance.ID)
		require.NoError(t, err)

		// Orphaned location should be in root group
		assert.Contains(t, reloaded.GameStructure.LocationIDs, loc3.ID, "Orphaned location should be added to root")
		assert.Len(t, reloaded.GameStructure.LocationIDs, 2, "Root should have loc1 and orphaned loc3")
	})

	t.Run("No orphans when all locations are included", func(t *testing.T) {
		// Create user
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		err := userService.CreateUser(ctx, user, "password")
		require.NoError(t, err)

		// Create instance
		instance, err := questService.CreateQuest(ctx, "Test Instance 3", user)
		require.NoError(t, err)

		// Create locations
		loc1, err := locationService.CreateLocation(ctx, instance.ID, "Location 1", 0, 0, 0)
		require.NoError(t, err)
		loc2, err := locationService.CreateLocation(ctx, instance.ID, "Location 2", 0, 0, 0)
		require.NoError(t, err)

		// Create structure with all locations
		structure := models.GameStructure{
			ID:          "root",
			IsRoot:      true,
			LocationIDs: []string{loc1.ID, loc2.ID},
		}

		// Save should not modify structure
		err = service.Save(ctx, instance.ID, &structure)
		require.NoError(t, err)

		// Reload instance to verify
		reloaded, err := questService.GetByID(ctx, instance.ID)
		require.NoError(t, err)

		// Structure should be unchanged
		assert.Len(t, reloaded.GameStructure.LocationIDs, 2)
		assert.Equal(t, loc1.ID, reloaded.GameStructure.LocationIDs[0], "Order should be preserved")
		assert.Equal(t, loc2.ID, reloaded.GameStructure.LocationIDs[1], "Order should be preserved")
	})
}

func TestGameStructureService_Save_Integration(t *testing.T) {
	service, locationService, _, questService, userService, cleanup := setupGameStructureService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Save validates structure before saving", func(t *testing.T) {
		// Create user
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		err := userService.CreateUser(ctx, user, "password")
		require.NoError(t, err)

		// Create instance
		instance, err := questService.CreateQuest(ctx, "Test Instance", user)
		require.NoError(t, err)

		// Create location
		loc1, err := locationService.CreateLocation(ctx, instance.ID, "Location 1", 0, 0, 0)
		require.NoError(t, err)

		// Create invalid structure (visible group without name)
		structure := models.GameStructure{
			ID:          "root",
			IsRoot:      true,
			LocationIDs: []string{loc1.ID},
			SubGroups: []models.GameStructure{
				{
					ID:          "group1",
					Name:        "", // Invalid: visible group must have name
					Color:       "primary",
					LocationIDs: []string{},
				},
			},
		}

		// Save should fail validation
		err = service.Save(ctx, instance.ID, &structure)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "must have a name")
	})

	t.Run("Save prevents duplicate location IDs", func(t *testing.T) {
		// Create user
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		err := userService.CreateUser(ctx, user, "password")
		require.NoError(t, err)

		// Create instance
		instance, err := questService.CreateQuest(ctx, "Test Instance 2", user)
		require.NoError(t, err)

		// Create location
		loc1, err := locationService.CreateLocation(ctx, instance.ID, "Location 1", 0, 0, 0)
		require.NoError(t, err)

		// Create structure with duplicate location IDs
		structure := models.GameStructure{
			ID:          "root",
			IsRoot:      true,
			LocationIDs: []string{loc1.ID, loc1.ID}, // Duplicate
		}

		// Save should fail validation
		err = service.Save(ctx, instance.ID, &structure)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "duplicate location ID")
	})

	t.Run("Save succeeds with valid structure", func(t *testing.T) {
		// Create user
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		err := userService.CreateUser(ctx, user, "password")
		require.NoError(t, err)

		// Create instance
		instance, err := questService.CreateQuest(ctx, "Test Instance 3", user)
		require.NoError(t, err)

		// Create locations
		loc1, err := locationService.CreateLocation(ctx, instance.ID, "Location 1", 0, 0, 0)
		require.NoError(t, err)
		loc2, err := locationService.CreateLocation(ctx, instance.ID, "Location 2", 0, 0, 0)
		require.NoError(t, err)

		// Create valid structure
		structure := models.GameStructure{
			ID:          "root",
			IsRoot:      true,
			LocationIDs: []string{loc1.ID},
			SubGroups: []models.GameStructure{
				{
					ID:          "group1",
					Name:        "Group 1",
					Color:       "primary",
					LocationIDs: []string{loc2.ID},
				},
			},
		}

		// Save should succeed
		err = service.Save(ctx, instance.ID, &structure)
		require.NoError(t, err)

		// Verify save
		reloaded, err := questService.GetByID(ctx, instance.ID)
		require.NoError(t, err)
		assert.Len(t, reloaded.GameStructure.SubGroups, 1)
		assert.Equal(t, "Group 1", reloaded.GameStructure.SubGroups[0].Name)
	})
}

// setupGameStructureServiceForObjectives mirrors setupGameStructureService but
// exposes dbc directly: objectives have no dedicated admin-facing service, so
// tests insert them straight into the DB, the same way
// objective_context_completion_test.go does.
func setupGameStructureServiceForObjectives(
	t *testing.T,
) (*services.GameStructureService, services.QuestService, services.UserService, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	instanceRepo := repositories.NewQuestRepository(dbc)
	instanceSettingsRepo := repositories.NewQuestSettingsRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	userRepo := repositories.NewUserRepository(dbc)

	gameStructureService := services.NewGameStructureService(locationRepo, objectiveRepo, instanceRepo)
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

func TestGameStructureService_GetAllObjectiveIDs(t *testing.T) {
	service, _, _, _, cleanup := setupGameStructureServiceForObjectives(t)
	defer cleanup()

	structure := &models.GameStructure{
		ID:           "root",
		IsRoot:       true,
		ObjectiveIDs: []string{"obj1", "obj2"},
		SubGroups: []models.GameStructure{
			{ID: "group1", ObjectiveIDs: []string{"obj3"}},
		},
	}

	ids := service.GetAllObjectiveIDs(structure)
	assert.Equal(t, []string{"obj1", "obj2", "obj3"}, ids)
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
