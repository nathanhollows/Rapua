package services_test

import (
	"context"
	"database/sql"
	"log/slog"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupDuplicationService(t *testing.T) (
	*services.DuplicationService,
	db.Transactor,
	repositories.QuestRepository,
	repositories.QuestSettingsRepository,
	repositories.LocationRepository,
	repositories.ObjectiveRepository,
	repositories.BlockRepository,
	*bun.DB,
	func(),
) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	instanceRepo := repositories.NewQuestRepository(dbc)
	instanceSettingsRepo := repositories.NewQuestSettingsRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)

	duplicationService := services.NewDuplicationService(
		slog.Default(),
		transactor,
		instanceRepo,
		instanceSettingsRepo,
		locationRepo,
		objectiveRepo,
		blockRepo,
	)

	return duplicationService, transactor, instanceRepo, instanceSettingsRepo,
		locationRepo, objectiveRepo, blockRepo, dbc, cleanup
}

func TestDuplicationService_DuplicateQuest(t *testing.T) {
	svc, transactor, instanceRepo, settingsRepo, locationRepo, objectiveRepo, blockRepo, dbc, cleanup := setupDuplicationService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("successfully duplicates instance with locations and blocks", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, user.ID)

		// Create source instance
		sourceInstance := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, sourceInstance)
		require.NoError(t, err)

		// Create settings
		settings := &models.QuestSettings{
			QuestID:         sourceInstance.ID,
			EnablePoints:    true,
			ShowLeaderboard: true,
		}
		err = settingsRepo.Create(ctx, settings)
		require.NoError(t, err)

		// Create location with blocks
		markerCode := gofakeit.LetterN(5)
		insertTestMarker(t, dbc, markerCode)
		location := &models.Location{
			Name:     gofakeit.Word(),
			QuestID:  sourceInstance.ID,
			MarkerID: markerCode,
			Points:   100,
		}
		err = locationRepo.Create(ctx, location)
		require.NoError(t, err)

		// Create blocks for the location
		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		err = blockRepo.DuplicateBlocksByOwnerTx(ctx, tx, "template-id", location.ID)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		objective := &models.Objective{
			QuestID: sourceInstance.ID,
			Slug:    gofakeit.Word() + "-objective",
			Title:   gofakeit.Sentence(3),
		}
		tx, err = transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		err = objectiveRepo.CreateTx(ctx, tx, objective)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		tx, err = transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		err = blockRepo.DuplicateBlocksByOwnerTx(ctx, tx, "template-id", objective.ID)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Duplicate the instance
		newName := gofakeit.Word()
		duplicated, err := svc.DuplicateQuest(ctx, user, sourceInstance.ID, newName)
		require.NoError(t, err)

		// Verify instance was duplicated
		assert.NotEqual(t, sourceInstance.ID, duplicated.ID)
		assert.Equal(t, newName, duplicated.Name)
		assert.Equal(t, user.ID, duplicated.UserID)
		assert.False(t, duplicated.IsTemplate)

		// Verify settings were duplicated
		duplicatedSettings, err := settingsRepo.GetByQuestID(ctx, duplicated.ID)
		require.NoError(t, err)
		assert.Equal(t, settings.EnablePoints, duplicatedSettings.EnablePoints)
		assert.Equal(t, settings.ShowLeaderboard, duplicatedSettings.ShowLeaderboard)

		// Verify locations were duplicated
		duplicatedLocations, err := locationRepo.FindByInstance(ctx, duplicated.ID)
		require.NoError(t, err)
		assert.Len(t, duplicatedLocations, 1)
		assert.NotEqual(t, location.ID, duplicatedLocations[0].ID)
		assert.Equal(t, location.Name, duplicatedLocations[0].Name)
		assert.Equal(t, location.Points, duplicatedLocations[0].Points)

		duplicatedObjectives, err := objectiveRepo.FindByQuestID(ctx, duplicated.ID)
		require.NoError(t, err)
		assert.Len(t, duplicatedObjectives, 1)
		assert.NotEqual(t, objective.ID, duplicatedObjectives[0].ID)
		assert.Equal(t, objective.Title, duplicatedObjectives[0].Title)
	})

	t.Run("duplicates game structure with remapped location IDs", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, user.ID)

		// Create source instance with game structure
		sourceInstance := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, sourceInstance)
		require.NoError(t, err)

		// Create locations
		marker1 := gofakeit.LetterN(5)
		insertTestMarker(t, dbc, marker1)
		location1 := &models.Location{
			Name:     "Location 1",
			QuestID:  sourceInstance.ID,
			MarkerID: marker1,
		}
		err = locationRepo.Create(ctx, location1)
		require.NoError(t, err)

		marker2 := gofakeit.LetterN(5)
		insertTestMarker(t, dbc, marker2)
		location2 := &models.Location{
			Name:     "Location 2",
			QuestID:  sourceInstance.ID,
			MarkerID: marker2,
		}
		err = locationRepo.Create(ctx, location2)
		require.NoError(t, err)

		marker3 := gofakeit.LetterN(5)
		insertTestMarker(t, dbc, marker3)
		location3 := &models.Location{
			Name:     "Location 3",
			QuestID:  sourceInstance.ID,
			MarkerID: marker3,
		}
		err = locationRepo.Create(ctx, location3)
		require.NoError(t, err)

		// Create game structure with subgroups
		gameStructure := models.GameStructure{
			ID:          gofakeit.UUID(),
			IsRoot:      true,
			LocationIDs: []string{location1.ID},
			SubGroups: []models.GameStructure{
				{
					ID:          gofakeit.UUID(),
					Name:        "Group 1",
					Color:       "primary",
					LocationIDs: []string{location2.ID},
				},
				{
					ID:          gofakeit.UUID(),
					Name:        "Group 2",
					Color:       "secondary",
					LocationIDs: []string{location3.ID},
				},
			},
		}
		sourceInstance.GameStructure = gameStructure

		// Update instance with game structure
		err = instanceRepo.Update(ctx, sourceInstance)
		require.NoError(t, err)

		// Create settings
		settings := &models.QuestSettings{
			QuestID: sourceInstance.ID,
		}
		err = settingsRepo.Create(ctx, settings)
		require.NoError(t, err)

		// Duplicate the instance
		newName := gofakeit.Word()
		duplicated, err := svc.DuplicateQuest(ctx, user, sourceInstance.ID, newName)
		require.NoError(t, err)

		// Verify game structure was copied
		assert.NotEmpty(t, duplicated.GameStructure.ID)
		assert.True(t, duplicated.GameStructure.IsRoot)
		assert.Len(t, duplicated.GameStructure.SubGroups, 2)

		// Get duplicated locations
		duplicatedLocations, err := locationRepo.FindByInstance(ctx, duplicated.ID)
		require.NoError(t, err)
		require.Len(t, duplicatedLocations, 3)

		// Build a map of duplicated locations by name for verification
		locationsByName := make(map[string]string)
		for _, loc := range duplicatedLocations {
			locationsByName[loc.Name] = loc.ID
		}

		// Verify location IDs were remapped in root group
		assert.Len(t, duplicated.GameStructure.LocationIDs, 1)
		assert.Equal(t, locationsByName["Location 1"], duplicated.GameStructure.LocationIDs[0])
		assert.NotEqual(t, location1.ID, duplicated.GameStructure.LocationIDs[0], "Location ID should be remapped")

		// Verify location IDs were remapped in subgroups
		assert.Equal(t, "Group 1", duplicated.GameStructure.SubGroups[0].Name)
		assert.Len(t, duplicated.GameStructure.SubGroups[0].LocationIDs, 1)
		assert.Equal(t, locationsByName["Location 2"], duplicated.GameStructure.SubGroups[0].LocationIDs[0])
		assert.NotEqual(
			t,
			location2.ID,
			duplicated.GameStructure.SubGroups[0].LocationIDs[0],
			"Location ID should be remapped",
		)

		assert.Equal(t, "Group 2", duplicated.GameStructure.SubGroups[1].Name)
		assert.Len(t, duplicated.GameStructure.SubGroups[1].LocationIDs, 1)
		assert.Equal(t, locationsByName["Location 3"], duplicated.GameStructure.SubGroups[1].LocationIDs[0])
		assert.NotEqual(
			t,
			location3.ID,
			duplicated.GameStructure.SubGroups[1].LocationIDs[0],
			"Location ID should be remapped",
		)
	})

	t.Run("duplicates game structure with remapped objective IDs", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, user.ID)

		sourceInstance := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, sourceInstance)
		require.NoError(t, err)

		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		objective1 := &models.Objective{QuestID: sourceInstance.ID, Slug: "obj-1", Title: "Objective 1"}
		require.NoError(t, objectiveRepo.CreateTx(ctx, tx, objective1))
		objective2 := &models.Objective{QuestID: sourceInstance.ID, Slug: "obj-2", Title: "Objective 2"}
		require.NoError(t, objectiveRepo.CreateTx(ctx, tx, objective2))
		require.NoError(t, tx.Commit())

		sourceInstance.GameStructure = models.GameStructure{
			ID:           gofakeit.UUID(),
			IsRoot:       true,
			ObjectiveIDs: []string{objective1.ID},
			SubGroups: []models.GameStructure{
				{
					ID:           gofakeit.UUID(),
					Name:         "Group 1",
					Color:        "primary",
					ObjectiveIDs: []string{objective2.ID},
				},
			},
		}
		require.NoError(t, instanceRepo.Update(ctx, sourceInstance))

		settings := &models.QuestSettings{QuestID: sourceInstance.ID}
		require.NoError(t, settingsRepo.Create(ctx, settings))

		duplicated, err := svc.DuplicateQuest(ctx, user, sourceInstance.ID, gofakeit.Word())
		require.NoError(t, err)

		duplicatedObjectives, err := objectiveRepo.FindByQuestID(ctx, duplicated.ID)
		require.NoError(t, err)
		require.Len(t, duplicatedObjectives, 2)

		objectivesByTitle := make(map[string]string)
		for _, obj := range duplicatedObjectives {
			objectivesByTitle[obj.Title] = obj.ID
		}

		require.Len(t, duplicated.GameStructure.ObjectiveIDs, 1)
		assert.Equal(t, objectivesByTitle["Objective 1"], duplicated.GameStructure.ObjectiveIDs[0])
		assert.NotEqual(t, objective1.ID, duplicated.GameStructure.ObjectiveIDs[0], "Objective ID should be remapped")

		require.Len(t, duplicated.GameStructure.SubGroups[0].ObjectiveIDs, 1)
		assert.Equal(t, objectivesByTitle["Objective 2"], duplicated.GameStructure.SubGroups[0].ObjectiveIDs[0])
		assert.NotEqual(
			t,
			objective2.ID,
			duplicated.GameStructure.SubGroups[0].ObjectiveIDs[0],
			"Objective ID should be remapped",
		)
	})

	t.Run("rejects template duplication", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, user.ID)

		// Create template instance
		template := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: true,
		}
		err := instanceRepo.Create(ctx, template)
		require.NoError(t, err)

		// Try to duplicate template
		_, err = svc.DuplicateQuest(ctx, user, template.ID, gofakeit.Word())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot duplicate a template")
	})

	t.Run("validates ownership", func(t *testing.T) {
		user1 := &models.User{ID: gofakeit.UUID()}
		user2 := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, user1.ID)

		// Create instance owned by user1
		instance := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     user1.ID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, instance)
		require.NoError(t, err)

		// Try to duplicate as user2
		_, err = svc.DuplicateQuest(ctx, user2, instance.ID, gofakeit.Word())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not authenticated")
	})

	t.Run("validates user is not nil", func(t *testing.T) {
		_, err := svc.DuplicateQuest(ctx, nil, gofakeit.UUID(), gofakeit.Word())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not authenticated")
	})
}

