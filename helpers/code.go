package helpers

import (
	"math/rand/v2"
)

// NewCode generates an alpha string of easily recognisable characters.
// Confusing letters such as I and L, O and Q have one pair removed.
func NewCode(length int) string {
	// The symbols team codes are created from.
	symbols := []rune("ABCDEFGHJKLMNPRSTUVWXYZ")

	b := make([]rune, length)
	for i := range length {
		b[i] = symbols[rand.IntN(len(symbols))] //nolint:gosec // Team codes don't need cryptographic randomness
	}
	return string(b)
}
