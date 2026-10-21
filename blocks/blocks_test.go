package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test that each registered block can be created from a base block.
func TestCreateFromBaseBlock(t *testing.T) {
	for _, block := range blocks.GetBlocksForContext(blocks.ContextObjectiveProof) {
		t.Run("creates "+block.GetName()+" from base block", func(t *testing.T) {
			baseBlock := blocks.BaseBlock{
				ID:      "1",
				OwnerID: "loc1",
				Type:    block.GetType(),
				Data:    json.RawMessage(`{}`),
				Order:   1,
				Points:  10,
			}

			newBlock, err := blocks.CreateFromBaseBlock(baseBlock)
			require.NoError(t, err)
			assert.IsType(t, block, newBlock)
			assert.Equal(t, block.GetType(), newBlock.GetType())
			assert.Equal(t, "1", newBlock.GetID())
			assert.Equal(t, 10, newBlock.GetPoints())
		})
	}
}

// Test that an error is returned when trying to create a block with an unknown type.
func TestCreateFromBaseBlockUnknownType(t *testing.T) {
	baseBlock := blocks.BaseBlock{
		ID:      "1",
		OwnerID: "loc1",
		Type:    "unknown",
		Data:    json.RawMessage(`{}`),
		Order:   1,
		Points:  10,
	}

	newBlock, err := blocks.CreateFromBaseBlock(baseBlock)
	require.Error(t, err)
	assert.Nil(t, newBlock)
	require.ErrorIs(t, err, blocks.ErrBlockTypeNotFound, "error should wrap ErrBlockTypeNotFound")
	assert.Contains(t, err.Error(), "unknown", "error message should contain the block type")
}

// Ensure that blocks have unique types, names, icons, and descriptions.
func TestBlockUniqueness(t *testing.T) {
	types := make(map[string]bool)
	names := make(map[string]bool)
	icons := make(map[string]bool)
	descriptions := make(map[string]bool)

	for _, block := range blocks.GetBlocksForContext(blocks.ContextObjectiveProof) {
		t.Run("block uniqueness", func(t *testing.T) {
			assert.False(t, types[block.GetType()], "duplicate type: "+block.GetType())
			types[block.GetType()] = true

			assert.False(t, names[block.GetName()], "duplicate name: "+block.GetName())
			names[block.GetName()] = true

			assert.False(t, icons[block.GetIconSVG()], "duplicate icon: "+block.GetIconSVG())
			icons[block.GetIconSVG()] = true

			assert.False(t, descriptions[block.GetDescription()], "duplicate description: "+block.GetDescription())
			descriptions[block.GetDescription()] = true
		})
	}
}

// Sets and depends ride in every block's data, so a save that goes through a
// block type's own marshalling must not drop them.
func TestBlockDataKeepsVars(t *testing.T) {
	for _, block := range blocks.GetBlocksForContext(blocks.ContextObjectiveProof) {
		t.Run(block.GetType(), func(t *testing.T) {
			newBlock, err := blocks.CreateFromBaseBlock(blocks.BaseBlock{
				ID:   "1",
				Type: block.GetType(),
				Data: json.RawMessage(`{"sets":"met_guide","depends":["a","b"]}`),
			})
			require.NoError(t, err)
			require.NoError(t, newBlock.ParseData())
			assert.Equal(t, "met_guide", newBlock.GetSets())
			assert.Equal(t, []string{"a", "b"}, []string(newBlock.GetDepends()))

			var data map[string]any
			require.NoError(t, json.Unmarshal(newBlock.GetData(), &data))
			assert.Equal(t, "met_guide", data["sets"])
			assert.Equal(t, []any{"a", "b"}, data["depends"])
		})
	}
}

// The editor's ordinary autosave goes through UpdateBlockData, which knows
// nothing of variables and must leave them be.
func TestBlockAutosaveKeepsVars(t *testing.T) {
	cases := []struct {
		blockType string
		form      map[string][]string
	}{
		{"text", map[string][]string{"content": {"Welcome"}}},
		{"free_text", map[string][]string{"prompt": {"Who?"}, "points": {"5"}}},
	}
	for _, c := range cases {
		t.Run(c.blockType, func(t *testing.T) {
			block, err := blocks.CreateFromBaseBlock(blocks.BaseBlock{
				ID: "1", Type: c.blockType, Data: json.RawMessage(`{"sets":"met_guide","depends":["a"]}`),
			})
			require.NoError(t, err)
			require.NoError(t, block.ParseData())
			require.NoError(t, block.UpdateBlockData(c.form))

			var data map[string]any
			require.NoError(t, json.Unmarshal(block.GetData(), &data))
			assert.Equal(t, "met_guide", data["sets"])
			assert.Equal(t, []any{"a"}, data["depends"])
		})
	}
}

// Naming a team completes through its own endpoint, never check-in, so a key
// on it would never turn.
func TestTeamNameCannotSetVars(t *testing.T) {
	block := &blocks.TeamNameChangerBlock{BaseBlock: blocks.BaseBlock{Sets: "named"}}
	assert.False(t, game.CanSetVars(block))
	assert.Nil(t, game.DeclaredVars(block))
	assert.True(t, game.CanSetVars(&blocks.FreeTextBlock{}))
	assert.False(t, game.CanSetVars(&blocks.ChoiceBlock{}))
	assert.False(t, game.CanSetVars(&blocks.MarkdownBlock{}))
}
