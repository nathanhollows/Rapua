package helpers_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v6/helpers"
)

func TestNewCode(t *testing.T) {
	tests := []struct {
		name   string
		length int
	}{
		{
			name:   "Short Code",
			length: 6,
		},
		{
			name:   "Long Code",
			length: 12,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code := helpers.NewCode(tt.length)
			if len(code) != tt.length {
				t.Errorf("NewCode() = %v, want %v", len(code), tt.length)
			}
		})
	}
}
