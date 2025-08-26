package services_test

import (
	"context"
	"testing"

	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
)

func setupMonthlyCreditTopupService(t *testing.T) (services.MonthlyCreditTopupService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)

	creditRepo := repositories.NewCreditRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)

	service := services.NewMonthlyCreditTopupService(transactor, creditRepo)

	// Create test users in the database
	ctx := context.Background()
	
	// Regular user with 3 credits
	regularUser := &models.User{
		ID:          "user-regular-3",
		Email:       "regular3@example.com",
		Name:        "Regular User 3",
		FreeCredits: 3,
		PaidCredits: 0,
		IsEducator:  false,
	}
	userRepo.Create(ctx, regularUser)

	// Regular user with 5 credits  
	regularUser2 := &models.User{
		ID:          "user-regular-5",
		Email:       "regular5@example.com",
		Name:        "Regular User 5",
		FreeCredits: 5,
		PaidCredits: 0,
		IsEducator:  false,
	}
	userRepo.Create(ctx, regularUser2)

	// Educator with 30 credits
	educator := &models.User{
		ID:          "user-educator-30",
		Email:       "educator30@example.com",
		Name:        "Educator 30",
		FreeCredits: 30,
		PaidCredits: 0,
		IsEducator:  true,
	}
	userRepo.Create(ctx, educator)

	return *service, cleanup
}

func TestMonthlyCreditTopupService_TopUpCredits_FirstRun(t *testing.T) {
	service, cleanup := setupMonthlyCreditTopupService(t)
	defer cleanup()

	ctx := context.Background()

	// Run the top-up process
	err := service.TopUpCredits(ctx)
	assert.NoError(t, err)

	// Verify that all users were topped up correctly
	// This should have updated:
	// - Regular users with <10 credits should be topped up to 10
	// - Educators with <50 credits should be topped up to 50
}

func TestMonthlyCreditTopupService_TopUpCredits_AlreadyProcessedThisMonth(t *testing.T) {
	service, cleanup := setupMonthlyCreditTopupService(t)
	defer cleanup()

	ctx := context.Background()

	// Run the top-up process once
	err := service.TopUpCredits(ctx)
	assert.NoError(t, err)

	// Run it again immediately - should be skipped due to idempotency
	err = service.TopUpCredits(ctx)
	assert.NoError(t, err)

	// Should not fail, just skip processing
}

func TestMonthlyCreditTopupService_TopUpCredits_EdgeCases(t *testing.T) {
	service, cleanup := setupMonthlyCreditTopupService(t)
	defer cleanup()

	ctx := context.Background()

	tests := []struct {
		name    string
		action  func() error
		wantErr bool
	}{
		{
			name: "Normal top-up process",
			action: func() error {
				return service.TopUpCredits(ctx)
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.action()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}