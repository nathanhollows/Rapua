package blocks_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamNameChangerBlock_UpdateBlockData(t *testing.T) {
	block := blocks.TeamNameChangerBlock{}
	data := map[string][]string{
		"button_text":    {"Choose your team name!"},
		"allow_changing": {"on"},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "Choose your team name!", block.ButtonText)
	assert.True(t, block.AllowChanging)
}

func TestTeamNameChangerBlock_UpdateBlockData_CheckboxChecked(t *testing.T) {
	block := blocks.TeamNameChangerBlock{}
	data := map[string][]string{
		"button_text":    {"Set your name"},
		"allow_changing": {"true"},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.True(t, block.AllowChanging)
}

func TestTeamNameChangerBlock_UpdateBlockData_CheckboxUnchecked(t *testing.T) {
	block := blocks.TeamNameChangerBlock{
		AllowChanging: true, // Start with it checked
	}
	data := map[string][]string{
		"button_text": {"Set your name"},
		// allow_changing not present = unchecked
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.False(t, block.AllowChanging, "AllowChanging should be false when checkbox is not in form data")
}

func TestTeamNameChangerBlock_UpdateBlockData_EmptyButtonText(t *testing.T) {
	block := blocks.TeamNameChangerBlock{
		ButtonText: "Original text",
	}
	data := map[string][]string{
		"button_text":    {""},
		"allow_changing": {"on"},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	// Empty button text in form data updates to empty (allows admin to clear it)
	assert.Empty(t, block.ButtonText, "Empty button text should update to empty string")
	assert.True(t, block.AllowChanging)
}

func TestTeamNameChangerBlock_UpdateBlockData_OnlyButtonText(t *testing.T) {
	block := blocks.TeamNameChangerBlock{
		AllowChanging: true,
	}
	data := map[string][]string{
		"button_text": {"New prompt text"},
	}

	err := block.UpdateBlockData(data)
	require.NoError(t, err)
	assert.Equal(t, "New prompt text", block.ButtonText)
	assert.False(t, block.AllowChanging, "AllowChanging should be false when not in form data")
}

func TestTeamNameChangerBlock_RequiresValidation(t *testing.T) {
	block := blocks.TeamNameChangerBlock{}
	assert.True(t, block.RequiresValidation())
}

func TestTeamNameChangerBlock_ValidatePlayerInput(t *testing.T) {
	block := blocks.TeamNameChangerBlock{
		BaseBlock: blocks.BaseBlock{
			Points: 10,
		},
		ButtonText:    "Set your team name!",
		AllowChanging: true,
	}

	state := &blocks.MockPlayerState{}

	// The block doesn't care about input - it just marks as complete
	input := map[string][]string{}
	newState, err := block.ValidatePlayerInput(state, input)
	require.NoError(t, err)

	// Assert that state is marked as complete
	assert.True(t, newState.IsComplete())
}
