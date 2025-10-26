package admin

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/nathanhollows/Rapua/v5/internal/services"
	templates "github.com/nathanhollows/Rapua/v5/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v5/models"
	"github.com/stripe/stripe-go/v83"
)

// StripeService provides Stripe-related operations.
type StripeService interface {
	CreateCheckoutSession(ctx context.Context, userID string, credits int) (*stripe.CheckoutSession, error)
	ProcessWebhook(ctx context.Context, payload []byte, signature string) error
}

// CreditPurchaseRepository provides credit purchase data operations.
type CreditPurchaseRepository interface {
	GetByStripeSessionID(ctx context.Context, sessionID string) (*models.CreditPurchase, error)
}

// CreateCheckoutSession creates a Stripe Checkout session for credit purchase.
func (h *Handler) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	r.ParseForm()

	// Validate credits amount
	creditsStr := r.FormValue("credits")
	credits, err := strconv.Atoi(creditsStr)
	if err != nil {
		h.handleError(w, r, "CreateCheckoutSession: parse credits",
			"Invalid credit amount", err)
		return
	}

	if credits < services.MinCreditsPerPurchase {
		h.handleError(w, r, "CreateCheckoutSession: invalid credits",
			fmt.Sprintf("Credit amount must be greater than %d", services.MinCreditsPerPurchase), nil)
		return
	}

	// Create Stripe checkout session
	session, err := h.stripeService.CreateCheckoutSession(r.Context(), user.ID, credits)
	if err != nil {
		if errors.Is(err, services.ErrStripeNotConfigured) {
			h.handleError(w, r, "CreateCheckoutSession: Stripe not configured",
				"Credit purchases are not currently available", err)
			return
		}
		h.handleError(w, r, "CreateCheckoutSession: create session",
			"Failed to create checkout session", err)
		return
	}

	h.redirect(w, r, session.URL)
}

// StripeWebhook handles Stripe webhook events.
func (h *Handler) StripeWebhook(w http.ResponseWriter, r *http.Request) {
	// Read request body
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		h.logger.Error("StripeWebhook: read body", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Get Stripe signature header
	signature := r.Header.Get("Stripe-Signature")
	if signature == "" {
		h.logger.Error("StripeWebhook: missing signature")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Process webhook
	err = h.stripeService.ProcessWebhook(r.Context(), payload, signature)
	if err != nil {
		if errors.Is(err, services.ErrPurchaseAlreadyProcessed) {
			// Idempotency: return 200 for already processed events
			h.logger.Warn("StripeWebhook: purchase already processed")
			w.WriteHeader(http.StatusOK)
			return
		}

		h.logger.Error("StripeWebhook: process webhook", "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// CreditPurchaseSuccess displays the success page after a Stripe purchase.
func (h *Handler) CreditPurchaseSuccess(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// Get session ID from query parameter
	sessionID := r.URL.Query().Get("session_id")
	if sessionID == "" {
		http.Redirect(w, r, "/admin/settings/credits", http.StatusSeeOther)
		return
	}

	// SECURITY: Verify session belongs to authenticated user
	purchase, err := h.creditPurchaseRepo.GetByStripeSessionID(r.Context(), sessionID)
	if err != nil {
		h.logger.Error("failed to get purchase by session ID",
			"error", err,
			"session_id", sessionID,
		)
		http.Redirect(w, r, "/admin/settings/credits", http.StatusSeeOther)
		return
	}

	if purchase == nil || purchase.UserID != user.ID {
		h.logger.Warn("unauthorized access to purchase success page",
			"user_id", user.ID,
			"session_id", sessionID,
			"purchase_user_id", func() string {
				if purchase != nil {
					return purchase.UserID
				}
				return "nil"
			}(),
		)
		http.Redirect(w, r, "/admin/settings/credits", http.StatusSeeOther)
		return
	}

	// Render success page
	c := templates.CreditPurchaseSuccess(*user, sessionID)
	err = templates.Layout(c, *user, "Purchase Successful", "").Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering credit purchase success page", "error", err.Error())
	}
}

// CreditPurchaseCancel displays the cancel page when a purchase is cancelled.
func (h *Handler) CreditPurchaseCancel(w http.ResponseWriter, r *http.Request) {
	user := h.UserFromContext(r.Context())

	// Render cancel page
	c := templates.CreditPurchaseCancel(*user)
	err := templates.Layout(c, *user, "Purchase Cancelled", "").Render(r.Context(), w)
	if err != nil {
		h.logger.Error("rendering credit purchase cancel page", "error", err.Error())
	}
}
