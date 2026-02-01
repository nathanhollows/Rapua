package blocks_test

import (
	"testing"

	templates "github.com/nathanhollows/Rapua/v6/internal/templates/blocks"
	"github.com/stretchr/testify/assert"
)

func TestGetColorForIcon_ReturnsValidColors(t *testing.T) {
	validColors := map[string]bool{
		"primary":   true,
		"secondary": true,
		"accent":    true,
		"neutral":   true,
		"info":      true,
		"success":   true,
		"warning":   true,
		"error":     true,
		"base-100":  true,
		"base-200":  true,
		"base-300":  true,
	}

	icons := []string{
		"camera", "map", "circle-question-mark", "video",
		"message-circle-more", "mic", "qr-code", "list-todo",
		"footprints", "nfc", "dices",
	}

	for _, icon := range icons {
		t.Run(icon, func(t *testing.T) {
			color := templates.GetColorForIcon(icon)
			assert.NotEmpty(t, color, "color for %s should not be empty", icon)
			assert.True(t, validColors[color], "color %q for icon %s is not a valid DaisyUI color", color, icon)
		})
	}
}

func TestGetColorForIcon_UnknownIconReturnsValidDefault(t *testing.T) {
	color := templates.GetColorForIcon("unknown-icon")
	assert.NotEmpty(t, color)
}

func TestGetColorForIcon_EmptyIconReturnsValidDefault(t *testing.T) {
	color := templates.GetColorForIcon("")
	assert.NotEmpty(t, color)
}
