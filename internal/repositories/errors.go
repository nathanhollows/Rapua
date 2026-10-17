package repositories

import "errors"

// ErrInsufficientCredits is the credit repository's answer when a deduction
// cannot be made. It lives here rather than in services because the repository
// must not import the layer above it; the credit service translates it into the
// sentinel the handlers already speak.
var ErrInsufficientCredits = errors.New("insufficient credits to start team")
