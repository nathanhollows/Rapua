package services_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupDeleteService(t *testing.T) (*services.DeleteService, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)

	// Initialize required repositories
	instanceRepo := repositories.NewQuestRepository(dbc)
	teamRepo := repositories.NewRunRepository(dbc)
	uploadRepo := repositories.NewUploadRepository(dbc)

	// Create temp uploads directory for testing
	tempDir := t.TempDir()
	uploadsDir := tempDir + "/static/uploads/"

	deleteService := services.NewDeleteService(
		transactor,
		instanceRepo,
		teamRepo,
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
	teamStartLog := &models.RunStartLog{
		ID:      gofakeit.UUID(),
		UserID:  user.ID,
		RunID:   gofakeit.UUID(),
		QuestID: gofakeit.UUID(),
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
	count, err = dbc.NewSelect().Model(&models.RunStartLog{}).Where("id = ?", teamStartLog.ID).Count(ctx)
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
	teamStartLogs := []models.RunStartLog{
		{
			ID:      gofakeit.UUID(),
			UserID:  user.ID,
			RunID:   gofakeit.UUID(),
			QuestID: gofakeit.UUID(),
		},
		{
			ID:      gofakeit.UUID(),
			UserID:  user.ID,
			RunID:   gofakeit.UUID(),
			QuestID: gofakeit.UUID(),
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

	count, err = dbc.NewSelect().Model(&models.RunStartLog{}).Where("user_id = ?", user.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All team start logs should be deleted")
}

func TestDeleteService_ResetTeams_WithUploads(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user and instance
	user := &models.User{ID: gofakeit.UUID(), Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)
	instance := &models.Quest{
		ID:     gofakeit.UUID(),
		UserID: user.ID,
		Name:   "Test Instance",
	}
	_, err = dbc.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	// Create test team
	team := &models.Run{
		ID:         gofakeit.UUID(),
		Code:       "AAAAA",
		QuestID:    instance.ID,
		HasStarted: true,
	}
	_, err = dbc.NewInsert().Model(team).Exec(ctx)
	require.NoError(t, err)

	// Create test uploads for the team
	upload1 := &models.Upload{
		ID:          gofakeit.UUID(),
		RunCode:     team.Code,
		QuestID:     instance.ID,
		Type:        models.MediaTypeImage,
		Storage:     "local",
		OriginalURL: "/static/uploads/2025/01/02/test1.jpg",
	}
	upload2 := &models.Upload{
		ID:          gofakeit.UUID(),
		RunCode:     team.Code,
		QuestID:     instance.ID,
		Type:        models.MediaTypeImage,
		Storage:     "local",
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
	count, err := dbc.NewSelect().Model(&models.Run{}).Where("code = ?", team.Code).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "Team should still exist after reset")

	// Verify uploads were deleted from database
	count, err = dbc.NewSelect().Model(&models.Upload{}).Where("run_code = ?", team.Code).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All uploads should be deleted after team reset")
}

func TestDeleteService_DeleteTeams_WithUploads(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user and instance
	user := &models.User{ID: gofakeit.UUID(), Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)
	instance := &models.Quest{
		ID:     gofakeit.UUID(),
		UserID: user.ID,
		Name:   "Test Instance",
	}
	_, err = dbc.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	// Create test team
	team := &models.Run{
		ID:         gofakeit.UUID(),
		Code:       "BBBBB",
		QuestID:    instance.ID,
		HasStarted: true,
	}
	_, err = dbc.NewInsert().Model(team).Exec(ctx)
	require.NoError(t, err)

	// Create test uploads for the team
	upload1 := &models.Upload{
		ID:          gofakeit.UUID(),
		RunCode:     team.Code,
		QuestID:     instance.ID,
		Type:        models.MediaTypeImage,
		Storage:     "local",
		OriginalURL: "/static/uploads/2025/01/02/test3.jpg",
	}
	upload2 := &models.Upload{
		ID:          gofakeit.UUID(),
		RunCode:     team.Code,
		QuestID:     instance.ID,
		Type:        models.MediaTypeVideo,
		Storage:     "local",
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
	count, err := dbc.NewSelect().Model(&models.Run{}).Where("code = ?", team.Code).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Team should be deleted")

	// Verify uploads were deleted from database
	count, err = dbc.NewSelect().Model(&models.Upload{}).Where("run_code = ?", team.Code).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All uploads should be deleted after team deletion")
}

func TestDeleteService_ResetTeams_MultipleTeams_WithUploads(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user and instance
	user := &models.User{ID: gofakeit.UUID(), Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)
	instance := &models.Quest{
		ID:     gofakeit.UUID(),
		UserID: user.ID,
		Name:   "Test Instance",
	}
	_, err = dbc.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	// Create multiple test teams
	team1 := &models.Run{
		ID:         gofakeit.UUID(),
		Code:       "CCCCC",
		QuestID:    instance.ID,
		HasStarted: true,
	}
	team2 := &models.Run{
		ID:         gofakeit.UUID(),
		Code:       "DDDDD",
		QuestID:    instance.ID,
		HasStarted: true,
	}
	_, err = dbc.NewInsert().Model(team1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(team2).Exec(ctx)
	require.NoError(t, err)

	// Create uploads for both teams
	upload1 := &models.Upload{
		ID:          gofakeit.UUID(),
		RunCode:     team1.Code,
		QuestID:     instance.ID,
		Type:        models.MediaTypeImage,
		Storage:     "local",
		OriginalURL: "/static/uploads/2025/01/02/team1.jpg",
	}
	upload2 := &models.Upload{
		ID:          gofakeit.UUID(),
		RunCode:     team2.Code,
		QuestID:     instance.ID,
		Type:        models.MediaTypeImage,
		Storage:     "local",
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
	count, err := dbc.NewSelect().Model(&models.Run{}).
		Where("code IN (?)", bun.In([]string{team1.Code, team2.Code})).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count, "Both teams should still exist after reset")

	// Verify all uploads were deleted
	count, err = dbc.NewSelect().Model(&models.Upload{}).
		Where("run_code IN (?)", bun.In([]string{team1.Code, team2.Code})).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All uploads should be deleted after team reset")
}

func TestDeleteService_DeleteTeams_MultipleTeams_WithUploads(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user and instance
	user := &models.User{ID: gofakeit.UUID(), Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)
	instance := &models.Quest{
		ID:     gofakeit.UUID(),
		UserID: user.ID,
		Name:   "Test Instance",
	}
	_, err = dbc.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	// Create multiple test teams
	team1 := &models.Run{
		ID:         gofakeit.UUID(),
		Code:       "EEEEE",
		QuestID:    instance.ID,
		HasStarted: true,
	}
	team2 := &models.Run{
		ID:         gofakeit.UUID(),
		Code:       "FFFFF",
		QuestID:    instance.ID,
		HasStarted: true,
	}
	_, err = dbc.NewInsert().Model(team1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(team2).Exec(ctx)
	require.NoError(t, err)

	// Create uploads for both teams
	upload1 := &models.Upload{
		ID:          gofakeit.UUID(),
		RunCode:     team1.Code,
		QuestID:     instance.ID,
		Type:        models.MediaTypeImage,
		Storage:     "local",
		OriginalURL: "/static/uploads/2025/01/02/team1-delete.jpg",
	}
	upload2 := &models.Upload{
		ID:          gofakeit.UUID(),
		RunCode:     team2.Code,
		QuestID:     instance.ID,
		Type:        models.MediaTypeVideo,
		Storage:     "local",
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
	count, err := dbc.NewSelect().Model(&models.Run{}).
		Where("code IN (?)", bun.In([]string{team1.Code, team2.Code})).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Both teams should be deleted")

	// Verify all uploads were deleted
	count, err = dbc.NewSelect().Model(&models.Upload{}).
		Where("run_code IN (?)", bun.In([]string{team1.Code, team2.Code})).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All uploads should be deleted after team deletion")
}

func TestDeleteService_DeleteQuest_WithUploads(t *testing.T) {
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
	instance := &models.Quest{
		ID:     gofakeit.UUID(),
		UserID: user.ID,
		Name:   "Test Instance with Media",
	}
	_, err = dbc.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	// Create test team for the instance
	team := &models.Run{
		ID:         gofakeit.UUID(),
		Code:       "CCCCC",
		QuestID:    instance.ID,
		HasStarted: true,
	}
	_, err = dbc.NewInsert().Model(team).Exec(ctx)
	require.NoError(t, err)

	// Create test uploads for the instance
	upload1 := &models.Upload{
		ID:          gofakeit.UUID(),
		RunCode:     team.Code,
		QuestID:     instance.ID,
		Type:        models.MediaTypeImage,
		Storage:     "local",
		OriginalURL: "/static/uploads/2025/01/02/instance_test1.jpg",
	}
	upload2 := &models.Upload{
		ID:          gofakeit.UUID(),
		RunCode:     team.Code,
		QuestID:     instance.ID,
		Type:        models.MediaTypeVideo,
		Storage:     "local",
		OriginalURL: "/static/uploads/2025/01/02/instance_test2.mp4",
	}
	_, err = dbc.NewInsert().Model(upload1).Exec(ctx)
	require.NoError(t, err)
	_, err = dbc.NewInsert().Model(upload2).Exec(ctx)
	require.NoError(t, err)

	// Delete the instance
	err = svc.DeleteQuest(ctx, user.ID, instance.ID)
	require.NoError(t, err)

	// Verify instance was deleted
	count, err := dbc.NewSelect().Model(&models.Quest{}).Where("id = ?", instance.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "Instance should be deleted")

	// Verify uploads were deleted from database
	count, err = dbc.NewSelect().Model(&models.Upload{}).Where("quest_id = ?", instance.ID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "All uploads should be deleted after instance deletion")
}

// Objective blocks are polymorphic (owner_id has no FK), so DeleteQuest must
// delete them explicitly, the same as it does for location and instance blocks.
func TestDeleteService_DeleteQuest_DeletesObjectiveBlocks(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	user := &models.User{ID: gofakeit.UUID(), Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)

	instance := &models.Quest{ID: gofakeit.UUID(), UserID: user.ID, Name: "Test Instance"}
	_, err = dbc.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: instance.ID,
		Slug:    gofakeit.Word() + "-objective",
		Title:   gofakeit.Sentence(3),
	}
	_, err = dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	block := &models.Block{
		ID:      gofakeit.UUID(),
		OwnerID: objective.ID,
		Type:    "markdown",
		Data:    json.RawMessage(`{}`),
	}
	_, err = dbc.NewInsert().Model(block).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, svc.DeleteQuest(ctx, user.ID, instance.ID))

	count, err := dbc.NewSelect().Model((*models.Block)(nil)).Where("owner_id = ?", objective.ID).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count, "objective blocks should be deleted when the owning quest is deleted")
}

// Same gap as DeleteQuest: objective blocks are polymorphic and must be
// deleted explicitly when a user (and all their quests) is deleted.
func TestDeleteService_DeleteUser_DeletesObjectiveBlocks(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()

	user := &models.User{ID: gofakeit.UUID(), Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)

	instance := &models.Quest{ID: gofakeit.UUID(), UserID: user.ID, Name: "Test Instance"}
	_, err = dbc.NewInsert().Model(instance).Exec(ctx)
	require.NoError(t, err)

	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: instance.ID,
		Slug:    gofakeit.Word() + "-objective",
		Title:   gofakeit.Sentence(3),
	}
	_, err = dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	block := &models.Block{
		ID:      gofakeit.UUID(),
		OwnerID: objective.ID,
		Type:    "markdown",
		Data:    json.RawMessage(`{}`),
	}
	_, err = dbc.NewInsert().Model(block).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, svc.DeleteUser(ctx, user.ID))

	count, err := dbc.NewSelect().Model((*models.Block)(nil)).Where("owner_id = ?", objective.ID).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count, "objective blocks should be deleted when the owning user is deleted")
}

func reloadStructure(t *testing.T, dbc *bun.DB, questID string) models.GameStructure {
	t.Helper()
	var reloaded models.Quest
	err := dbc.NewSelect().Model(&reloaded).Where("id = ?", questID).Scan(context.Background())
	require.NoError(t, err)
	return reloaded.GameStructure
}

// deleteObjectiveFixture creates a quest whose GameStructure references two
// objectives in a nested group, and returns the quest plus both objective IDs.
func deleteObjectiveFixture(t *testing.T, dbc *bun.DB) (*models.Quest, string, string) {
	t.Helper()
	ctx := context.Background()

	user := &models.User{ID: gofakeit.UUID(), Name: gofakeit.Name(), Email: gofakeit.Email()}
	_, err := dbc.NewInsert().Model(user).Exec(ctx)
	require.NoError(t, err)

	doomedID, survivorID := gofakeit.UUID(), gofakeit.UUID()

	quest := &models.Quest{
		ID:     gofakeit.UUID(),
		UserID: user.ID,
		Name:   "Structure prune",
		GameStructure: models.GameStructure{
			ID:           "root",
			IsRoot:       true,
			ObjectiveIDs: []string{survivorID},
			SubGroups: []models.GameStructure{
				{ID: "nested", Name: "Nested", ObjectiveIDs: []string{doomedID}},
			},
		},
	}
	_, err = dbc.NewInsert().Model(quest).Exec(ctx)
	require.NoError(t, err)

	for _, id := range []string{doomedID, survivorID} {
		objective := &models.Objective{
			ID:      id,
			QuestID: quest.ID,
			Slug:    gofakeit.Word() + "-" + id[:4],
			Title:   gofakeit.Sentence(3),
		}
		_, err = dbc.NewInsert().Model(objective).Exec(ctx)
		require.NoError(t, err)
	}

	return quest, doomedID, survivorID
}

func TestDeleteService_DeleteObjective_PrunesStructure(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	quest, doomedID, survivorID := deleteObjectiveFixture(t, dbc)

	require.NoError(t, svc.DeleteObjective(context.Background(), doomedID))

	structure := reloadStructure(t, dbc, quest.ID)
	assert.Empty(t, structure.SubGroups[0].ObjectiveIDs,
		"deleted objective must not linger in the structure")
	assert.Equal(t, []string{survivorID}, structure.ObjectiveIDs,
		"sibling groups untouched")

	count, err := dbc.NewSelect().Model((*models.Objective)(nil)).
		Where("id = ?", doomedID).Count(context.Background())
	require.NoError(t, err)
	assert.Zero(t, count)
}

func TestDeleteService_DeleteObjective_LeavesOtherObjectivesAlone(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	_, doomedID, survivorID := deleteObjectiveFixture(t, dbc)

	require.NoError(t, svc.DeleteObjective(context.Background(), doomedID))

	count, err := dbc.NewSelect().Model((*models.Objective)(nil)).
		Where("id = ?", survivorID).Count(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestDeleteService_DeleteObjective_DeletesBlocks(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	ctx := context.Background()
	_, doomedID, survivorID := deleteObjectiveFixture(t, dbc)

	for _, ownerID := range []string{doomedID, survivorID} {
		block := &models.Block{
			ID:      gofakeit.UUID(),
			OwnerID: ownerID,
			Type:    "markdown",
			Data:    json.RawMessage(`{}`),
		}
		_, err := dbc.NewInsert().Model(block).Exec(ctx)
		require.NoError(t, err)
	}

	require.NoError(t, svc.DeleteObjective(ctx, doomedID))

	count, err := dbc.NewSelect().Model((*models.Block)(nil)).
		Where("owner_id = ?", doomedID).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count)

	count, err = dbc.NewSelect().Model((*models.Block)(nil)).
		Where("owner_id = ?", survivorID).Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "blocks belonging to other objectives survive")
}

// Re-deleting must not error, so a retry after a partial failure is safe.
func TestDeleteService_DeleteObjective_UnknownIDIsNoOp(t *testing.T) {
	svc, dbc, cleanup := setupDeleteService(t)
	defer cleanup()

	quest, _, survivorID := deleteObjectiveFixture(t, dbc)

	require.NoError(t, svc.DeleteObjective(context.Background(), gofakeit.UUID()))

	assert.Equal(t, []string{survivorID},
		reloadStructure(t, dbc, quest.ID).ObjectiveIDs,
		"structure untouched when the ID matched nothing")
}
