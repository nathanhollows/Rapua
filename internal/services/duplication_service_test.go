package services_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v5/db"
	"github.com/nathanhollows/Rapua/v5/internal/services"
	"github.com/nathanhollows/Rapua/v5/models"
	"github.com/nathanhollows/Rapua/v5/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDuplicationService(t *testing.T) (
	*services.DuplicationService,
	db.Transactor,
	repositories.InstanceRepository,
	repositories.InstanceSettingsRepository,
	repositories.LocationRepository,
	repositories.BlockRepository,
	func(),
) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	instanceRepo := repositories.NewInstanceRepository(dbc)
	instanceSettingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)

	duplicationService := services.NewDuplicationService(
		transactor,
		instanceRepo,
		instanceSettingsRepo,
		locationRepo,
		blockRepo,
	)

	return duplicationService, transactor, instanceRepo, instanceSettingsRepo, locationRepo, blockRepo, cleanup
}

func TestDuplicationService_DuplicateInstance(t *testing.T) {
	svc, transactor, instanceRepo, settingsRepo, locationRepo, blockRepo, cleanup := setupDuplicationService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("successfully duplicates instance with locations and blocks", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}

		// Create source instance
		sourceInstance := &models.Instance{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, sourceInstance)
		require.NoError(t, err)

		// Create settings
		settings := &models.InstanceSettings{
			InstanceID:       sourceInstance.ID,
			EnablePoints:     true,
			ShowLeaderboard:  true,
			MaxNextLocations: 3,
		}
		err = settingsRepo.Create(ctx, settings)
		require.NoError(t, err)

		// Create location with blocks
		location := &models.Location{
			Name:       gofakeit.Word(),
			InstanceID: sourceInstance.ID,
			MarkerID:   gofakeit.UUID(),
			Points:     100,
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

		// Duplicate the instance
		newName := gofakeit.Word()
		duplicated, err := svc.DuplicateInstance(ctx, user, sourceInstance.ID, newName)
		require.NoError(t, err)

		// Verify instance was duplicated
		assert.NotEqual(t, sourceInstance.ID, duplicated.ID)
		assert.Equal(t, newName, duplicated.Name)
		assert.Equal(t, user.ID, duplicated.UserID)
		assert.False(t, duplicated.IsTemplate)

		// Verify settings were duplicated
		duplicatedSettings, err := settingsRepo.GetByInstanceID(ctx, duplicated.ID)
		require.NoError(t, err)
		assert.Equal(t, settings.EnablePoints, duplicatedSettings.EnablePoints)
		assert.Equal(t, settings.ShowLeaderboard, duplicatedSettings.ShowLeaderboard)
		assert.Equal(t, settings.MaxNextLocations, duplicatedSettings.MaxNextLocations)

		// Verify locations were duplicated
		duplicatedLocations, err := locationRepo.FindByInstance(ctx, duplicated.ID)
		require.NoError(t, err)
		assert.Len(t, duplicatedLocations, 1)
		assert.NotEqual(t, location.ID, duplicatedLocations[0].ID)
		assert.Equal(t, location.Name, duplicatedLocations[0].Name)
		assert.Equal(t, location.Points, duplicatedLocations[0].Points)
	})

	t.Run("rejects template duplication", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}

		// Create template instance
		template := &models.Instance{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: true,
		}
		err := instanceRepo.Create(ctx, template)
		require.NoError(t, err)

		// Try to duplicate template
		_, err = svc.DuplicateInstance(ctx, user, template.ID, gofakeit.Word())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "cannot duplicate a template")
	})

	t.Run("validates ownership", func(t *testing.T) {
		user1 := &models.User{ID: gofakeit.UUID()}
		user2 := &models.User{ID: gofakeit.UUID()}

		// Create instance owned by user1
		instance := &models.Instance{
			Name:       gofakeit.Word(),
			UserID:     user1.ID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, instance)
		require.NoError(t, err)

		// Try to duplicate as user2
		_, err = svc.DuplicateInstance(ctx, user2, instance.ID, gofakeit.Word())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not authenticated")
	})

	t.Run("validates user is not nil", func(t *testing.T) {
		_, err := svc.DuplicateInstance(ctx, nil, gofakeit.UUID(), gofakeit.Word())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not authenticated")
	})
}

func TestDuplicationService_CreateTemplateFromInstance(t *testing.T) {
	svc, _, instanceRepo, settingsRepo, _, _, cleanup := setupDuplicationService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("successfully creates template from instance", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}

		// Create source instance
		sourceInstance := &models.Instance{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, sourceInstance)
		require.NoError(t, err)

		// Create settings
		settings := &models.InstanceSettings{
			InstanceID:   sourceInstance.ID,
			EnablePoints: true,
		}
		err = settingsRepo.Create(ctx, settings)
		require.NoError(t, err)

		// Create template from instance
		templateName := gofakeit.Word()
		template, err := svc.CreateTemplateFromInstance(ctx, user, sourceInstance.ID, templateName)
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

		instance := &models.Instance{
			Name:       gofakeit.Word(),
			UserID:     user1.ID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, instance)
		require.NoError(t, err)

		_, err = svc.CreateTemplateFromInstance(ctx, user2, instance.ID, gofakeit.Word())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not authenticated")
	})
}

