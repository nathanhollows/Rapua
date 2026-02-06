package services_test

import (
	"context"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupNavigationService(t *testing.T) (
	*services.NavigationService,
	repositories.LocationRepository,
	repositories.TeamRepository,
	repositories.CheckInRepository,
	repositories.InstanceRepository,
	*bun.DB,
	func(),
) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	locationRepo := repositories.NewLocationRepository(dbc)
	teamRepo := repositories.NewTeamRepository(dbc)
	checkInRepo := repositories.NewCheckInRepository(dbc)
	instanceRepo := repositories.NewInstanceRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)

	gameStructureService := services.NewGameStructureService(locationRepo, instanceRepo)
	markerService := services.NewMarkerService(markerRepo)
	blockService := services.NewBlockService(blockRepo, blockStateRepo)
	locationService := services.NewLocationService(locationRepo, markerRepo, blockRepo, markerService)
	gameStructureService.SetRelationLoader(locationService)

	navigationService := services.NewNavigationService(
		locationRepo,
		teamRepo,
		gameStructureService,
		blockService,
	)

	return navigationService, locationRepo, teamRepo, checkInRepo, instanceRepo, dbc, cleanup
}

// createTestGameStructure creates a game structure for testing with 3 groups.
func createTestGameStructure() models.GameStructure {
	return models.GameStructure{
		ID:     gofakeit.UUID(),
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             gofakeit.UUID(),
				Name:           "Group 1",
				Color:          "blue",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				Routing:        models.RouteStrategyFreeRoam,
				Navigation:     models.NavigationDisplayNames,
				LocationIDs:    []string{}, // Will be filled with actual location IDs
			},
			{
				ID:              gofakeit.UUID(),
				Name:            "Group 2",
				Color:           "green",
				CompletionType:  models.CompletionMinimum,
				MinimumRequired: 2,
				AutoAdvance:     false, // Key: can advance early
				Routing:         models.RouteStrategyFreeRoam,
				Navigation:      models.NavigationDisplayNames,
				LocationIDs:     []string{}, // Will be filled
			},
			{
				ID:             gofakeit.UUID(),
				Name:           "Group 3",
				Color:          "red",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				Routing:        models.RouteStrategyFreeRoam,
				Navigation:     models.NavigationDisplayNames,
				LocationIDs:    []string{}, // Will be filled
			},
		},
	}
}