func TestDuplicationService_CreateTemplateFromQuest(t *testing.T) {
	svc, _, instanceRepo, settingsRepo, _, _, _, dbc, cleanup := setupDuplicationService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("successfully creates template from instance", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, user.ID)

		// Create source instance
		sourceInstance := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, sourceInstance)
		require.NoError(t, err)

		// Create settings
		settings := &models.QuestSettings{
			QuestID:      sourceInstance.ID,
			EnablePoints: true,
		}
		err = settingsRepo.Create(ctx, settings)
		require.NoError(t, err)

		// Create template from instance
		templateName := gofakeit.Word()
		template, err := svc.CreateTemplateFromQuest(ctx, user, sourceInstance.ID, templateName)
		require.NoError(t, err)

		// Verify template
		assert.NotEqual(t, sourceInstance.ID, template.ID)
		assert.Equal(t, templateName, template.Name)
		assert.Equal(t, user.ID, template.UserID)
		assert.True(t, template.IsTemplate)
	})

	t.Run("validates ownership", func(t *testing.T) {
		user1 := &models.User{ID: gofakeit.UUID()}
		user2 := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, user1.ID)

		instance := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     user1.ID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, instance)
		require.NoError(t, err)

		_, err = svc.CreateTemplateFromQuest(ctx, user2, instance.ID, gofakeit.Word())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not authenticated")
	})
}

