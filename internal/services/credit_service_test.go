package services_test

import (
	"context"
	"testing"

	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

func setupCreditService(t *testing.T) (services.CreditService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)

	creditRepo := repositories.NewCreditRepository(dbc)
	teamStartLogRepo := repositories.NewTeamStartLogRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)

	service := services.NewCreditService(transactor, creditRepo, teamStartLogRepo, userRepo)

	// Create test users
	ctx := context.Background()
	
	user1 := &models.User{
		ID:          "user-credit-1",
		Email:       "user1@example.com",
		Name:        "Test User 1",
		FreeCredits: 5,
		PaidCredits: 10,
		IsEducator:  false,
	}
	userRepo.Create(ctx, user1)

	user2 := &models.User{
		ID:          "user-credit-2",
		Email:       "user2@example.com",
		Name:        "Test User 2",
		FreeCredits: 0,
		PaidCredits: 3,
		IsEducator:  false,
	}
	userRepo.Create(ctx, user2)

	return *service, cleanup
}

func TestCreditService_GetCreditBalance(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name            string
		userID          string
		expectedFree    int
		expectedPaid    int
		wantErr         bool
	}{
		{
			name:         "Valid user with mixed credits",
			userID:       "user-credit-1",
			expectedFree: 5,
			expectedPaid: 10,
			wantErr:      false,
		},
		{
			name:         "Valid user with only paid credits",
			userID:       "user-credit-2",
			expectedFree: 0,
			expectedPaid: 3,
			wantErr:      false,
		},
		{
			name:         "Non-existent user",
			userID:       "non-existent",
			expectedFree: 0,
			expectedPaid: 0,
			wantErr:      true,
		},
		{
			name:         "Empty user ID",
			userID:       "",
			expectedFree: 0,
			expectedPaid: 0,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			freeCredits, paidCredits, err := service.GetCreditBalance(ctx, tt.userID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedFree, freeCredits)
				assert.Equal(t, tt.expectedPaid, paidCredits)
			}
		})
	}
}

func TestCreditService_AddCredits(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a fresh user for this test to avoid state interference
	freshUser := &models.User{
		ID:          "user-add-credits-test",
		Email:       "addtest@example.com",
		Name:        "Add Credits Test User",
		FreeCredits: 2,
		PaidCredits: 1,
		IsEducator:  false,
	}
	dbc, _ := setupDB(t)
	userRepo := repositories.NewUserRepository(dbc)
	userRepo.Create(ctx, freshUser)

	tests := []struct {
		name        string
		userID      string
		freeCredits int
		paidCredits int
		reason      string
		wantErr     bool
	}{
		{
			name:        "Add free credits only",
			userID:      "user-add-credits-test",
			freeCredits: 5,
			paidCredits: 0,
			reason:      "Test free credit addition",
			wantErr:     false,
		},
		{
			name:        "Add paid credits only",
			userID:      "user-add-credits-test",
			freeCredits: 0,
			paidCredits: 7,
			reason:      "Test paid credit addition",
			wantErr:     false,
		},
		{
			name:        "Cannot add both free and paid credits",
			userID:      "user-credit-1",
			freeCredits: 5,
			paidCredits: 5,
			reason:      "Invalid addition",
			wantErr:     true,
		},
		{
			name:        "Cannot add negative free credits",
			userID:      "user-credit-1",
			freeCredits: -5,
			paidCredits: 0,
			reason:      "Invalid negative addition",
			wantErr:     true,
		},
		{
			name:        "Cannot add negative paid credits",
			userID:      "user-credit-1",
			freeCredits: 0,
			paidCredits: -5,
			reason:      "Invalid negative addition",
			wantErr:     true,
		},
		{
			name:        "Non-existent user",
			userID:      "non-existent",
			freeCredits: 5,
			paidCredits: 0,
			reason:      "Test addition",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.AddCredits(ctx, tt.userID, tt.freeCredits, tt.paidCredits, tt.reason)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreditService_DeductCredits(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name         string
		userID       string
		creditsToDeduct int
		expectedSuccess bool
		wantErr      bool
	}{
		{
			name:            "Deduct from free credits",
			userID:          "user-credit-1",
			creditsToDeduct: 3,
			expectedSuccess: true,
			wantErr:         false,
		},
		{
			name:            "Deduct more than free, use paid",
			userID:          "user-credit-1", 
			creditsToDeduct: 8,
			expectedSuccess: true,
			wantErr:         false,
		},
		{
			name:            "Insufficient total credits",
			userID:          "user-credit-1",
			creditsToDeduct: 20,
			expectedSuccess: false,
			wantErr:         false,
		},
		{
			name:            "Deduct from paid only user",
			userID:          "user-credit-2",
			creditsToDeduct: 2,
			expectedSuccess: true,
			wantErr:         false,
		},
		{
			name:            "Non-existent user",
			userID:          "non-existent",
			creditsToDeduct: 1,
			expectedSuccess: false,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			success, err := service.DeductCredits(ctx, tt.userID, tt.creditsToDeduct)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedSuccess, success)
			}
		})
	}
}

