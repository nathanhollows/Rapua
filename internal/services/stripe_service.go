package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v6/config"
	"github.com/nathanhollows/Rapua/v6/db"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stripe/stripe-go/v83"
	"github.com/stripe/stripe-go/v83/charge"
	"github.com/stripe/stripe-go/v83/checkout/session"
	"github.com/stripe/stripe-go/v83/customer"
	"github.com/stripe/stripe-go/v83/webhook"
)

const (
	// MinCreditsPerPurchase is the minimum number of credits that can be purchased.
	MinCreditsPerPurchase = 3
)

var (
	// ErrInvalidCreditAmount is returned when the credit amount is invalid.
	ErrInvalidCreditAmount = errors.New("credit amount must be between 1 and 1000")
	// ErrStripeNotConfigured is returned when Stripe is not properly configured.
	ErrStripeNotConfigured = errors.New("stripe is not properly configured")
	// ErrPurchaseAlreadyProcessed is returned when a purchase has already been processed.
	ErrPurchaseAlreadyProcessed = errors.New("purchase has already been processed")
)

type StripeService struct {
	transactor      db.Transactor
	creditService   *CreditService
	purchaseRepo    *repositories.CreditPurchaseRepository
	userRepo        repositories.UserRepository
	logger          *slog.Logger
	stripeSecretKey string
	webhookSecret   string
	siteURL         string
}

func NewStripeService(
	transactor db.Transactor,
	creditService *CreditService,
	purchaseRepo *repositories.CreditPurchaseRepository,
	userRepo repositories.UserRepository,
	logger *slog.Logger,
) *StripeService {
	stripeSecretKey := os.Getenv("STRIPE_SECRET_KEY")
	webhookSecret := os.Getenv("STRIPE_WEBHOOK_SECRET")
	siteURL := os.Getenv("SITE_URL")

	// Initialize Stripe API key
	if stripeSecretKey != "" {
		//nolint:reassign // Correct usage as per the docs.
		stripe.Key = stripeSecretKey
	}

	return &StripeService{
		transactor:      transactor,
		creditService:   creditService,
		purchaseRepo:    purchaseRepo,
		userRepo:        userRepo,
		logger:          logger,
		stripeSecretKey: stripeSecretKey,
		webhookSecret:   webhookSecret,
		siteURL:         siteURL,
	}
}

// CreateCheckoutSession creates a Stripe Checkout session for credit purchase.
func (s *StripeService) CreateCheckoutSession(
	ctx context.Context,
	userID string,
	credits int,
) (*stripe.CheckoutSession, error) {
	// Validate inputs
	if credits < MinCreditsPerPurchase {
		return nil, ErrInvalidCreditAmount
	}

	if s.stripeSecretKey == "" || s.webhookSecret == "" {
		return nil, ErrStripeNotConfigured
	}

	// Get or create Stripe customer
	customerID, err := s.getOrCreateCustomer(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("getting or creating customer: %w", err)
	}

	// Calculate amount
	amountInCents := models.CalculatePurchaseAmount(credits)

	// Create checkout session
	purchaseID := uuid.New().String()
	params := &stripe.CheckoutSessionParams{
		Customer: stripe.String(customerID),
		Mode:     stripe.String(string(stripe.CheckoutSessionModePayment)),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
					Currency: stripe.String("nzd"),
					ProductData: &stripe.CheckoutSessionLineItemPriceDataProductDataParams{
						Name:        stripe.String("Rapua Credits"),
						Description: stripe.String(fmt.Sprintf("%d credits for team starts", credits)),
					},
					UnitAmount: stripe.Int64(int64(config.CreditPriceCents())),
				},
				Quantity: stripe.Int64(int64(credits)),
			},
		},
		SuccessURL: stripe.String(fmt.Sprintf("%s/admin/credits/success?session_id={CHECKOUT_SESSION_ID}", s.siteURL)),
		CancelURL:  stripe.String(fmt.Sprintf("%s/admin/credits/cancel", s.siteURL)),
		Metadata: map[string]string{
			"user_id":     userID,
			"purchase_id": purchaseID,
			"credits":     strconv.Itoa(credits),
		},
	}

	params.SetIdempotencyKey(purchaseID)

	sess, err := session.New(params)
	if err != nil {
		return nil, fmt.Errorf("creating checkout session: %w", err)
	}

	// Store pending purchase in database
	purchase := &models.CreditPurchase{
		ID:              purchaseID,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		UserID:          userID,
		Credits:         credits,
		AmountPaid:      amountInCents,
		StripeSessionID: sess.ID,
		StripeCustomerID: sql.NullString{
			String: customerID,
			Valid:  true,
		},
		Status: models.CreditPurchaseStatusPending,
	}

	err = s.purchaseRepo.Create(ctx, purchase)
	if err != nil {
		return nil, fmt.Errorf("creating purchase record: %w", err)
	}

	s.logger.InfoContext(ctx, "Created checkout session",
		"user_id", userID,
		"purchase_id", purchaseID,
		"credits", credits,
		"session_id", sess.ID,
	)

	return sess, nil
}