func TestDuplicationService_CreateQuestFromTemplate(t *testing.T) {
	svc, _, instanceRepo, settingsRepo, _, _, blockRepo, dbc, cleanup := setupDuplicationService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("successfully creates instance from template", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, user.ID)

		// Create template
		template := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: true,
		}
		err := instanceRepo.Create(ctx, template)
		require.NoError(t, err)

		settings := &models.QuestSettings{
			QuestID:      template.ID,
			EnablePoints: false,
		}
		err = settingsRepo.Create(ctx, settings)
		require.NoError(t, err)

		// Create instance from template
		instanceName := gofakeit.Word()
		instance, err := svc.CreateQuestFromTemplate(ctx, user, template.ID, instanceName)
		require.NoError(t, err)

		// Verify instance
		assert.NotEqual(t, template.ID, instance.ID)
		assert.Equal(t, instanceName, instance.Name)
		assert.Equal(t, user.ID, instance.UserID)
		assert.False(t, instance.IsTemplate)
	})

	t.Run("rejects non-template source", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, user.ID)

		instance := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, instance)
		require.NoError(t, err)

		_, err = svc.CreateQuestFromTemplate(ctx, user, instance.ID, gofakeit.Word())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "source is not a template")
	})

	t.Run("copies instance-owned blocks (ContextStart and ContextFinish)", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, user.ID)

		// Create template
		template := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: true,
		}
		err := instanceRepo.Create(ctx, template)
		require.NoError(t, err)

		settings := &models.QuestSettings{
			QuestID: template.ID,
		}
		err = settingsRepo.Create(ctx, settings)
		require.NoError(t, err)

		// Create ContextStart blocks for the template (owner_id = template.ID)
		startBlocks := []blocks.Block{
			&blocks.MarkdownBlock{
				BaseBlock: blocks.BaseBlock{Order: 0},
				Content:   "Welcome to the game!",
			},
			&blocks.MarkdownBlock{
				BaseBlock: blocks.BaseBlock{Order: 1},
				Content:   "These are the rules.",
			},
		}
		err = blockRepo.BulkCreate(ctx, startBlocks, template.ID, blocks.ContextStart)
		require.NoError(t, err)

		// Create ContextFinish blocks for the template (owner_id = template.ID)
		finishBlocks := []blocks.Block{
			&blocks.MarkdownBlock{
				BaseBlock: blocks.BaseBlock{Order: 0},
				Content:   "Congratulations!",
			},
		}
		err = blockRepo.BulkCreate(ctx, finishBlocks, template.ID, blocks.ContextFinish)
		require.NoError(t, err)

		// Verify template has the blocks we created
		templateStartBlocks, err := blockRepo.FindByOwnerIDAndContext(ctx, template.ID, blocks.ContextStart)
		require.NoError(t, err)
		require.Len(t, templateStartBlocks, 2, "template should have 2 ContextStart blocks")

		templateFinishBlocks, err := blockRepo.FindByOwnerIDAndContext(ctx, template.ID, blocks.ContextFinish)
		require.NoError(t, err)
		require.Len(t, templateFinishBlocks, 1, "template should have 1 ContextFinish block")

		// Create instance from template
		instanceName := gofakeit.Word()
		instance, err := svc.CreateQuestFromTemplate(ctx, user, template.ID, instanceName)
		require.NoError(t, err)

		// Verify ContextStart blocks were copied to new instance
		instanceStartBlocks, err := blockRepo.FindByOwnerIDAndContext(ctx, instance.ID, blocks.ContextStart)
		require.NoError(t, err)
		assert.Len(t, instanceStartBlocks, 2, "instance should have 2 ContextStart blocks")

		// Verify ContextFinish blocks were copied to new instance
		instanceFinishBlocks, err := blockRepo.FindByOwnerIDAndContext(ctx, instance.ID, blocks.ContextFinish)
		require.NoError(t, err)
		assert.Len(t, instanceFinishBlocks, 1, "instance should have 1 ContextFinish block")

		// Verify ContextStart blocks have different IDs but same types and order
		for i, templateBlock := range templateStartBlocks {
			instanceBlock := instanceStartBlocks[i]
			assert.NotEqual(t, templateBlock.GetID(), instanceBlock.GetID(), "block IDs should be different")
			assert.Equal(t, templateBlock.GetType(), instanceBlock.GetType(), "block types should match")
			assert.Equal(t, templateBlock.GetOrder(), instanceBlock.GetOrder(), "block order should match")
		}

		// Verify ContextFinish blocks have different IDs but same types and order
		for i, templateBlock := range templateFinishBlocks {
			instanceBlock := instanceFinishBlocks[i]
			assert.NotEqual(t, templateBlock.GetID(), instanceBlock.GetID(), "block IDs should be different")
			assert.Equal(t, templateBlock.GetType(), instanceBlock.GetType(), "block types should match")
			assert.Equal(t, templateBlock.GetOrder(), instanceBlock.GetOrder(), "block order should match")
		}
	})
}

