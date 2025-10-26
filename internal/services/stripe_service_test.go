package services_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v5/db"
	"github.com/nathanhollows/Rapua/v5/internal/services"
	"github.com/nathanhollows/Rapua/v5/models"
	"github.com/nathanhollows/Rapua/v5/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v83"
)

func setupStripeService(
	t *testing.T,
) (*services.StripeService, repositories.UserRepository, *repositories.CreditPurchaseRepository, db.Transactor, *repositories.CreditRepository, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)

	creditRepo := repositories.NewCreditRepository(dbc)
	teamStartLogRepo := repositories.NewTeamStartLogRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)
	purchaseRepo := repositories.NewCreditPurchaseRepository(dbc)

	creditService := services.NewCreditService(transactor, creditRepo, teamStartLogRepo, userRepo)
	stripeService := services.NewStripeService(transactor, creditService, purchaseRepo, userRepo, newTLogger(t))

	return stripeService, userRepo, purchaseRepo, transactor, creditRepo, cleanup
}

func TestStripeService_CreateCheckoutSession_ValidInputs(t *testing.T) {
	testCases := []struct {
		name    string
		credits int
		wantErr bool
	}{
		{
			name:    "Minimum credits (1)",
			credits: 1,
			wantErr: false,
		},
		{
			name:    "Valid credits (50)",
			credits: 50,
			wantErr: false,
		},
		{
			name:    "Maximum credits (1000)",
			credits: 1000,
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Skip if Stripe is not configured
			if !isStripeConfigured() {
				t.Skip("Stripe not configured")
			}

			svc, userRepo, purchaseRepo, _, _, cleanup := setupStripeService(t)
			defer cleanup()

			ctx := context.Background()

			// Create user
			user := &models.User{
				ID:    gofakeit.UUID(),
				Email: gofakeit.Email(),
				Name:  gofakeit.Name(),
			}
			err := userRepo.Create(ctx, user)
			require.NoError(t, err)

			// Create checkout session
			session, err := svc.CreateCheckoutSession(ctx, user.ID, tc.credits)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, session)
			assert.NotEmpty(t, session.URL)

			// Verify purchase record was created
			purchase, err := purchaseRepo.GetByStripeSessionID(ctx, session.ID)
			require.NoError(t, err)
			assert.Equal(t, user.ID, purchase.UserID)
			assert.Equal(t, tc.credits, purchase.Credits)
			assert.Equal(t, models.CalculatePurchaseAmount(tc.credits), purchase.AmountPaid)
			assert.Equal(t, models.CreditPurchaseStatusPending, purchase.Status)
		})
	}
}