// getOrCreateCustomer gets or creates a Stripe customer for the user.
func (s *StripeService) getOrCreateCustomer(ctx context.Context, userID string) (string, error) {
	// Get user information
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("getting user: %w", err)
	}

	// If user already has a Stripe customer ID, return it
	if user.StripeCustomerID.Valid && user.StripeCustomerID.String != "" {
		return user.StripeCustomerID.String, nil
	}

	// Create new Stripe customer
	params := &stripe.CustomerParams{
		Email: stripe.String(user.Email),
		Name:  stripe.String(user.Name),
		Metadata: map[string]string{
			"user_id": user.ID,
		},
	}

	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("creating Stripe customer: %w", err)
	}

	// Update user with Stripe customer ID
	user.StripeCustomerID = sql.NullString{
		String: cust.ID,
		Valid:  true,
	}

	err = s.userRepo.Update(ctx, user)
	if err != nil {
		return "", fmt.Errorf("updating user with customer ID: %w", err)
	}

	s.logger.InfoContext(ctx, "Created Stripe customer",
		"user_id", user.ID,
		"customer_id", cust.ID,
	)

	return cust.ID, nil
}

// ProcessWebhook processes Stripe webhook events.
func (s *StripeService) ProcessWebhook(ctx context.Context, payload []byte, signature string) error {
	if s.webhookSecret == "" {
		return ErrStripeNotConfigured
	}

	// Verify webhook signature with API version mismatch tolerance ONLY for development
	// In production, strict version matching prevents potential security issues
	event, err := webhook.ConstructEventWithOptions(
		payload,
		signature,
		s.webhookSecret,
		webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: os.Getenv("IS_PROD") != "1",
		},
	)
	if err != nil {
		return fmt.Errorf("webhook signature verification failed: %w", err)
	}

	// Handle different event types
	//nolint:exhaustive // Only handling relevant checkout events, others logged in default case
	switch event.Type {
	case "checkout.session.completed":
		return s.handleCheckoutSessionCompleted(ctx, &event)
	case "checkout.session.async_payment_succeeded":
		return s.handleCheckoutSessionCompleted(ctx, &event)
	case "checkout.session.async_payment_failed":
		return s.handleCheckoutSessionFailed(ctx, &event)
	default:
		s.logger.InfoContext(ctx, "Unhandled webhook event type", "type", event.Type)
		return nil
	}
}

