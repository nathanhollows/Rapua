package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v4/blocks"
	"github.com/stretchr/testify/assert"
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
			assert.NoError(t, err)
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
	assert.Error(t, err)
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