func TestDuplicationService_CreateInstanceFromTemplate(t *testing.T) {
	svc, _, instanceRepo, settingsRepo, _, _, cleanup := setupDuplicationService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("successfully creates instance from template", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}

		// Create template
		template := &models.Instance{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: true,
		}
		err := instanceRepo.Create(ctx, template)
		require.NoError(t, err)

		settings := &models.InstanceSettings{
			InstanceID:   template.ID,
			EnablePoints: false,
		}
		err = settingsRepo.Create(ctx, settings)
		require.NoError(t, err)

		// Create instance from template
		instanceName := gofakeit.Word()
		instance, err := svc.CreateInstanceFromTemplate(ctx, user, template.ID, instanceName)
		require.NoError(t, err)

		// Verify instance
		assert.NotEqual(t, template.ID, instance.ID)
		assert.Equal(t, instanceName, instance.Name)
		assert.Equal(t, user.ID, instance.UserID)
		assert.False(t, instance.IsTemplate)
	})

	t.Run("rejects non-template source", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}

		instance := &models.Instance{
			Name:       gofakeit.Word(),
			UserID:     user.ID,
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, instance)
		require.NoError(t, err)

		_, err = svc.CreateInstanceFromTemplate(ctx, user, instance.ID, gofakeit.Word())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "source is not a template")
	})

	t.Run("validates ownership", func(t *testing.T) {
		user1 := &models.User{ID: gofakeit.UUID()}
		user2 := &models.User{ID: gofakeit.UUID()}

		template := &models.Instance{
			Name:       gofakeit.Word(),
			UserID:     user1.ID,
			IsTemplate: true,
		}
		err := instanceRepo.Create(ctx, template)
		require.NoError(t, err)

		_, err = svc.CreateInstanceFromTemplate(ctx, user2, template.ID, gofakeit.Word())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "user not authenticated")
	})
}

func TestDuplicationService_CreateInstanceFromSharedTemplate(t *testing.T) {
	svc, _, instanceRepo, settingsRepo, _, _, cleanup := setupDuplicationService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("successfully creates instance from shared template without ownership check", func(t *testing.T) {
		owner := &models.User{ID: gofakeit.UUID()}
		recipient := &models.User{ID: gofakeit.UUID()}

		// Create template owned by owner
		template := &models.Instance{
			Name:       gofakeit.Word(),
			UserID:     owner.ID,
			IsTemplate: true,
		}
		err := instanceRepo.Create(ctx, template)
		require.NoError(t, err)

		settings := &models.InstanceSettings{
			InstanceID: template.ID,
		}
		err = settingsRepo.Create(ctx, settings)
		require.NoError(t, err)

		// Recipient creates instance from shared template
		instanceName := gofakeit.Word()
		instance, err := svc.CreateInstanceFromSharedTemplate(ctx, recipient, template.ID, instanceName)
		require.NoError(t, err)

		// Verify instance is owned by recipient
		assert.Equal(t, recipient.ID, instance.UserID)
		assert.Equal(t, instanceName, instance.Name)
		assert.False(t, instance.IsTemplate)
	})

	t.Run("rejects non-template source", func(t *testing.T) {
		user := &models.User{ID: gofakeit.UUID()}

		instance := &models.Instance{
			Name:       gofakeit.Word(),
			UserID:     gofakeit.UUID(),
			IsTemplate: false,
		}
		err := instanceRepo.Create(ctx, instance)
		require.NoError(t, err)

		_, err = svc.CreateInstanceFromSharedTemplate(ctx, user, instance.ID, gofakeit.Word())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "source is not a template")
	})
}

func TestDuplicationService_DuplicateLocation(t *testing.T) {
	svc, transactor, _, _, locationRepo, blockRepo, cleanup := setupDuplicationService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("successfully duplicates location with blocks", func(t *testing.T) {
		sourceInstanceID := gofakeit.UUID()
		targetInstanceID := gofakeit.UUID()

		// Create source location
		sourceLocation := models.Location{
			Name:       gofakeit.Word(),
			InstanceID: sourceInstanceID,
			MarkerID:   gofakeit.UUID(),
			Points:     50,
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
		duplicated, err := svc.DuplicateLocation(ctx, sourceLocation, targetInstanceID)
		require.NoError(t, err)

		// Verify duplicated location
		assert.NotEqual(t, sourceLocation.ID, duplicated.ID)
		assert.Equal(t, sourceLocation.Name, duplicated.Name)
		assert.Equal(t, targetInstanceID, duplicated.InstanceID)
		assert.Equal(t, sourceLocation.Points, duplicated.Points)
	})
}
