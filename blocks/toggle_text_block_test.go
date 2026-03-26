package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToggleTextBlock_ParseData(t *testing.T) {
	t.Run("parses title, content, and small", func(t *testing.T) {
		data := `{"title":"References","content":"Some content","small":true}`
		block := blocks.ToggleTextBlock{
			BaseBlock: blocks.BaseBlock{Data: json.RawMessage(data)},
		}
		err := block.ParseData()
		require.NoError(t, err)
		assert.Equal(t, "References", block.Title)
		assert.Equal(t, "Some content", block.Content)
		assert.True(t, block.Small)
	})

	t.Run("small defaults to false when absent", func(t *testing.T) {
		data := `{"title":"Hint","content":"Look closer"}`
		block := blocks.ToggleTextBlock{
			BaseBlock: blocks.BaseBlock{Data: json.RawMessage(data)},
		}
		err := block.ParseData()
		require.NoError(t, err)
		assert.False(t, block.Small)
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
		assert.False(t, block.Small)
	})

	t.Run("small is true when key present", func(t *testing.T) {
		block := blocks.ToggleTextBlock{}
		err := block.UpdateBlockData(map[string][]string{
			"title":   {"Hint"},
			"content": {"Check the sign"},
			"small":   {"true"},
		})
		require.NoError(t, err)
		assert.True(t, block.Small)
	})

	t.Run("small is false when key absent", func(t *testing.T) {
		block := blocks.ToggleTextBlock{Small: true}
		err := block.UpdateBlockData(map[string][]string{
			"title":   {"Hint"},
			"content": {"Check the sign"},
		})
		require.NoError(t, err)
		assert.False(t, block.Small)
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
