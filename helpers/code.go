package helpers

import (
	"math/rand"
)

// The symbols team codes are created from.
// Confusing letters such as I and L, O and Q have one pair removed.
var symbols = []rune("ABCDEFGHJKLMNPRSTUVWXYZ")

// NewCode generates an alpha string of easily recognisable characters.
func NewCode(length int) string {
	b := make([]rune, length)
	for i := range length {
		b[i] = symbols[rand.Intn(len(symbols))]
	}
	return string(b)
}