func TestDuplicationService_CreateQuestFromSharedTemplate(t *testing.T) {
	svc, _, instanceRepo, settingsRepo, _, _, _, dbc, cleanup := setupDuplicationService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("successfully creates instance from shared template without ownership check", func(t *testing.T) {
		owner := &models.User{ID: gofakeit.UUID()}
		recipient := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, owner.ID)
		insertTestUser(t, dbc, recipient.ID)

		// Create template owned by owner
		template := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     owner.ID,
			IsTemplate: true,
		}
		err := instanceRepo.Create(ctx, template)
		require.NoError(t, err)

		settings := &models.QuestSettings{
			QuestID: template.ID,
		}
		err = settingsRepo.Create(ctx, settings)
		require.NoError(t, err)

		// Recipient creates instance from shared template
		instanceName := gofakeit.Word()
		instance, err := svc.CreateQuestFromSharedTemplate(ctx, recipient, template.ID, instanceName)
		require.NoError(t, err)

		// Verify instance is owned by recipient
		assert.Equal(t, recipient.ID, instance.UserID)
		assert.Equal(t, instanceName, instance.Name)
		assert.False(t, instance.IsTemplate)
	})

	t.Run("rejects non-template source", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, user.ID)

		ownerID := gofakeit.UUID()
		insertTestUser(t, dbc, ownerID)

		instance := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     ownerID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, instance)
		require.NoError(t, err)

		_, err = svc.CreateQuestFromSharedTemplate(ctx, user, instance.ID, gofakeit.Word())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "source is not a template")
	})
}

