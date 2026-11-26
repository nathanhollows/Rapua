package services_test

import (
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/internal/config"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v83"
	"github.com/uptrace/bun"
)

func setupStripeService(
	t *testing.T,
) (*services.StripeService, repositories.UserRepository, *repositories.CreditPurchaseRepository, db.Transactor, *repositories.CreditRepository, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)

	creditRepo := repositories.NewCreditRepository(dbc)
	runStartLogRepo := repositories.NewRunStartLogRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)
	purchaseRepo := repositories.NewCreditPurchaseRepository(dbc)

	creditService := services.NewCreditService(transactor, creditRepo, runStartLogRepo, userRepo)
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
			assert.Equal(t, tc.credits*config.CreditPriceCents(), purchase.AmountPaid)
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
		AmountPaid:      25 * config.CreditPriceCents(),
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
		AmountPaid:      25 * config.CreditPriceCents(),
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
		AmountPaid:      25 * config.CreditPriceCents(),
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

// TestStripeService_ProcessWebhook_FetchesChargeBeforeTransaction pins the
// ordering this path depends on: the charge lookup that fills the receipt URL
// is a network call, and a network call inside the write transaction holds the
// lock for as long as Stripe takes to answer.
func TestStripeService_ProcessWebhook_FetchesChargeBeforeTransaction(t *testing.T) {
	t.Setenv("STRIPE_SECRET_KEY", "sk_test_ordering")
	t.Setenv("STRIPE_WEBHOOK_SECRET", "whsec_test_ordering")

	log := &eventLog{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		log.add("charge-fetch")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(
			`{"object":"list","data":[{"id":"ch_test_order","object":"charge",` +
				`"receipt_url":"https://pay.stripe.com/receipt/test"}],` +
				`"has_more":false,"url":"/v1/charges"}`,
		))
	}))
	defer server.Close()

	originalBackend := stripe.GetBackend(stripe.APIBackend)
	stripe.SetBackend(stripe.APIBackend, stripe.GetBackendWithConfig(
		stripe.APIBackend,
		&stripe.BackendConfig{
			URL:        stripe.String(server.URL),
			HTTPClient: server.Client(),
		},
	))
	t.Cleanup(func() { stripe.SetBackend(stripe.APIBackend, originalBackend) })

	dbc, cleanup := setupDB(t)
	defer cleanup()

	transactor := &recordingTransactor{inner: db.NewTransactor(dbc), log: log}
	creditRepo := repositories.NewCreditRepository(dbc)
	runStartLogRepo := repositories.NewRunStartLogRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)
	purchaseRepo := repositories.NewCreditPurchaseRepository(dbc)

	creditService := services.NewCreditService(db.NewTransactor(dbc), creditRepo, runStartLogRepo, userRepo)
	svc := services.NewStripeService(transactor, creditService, purchaseRepo, userRepo, newTLogger(t))

	ctx := context.Background()

	user := &models.User{
		ID:          gofakeit.UUID(),
		Email:       gofakeit.Email(),
		Name:        gofakeit.Name(),
		FreeCredits: 10,
		PaidCredits: 0,
	}
	require.NoError(t, userRepo.Create(ctx, user))

	purchase := &models.CreditPurchase{
		ID:               gofakeit.UUID(),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
		UserID:           user.ID,
		Credits:          25,
		AmountPaid:       25 * config.CreditPriceCents(),
		StripeSessionID:  "cs_test_" + gofakeit.UUID(),
		StripeCustomerID: sql.NullString{String: "cus_test_" + gofakeit.UUID(), Valid: true},
		Status:           models.CreditPurchaseStatusPending,
	}
	require.NoError(t, purchaseRepo.Create(ctx, purchase))

	session := stripe.CheckoutSession{
		ID:            purchase.StripeSessionID,
		AmountTotal:   int64(purchase.AmountPaid),
		PaymentIntent: &stripe.PaymentIntent{ID: "pi_test_" + gofakeit.UUID()},
		Metadata:      map[string]string{"credits": strconv.Itoa(purchase.Credits)},
	}
	sessionJSON, err := json.Marshal(session)
	require.NoError(t, err)
	payload, err := json.Marshal(stripe.Event{
		Object: "event",
		Type:   "checkout.session.completed",
		Data:   &stripe.EventData{Raw: sessionJSON},
	})
	require.NoError(t, err)

	now := time.Now()
	signature := fmt.Sprintf(
		"t=%d,v1=%s",
		now.Unix(),
		hex.EncodeToString(stripe.ComputeSignature(now, payload, "whsec_test_ordering")),
	)
	require.NoError(t, svc.ProcessWebhook(ctx, payload, signature))

	events := log.snapshot()
	fetchIdx := slices.Index(events, "charge-fetch")
	beginIdx := slices.Index(events, "begin-tx")
	require.NotEqual(t, -1, fetchIdx, "the charge fetch never happened")
	require.NotEqual(t, -1, beginIdx, "the transaction never opened")
	assert.Less(t, fetchIdx, beginIdx, "the charge fetch happened inside the write transaction")

	got, err := purchaseRepo.GetByStripeSessionID(ctx, purchase.StripeSessionID)
	require.NoError(t, err)
	assert.Equal(t, models.CreditPurchaseStatusCompleted, got.Status)
	assert.Equal(t, session.PaymentIntent.ID, got.StripePaymentID.String)
	assert.Equal(t, "https://pay.stripe.com/receipt/test", got.ReceiptURL.String)
}

// eventLog records events the test cares about in order. The webhook handler
// and the fake Stripe server run in different goroutines, so access is guarded.
type eventLog struct {
	mu     sync.Mutex
	events []string
}

func (l *eventLog) add(event string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *eventLog) snapshot() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.events)
}

// recordingTransactor wraps a real transactor and notes each BeginTx in the
// shared log, so the test can see where the transaction falls in the order.
type recordingTransactor struct {
	inner db.Transactor
	log   *eventLog
}

func (t *recordingTransactor) BeginTx(ctx context.Context, opts *sql.TxOptions) (*bun.Tx, error) {
	t.log.add("begin-tx")
	return t.inner.BeginTx(ctx, opts)
}
