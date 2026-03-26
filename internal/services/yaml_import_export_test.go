package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/db"
	"github.com/nathanhollows/Rapua/v7/internal/services"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/nathanhollows/Rapua/v7/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type yamlTestSuite struct {
	ctx          context.Context
	importSvc    *services.YAMLImportService
	exportSvc    *services.YAMLExportService
	instanceRepo repositories.InstanceRepository
	settingsRepo repositories.InstanceSettingsRepository
	locationRepo repositories.LocationRepository
	blockRepo    repositories.BlockRepository
	userID       string
}

func setupYAMLServices(t *testing.T) (*yamlTestSuite, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	instanceRepo := repositories.NewInstanceRepository(dbc)
	instanceSettingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	teamRepo := repositories.NewTeamRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)

	gameStructureService := services.NewGameStructureService(locationRepo, instanceRepo)
	gameStructureService.SetRelationLoader(locationRepo)

	importSvc := services.NewYAMLImportService(
		transactor,
		instanceRepo,
		instanceSettingsRepo,
		locationRepo,
		markerRepo,
		blockRepo,
		teamRepo,
	)
	exportSvc := services.NewYAMLExportService(
		instanceRepo,
		instanceSettingsRepo,
		gameStructureService,
		blockRepo,
	)

	// Create a test user with unique email to avoid collisions in shared in-memory DB
	user := &models.User{Email: gofakeit.Email(), Password: "password"}
	userService := services.NewUserService(userRepo, instanceRepo)
	err := userService.CreateUser(context.Background(), user, "password")
	require.NoError(t, err)

	return &yamlTestSuite{
		ctx:          context.Background(),
		importSvc:    importSvc,
		exportSvc:    exportSvc,
		instanceRepo: instanceRepo,
		settingsRepo: instanceSettingsRepo,
		locationRepo: locationRepo,
		blockRepo:    blockRepo,
		userID:       user.ID,
	}, cleanup
}

// minimalQuestDef returns a minimal valid QuestDef for testing.
func minimalQuestDef() *models.QuestDef {
	return &models.QuestDef{
		Version: 1,
		Name:    "Test Quest",
		Stops: []models.StopDef{
			{Slug: "alpha", Name: "Alpha"},
			{Slug: "beta", Name: "Beta"},
		},
	}
}

func TestYAMLImport_CreateFromMinimalDef(t *testing.T) {
	suite, cleanup := setupYAMLServices(t)
	defer cleanup()

	def := minimalQuestDef()
	instance, warnings, err := suite.importSvc.ImportCreate(suite.ctx, suite.userID, def, false)

	require.NoError(t, err)
	require.NotNil(t, instance)
	assert.Empty(t, warnings)
	assert.Equal(t, "Test Quest", instance.Name)
	assert.Equal(t, suite.userID, instance.UserID)
	assert.False(t, instance.IsTemplate)

	// Verify locations were created
	locations, err := suite.locationRepo.FindByInstance(suite.ctx, instance.ID)
	require.NoError(t, err)
	assert.Len(t, locations, 2)

	slugs := make(map[string]bool)
	for _, loc := range locations {
		slugs[loc.Slug] = true
	}
	assert.True(t, slugs["alpha"])
	assert.True(t, slugs["beta"])
}

func TestYAMLImport_CreateAsTemplate(t *testing.T) {
	suite, cleanup := setupYAMLServices(t)
	defer cleanup()

	def := minimalQuestDef()
	instance, _, err := suite.importSvc.ImportCreate(suite.ctx, suite.userID, def, true)

	require.NoError(t, err)
	assert.True(t, instance.IsTemplate)
}

func TestYAMLImport_CreateWithSettings(t *testing.T) {
	suite, cleanup := setupYAMLServices(t)
	defer cleanup()

	def := minimalQuestDef()
	def.Settings = models.SettingsDef{
		EnablePoints:    true,
		ShowLeaderboard: true,
		MustCheckOut:    true,
	}

	instance, _, err := suite.importSvc.ImportCreate(suite.ctx, suite.userID, def, false)
	require.NoError(t, err)

	settings, err := suite.settingsRepo.GetByInstanceID(suite.ctx, instance.ID)
	require.NoError(t, err)
	assert.True(t, settings.EnablePoints)
	assert.True(t, settings.ShowLeaderboard)
	assert.True(t, settings.MustCheckOut)
}

