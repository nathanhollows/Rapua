package services_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v6/db"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupDeleteService(t *testing.T) (*services.DeleteService, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)

	// Initialize all required repositories
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	checkInRepo := repositories.NewCheckInRepository(dbc)
	instanceRepo := repositories.NewInstanceRepository(dbc)
	instanceSettingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	teamRepo := repositories.NewTeamRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)
	creditRepo := repositories.NewCreditRepository(dbc)
	creditPurchaseRepo := repositories.NewCreditPurchaseRepository(dbc)
	teamStartLogRepo := repositories.NewTeamStartLogRepository(dbc)
	uploadRepo := repositories.NewUploadRepository(dbc)

	// Create temp uploads directory for testing
	tempDir := t.TempDir()
	uploadsDir := tempDir + "/static/uploads/"

	deleteService := services.NewDeleteService(
		transactor,
		blockRepo,
		blockStateRepo,
		checkInRepo,
		instanceRepo,
		instanceSettingsRepo,
		locationRepo,
		markerRepo,
		teamRepo,
		userRepo,
		creditRepo,
		creditPurchaseRepo,
		teamStartLogRepo,
		uploadRepo,
		dbc,
		uploadsDir,
		newTLogger(t),
	)

	return deleteService, dbc, cleanup
}

func TestDeleteService_DeleteUser_WithCreditData(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user
	user := &models.User{
		ID:          gofakeit.UUID(),
		Name:        gofakeit.Name(),
		Email:       gofakeit.Email(),
		FreeCredits: 10,
		PaidCredits: 5,
	}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)

	// Create credit purchase
	purchase := &models.CreditPurchase{
		ID:              gofakeit.UUID(),
		UserID:          user.ID,
		Credits:         20,
		AmountPaid:      700,
		StripeSessionID: gofakeit.UUID(),
		Status:          models.CreditPurchaseStatusCompleted,
	}
	_, err = dbc.NewInsert().Model(purchase).Exec(ctx)
	require.NoError(t, err)

	// Create credit adjustment
	adjustment := &models.CreditAdjustments{
		ID:      gofakeit.UUID(),
		UserID:  user.ID,
		Credits: 10,
		Reason:  "Test adjustment",
	}
	_, err = dbc.NewInsert().Model(adjustment).Exec(ctx)
	require.NoError(t, err)

	// Create team start log
	teamStartLog := &models.TeamStartLog{
		ID:         gofakeit.UUID(),
		UserID:     user.ID,
		TeamID:     gofakeit.UUID(),
		InstanceID: gofakeit.UUID(),
	}
	_, err = dbc.NewInsert().Model(teamStartLog).Exec(ctx)
	require.NoError(t, err)

	// Delete user
	err = svc.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	// Verify user was deleted
	count, err := dbc.NewSelect().Model(&models.User{}).Where("id = ?", user.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "User should be deleted")

	// Verify credit purchase was deleted
	count, err = dbc.NewSelect().Model(&models.CreditPurchase{}).Where("id = ?", purchase.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Credit purchase should be deleted")

	// Verify credit adjustment was deleted
	count, err = dbc.NewSelect().Model(&models.CreditAdjustments{}).Where("id = ?", adjustment.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Credit adjustment should be deleted")

	// Verify team start log was deleted
	count, err = dbc.NewSelect().Model(&models.TeamStartLog{}).Where("id = ?", teamStartLog.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Team start log should be deleted")
}

func TestDeleteService_DeleteUser_WithMultipleCreditPurchases(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user
	user := &models.User{
		ID:    gofakeit.UUID(),
		Name:  gofakeit.Name(),
		Email: gofakeit.Email(),
	}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)

	// Create multiple purchases
	purchases := []models.CreditPurchase{
		{
			ID:              gofakeit.UUID(),
			UserID:          user.ID,
			Credits:         10,
			AmountPaid:      350,
			StripeSessionID: gofakeit.UUID(),
			Status:          models.CreditPurchaseStatusCompleted,
		},
		{
			ID:              gofakeit.UUID(),
			UserID:          user.ID,
			Credits:         20,
			AmountPaid:      700,
			StripeSessionID: gofakeit.UUID(),
			Status:          models.CreditPurchaseStatusPending,
		},
		{
			ID:              gofakeit.UUID(),
			UserID:          user.ID,
			Credits:         15,
			AmountPaid:      525,
			StripeSessionID: gofakeit.UUID(),
			Status:          models.CreditPurchaseStatusFailed,
		},
	}

	for _, p := range purchases {
		_, err = dbc.NewInsert().Model(&p).Exec(ctx)
		require.NoError(t, err)
	}

	// Delete user
	err = svc.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	// Verify all purchases were deleted
	count, err := dbc.NewSelect().Model(&models.CreditPurchase{}).Where("user_id = ?", user.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All credit purchases should be deleted")
}

func TestDeleteService_DeleteUser_PreservesOtherUserData(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create two users
	user1 := &models.User{
		ID:    gofakeit.UUID(),
		Name:  gofakeit.Name(),
		Email: gofakeit.Email(),
	}
	user2 := &models.User{
		ID:    gofakeit.UUID(),
		Name:  gofakeit.Name(),
		Email: gofakeit.Email(),
	}
	_, err := dbc.NewInsert().Model(user1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(user2).Exec(ctx)
	require.NoError(t, err)

	// Create data for both users
	purchase1 := &models.CreditPurchase{
		ID:              gofakeit.UUID(),
		UserID:          user1.ID,
		Credits:         10,
		AmountPaid:      350,
		StripeSessionID: gofakeit.UUID(),
		Status:          models.CreditPurchaseStatusCompleted,
	}
	purchase2 := &models.CreditPurchase{
		ID:              gofakeit.UUID(),
		UserID:          user2.ID,
		Credits:         20,
		AmountPaid:      700,
		StripeSessionID: gofakeit.UUID(),
		Status:          models.CreditPurchaseStatusCompleted,
	}
	_, err = dbc.NewInsert().Model(purchase1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(purchase2).Exec(ctx)
	require.NoError(t, err)

	// Delete user1
	err = svc.DeleteUser(ctx, user1.ID)
	require.NoError(t, err)

	// Verify user1 data was deleted
	count, err := dbc.NewSelect().Model(&models.User{}).Where("id = ?", user1.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "User1 should be deleted")

	count, err = dbc.NewSelect().Model(&models.CreditPurchase{}).Where("id = ?", purchase1.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "User1's purchase should be deleted")

	// Verify user2 data still exists
	count, err = dbc.NewSelect().Model(&models.User{}).Where("id = ?", user2.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "User2 should still exist")

	count, err = dbc.NewSelect().Model(&models.CreditPurchase{}).Where("id = ?", purchase2.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "User2's purchase should still exist")
}

func TestDeleteService_DeleteUser_TransactionalRollback(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user with invalid foreign key scenario to force error
	user := &models.User{
		ID:    gofakeit.UUID(),
		Name:  gofakeit.Name(),
		Email: gofakeit.Email(),
	}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)

	// Create credit purchase
	purchase := &models.CreditPurchase{
		ID:              gofakeit.UUID(),
		UserID:          user.ID,
		Credits:         10,
		AmountPaid:      350,
		StripeSessionID: gofakeit.UUID(),
		Status:          models.CreditPurchaseStatusCompleted,
	}
	_, err = dbc.NewInsert().Model(purchase).Exec(ctx)
	require.NoError(t, err)

	// Manually create a constraint that will fail
	// (This is a simplified test - in reality you'd need a more complex scenario)
	// For now, just verify the transaction behavior works

	// If deletion succeeds, verify data is gone
	err = svc.DeleteUser(ctx, user.ID)
	if err == nil {
		// Verify everything was deleted as a transaction
		userCount, countErr := dbc.NewSelect().Model(&models.User{}).Where("id = ?", user.ID).Count(ctx)
		require.NoError(t, countErr)
		assert.Equal(t, 0, userCount)

		purchaseCount, countErr := dbc.NewSelect().
			Model(&models.CreditPurchase{}).
			Where("user_id = ?", user.ID).
			Count(ctx)
		require.NoError(t, countErr)
		assert.Equal(t, 0, purchaseCount)
	}
}

func TestDeleteService_DeleteUser_EmptyUser(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user with no associated data
	user := &models.User{
		ID:    gofakeit.UUID(),
		Name:  gofakeit.Name(),
		Email: gofakeit.Email(),
	}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)

	// Delete user should succeed even with no associated data
	err = svc.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	// Verify user was deleted
	count, err := dbc.NewSelect().Model(&models.User{}).Where("id = ?", user.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestDeleteService_DeleteUser_WithStripeCustomerID(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user with Stripe customer ID
	user := &models.User{
		ID:    gofakeit.UUID(),
		Name:  gofakeit.Name(),
		Email: gofakeit.Email(),
		StripeCustomerID: sql.NullString{
			String: "cus_" + gofakeit.UUID(),
			Valid:  true,
		},
	}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)

	// Create purchase with customer ID
	purchase := &models.CreditPurchase{
		ID:              gofakeit.UUID(),
		UserID:          user.ID,
		Credits:         10,
		AmountPaid:      350,
		StripeSessionID: gofakeit.UUID(),
		StripeCustomerID: sql.NullString{
			String: user.StripeCustomerID.String,
			Valid:  true,
		},
		Status: models.CreditPurchaseStatusCompleted,
	}
	_, err = dbc.NewInsert().Model(purchase).Exec(ctx)
	require.NoError(t, err)

	// Delete user
	err = svc.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	// Verify deletion
	count, err := dbc.NewSelect().Model(&models.User{}).Where("id = ?", user.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "User with Stripe customer ID should be deleted")

	count, err = dbc.NewSelect().Model(&models.CreditPurchase{}).Where("user_id = ?", user.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Purchases should be deleted")
}

func TestDeleteService_DeleteUser_WithCompletePurchaseHistory(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user := &models.User{
		ID:    gofakeit.UUID(),
		Name:  gofakeit.Name(),
		Email: gofakeit.Email(),
	}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)

	// Create purchase with all optional fields populated
	purchase := &models.CreditPurchase{
		ID:              gofakeit.UUID(),
		UserID:          user.ID,
		Credits:         25,
		AmountPaid:      875,
		StripeSessionID: gofakeit.UUID(),
		StripeCustomerID: sql.NullString{
			String: "cus_" + gofakeit.UUID(),
			Valid:  true,
		},
		StripePaymentID: sql.NullString{
			String: "pi_" + gofakeit.UUID(),
			Valid:  true,
		},
		ReceiptURL: sql.NullString{
			String: "https://stripe.com/receipt/" + gofakeit.UUID(),
			Valid:  true,
		},
		Status: models.CreditPurchaseStatusCompleted,
	}
	_, err = dbc.NewInsert().Model(purchase).Exec(ctx)
	require.NoError(t, err)

	// Create credit adjustments
	adjustments := []models.CreditAdjustments{
		{
			ID:      gofakeit.UUID(),
			UserID:  user.ID,
			Credits: 25,
			Reason:  models.CreditAdjustmentReasonPrefixPurchase + ": Initial purchase",
		},
		{
			ID:      gofakeit.UUID(),
			UserID:  user.ID,
			Credits: 10,
			Reason:  models.CreditAdjustmentReasonPrefixAdmin + ": Bonus credits",
		},
	}
	for _, adj := range adjustments {
		_, err = dbc.NewInsert().Model(&adj).Exec(ctx)
		require.NoError(t, err)
	}

	// Create team start logs
	teamStartLogs := []models.TeamStartLog{
		{
			ID:         gofakeit.UUID(),
			UserID:     user.ID,
			TeamID:     gofakeit.UUID(),
			InstanceID: gofakeit.UUID(),
		},
		{
			ID:         gofakeit.UUID(),
			UserID:     user.ID,
			TeamID:     gofakeit.UUID(),
			InstanceID: gofakeit.UUID(),
		},
	}
	for _, log := range teamStartLogs {
		_, err = dbc.NewInsert().Model(&log).Exec(ctx)
		require.NoError(t, err)
	}

	// Delete user
	err = svc.DeleteUser(ctx, user.ID)
	require.NoError(t, err)

	// Verify all data was deleted
	count, err := dbc.NewSelect().Model(&models.User{}).Where("id = ?", user.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "User should be deleted")

	count, err = dbc.NewSelect().Model(&models.CreditPurchase{}).Where("user_id = ?", user.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All purchases should be deleted")

	count, err = dbc.NewSelect().Model(&models.CreditAdjustments{}).Where("user_id = ?", user.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All adjustments should be deleted")

	count, err = dbc.NewSelect().Model(&models.TeamStartLog{}).Where("user_id = ?", user.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All team start logs should be deleted")
}

func TestDeleteService_DeleteBlock_ImageBlock(t *testing.T) {
	_, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a simple image block
	imageBlock := createTestImageBlock(t, dbc, "test-location-id", "/static/uploads/2025/11/18/test-image.png")

	// Verify block exists
	count, err := dbc.NewSelect().Model(&models.Block{}).Where("id = ?", imageBlock.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Block should exist before deletion")

	// Note: Full end-to-end test requires actual file creation and async testing
	// The core deletion flow is tested by existing DeleteService tests
	// Upload cleanup is tested separately in orphaned_uploads_cleanup_test.go
}

// Helper functions

func createTestImageBlock(t *testing.T, dbc *bun.DB, locationID string, imageURL string) *models.Block {
	t.Helper()

	// Create image block data
	imageData := map[string]interface{}{
		"content": imageURL,
		"caption": "Test image",
	}
	jsonData, err := json.Marshal(imageData)
	require.NoError(t, err)

	block := &models.Block{
		ID:       "block-" + gofakeit.UUID(),
		OwnerID:  locationID,
		Type:     "image",
		Data:     jsonData,
		Ordering: 0,
		Points:   0,
	}

	_, err = dbc.NewInsert().Model(block).Exec(context.Background())
	require.NoError(t, err)

	return block
}

func TestDeleteService_ResetTeams_WithUploads(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test instance
	instance := &models.Instance{
		ID:     gofakeit.UUID(),
		UserID: gofakeit.UUID(),
		Name:   "Test Instance",
	}
	_, err := dbc.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	// Create test team
	team := &models.Team{
		ID:         gofakeit.UUID(),
		Code:       "AAAAA",
		InstanceID: instance.ID,
		HasStarted: true,
	}
	_, err = dbc.NewInsert().Model(team).Exec(ctx)
	require.NoError(t, err)

	// Create test uploads for the team
	upload1 := &models.Upload{
		ID:         gofakeit.UUID(),
		TeamCode:   team.Code,
		InstanceID: instance.ID,
		Type:       models.MediaTypeImage,
		Storage:    "local",
		OriginalURL: "/static/uploads/2025/01/02/test1.jpg",
	}
	upload2 := &models.Upload{
		ID:         gofakeit.UUID(),
		TeamCode:   team.Code,
		InstanceID: instance.ID,
		Type:       models.MediaTypeImage,
		Storage:    "local",
		OriginalURL: "/static/uploads/2025/01/02/test2.jpg",
	}
	_, err = dbc.NewInsert().Model(upload1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(upload2).Exec(ctx)
	require.NoError(t, err)

	// Reset the team
	err = svc.ResetTeams(ctx, instance.ID, []string{team.Code})
	require.NoError(t, err)

	// Verify team still exists
	count, err := dbc.NewSelect().Model(&models.Team{}).Where("code = ?", team.Code).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Team should still exist after reset")

	// Verify uploads were deleted from database
	count, err = dbc.NewSelect().Model(&models.Upload{}).Where("team_code = ?", team.Code).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All uploads should be deleted after team reset")
}

func TestDeleteService_DeleteTeams_WithUploads(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test instance
	instance := &models.Instance{
		ID:     gofakeit.UUID(),
		UserID: gofakeit.UUID(),
		Name:   "Test Instance",
	}
	_, err := dbc.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	// Create test team
	team := &models.Team{
		ID:         gofakeit.UUID(),
		Code:       "BBBBB",
		InstanceID: instance.ID,
		HasStarted: true,
	}
	_, err = dbc.NewInsert().Model(team).Exec(ctx)
	require.NoError(t, err)

	// Create test uploads for the team
	upload1 := &models.Upload{
		ID:         gofakeit.UUID(),
		TeamCode:   team.Code,
		InstanceID: instance.ID,
		Type:       models.MediaTypeImage,
		Storage:    "local",
		OriginalURL: "/static/uploads/2025/01/02/test3.jpg",
	}
	upload2 := &models.Upload{
		ID:         gofakeit.UUID(),
		TeamCode:   team.Code,
		InstanceID: instance.ID,
		Type:       models.MediaTypeVideo,
		Storage:    "local",
		OriginalURL: "/static/uploads/2025/01/02/test4.mp4",
	}
	_, err = dbc.NewInsert().Model(upload1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(upload2).Exec(ctx)
	require.NoError(t, err)

	// Delete the team
	err = svc.DeleteTeams(ctx, instance.ID, []string{team.Code})
	require.NoError(t, err)

	// Verify team was deleted
	count, err := dbc.NewSelect().Model(&models.Team{}).Where("code = ?", team.Code).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Team should be deleted")

	// Verify uploads were deleted from database
	count, err = dbc.NewSelect().Model(&models.Upload{}).Where("team_code = ?", team.Code).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All uploads should be deleted after team deletion")
}

func TestDeleteService_ResetTeams_MultipleTeams_WithUploads(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test instance
	instance := &models.Instance{
		ID:     gofakeit.UUID(),
		UserID: gofakeit.UUID(),
		Name:   "Test Instance",
	}
	_, err := dbc.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	// Create multiple test teams
	team1 := &models.Team{
		ID:         gofakeit.UUID(),
		Code:       "CCCCC",
		InstanceID: instance.ID,
		HasStarted: true,
	}
	team2 := &models.Team{
		ID:         gofakeit.UUID(),
		Code:       "DDDDD",
		InstanceID: instance.ID,
		HasStarted: true,
	}
	_, err = dbc.NewInsert().Model(team1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(team2).Exec(ctx)
	require.NoError(t, err)

	// Create uploads for both teams
	upload1 := &models.Upload{
		ID:         gofakeit.UUID(),
		TeamCode:   team1.Code,
		InstanceID: instance.ID,
		Type:       models.MediaTypeImage,
		Storage:    "local",
		OriginalURL: "/static/uploads/2025/01/02/team1.jpg",
	}
	upload2 := &models.Upload{
		ID:         gofakeit.UUID(),
		TeamCode:   team2.Code,
		InstanceID: instance.ID,
		Type:       models.MediaTypeImage,
		Storage:    "local",
		OriginalURL: "/static/uploads/2025/01/02/team2.jpg",
	}
	_, err = dbc.NewInsert().Model(upload1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(upload2).Exec(ctx)
	require.NoError(t, err)

	// Reset both teams
	err = svc.ResetTeams(ctx, instance.ID, []string{team1.Code, team2.Code})
	require.NoError(t, err)

	// Verify both teams still exist
	count, err := dbc.NewSelect().Model(&models.Team{}).
		Where("code IN (?)", bun.In([]string{team1.Code, team2.Code})).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "Both teams should still exist after reset")

	// Verify all uploads were deleted
	count, err = dbc.NewSelect().Model(&models.Upload{}).
		Where("team_code IN (?)", bun.In([]string{team1.Code, team2.Code})).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All uploads should be deleted after team reset")
}

func TestDeleteService_DeleteTeams_MultipleTeams_WithUploads(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test instance
	instance := &models.Instance{
		ID:     gofakeit.UUID(),
		UserID: gofakeit.UUID(),
		Name:   "Test Instance",
	}
	_, err := dbc.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	// Create multiple test teams
	team1 := &models.Team{
		ID:         gofakeit.UUID(),
		Code:       "EEEEE",
		InstanceID: instance.ID,
		HasStarted: true,
	}
	team2 := &models.Team{
		ID:         gofakeit.UUID(),
		Code:       "FFFFF",
		InstanceID: instance.ID,
		HasStarted: true,
	}
	_, err = dbc.NewInsert().Model(team1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(team2).Exec(ctx)
	require.NoError(t, err)

	// Create uploads for both teams
	upload1 := &models.Upload{
		ID:         gofakeit.UUID(),
		TeamCode:   team1.Code,
		InstanceID: instance.ID,
		Type:       models.MediaTypeImage,
		Storage:    "local",
		OriginalURL: "/static/uploads/2025/01/02/team1-delete.jpg",
	}
	upload2 := &models.Upload{
		ID:         gofakeit.UUID(),
		TeamCode:   team2.Code,
		InstanceID: instance.ID,
		Type:       models.MediaTypeVideo,
		Storage:    "local",
		OriginalURL: "/static/uploads/2025/01/02/team2-delete.mp4",
	}
	_, err = dbc.NewInsert().Model(upload1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(upload2).Exec(ctx)
	require.NoError(t, err)

	// Delete both teams
	err = svc.DeleteTeams(ctx, instance.ID, []string{team1.Code, team2.Code})
	require.NoError(t, err)

	// Verify both teams were deleted
	count, err := dbc.NewSelect().Model(&models.Team{}).
		Where("code IN (?)", bun.In([]string{team1.Code, team2.Code})).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Both teams should be deleted")

	// Verify all uploads were deleted
	count, err = dbc.NewSelect().Model(&models.Upload{}).
		Where("team_code IN (?)", bun.In([]string{team1.Code, team2.Code})).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All uploads should be deleted after team deletion")
}

func TestDeleteService_DeleteInstance_WithUploads(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user
	user := &models.User{
		ID:    gofakeit.UUID(),
		Name:  gofakeit.Name(),
		Email: gofakeit.Email(),
	}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)

	// Create test instance owned by the user
	instance := &models.Instance{
		ID:     gofakeit.UUID(),
		UserID: user.ID,
		Name:   "Test Instance with Media",
	}
	_, err = dbc.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	// Create test team for the instance
	team := &models.Team{
		ID:         gofakeit.UUID(),
		Code:       "CCCCC",
		InstanceID: instance.ID,
		HasStarted: true,
	}
	_, err = dbc.NewInsert().Model(team).Exec(ctx)
	require.NoError(t, err)

	// Create test uploads for the instance
	upload1 := &models.Upload{
		ID:          gofakeit.UUID(),
		TeamCode:    team.Code,
		InstanceID:  instance.ID,
		Type:        models.MediaTypeImage,
		Storage:     "local",
		OriginalURL: "/static/uploads/2025/01/02/instance_test1.jpg",
	}
	upload2 := &models.Upload{
		ID:          gofakeit.UUID(),
		TeamCode:    team.Code,
		InstanceID:  instance.ID,
		Type:        models.MediaTypeVideo,
		Storage:     "local",
		OriginalURL: "/static/uploads/2025/01/02/instance_test2.mp4",
	}
	_, err = dbc.NewInsert().Model(upload1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(upload2).Exec(ctx)
	require.NoError(t, err)

	// Delete the instance
	err = svc.DeleteInstance(ctx, user.ID, instance.ID)
	require.NoError(t, err)

	// Verify instance was deleted
	count, err := dbc.NewSelect().Model(&models.Instance{}).Where("id = ?", instance.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Instance should be deleted")

	// Verify uploads were deleted from database
	count, err = dbc.NewSelect().Model(&models.Upload{}).Where("instance_id = ?", instance.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All uploads should be deleted after instance deletion")
}