func TestDuplicationService_DuplicateLocation(t *testing.T) {
	svc, transactor, _, _, locationRepo, _, blockRepo, dbc, cleanup := setupDuplicationService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("successfully duplicates location with blocks", func(t *testing.T) {
		// Create FK-valid parent chain for source and target instances
		userID := gofakeit.UUID()
		insertTestUser(t, dbc, userID)
		sourceQuestID := insertTestInstance(t, dbc, userID)
		targetQuestID := insertTestInstance(t, dbc, userID)

		// Create source location with valid marker
		markerCode := gofakeit.LetterN(5)
		insertTestMarker(t, dbc, markerCode)
		sourceLocation := models.Location{
			Name:     gofakeit.Word(),
			QuestID:  sourceQuestID,
			MarkerID: markerCode,
			Points:   50,
		}
		err := locationRepo.Create(ctx, &sourceLocation)
		require.NoError(t, err)

		// Add blocks to source location
		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		err = blockRepo.DuplicateBlocksByOwnerTx(ctx, tx, "template-id", sourceLocation.ID)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Duplicate location
		duplicated, err := svc.DuplicateLocation(ctx, sourceLocation, targetQuestID)
		require.NoError(t, err)

		// Verify duplicated location
		assert.NotEqual(t, sourceLocation.ID, duplicated.ID)
		assert.Equal(t, sourceLocation.Name, duplicated.Name)
		assert.Equal(t, targetQuestID, duplicated.QuestID)
		assert.Equal(t, sourceLocation.Points, duplicated.Points)
	})
}

