package blocks_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGameStatusAlertBlock_UpdateBlockData(t *testing.T) {
	block := blocks.GameStatusAlertBlock{}
	data := map[string][]string{
		"closed_message":    {"Game is closed"},
		"scheduled_message": {"Starting soon"},
		"show_countdown":    {"on"},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "Game is closed", block.ClosedMessage)
	assert.Equal(t, "Starting soon", block.ScheduledMessage)
	assert.True(t, block.ShowCountdown)
}

func TestGameStatusAlertBlock_UpdateBlockData_CheckboxUnchecked(t *testing.T) {
	block := blocks.GameStatusAlertBlock{
		ShowCountdown: true,
	}
	data := map[string][]string{
		"closed_message":    {"Game closed"},
		"scheduled_message": {"Game scheduled"},
		// Checkboxes not present = unchecked
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.False(t, block.ShowCountdown, "ShowCountdown should be false when not in form data")
}

func TestGameStatusAlertBlock_UpdateBlockData_EmptyMessages(t *testing.T) {
	block := blocks.GameStatusAlertBlock{
		ClosedMessage:    "Original closed",
		ScheduledMessage: "Original scheduled",
	}
	data := map[string][]string{
		"closed_message":    {""},
		"scheduled_message": {""},
		"show_countdown":    {"true"},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Empty(t, block.ClosedMessage)
	assert.Empty(t, block.ScheduledMessage)
	assert.True(t, block.ShowCountdown)
}