func TestYAMLImport_CreateWithBlocks(t *testing.T) {
	suite, cleanup := setupYAMLServices(t)
	defer cleanup()

	def := minimalQuestDef()
	def.Stops[0].Content = []models.BlockDef{
		{Type: "text", Fields: map[string]any{"content": "Hello world"}},
	}
	def.Stops[0].Tasks = []models.BlockDef{
		{Type: "task", Fields: map[string]any{"task": "Find the hidden object"}},
	}

	instance, warnings, err := suite.importSvc.ImportCreate(suite.ctx, suite.userID, def, false)
	require.NoError(t, err)
	assert.Empty(t, warnings)

	// Find the alpha location
	locations, err := suite.locationRepo.FindByInstance(suite.ctx, instance.ID)
	require.NoError(t, err)
	var alphaID string
	for _, loc := range locations {
		if loc.Slug == "alpha" {
			alphaID = loc.ID
		}
	}
	require.NotEmpty(t, alphaID, "alpha location not found")

	contentBlocks, err := suite.blockRepo.FindByOwnerIDAndContext(suite.ctx, alphaID, blocks.ContextLocationContent)
	require.NoError(t, err)
	assert.Len(t, contentBlocks, 1)
	assert.Equal(t, "text", contentBlocks[0].GetType())

	taskBlocks, err := suite.blockRepo.FindByOwnerIDAndContext(suite.ctx, alphaID, blocks.ContextTask)
	require.NoError(t, err)
	assert.Len(t, taskBlocks, 1)
	assert.Equal(t, "task", taskBlocks[0].GetType())
}

func TestYAMLImport_CreateWithStartFinishBlocks(t *testing.T) {
	suite, cleanup := setupYAMLServices(t)
	defer cleanup()

	def := minimalQuestDef()
	def.Start = []models.BlockDef{
		{Type: "text", Fields: map[string]any{"content": "Welcome!"}},
	}
	def.Finish = []models.BlockDef{
		{Type: "text", Fields: map[string]any{"content": "Goodbye!"}},
	}

	instance, _, err := suite.importSvc.ImportCreate(suite.ctx, suite.userID, def, false)
	require.NoError(t, err)

	startBlocks, err := suite.blockRepo.FindByOwnerIDAndContext(suite.ctx, instance.ID, blocks.ContextStart)
	require.NoError(t, err)
	assert.Len(t, startBlocks, 1)

	finishBlocks, err := suite.blockRepo.FindByOwnerIDAndContext(suite.ctx, instance.ID, blocks.ContextFinish)
	require.NoError(t, err)
	assert.Len(t, finishBlocks, 1)
}

func TestYAMLImport_CreateWithStages(t *testing.T) {
	suite, cleanup := setupYAMLServices(t)
	defer cleanup()

	def := minimalQuestDef()
	def.Structure.Stages = []models.StageDef{
		{Name: "Stage 1", Stops: []string{"alpha"}},
		{Name: "Stage 2", Stops: []string{"beta"}},
	}

	instance, _, err := suite.importSvc.ImportCreate(suite.ctx, suite.userID, def, false)
	require.NoError(t, err)

	// Verify game structure has sub-groups
	assert.Len(t, instance.GameStructure.SubGroups, 2)
	assert.Equal(t, "Stage 1", instance.GameStructure.SubGroups[0].Name)
	assert.Equal(t, "Stage 2", instance.GameStructure.SubGroups[1].Name)
}

func TestYAMLImport_RejectsInvalidDef(t *testing.T) {
	suite, cleanup := setupYAMLServices(t)
	defer cleanup()

	def := minimalQuestDef()
	def.Version = 99 // invalid

	_, _, err := suite.importSvc.ImportCreate(suite.ctx, suite.userID, def, false)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")
}

func TestYAMLImport_UpdateRequiresID(t *testing.T) {
	suite, cleanup := setupYAMLServices(t)
	defer cleanup()

	def := minimalQuestDef()
	// def.ID is empty — should fail

	_, _, err := suite.importSvc.ImportUpdate(suite.ctx, "some-instance-id", def)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no instance ID")
}

