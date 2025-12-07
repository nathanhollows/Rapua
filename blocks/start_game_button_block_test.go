package blocks_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartGameButtonBlock_UpdateBlockData(t *testing.T) {
	block := blocks.StartGameButtonBlock{}
	data := map[string][]string{
		"scheduled_button_text": {"Please wait"},
		"active_button_text":    {"Let's go!"},
		"button_style":          {"primary"},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "Please wait", block.ScheduledButtonText)
	assert.Equal(t, "Let's go!", block.ActiveButtonText)
	assert.Equal(t, "primary", block.ButtonStyle)
}

func TestStartGameButtonBlock_UpdateBlockData_EmptyButtonText(t *testing.T) {
	block := blocks.StartGameButtonBlock{
		ScheduledButtonText: "Original",
		ActiveButtonText:    "Original",
	}
	data := map[string][]string{
		"scheduled_button_text": {""},
		"active_button_text":    {""},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "", block.ScheduledButtonText)
	assert.Equal(t, "", block.ActiveButtonText)
}

func TestStartGameButtonBlock_UpdateBlockData_ButtonStyle(t *testing.T) {
	block := blocks.StartGameButtonBlock{}
	data := map[string][]string{
		"button_style": {"accent"},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "accent", block.ButtonStyle)
}
