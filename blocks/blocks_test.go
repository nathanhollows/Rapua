package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test that each registered block can be created from a base block.
func TestCreateFromBaseBlock(t *testing.T) {
	for _, block := range blocks.GetBlocksForContext(blocks.ContextLocationContent) {
		t.Run("creates "+block.GetName()+" from base block", func(t *testing.T) {
			baseBlock := blocks.BaseBlock{
				ID:         "1",
				LocationID: "loc1",
				Type:       block.GetType(),
				Data:       json.RawMessage(`{}`),
				Order:      1,
				Points:     10,
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
		ID:         "1",
		LocationID: "loc1",
		Type:       "unknown",
		Data:       json.RawMessage(`{}`),
		Order:      1,
		Points:     10,
	}

	newBlock, err := blocks.CreateFromBaseBlock(baseBlock)
	require.Error(t, err)
	assert.Nil(t, newBlock)
}

// Ensure that blocks have unique types, names, icons, and descriptions.
func TestBlockUniqueness(t *testing.T) {
	types := make(map[string]bool)
	names := make(map[string]bool)
	icons := make(map[string]bool)
	descriptions := make(map[string]bool)

	for _, block := range blocks.GetBlocksForContext(blocks.ContextLocationContent) {
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

// Ensure that all blocks registered for ContextCheckpoint are interactive (require validation).
func TestCheckpointBlocksAreInteractive(t *testing.T) {
	checkpointBlocks := blocks.GetBlocksForContext(blocks.ContextCheckpoint)

	for _, block := range checkpointBlocks {
		t.Run(block.GetName()+" requires validation", func(t *testing.T) {
			assert.True(t, block.RequiresValidation(),
				"Checkpoint block %s (%s) must require validation to ensure player interaction",
				block.GetName(), block.GetType())
		})
	}

	// If no blocks are registered for checkpoint context, this test still passes
	// but will provide coverage when blocks are added in the future.
	if len(checkpointBlocks) == 0 {
		t.Log("No blocks registered for ContextCheckpoint - test will validate them when added")
	}
}