// handleCheckoutSessionCompleted processes successful checkout sessions.
func (s *StripeService) handleCheckoutSessionCompleted(ctx context.Context, event *stripe.Event) error {
	var sess stripe.CheckoutSession
	err := json.Unmarshal(event.Data.Raw, &sess)
	if err != nil {
		return fmt.Errorf("unmarshalling checkout session: %w", err)
	}

	// Get purchase record
	purchase, err := s.purchaseRepo.GetByStripeSessionID(ctx, sess.ID)
	if err != nil {
		return fmt.Errorf("getting purchase: %w", err)
	}
	if purchase == nil {
		return fmt.Errorf("purchase not found for session: %s", sess.ID)
	}

	// Check if already processed (idempotency)
	if purchase.Status == models.CreditPurchaseStatusCompleted {
		s.logger.WarnContext(ctx, "Purchase already processed", "purchase_id", purchase.ID)
		return ErrPurchaseAlreadyProcessed
	}

	// Validate webhook data matches our purchase record
	if sess.AmountTotal != int64(purchase.AmountPaid) {
		s.logger.ErrorContext(ctx, "Amount mismatch in webhook",
			"expected", purchase.AmountPaid,
			"received", sess.AmountTotal,
			"purchase_id", purchase.ID,
			"session_id", sess.ID,
		)
		return fmt.Errorf("payment amount mismatch: expected %d cents, received %d cents",
			purchase.AmountPaid, sess.AmountTotal)
	}

	// Validate credit quantity from metadata
	if creditsStr, ok := sess.Metadata["credits"]; ok {
		expectedCredits := strconv.Itoa(purchase.Credits)
		if creditsStr != expectedCredits {
			s.logger.ErrorContext(ctx, "Credits mismatch in webhook",
				"expected", purchase.Credits,
				"received", creditsStr,
				"purchase_id", purchase.ID,
				"session_id", sess.ID,
			)
			return fmt.Errorf("credits mismatch: expected %d, received %s",
				purchase.Credits, creditsStr)
		}
	}

	// Start transaction to ensure atomicity
	tx, err := s.transactor.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Add credits to user account
	reason := fmt.Sprintf("%s: via Stripe",
		models.CreditAdjustmentReasonPrefixPurchase,
	)

	err = s.creditService.creditRepo.AddCreditsWithTx(ctx, tx, purchase.UserID, 0, purchase.Credits)
	if err != nil {
		return fmt.Errorf("adding credits: %w", err)
	}

	// Log credit adjustment
	adjustment := &models.CreditAdjustments{
		ID:               uuid.New().String(),
		CreatedAt:        time.Now(),
		UserID:           purchase.UserID,
		Credits:          purchase.Credits,
		Reason:           reason,
		CreditPurchaseID: sql.NullString{String: purchase.ID, Valid: true},
	}
	err = s.creditService.creditRepo.CreateCreditAdjustmentWithTx(ctx, tx, adjustment)
	if err != nil {
		return fmt.Errorf("creating credit adjustment: %w", err)
	}

	// Update purchase status
	err = s.purchaseRepo.UpdateStatusWithTx(ctx, tx, purchase.ID, models.CreditPurchaseStatusCompleted)
	if err != nil {
		return fmt.Errorf("updating purchase status: %w", err)
	}

	// Update payment ID and fetch receipt URL if available
	if sess.PaymentIntent != nil {
		paymentID := sess.PaymentIntent.ID
		err = s.purchaseRepo.UpdateStripePaymentIDWithTx(ctx, tx, purchase.ID, paymentID)
		if err != nil {
			return fmt.Errorf("updating payment ID: %w", err)
		}

		// Fetch the charge to get the receipt URL
		// Note: We retrieve the latest charge for this payment intent
		chargeParams := &stripe.ChargeListParams{
			PaymentIntent: stripe.String(paymentID),
		}
		chargeParams.Limit = stripe.Int64(1)
		chargeIter := charge.List(chargeParams)
		if chargeIter.Next() {
			ch := chargeIter.Charge()
			if ch.ReceiptURL != "" {
				err = s.purchaseRepo.UpdateReceiptURLWithTx(ctx, tx, purchase.ID, ch.ReceiptURL)
				if err != nil {
					// Log error but don't fail the transaction - receipt URL is nice to have
					s.logger.ErrorContext(ctx, "updating receipt URL",
						"purchase_id", purchase.ID,
						"error", err,
					)
				}
			}
		}
	}

	// Commit transaction
	err = tx.Commit()
	if err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	s.logger.InfoContext(ctx, "Purchase completed successfully",
		"purchase_id", purchase.ID,
		"user_id", purchase.UserID,
		"credits", purchase.Credits,
		"session_id", sess.ID,
	)

	return nil
}

// handleCheckoutSessionFailed processes failed checkout sessions.
func (s *StripeService) handleCheckoutSessionFailed(ctx context.Context, event *stripe.Event) error {
	var sess stripe.CheckoutSession
	err := json.Unmarshal(event.Data.Raw, &sess)
	if err != nil {
		return fmt.Errorf("unmarshalling checkout session: %w", err)
	}

	// Get purchase record
	purchase, err := s.purchaseRepo.GetByStripeSessionID(ctx, sess.ID)
	if err != nil {
		return fmt.Errorf("getting purchase: %w", err)
	}
	if purchase == nil {
		return fmt.Errorf("purchase not found for session: %s", sess.ID)
	}

	// Update purchase status
	err = s.purchaseRepo.UpdateStatus(ctx, purchase.ID, models.CreditPurchaseStatusFailed)
	if err != nil {
		return fmt.Errorf("updating purchase status: %w", err)
	}

	s.logger.WarnContext(ctx, "Purchase failed",
		"purchase_id", purchase.ID,
		"user_id", purchase.UserID,
		"session_id", sess.ID,
	)

	return nil
}