func TestNavigationService_GetPlayerNavigationView_TeamBlocked(t *testing.T) {
	navService, locationRepo, teamRepo, _, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create instance with game structure
	instance := &models.Instance{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        gofakeit.UUID(),
		GameStructure: createTestGameStructure(),
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	// Create instance settings
	settings := &models.InstanceSettings{
		InstanceID: instance.ID,
	}
	settingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create blocking location
	blockingLocation := &models.Location{
		InstanceID: instance.ID,
		Name:       "Blocking Location",
		MarkerID:   gofakeit.UUID(),
	}
	err = locationRepo.Create(ctx, blockingLocation)
	require.NoError(t, err)

	// Create team with MustCheckOut set
	team := models.Team{
		ID:           gofakeit.UUID(),
		Code:         strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:         "Test Team",
		InstanceID:   instance.ID,
		MustCheckOut: blockingLocation.ID, // Use location.ID
	}
	err = teamRepo.InsertBatch(ctx, []models.Team{team})
	require.NoError(t, err)

	// Load team with relations
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute
	view, err := navService.GetPlayerNavigationView(ctx, teamPtr)

	// Assert
	require.NoError(t, err)
	assert.True(t, view.MustCheckOut)
	assert.NotNil(t, view.BlockingLocation)
	assert.Equal(t, blockingLocation.ID, view.BlockingLocation.ID)
	assert.Empty(t, view.NextLocations)
	assert.False(t, view.CanAdvanceEarly)
}

func TestNavigationService_GetPlayerNavigationView_FirstGroup(t *testing.T) {
	navService, locationRepo, teamRepo, _, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create game structure
	gameStructure := createTestGameStructure()

	// Create instance
	instance := &models.Instance{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        gofakeit.UUID(),
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	// Create instance settings
	settings := &models.InstanceSettings{
		InstanceID: instance.ID,
	}
	settingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create locations for group 1
	loc1 := &models.Location{
		InstanceID: instance.ID,
		Name:       "Location 1",
		MarkerID:   gofakeit.UUID(),
	}
	loc2 := &models.Location{
		InstanceID: instance.ID,
		Name:       "Location 2",
		MarkerID:   gofakeit.UUID(),
	}
	err = locationRepo.Create(ctx, loc1)
	require.NoError(t, err)
	err = locationRepo.Create(ctx, loc2)
	require.NoError(t, err)

	// Update game structure with location IDs
	instance.GameStructure.SubGroups[0].LocationIDs = []string{loc1.ID, loc2.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team with no check-ins
	team := models.Team{
		ID:         gofakeit.UUID(),
		Code:       strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:       "Test Team",
		InstanceID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Team{team})
	require.NoError(t, err)

	// Load team
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute
	view, err := navService.GetPlayerNavigationView(ctx, teamPtr)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, view.CurrentGroup)
	assert.Equal(t, instance.GameStructure.SubGroups[0].ID, view.CurrentGroup.ID)
	assert.False(t, view.MustCheckOut)
	assert.False(t, view.CanAdvanceEarly) // Not minimum met yet
	assert.Len(t, view.NextLocations, 2)
}

func TestNavigationService_GetPlayerNavigationView_CanAdvanceEarly(t *testing.T) {
	navService, locationRepo, teamRepo, checkInRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create game structure
	gameStructure := createTestGameStructure()

	// Create instance
	instance := &models.Instance{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        gofakeit.UUID(),
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	// Create instance settings
	settings := &models.InstanceSettings{
		InstanceID: instance.ID,
	}
	settingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create locations for group 1 and 2
	group1Locs := []*models.Location{}
	for range 2 {
		loc := &models.Location{
			InstanceID: instance.ID,
			Name:       gofakeit.StreetName(),
			MarkerID:   gofakeit.UUID(),
		}
		err = locationRepo.Create(ctx, loc)
		require.NoError(t, err)
		group1Locs = append(group1Locs, loc)
	}

	group2Locs := []*models.Location{}
	for range 3 {
		loc := &models.Location{
			InstanceID: instance.ID,
			Name:       gofakeit.StreetName(),
			MarkerID:   gofakeit.UUID(),
		}
		err = locationRepo.Create(ctx, loc)
		require.NoError(t, err)
		group2Locs = append(group2Locs, loc)
	}

	// Update game structure
	instance.GameStructure.SubGroups[0].LocationIDs = []string{group1Locs[0].ID, group1Locs[1].ID}
	instance.GameStructure.SubGroups[1].LocationIDs = []string{group2Locs[0].ID, group2Locs[1].ID, group2Locs[2].ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team
	team := models.Team{
		ID:         gofakeit.UUID(),
		Code:       strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:       "Test Team",
		InstanceID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Team{team})
	require.NoError(t, err)

	// Create check-ins for group 1 (complete) and group 2 (minimum met: 2/3)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group1Locs[0], false, false)
	require.NoError(t, err)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group1Locs[1], false, false)
	require.NoError(t, err)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group2Locs[0], false, false)
	require.NoError(t, err)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group2Locs[1], false, false)
	require.NoError(t, err)

	// Load team
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute
	view, err := navService.GetPlayerNavigationView(ctx, teamPtr)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, view.CurrentGroup)
	assert.Equal(t, instance.GameStructure.SubGroups[1].ID, view.CurrentGroup.ID, "should be in group 2")
	assert.True(t, view.CanAdvanceEarly, "minimum met (2/3), AutoAdvance=false, not 100%")
	assert.Len(t, view.NextLocations, 1, "only one location left in group 2")
}