func TestYAMLImport_UpdateRequiresMatchingID(t *testing.T) {
	suite, cleanup := setupYAMLServices(t)
	defer cleanup()

	// First, create an instance via import
	def := minimalQuestDef()
	instance, _, err := suite.importSvc.ImportCreate(suite.ctx, suite.userID, def, false)
	require.NoError(t, err)

	// Try to update using def with a different ID
	def.ID = "wrong-id"
	_, _, err = suite.importSvc.ImportUpdate(suite.ctx, instance.ID, def)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestYAMLExport_TemplateMode(t *testing.T) {
	suite, cleanup := setupYAMLServices(t)
	defer cleanup()

	// Create an instance to export
	def := minimalQuestDef()
	def.Settings = models.SettingsDef{EnablePoints: true}
	instance, _, err := suite.importSvc.ImportCreate(suite.ctx, suite.userID, def, false)
	require.NoError(t, err)

	exported, err := suite.exportSvc.Export(suite.ctx, instance.ID, services.ExportTemplate)
	require.NoError(t, err)
	require.NotNil(t, exported)

	assert.Equal(t, 1, exported.Version)
	assert.Equal(t, "Test Quest", exported.Name)
	assert.Empty(t, exported.ID, "template export should not include instance ID")
	assert.Len(t, exported.Stops, 2)
	assert.True(t, exported.Settings.EnablePoints)

	// Stop IDs should be omitted in template mode
	for _, stop := range exported.Stops {
		assert.Empty(t, stop.ID, "stop ID should be omitted in template mode")
	}
}

func TestYAMLExport_InstanceMode(t *testing.T) {
	suite, cleanup := setupYAMLServices(t)
	defer cleanup()

	def := minimalQuestDef()
	instance, _, err := suite.importSvc.ImportCreate(suite.ctx, suite.userID, def, false)
	require.NoError(t, err)

	exported, err := suite.exportSvc.Export(suite.ctx, instance.ID, services.ExportInstance)
	require.NoError(t, err)

	assert.Equal(t, instance.ID, exported.ID, "instance export should include instance ID")
	assert.Len(t, exported.Stops, 2)

	// Stop IDs should be present in instance mode
	for _, stop := range exported.Stops {
		assert.NotEmpty(t, stop.ID, "stop ID should be included in instance mode")
	}
}

func TestYAMLRoundTrip_TemplateExportCreateImport(t *testing.T) {
	suite, cleanup := setupYAMLServices(t)
	defer cleanup()

	// Create original instance
	original := &models.QuestDef{
		Version: 1,
		Name:    "Round Trip Game",
		Settings: models.SettingsDef{
			EnablePoints:    true,
			ShowLeaderboard: true,
		},
		Stops: []models.StopDef{
			{
				Slug:  "first-stop",
				Name:  "First Stop",
				Points: 10,
				Content: []models.BlockDef{
					{Type: "text", Fields: map[string]any{"content": "Welcome to the first stop!"}},
				},
			},
			{
				Slug:  "second-stop",
				Name:  "Second Stop",
				Tasks: []models.BlockDef{
					{Type: "task", Fields: map[string]any{"task": "Find the hidden object"}},
				},
			},
		},
		Start: []models.BlockDef{
			{Type: "text", Fields: map[string]any{"content": "Game starting!"}},
		},
	}

	instance1, _, err := suite.importSvc.ImportCreate(suite.ctx, suite.userID, original, false)
	require.NoError(t, err)

	// Export as template (no IDs)
	exported, err := suite.exportSvc.Export(suite.ctx, instance1.ID, services.ExportTemplate)
	require.NoError(t, err)

	// Import exported def as a new instance
	instance2, _, err := suite.importSvc.ImportCreate(suite.ctx, suite.userID, exported, false)

	assert.NotEqual(t, instance1.ID, instance2.ID, "should create a new instance")
	assert.Equal(t, instance1.Name, instance2.Name)

	// Verify stop count matches
	locs1, err := suite.locationRepo.FindByInstance(suite.ctx, instance1.ID)
	require.NoError(t, err)
	locs2, err := suite.locationRepo.FindByInstance(suite.ctx, instance2.ID)
	require.NoError(t, err)
	assert.Equal(t, len(locs1), len(locs2))

	// Verify blocks on instance-level contexts
	start1, err := suite.blockRepo.FindByOwnerIDAndContext(suite.ctx, instance1.ID, blocks.ContextStart)
	require.NoError(t, err)
	start2, err := suite.blockRepo.FindByOwnerIDAndContext(suite.ctx, instance2.ID, blocks.ContextStart)
	require.NoError(t, err)
	assert.Equal(t, len(start1), len(start2))
}

func TestYAMLRoundTrip_InstanceExportUpdate(t *testing.T) {
	suite, cleanup := setupYAMLServices(t)
	defer cleanup()

	// Create initial instance
	def := &models.QuestDef{
		Version: 1,
		Name:    "Update Test",
		Stops: []models.StopDef{
			{Slug: "stop-a", Name: "Stop A"},
			{Slug: "stop-b", Name: "Stop B"},
		},
	}

	instance, _, err := suite.importSvc.ImportCreate(suite.ctx, suite.userID, def, false)
	require.NoError(t, err)

	// Export as instance (includes IDs for round-trip)
	exported, err := suite.exportSvc.Export(suite.ctx, instance.ID, services.ExportInstance)
	require.NoError(t, err)
	assert.Equal(t, instance.ID, exported.ID)

	// Modify the exported def
	exported.Name = "Updated Test"
	exported.Stops[0].Name = "Stop A Renamed"
	exported.Stops = append(exported.Stops, models.StopDef{
		Slug: "stop-c",
		Name: "Stop C New",
	})

	// Update the instance
	updated, warnings, err := suite.importSvc.ImportUpdate(suite.ctx, instance.ID, exported)
	require.NoError(t, err)
	assert.Equal(t, "Updated Test", updated.Name)

	// Only warn about unmatched locations if any; stop-c is new so no warning expected
	for _, w := range warnings {
		// No "not in YAML" warnings expected — all original stops were included
		assert.NotContains(t, w.Message, "not in YAML")
	}

	// Verify all 3 stops now exist
	locs, err := suite.locationRepo.FindByInstance(suite.ctx, instance.ID)
	require.NoError(t, err)
	assert.Len(t, locs, 3)

	slugs := make(map[string]string)
	for _, loc := range locs {
		slugs[loc.Slug] = loc.Name
	}
	assert.Equal(t, "Stop A Renamed", slugs["stop-a"])
	assert.Equal(t, "Stop B", slugs["stop-b"])
	assert.Equal(t, "Stop C New", slugs["stop-c"])
}

func TestYAMLImport_UpdateWarnsAboutRemovedStops(t *testing.T) {
	suite, cleanup := setupYAMLServices(t)
	defer cleanup()

	// Create instance with 3 stops
	def := &models.QuestDef{
		Version: 1,
		Name:    "Removal Test",
		Stops: []models.StopDef{
			{Slug: "keep-a", Name: "Keep A"},
			{Slug: "keep-b", Name: "Keep B"},
			{Slug: "remove-c", Name: "Remove C"},
		},
	}

	instance, _, err := suite.importSvc.ImportCreate(suite.ctx, suite.userID, def, false)
	require.NoError(t, err)

	// Export instance (to get IDs)
	exported, err := suite.exportSvc.Export(suite.ctx, instance.ID, services.ExportInstance)
	require.NoError(t, err)

	// Remove stop-c from the export (both stops list and any stage references)
	filtered := make([]models.StopDef, 0, 2)
	for _, s := range exported.Stops {
		if s.Slug != "remove-c" {
			filtered = append(filtered, s)
		}
	}
	exported.Stops = filtered

	// Also remove from stage stops to avoid validation error
	for i := range exported.Structure.Stages {
		stageSlugs := make([]string, 0)
		for _, slug := range exported.Structure.Stages[i].Stops {
			if slug != "remove-c" {
				stageSlugs = append(stageSlugs, slug)
			}
		}
		exported.Structure.Stages[i].Stops = stageSlugs
	}

	_, warnings, err := suite.importSvc.ImportUpdate(suite.ctx, instance.ID, exported)
	require.NoError(t, err)

	// Expect a warning about the removed stop
	hasWarning := false
	for _, w := range warnings {
		if w.Field == "stops" {
			hasWarning = true
		}
	}
	assert.True(t, hasWarning, "expected warning about removed stop")
}
