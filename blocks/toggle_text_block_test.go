package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToggleTextBlock_ParseData(t *testing.T) {
	t.Run("parses title and content", func(t *testing.T) {
		data := `{"title":"References","content":"Some content"}`
		block := blocks.ToggleTextBlock{
			BaseBlock: blocks.BaseBlock{Data: json.RawMessage(data)},
		}
		err := block.ParseData()
		require.NoError(t, err)
		assert.Equal(t, "References", block.Title)
		assert.Equal(t, "Some content", block.Content)
	})
}

func TestToggleTextBlock_UpdateBlockData(t *testing.T) {
	t.Run("updates title and content", func(t *testing.T) {
		block := blocks.ToggleTextBlock{}
		err := block.UpdateBlockData(map[string][]string{
			"title":   {"References"},
			"content": {"- Source 1\n- Source 2"},
		})
		require.NoError(t, err)
		assert.Equal(t, "References", block.Title)
		assert.Equal(t, "- Source 1\n- Source 2", block.Content)
	})
}

func TestToggleTextBlock_ValidatePlayerInput(t *testing.T) {
	block := blocks.ToggleTextBlock{
		Title:   "References",
		Content: "Some content",
	}
	state := &blocks.MockPlayerState{}
	newState, err := block.ValidatePlayerInput(state, map[string][]string{})
	require.NoError(t, err)
	assert.True(t, newState.IsComplete())
}