func TestDuplicationService_QuickStartDismissed(t *testing.T) {
	svc, _, instanceRepo, settingsRepo, _, _, _, dbc, cleanup := setupDuplicationService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("DuplicateQuest dismisses quickstart", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, user.ID)

		sourceInstance := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, sourceInstance)
		require.NoError(t, err)

		settings := &models.QuestSettings{QuestID: sourceInstance.ID}
		err = settingsRepo.Create(ctx, settings)
		require.NoError(t, err)

		duplicated, err := svc.DuplicateQuest(ctx, user, sourceInstance.ID, gofakeit.Word())
		require.NoError(t, err)
		assert.True(t, duplicated.IsQuickStartDismissed, "duplicated instance should have quickstart dismissed")
	})

	t.Run("CreateTemplateFromQuest dismisses quickstart", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, user.ID)

		sourceInstance := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, sourceInstance)
		require.NoError(t, err)

		settings := &models.QuestSettings{QuestID: sourceInstance.ID}
		err = settingsRepo.Create(ctx, settings)
		require.NoError(t, err)

		template, err := svc.CreateTemplateFromQuest(ctx, user, sourceInstance.ID, gofakeit.Word())
		require.NoError(t, err)
		assert.True(t, template.IsQuickStartDismissed, "template should have quickstart dismissed")
	})

	t.Run("CreateQuestFromTemplate dismisses quickstart", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, user.ID)

		template := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: true,
		}
		err := instanceRepo.Create(ctx, template)
		require.NoError(t, err)

		settings := &models.QuestSettings{QuestID: template.ID}
		err = settingsRepo.Create(ctx, settings)
		require.NoError(t, err)

		instance, err := svc.CreateQuestFromTemplate(ctx, user, template.ID, gofakeit.Word())
		require.NoError(t, err)
		assert.True(t, instance.IsQuickStartDismissed, "instance from template should have quickstart dismissed")
	})

	t.Run("CreateQuestFromSharedTemplate dismisses quickstart", func(t *testing.T) {
		owner := &models.User{ID: gofakeit.UUID()}
		recipient := &models.User{ID: gofakeit.UUID()}
		insertTestUser(t, dbc, owner.ID)
		insertTestUser(t, dbc, recipient.ID)

		template := &models.Quest{
			Name:       gofakeit.Word(),
			UserID:     owner.ID,
			IsTemplate: true,
		}
		err := instanceRepo.Create(ctx, template)
		require.NoError(t, err)

		settings := &models.QuestSettings{QuestID: template.ID}
		err = settingsRepo.Create(ctx, settings)
		require.NoError(t, err)

		instance, err := svc.CreateQuestFromSharedTemplate(ctx, recipient, template.ID, gofakeit.Word())
		require.NoError(t, err)
		assert.True(t, instance.IsQuickStartDismissed, "instance from shared template should have quickstart dismissed")
	})
}