func TestStripeService_CreateCheckoutSession_InvalidCredits(t *testing.T) {
	testCases := []struct {
		name    string
		credits int
		wantErr error
	}{
		{
			name:    "Zero credits",
			credits: 0,
			wantErr: services.ErrInvalidCreditAmount,
		},
		{
			name:    "Negative credits",
			credits: -1,
			wantErr: services.ErrInvalidCreditAmount,
		},
		{
			name:    "Over maximum (1001)",
			credits: 1001,
			wantErr: services.ErrInvalidCreditAmount,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if !isStripeConfigured() {
				t.Skip("Stripe not configured")
			}

			svc, userRepo, _, _, _, cleanup := setupStripeService(t)
			defer cleanup()

			ctx := context.Background()

			user := &models.User{
				ID:    gofakeit.UUID(),
				Email: gofakeit.Email(),
				Name:  gofakeit.Name(),
			}
			err := userRepo.Create(ctx, user)
			require.NoError(t, err)

			_, err = svc.CreateCheckoutSession(ctx, user.ID, tc.credits)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

func TestStripeService_CreateCheckoutSession_StripeCustomerCreation(t *testing.T) {
	if !isStripeConfigured() {
		t.Skip("Stripe not configured")
	}

	svc, userRepo, _, _, _, cleanup := setupStripeService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user without Stripe customer ID
	user := &models.User{
		ID:    gofakeit.UUID(),
		Email: gofakeit.Email(),
		Name:  gofakeit.Name(),
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)

	// Create first checkout session - should create Stripe customer
	session1, err := svc.CreateCheckoutSession(ctx, user.ID, 10)
	require.NoError(t, err)
	require.NotNil(t, session1)

	// Verify user now has Stripe customer ID
	updatedUser, err := userRepo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	require.True(t, updatedUser.StripeCustomerID.Valid)
	require.NotEmpty(t, updatedUser.StripeCustomerID.String)
	firstCustomerID := updatedUser.StripeCustomerID.String

	// Create second checkout session - should reuse existing customer
	session2, err := svc.CreateCheckoutSession(ctx, user.ID, 20)
	require.NoError(t, err)
	require.NotNil(t, session2)

	// Verify customer ID hasn't changed
	updatedUser, err = userRepo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, firstCustomerID, updatedUser.StripeCustomerID.String)
}

func TestStripeService_ProcessWebhook_CheckoutSessionCompleted(t *testing.T) {
	if !isStripeConfigured() {
		t.Skip("Stripe not configured")
	}

	_, userRepo, purchaseRepo, _, _, cleanup := setupStripeService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user := &models.User{
		ID:          gofakeit.UUID(),
		Email:       gofakeit.Email(),
		Name:        gofakeit.Name(),
		FreeCredits: 10,
		PaidCredits: 0,
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)

	// Create pending purchase
	purchase := &models.CreditPurchase{
		ID:              gofakeit.UUID(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		UserID:          user.ID,
		Credits:         25,
		AmountPaid:      models.CalculatePurchaseAmount(25),
		StripeSessionID: "cs_test_" + gofakeit.UUID(),
		StripeCustomerID: sql.NullString{
			String: "cus_test_" + gofakeit.UUID(),
			Valid:  true,
		},
		Status: models.CreditPurchaseStatusPending,
	}
	err = purchaseRepo.Create(ctx, purchase)
	require.NoError(t, err)

	// Create mock webhook event
	_ = createMockCheckoutSessionCompletedEvent(purchase.StripeSessionID)

	// Note: This test would require a valid Stripe webhook signature
	// In a real test, you would use Stripe's test signature or mock the webhook.ConstructEvent function
	// For now, we're testing the business logic assuming signature verification passes
	t.Skip("Webhook signature verification requires Stripe test environment setup")
}

func TestStripeService_ProcessWebhook_IdempotentProcessing(t *testing.T) {
	if !isStripeConfigured() {
		t.Skip("Stripe not configured")
	}

	_, userRepo, purchaseRepo, _, _, cleanup := setupStripeService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user := &models.User{
		ID:          gofakeit.UUID(),
		Email:       gofakeit.Email(),
		Name:        gofakeit.Name(),
		FreeCredits: 10,
		PaidCredits: 0,
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)

	// Create already-completed purchase
	purchase := &models.CreditPurchase{
		ID:              gofakeit.UUID(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		UserID:          user.ID,
		Credits:         25,
		AmountPaid:      models.CalculatePurchaseAmount(25),
		StripeSessionID: "cs_test_" + gofakeit.UUID(),
		StripeCustomerID: sql.NullString{
			String: "cus_test_" + gofakeit.UUID(),
			Valid:  true,
		},
		Status: models.CreditPurchaseStatusCompleted,
	}
	err = purchaseRepo.Create(ctx, purchase)
	require.NoError(t, err)

	// Attempting to process the same webhook again should return ErrPurchaseAlreadyProcessed
	t.Skip("Webhook processing test requires Stripe test environment setup")
}

func TestStripeService_PurchaseAmountCalculation(t *testing.T) {
	testCases := []struct {
		name           string
		credits        int
		expectedAmount int // in cents
	}{
		{
			name:           "1 credit",
			credits:        1,
			expectedAmount: 35, // $0.35
		},
		{
			name:           "10 credits",
			credits:        10,
			expectedAmount: 350, // $3.50
		},
		{
			name:           "100 credits",
			credits:        100,
			expectedAmount: 3500, // $35.00
		},
		{
			name:           "1000 credits",
			credits:        1000,
			expectedAmount: 35000, // $350.00
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			amount := models.CalculatePurchaseAmount(tc.credits)
			assert.Equal(t, tc.expectedAmount, amount)
		})
	}
}

func TestStripeService_CreditAdjustmentPurchaseLink(t *testing.T) {
	// This test verifies that credit adjustments created from purchases
	// are properly linked via the CreditPurchaseID field
	_, userRepo, purchaseRepo, transactor, creditRepo, cleanup := setupStripeService(t)
	defer cleanup()

	ctx := context.Background()

	// Create user
	user := &models.User{
		ID:          gofakeit.UUID(),
		Email:       gofakeit.Email(),
		Name:        gofakeit.Name(),
		FreeCredits: 10,
		PaidCredits: 0,
	}
	err := userRepo.Create(ctx, user)
	require.NoError(t, err)

	// Create pending purchase
	purchase := &models.CreditPurchase{
		ID:              gofakeit.UUID(),
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		UserID:          user.ID,
		Credits:         25,
		AmountPaid:      models.CalculatePurchaseAmount(25),
		StripeSessionID: "cs_test_" + gofakeit.UUID(),
		StripeCustomerID: sql.NullString{
			String: "cus_test_" + gofakeit.UUID(),
			Valid:  true,
		},
		Status: models.CreditPurchaseStatusPending,
	}
	err = purchaseRepo.Create(ctx, purchase)
	require.NoError(t, err)

	// Simulate the handleCheckoutSessionCompleted logic directly
	// Start transaction
	tx, err := transactor.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer func() { _ = tx.Rollback() }()

	// Add credits to user account
	reason := fmt.Sprintf("%s: via Stripe", models.CreditAdjustmentReasonPrefixPurchase)

	err = creditRepo.AddCreditsWithTx(ctx, tx, purchase.UserID, 0, purchase.Credits)
	require.NoError(t, err)

	// Create credit adjustment with purchase link
	adjustment := &models.CreditAdjustments{
		ID:               gofakeit.UUID(),
		CreatedAt:        time.Now(),
		UserID:           purchase.UserID,
		Credits:          purchase.Credits,
		Reason:           reason,
		CreditPurchaseID: sql.NullString{String: purchase.ID, Valid: true},
	}
	err = creditRepo.CreateCreditAdjustmentWithTx(ctx, tx, adjustment)
	require.NoError(t, err)

	// Update purchase status
	err = purchaseRepo.UpdateStatusWithTx(ctx, tx, purchase.ID, models.CreditPurchaseStatusCompleted)
	require.NoError(t, err)

	// Commit transaction
	err = tx.Commit()
	require.NoError(t, err)

	// Verify the credit adjustment was created with proper link
	adjustments, err := creditRepo.GetCreditAdjustmentsByUserID(ctx, user.ID)
	require.NoError(t, err)
	require.Len(t, adjustments, 1)

	adj := adjustments[0]
	assert.Equal(t, purchase.UserID, adj.UserID)
	assert.Equal(t, purchase.Credits, adj.Credits)
	assert.True(t, adj.CreditPurchaseID.Valid, "CreditPurchaseID should be set")
	assert.Equal(t, purchase.ID, adj.CreditPurchaseID.String)

	// Verify the relationship is loaded
	assert.NotNil(t, adj.CreditPurchase, "CreditPurchase relationship should be loaded")
	if adj.CreditPurchase != nil {
		assert.Equal(t, purchase.ID, adj.CreditPurchase.ID)
		assert.Equal(t, purchase.Credits, adj.CreditPurchase.Credits)
		assert.Equal(t, models.CreditPurchaseStatusCompleted, adj.CreditPurchase.Status)
	}

	// Verify user credits were updated
	updatedUser, err := userRepo.GetByID(ctx, user.ID)
	require.NoError(t, err)
	assert.Equal(t, 10, updatedUser.FreeCredits, "Free credits should remain unchanged")
	assert.Equal(t, 25, updatedUser.PaidCredits, "Paid credits should be added")
}

// Helper function to check if Stripe is configured.
func isStripeConfigured() bool {
	// In a real test environment, you would check for test API keys
	// For now, we return false to skip tests that require Stripe
	return false
}

// Helper function to create mock Stripe event.
func createMockCheckoutSessionCompletedEvent(sessionID string) stripe.Event {
	session := stripe.CheckoutSession{
		ID: sessionID,
		PaymentIntent: &stripe.PaymentIntent{
			ID: "pi_test_" + gofakeit.UUID(),
		},
	}

	sessionJSON, _ := json.Marshal(session)

	return stripe.Event{
		Type: "checkout.session.completed",
		Data: &stripe.EventData{
			Raw: sessionJSON,
		},
	}
}
