package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGameStructureService(
	t *testing.T,
) (*services.GameStructureService, services.LocationService, services.BlockService, services.InstanceService, services.UserService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	// Initialize repositories
	instanceRepo := repositories.NewInstanceRepository(dbc)
	instanceSettingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	userRepo := repositories.NewUserRepository(dbc)

	// Initialize services
	gameStructureService := services.NewGameStructureService(locationRepo, instanceRepo)
	markerService := services.NewMarkerService(markerRepo)
	locationService := services.NewLocationService(locationRepo, markerRepo, blockRepo, markerService)
	blockService := services.NewBlockService(blockRepo, blockStateRepo)
	instanceService := services.NewInstanceService(instanceRepo, instanceSettingsRepo)
	userService := services.NewUserService(userRepo, instanceRepo)

	// Set up relation loader
	gameStructureService.SetRelationLoader(locationRepo)

	return gameStructureService, locationService, *blockService, *instanceService, *userService, cleanup
}

func TestGameStructureService_LoadBlocksForStructure(t *testing.T) {
	service, locationService, blockService, instanceService, userService, cleanup := setupGameStructureService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Load blocks for simple structure", func(t *testing.T) {
		// Create user
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		err := userService.CreateUser(ctx, user, "password")
		require.NoError(t, err)

		// Create instance
		instance, err := instanceService.CreateInstance(ctx, "Test Instance", user)
		require.NoError(t, err)

		// Create locations with blocks
		loc1, err := locationService.CreateLocation(ctx, instance.ID, "Location 1", 0, 0, 0)
		require.NoError(t, err)

		// Add block to location 1
		_, err = blockService.NewBlockWithOwnerAndContext(ctx, loc1.ID, blocks.ContextLocationClues, "markdown")
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
		assert.Equal(t, blocks.ContextLocationClues, structure.Locations[0].Blocks[1].Context)
		assert.Equal(t, "markdown", structure.Locations[0].Blocks[1].Type)
	})

	t.Run("Load blocks recursively for nested structure", func(t *testing.T) {
		// Create user
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		err := userService.CreateUser(ctx, user, "password")
		require.NoError(t, err)

		// Create instance
		instance, err := instanceService.CreateInstance(ctx, "Test Instance 2", user)
		require.NoError(t, err)

		// Create locations
		loc1, err := locationService.CreateLocation(ctx, instance.ID, "Location 1", 0, 0, 0)
		require.NoError(t, err)
		loc2, err := locationService.CreateLocation(ctx, instance.ID, "Location 2", 0, 0, 0)
		require.NoError(t, err)

		// Add blocks
		_, err = blockService.NewBlockWithOwnerAndContext(ctx, loc1.ID, blocks.ContextLocationClues, "markdown")
		require.NoError(t, err)
		_, err = blockService.NewBlockWithOwnerAndContext(ctx, loc2.ID, blocks.ContextLocationContent, "markdown")
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
		instance, err := instanceService.CreateInstance(ctx, "Test Instance 3", user)
		require.NoError(t, err)

		// Create locations
		loc1, err := locationService.CreateLocation(ctx, instance.ID, "Location 1", 0, 0, 0)
		require.NoError(t, err)
		loc2, err := locationService.CreateLocation(ctx, instance.ID, "Location 2", 0, 0, 0)
		require.NoError(t, err)

		// Add blocks
		_, err = blockService.NewBlockWithOwnerAndContext(ctx, loc1.ID, blocks.ContextLocationClues, "markdown")
		require.NoError(t, err)
		_, err = blockService.NewBlockWithOwnerAndContext(ctx, loc2.ID, blocks.ContextLocationContent, "markdown")
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
	service, locationService, _, instanceService, userService, cleanup := setupGameStructureService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Orphaned locations are added to root group", func(t *testing.T) {
		// Create user
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		err := userService.CreateUser(ctx, user, "password")
		require.NoError(t, err)

		// Create instance
		instance, err := instanceService.CreateInstance(ctx, "Test Instance", user)
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
		reloaded, err := instanceService.GetByID(ctx, instance.ID)
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
		instance, err := instanceService.CreateInstance(ctx, "Test Instance 2", user)
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
		reloaded, err := instanceService.GetByID(ctx, instance.ID)
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
		instance, err := instanceService.CreateInstance(ctx, "Test Instance 3", user)
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
		reloaded, err := instanceService.GetByID(ctx, instance.ID)
		require.NoError(t, err)

		// Structure should be unchanged
		assert.Len(t, reloaded.GameStructure.LocationIDs, 2)
		assert.Equal(t, loc1.ID, reloaded.GameStructure.LocationIDs[0], "Order should be preserved")
		assert.Equal(t, loc2.ID, reloaded.GameStructure.LocationIDs[1], "Order should be preserved")
	})
}

func TestGameStructureService_Save_Integration(t *testing.T) {
	service, locationService, _, instanceService, userService, cleanup := setupGameStructureService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Save validates structure before saving", func(t *testing.T) {
		// Create user
		user := &models.User{Email: gofakeit.Email(), Password: "password"}
		err := userService.CreateUser(ctx, user, "password")
		require.NoError(t, err)

		// Create instance
		instance, err := instanceService.CreateInstance(ctx, "Test Instance", user)
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
		instance, err := instanceService.CreateInstance(ctx, "Test Instance 2", user)
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
		instance, err := instanceService.CreateInstance(ctx, "Test Instance 3", user)
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
		reloaded, err := instanceService.GetByID(ctx, instance.ID)
		require.NoError(t, err)
		assert.Len(t, reloaded.GameStructure.SubGroups, 1)
		assert.Equal(t, "Group 1", reloaded.GameStructure.SubGroups[0].Name)
	})
}