func TestNavigationService_GetPlayerNavigationView_SkippedGroup(t *testing.T) {
	navService, locationRepo, teamRepo, checkInRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create game structure
	gameStructure := createTestGameStructure()

	// Create instance
	instance := &models.Instance{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        gofakeit.UUID(),
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	// Create instance settings
	settings := &models.InstanceSettings{
		InstanceID: instance.ID,
	}
	settingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create locations for all groups
	group1Locs := []*models.Location{}
	for range 2 {
		loc := &models.Location{
			InstanceID: instance.ID,
			Name:       gofakeit.StreetName(),
			MarkerID:   gofakeit.UUID(),
		}
		err = locationRepo.Create(ctx, loc)
		require.NoError(t, err)
		group1Locs = append(group1Locs, loc)
	}

	group2Locs := []*models.Location{}
	for range 3 {
		loc := &models.Location{
			InstanceID: instance.ID,
			Name:       gofakeit.StreetName(),
			MarkerID:   gofakeit.UUID(),
		}
		err = locationRepo.Create(ctx, loc)
		require.NoError(t, err)
		group2Locs = append(group2Locs, loc)
	}

	group3Loc := &models.Location{
		InstanceID: instance.ID,
		Name:       gofakeit.StreetName(),
		MarkerID:   gofakeit.UUID(),
	}
	err = locationRepo.Create(ctx, group3Loc)
	require.NoError(t, err)

	// Update game structure
	instance.GameStructure.SubGroups[0].LocationIDs = []string{group1Locs[0].ID, group1Locs[1].ID}
	instance.GameStructure.SubGroups[1].LocationIDs = []string{group2Locs[0].ID, group2Locs[1].ID, group2Locs[2].ID}
	instance.GameStructure.SubGroups[2].LocationIDs = []string{group3Loc.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team with group2 skipped
	team := models.Team{
		ID:              gofakeit.UUID(),
		Code:            strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:            "Test Team",
		InstanceID:      instance.ID,
		SkippedGroupIDs: []string{instance.GameStructure.SubGroups[1].ID}, // Skip group 2
	}
	err = teamRepo.InsertBatch(ctx, []models.Team{team})
	require.NoError(t, err)

	// Create check-ins for group 1 (complete) and group 2 (minimum met but will be skipped)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group1Locs[0], false, false)
	require.NoError(t, err)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group1Locs[1], false, false)
	require.NoError(t, err)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group2Locs[0], false, false)
	require.NoError(t, err)
	_, err = checkInRepo.LogCheckIn(ctx, team, *group2Locs[1], false, false)
	require.NoError(t, err)

	// Load team
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute
	view, err := navService.GetPlayerNavigationView(ctx, teamPtr)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, view.CurrentGroup)
	assert.Equal(t, instance.GameStructure.SubGroups[2].ID, view.CurrentGroup.ID, "should skip to group 3")
	assert.False(t, view.CanAdvanceEarly, "group 3 has AutoAdvance=true")
	assert.Len(t, view.NextLocations, 1)
}