func TestCreditService_DeductCreditForTeamStartWithTx(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()
	transactor := db.NewTransactor(setupDBConnection(t))

	tests := []struct {
		name       string
		userID     string
		teamID     string
		instanceID string
		wantErr    bool
	}{
		{
			name:       "Valid team start deduction",
			userID:     "user-credit-1",
			teamID:     "team-123",
			instanceID: "instance-456",
			wantErr:    false,
		},
		{
			name:       "User with only paid credits",
			userID:     "user-credit-2",
			teamID:     "team-124",
			instanceID: "instance-456",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx, err := transactor.BeginTx(ctx, nil)
			assert.NoError(t, err)
			defer tx.Rollback()

			err = service.DeductCreditForTeamStartWithTx(ctx, tx, tt.userID, tt.teamID, tt.instanceID)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreditService_GetCreditAdjustments(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	// Add some credits to create adjustment records
	err := service.AddCredits(ctx, "user-credit-1", 5, 0, "Test adjustment 1")
	assert.NoError(t, err)
	
	err = service.AddCredits(ctx, "user-credit-1", 3, 0, "Test adjustment 2")
	assert.NoError(t, err)

	tests := []struct {
		name           string
		filter         services.CreditAdjustmentFilter
		expectedCount  int
		wantErr        bool
	}{
		{
			name: "Get all adjustments for user",
			filter: services.CreditAdjustmentFilter{
				UserID: "user-credit-1",
				Limit:  0,
				Offset: 0,
			},
			expectedCount: 2,
			wantErr:       false,
		},
		{
			name: "Get adjustments with limit",
			filter: services.CreditAdjustmentFilter{
				UserID: "user-credit-1",
				Limit:  1,
				Offset: 0,
			},
			expectedCount: 1,
			wantErr:       false,
		},
		{
			name: "Get adjustments for non-existent user",
			filter: services.CreditAdjustmentFilter{
				UserID: "non-existent",
				Limit:  0,
				Offset: 0,
			},
			expectedCount: 0,
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			adjustments, err := service.GetCreditAdjustments(ctx, tt.filter)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, adjustments, tt.expectedCount)
			}
		})
	}
}

func TestCreditService_GetTeamStartLogsSummary(t *testing.T) {
	service, cleanup := setupCreditService(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name    string
		filter  services.TeamStartLogFilter
		wantErr bool
	}{
		{
			name: "Valid daily summary",
			filter: services.TeamStartLogFilter{
				UserID:  "user-credit-1",
				GroupBy: "day",
			},
			wantErr: false,
		},
		{
			name: "Valid weekly summary",
			filter: services.TeamStartLogFilter{
				UserID:  "user-credit-1",
				GroupBy: "week",
			},
			wantErr: false,
		},
		{
			name: "Valid monthly summary",
			filter: services.TeamStartLogFilter{
				UserID:  "user-credit-1",
				GroupBy: "month",
			},
			wantErr: false,
		},
		{
			name: "Invalid groupBy parameter",
			filter: services.TeamStartLogFilter{
				UserID:  "user-credit-1",
				GroupBy: "invalid",
			},
			wantErr: true,
		},
		{
			name: "Default groupBy (empty)",
			filter: services.TeamStartLogFilter{
				UserID:  "user-credit-1",
				GroupBy: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary, err := service.GetTeamStartLogsSummary(ctx, tt.filter)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, summary)
			}
		})
	}
}

// Helper function to get DB connection for transaction tests
func setupDBConnection(t *testing.T) *bun.DB {
	t.Helper()
	dbc, _ := setupDB(t)
	return dbc
}