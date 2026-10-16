package admin //nolint:testpackage // The Handler's fields are unexported.

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"errors"

	"github.com/stretchr/testify/assert"
	"github.com/stripe/stripe-go/v83"
)

// recordingStripe remembers whether a webhook reached the service, so a test
// can tell a refused body from a delivered one.
type recordingStripe struct {
	called bool
}

func (s *recordingStripe) CreateCheckoutSession(
	context.Context, string, int,
) (*stripe.CheckoutSession, error) {
	return nil, errors.New("not used by this test")
}

func (s *recordingStripe) ProcessWebhook(context.Context, []byte, string) error {
	s.called = true
	return nil
}

func newWebhookHandler(stripe StripeService) *Handler {
	return &Handler{
		logger:        slog.New(slog.DiscardHandler),
		stripeService: stripe,
	}
}

// The webhook route is public and answers to a signature rather than a session,
// so the body is capped before the read: an anonymous caller does not get to
// decide how much memory the request costs.
func TestStripeWebhook_OversizedBodyIsRefused(t *testing.T) {
	stripe := &recordingStripe{}
	h := newWebhookHandler(stripe)

	body := bytes.Repeat([]byte("x"), maxWebhookBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/stripe", bytes.NewReader(body))
	req.Header.Set("Stripe-Signature", "t=1,v1=whatever")
	w := httptest.NewRecorder()

	h.StripeWebhook(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, stripe.called, "the body is refused before the signature is even checked")
}