func TestNavigationService_GetPreviewNavigationView_Success(t *testing.T) {
	navService, locationRepo, teamRepo, _, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create game structure
	gameStructure := createTestGameStructure()

	// Create instance
	instance := &models.Instance{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        gofakeit.UUID(),
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	// Create instance settings
	settings := &models.InstanceSettings{
		InstanceID: instance.ID,
	}
	settingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create location for group 1
	location := &models.Location{
		InstanceID: instance.ID,
		Name:       "Preview Location",
		MarkerID:   gofakeit.UUID(),
	}
	err = locationRepo.Create(ctx, location)
	require.NoError(t, err)

	// Update game structure with location ID
	instance.GameStructure.SubGroups[0].LocationIDs = []string{location.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team
	team := models.Team{
		ID:         gofakeit.UUID(),
		Code:       strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:       "Test Team",
		InstanceID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Team{team})
	require.NoError(t, err)

	// Load team
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute
	view, err := navService.GetPreviewNavigationView(ctx, teamPtr, location.ID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, view)
	assert.NotNil(t, view.CurrentGroup)
	assert.Equal(t, instance.GameStructure.SubGroups[0].ID, view.CurrentGroup.ID)
	assert.Len(t, view.NextLocations, 1)
	assert.Equal(t, location.ID, view.NextLocations[0].ID)
	assert.False(t, view.MustCheckOut)
	assert.False(t, view.CanAdvanceEarly)
}

func TestNavigationService_GetPreviewNavigationView_LocationNotFound(t *testing.T) {
	navService, _, teamRepo, _, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create game structure
	gameStructure := createTestGameStructure()

	// Create instance
	instance := &models.Instance{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        gofakeit.UUID(),
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	// Create instance settings
	settings := &models.InstanceSettings{
		InstanceID: instance.ID,
	}
	settingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create team
	team := models.Team{
		ID:         gofakeit.UUID(),
		Code:       strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:       "Test Team",
		InstanceID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Team{team})
	require.NoError(t, err)

	// Load team
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute with non-existent location ID
	_, err = navService.GetPreviewNavigationView(ctx, teamPtr, "non-existent-id")

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "location not found in game structure")
}

func TestNavigationService_GetPreviewNavigationView_CustomNavigationMode(t *testing.T) {
	navService, locationRepo, teamRepo, _, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create game structure with custom navigation mode
	gameStructure := createTestGameStructure()
	gameStructure.SubGroups[0].Navigation = models.NavigationDisplayCustom

	// Create instance
	instance := &models.Instance{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        gofakeit.UUID(),
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	// Create instance settings
	settings := &models.InstanceSettings{
		InstanceID: instance.ID,
	}
	settingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create location for group 1
	location := &models.Location{
		InstanceID: instance.ID,
		Name:       "Preview Location",
		MarkerID:   gofakeit.UUID(),
	}
	err = locationRepo.Create(ctx, location)
	require.NoError(t, err)

	// Update game structure with location ID
	instance.GameStructure.SubGroups[0].LocationIDs = []string{location.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team
	team := models.Team{
		ID:         gofakeit.UUID(),
		Code:       strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:       "Test Team",
		InstanceID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Team{team})
	require.NoError(t, err)

	// Load team
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute
	view, err := navService.GetPreviewNavigationView(ctx, teamPtr, location.ID)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, view)
	assert.NotNil(t, view.CurrentGroup)
	assert.Equal(t, models.NavigationDisplayCustom, view.CurrentGroup.Navigation)
	// Blocks and BlockStates should be initialized (may be empty if no blocks exist)
	assert.NotNil(t, view.Blocks)
	assert.NotNil(t, view.BlockStates)
}

func TestNavigationService_GetPreviewNavigationView_FirstGroupDefault(t *testing.T) {
	navService, locationRepo, teamRepo, _, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create game structure with multiple groups
	gameStructure := createTestGameStructure()

	// Create instance
	instance := &models.Instance{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        gofakeit.UUID(),
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	// Create instance settings
	settings := &models.InstanceSettings{
		InstanceID: instance.ID,
	}
	settingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create locations for all three groups
	group1Loc := &models.Location{
		InstanceID: instance.ID,
		Name:       "Group 1 Location",
		MarkerID:   gofakeit.UUID(),
	}
	err = locationRepo.Create(ctx, group1Loc)
	require.NoError(t, err)

	group2Loc := &models.Location{
		InstanceID: instance.ID,
		Name:       "Group 2 Location",
		MarkerID:   gofakeit.UUID(),
	}
	err = locationRepo.Create(ctx, group2Loc)
	require.NoError(t, err)

	group3Loc := &models.Location{
		InstanceID: instance.ID,
		Name:       "Group 3 Location",
		MarkerID:   gofakeit.UUID(),
	}
	err = locationRepo.Create(ctx, group3Loc)
	require.NoError(t, err)

	// Update game structure with location IDs
	instance.GameStructure.SubGroups[0].LocationIDs = []string{group1Loc.ID}
	instance.GameStructure.SubGroups[1].LocationIDs = []string{group2Loc.ID}
	instance.GameStructure.SubGroups[2].LocationIDs = []string{group3Loc.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team
	team := models.Team{
		ID:         gofakeit.UUID(),
		Code:       strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:       "Test Team",
		InstanceID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Team{team})
	require.NoError(t, err)

	// Load team
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute preview for first group's first location
	view, err := navService.GetPreviewNavigationView(ctx, teamPtr, group1Loc.ID)

	// Assert - should show first group (SubGroups[0]), not root or current team state
	require.NoError(t, err)
	assert.NotNil(t, view)
	assert.NotNil(t, view.CurrentGroup)
	assert.Equal(t, instance.GameStructure.SubGroups[0].ID, view.CurrentGroup.ID, "should show first group")
	assert.Equal(t, "Group 1", view.CurrentGroup.Name)
	assert.Len(t, view.NextLocations, 1)
	assert.Equal(t, group1Loc.ID, view.NextLocations[0].ID)
}

func TestNavigationService_GetPlayerNavigationView_AllLocationsVisited(t *testing.T) {
	navService, locationRepo, teamRepo, checkInRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create a simple game structure with one group
	gameStructure := models.GameStructure{
		ID:     gofakeit.UUID(),
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             gofakeit.UUID(),
				Name:           "Only Group",
				Color:          "blue",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				Routing:        models.RouteStrategyFreeRoam,
				Navigation:     models.NavigationDisplayNames,
				LocationIDs:    []string{},
			},
		},
	}

	// Create instance
	instance := &models.Instance{
		ID:            gofakeit.UUID(),
		Name:          "Test Game",
		UserID:        gofakeit.UUID(),
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	// Create instance settings
	settings := &models.InstanceSettings{
		InstanceID: instance.ID,
	}
	settingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create two locations
	loc1 := &models.Location{
		InstanceID: instance.ID,
		Name:       "Location 1",
		MarkerID:   strings.ToUpper(gofakeit.LetterN(5)),
	}
	loc2 := &models.Location{
		InstanceID: instance.ID,
		Name:       "Location 2",
		MarkerID:   strings.ToUpper(gofakeit.LetterN(5)),
	}
	err = locationRepo.Create(ctx, loc1)
	require.NoError(t, err)
	err = locationRepo.Create(ctx, loc2)
	require.NoError(t, err)

	// Update game structure with location IDs
	instance.GameStructure.SubGroups[0].LocationIDs = []string{loc1.ID, loc2.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team
	team := models.Team{
		ID:         gofakeit.UUID(),
		Code:       strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:       "Test Team",
		InstanceID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Team{team})
	require.NoError(t, err)

	// Create check-ins for ALL locations (completing the game)
	_, err = checkInRepo.LogCheckIn(ctx, team, *loc1, false, false)
	require.NoError(t, err)
	_, err = checkInRepo.LogCheckIn(ctx, team, *loc2, false, false)
	require.NoError(t, err)

	// Load team with relations
	teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
	require.NoError(t, err)
	err = teamRepo.LoadRelations(ctx, teamPtr)
	require.NoError(t, err)

	// Execute
	view, err := navService.GetPlayerNavigationView(ctx, teamPtr)

	// Assert - should return ErrAllLocationsVisited
	require.Error(t, err)
	require.ErrorIs(t, err, services.ErrAllLocationsVisited)
	assert.Nil(t, view)
}

func TestNavigationService_GetPlayerNavigationView_ScavengerHuntMode(t *testing.T) {
	navService, locationRepo, teamRepo, checkInRepo, instanceRepo, dbc, cleanup := setupNavigationService(t)
	defer cleanup()
	ctx := context.Background()

	// Create a game structure with scavenger hunt navigation mode
	gameStructure := models.GameStructure{
		ID:     gofakeit.UUID(),
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             gofakeit.UUID(),
				Name:           "Scavenger Hunt Group",
				Color:          "blue",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				Routing:        models.RouteStrategyFreeRoam,
				Navigation:     models.NavigationDisplayTasks, // Key: scavenger hunt mode
				LocationIDs:    []string{},
			},
		},
	}

	// Create instance
	instance := &models.Instance{
		ID:            gofakeit.UUID(),
		Name:          "Scavenger Hunt Game",
		UserID:        gofakeit.UUID(),
		GameStructure: gameStructure,
	}
	err := instanceRepo.Create(ctx, instance)
	require.NoError(t, err)

	// Create instance settings
	settings := &models.InstanceSettings{
		InstanceID: instance.ID,
	}
	settingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	err = settingsRepo.Create(ctx, settings)
	require.NoError(t, err)

	// Create three locations (tasks)
	loc1 := &models.Location{
		InstanceID: instance.ID,
		Name:       "Photograph the fountain",
		MarkerID:   strings.ToUpper(gofakeit.LetterN(5)),
		Points:     10,
	}
	loc2 := &models.Location{
		InstanceID: instance.ID,
		Name:       "Take a group photo",
		MarkerID:   strings.ToUpper(gofakeit.LetterN(5)),
		Points:     20,
	}
	loc3 := &models.Location{
		InstanceID: instance.ID,
		Name:       "Find something blue",
		MarkerID:   strings.ToUpper(gofakeit.LetterN(5)),
		Points:     15,
	}
	err = locationRepo.Create(ctx, loc1)
	require.NoError(t, err)
	err = locationRepo.Create(ctx, loc2)
	require.NoError(t, err)
	err = locationRepo.Create(ctx, loc3)
	require.NoError(t, err)

	// Update game structure with location IDs
	instance.GameStructure.SubGroups[0].LocationIDs = []string{loc1.ID, loc2.ID, loc3.ID}
	err = instanceRepo.Update(ctx, instance)
	require.NoError(t, err)

	// Create team
	team := models.Team{
		ID:         gofakeit.UUID(),
		Code:       strings.ToUpper(gofakeit.Password(false, true, false, false, false, 4)),
		Name:       "Test Team",
		InstanceID: instance.ID,
	}
	err = teamRepo.InsertBatch(ctx, []models.Team{team})
	require.NoError(t, err)

	t.Run("all locations uncompleted", func(t *testing.T) {
		// Load fresh team
		//nolint:govet // Shadow variable in test subtest
		teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
		require.NoError(t, err)
		err = teamRepo.LoadRelations(ctx, teamPtr)
		require.NoError(t, err)

		// Execute
		view, err := navService.GetPlayerNavigationView(ctx, teamPtr)

		// Assert
		require.NoError(t, err)
		assert.NotNil(t, view)
		assert.Equal(t, models.NavigationDisplayTasks, view.CurrentGroup.Navigation)
		assert.Len(t, view.NextLocations, 3, "all 3 locations should be uncompleted")
		assert.Empty(t, view.CompletedLocations, "no locations should be completed")
	})

	t.Run("one location completed via check-in", func(t *testing.T) {
		// Check in to loc1 - since location has no blocks, it's immediately complete
		//nolint:govet // Shadow variable in test subtest
		_, err := checkInRepo.LogCheckIn(ctx, team, *loc1, false, false)
		require.NoError(t, err)

		// Load fresh team
		teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
		require.NoError(t, err)
		err = teamRepo.LoadRelations(ctx, teamPtr)
		require.NoError(t, err)

		// Execute
		view, err := navService.GetPlayerNavigationView(ctx, teamPtr)

		// Assert - location with no blocks is immediately complete on check-in
		require.NoError(t, err)
		assert.Len(t, view.NextLocations, 2, "2 locations should be uncompleted")
		assert.Len(t, view.CompletedLocations, 1, "1 location should be completed")
	})

	t.Run("two locations completed", func(t *testing.T) {
		// Check in to loc2 - also immediately complete (no blocks)
		//nolint:govet // Shadow variable in test subtest
		_, err := checkInRepo.LogCheckIn(ctx, team, *loc2, false, false)
		require.NoError(t, err)

		// Load fresh team
		teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
		require.NoError(t, err)
		err = teamRepo.LoadRelations(ctx, teamPtr)
		require.NoError(t, err)

		// Execute
		view, err := navService.GetPlayerNavigationView(ctx, teamPtr)

		// Assert
		require.NoError(t, err)
		assert.Len(t, view.NextLocations, 1, "1 location should be uncompleted")
		assert.Len(t, view.CompletedLocations, 2, "2 locations should be completed")
	})

	t.Run("all locations completed", func(t *testing.T) {
		// Check in to loc3 - immediately complete (no blocks)
		//nolint:govet // Shadow variable in test subtest
		_, err := checkInRepo.LogCheckIn(ctx, team, *loc3, false, false)
		require.NoError(t, err)

		// Load fresh team
		teamPtr, err := teamRepo.GetByCode(ctx, team.Code)
		require.NoError(t, err)
		err = teamRepo.LoadRelations(ctx, teamPtr)
		require.NoError(t, err)

		// Execute
		view, err := navService.GetPlayerNavigationView(ctx, teamPtr)

		// Assert - all completed, scavenger hunt mode returns the view (not ErrAllLocationsVisited)
		require.NoError(t, err)
		assert.Empty(t, view.NextLocations, "no locations should be uncompleted")
		assert.Len(t, view.CompletedLocations, 3, "all 3 locations should be completed")
	})
}
