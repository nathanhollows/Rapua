package blocks_test

import (
	"encoding/json"
	"testing"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQRCodeBlock_Getters(t *testing.T) {
	block := blocks.QRCodeBlock{
		BaseBlock: blocks.BaseBlock{
			ID:         "test-qr-id",
			LocationID: "location-123",
			Order:      2,
			Points:     0,
		},
		Instructions: "Scan the QR code on the statue",
	}

	assert.Equal(t, "qr_code", block.GetType())
	assert.Equal(t, "test-qr-id", block.GetID())
	assert.Equal(t, "location-123", block.GetLocationID())
	assert.Equal(t, "QR Code", block.GetName())
	assert.Equal(t, 2, block.GetOrder())
	assert.Equal(t, 0, block.GetPoints())
	assert.Contains(t, block.GetIconSVG(), "qr-code")
}

func TestQRCodeBlock_ParseData(t *testing.T) {
	data := map[string]any{
		"instructions": "Find and scan the QR code",
	}
	jsonData, err := json.Marshal(data)
	require.NoError(t, err)

	block := blocks.QRCodeBlock{
		BaseBlock: blocks.BaseBlock{
			ID:   "qr-1",
			Type: "qr_code",
			Data: json.RawMessage(jsonData),
		},
	}

	err = block.ParseData()
	require.NoError(t, err)
	assert.Equal(t, "Find and scan the QR code", block.Instructions)
}

func TestQRCodeBlock_UpdateBlockData(t *testing.T) {
	block := blocks.QRCodeBlock{
		BaseBlock: blocks.BaseBlock{},
	}

	input := map[string][]string{
		"instructions": {"Scan the QR code on the door"},
	}

	err := block.UpdateBlockData(input)
	require.NoError(t, err)
	assert.Equal(t, "Scan the QR code on the door", block.Instructions)

	// Verify serialization
	var parsed blocks.QRCodeBlock
	err = json.Unmarshal(block.GetData(), &parsed)
	require.NoError(t, err)
	assert.Equal(t, "Scan the QR code on the door", parsed.Instructions)
}

func TestQRCodeBlock_RequiresValidation(t *testing.T) {
	block := blocks.QRCodeBlock{}
	assert.False(t, block.RequiresValidation())
}

func TestQRCodeBlock_ValidatePlayerInput(t *testing.T) {
	block := blocks.QRCodeBlock{}
	state := &blocks.MockPlayerState{
		BlockID:  "qr-1",
		PlayerID: "player-1",
	}

	// QR code blocks don't validate - they just display instructions
	newState, err := block.ValidatePlayerInput(state, map[string][]string{})
	require.NoError(t, err)
	assert.Equal(t, state, newState)
}

func TestQRCodeBlock_OnlyInTaskValidationContext(t *testing.T) {
	// QR code should only be available in task validation context
	assert.True(t, blocks.CanBlockBeUsedInContext("qr_code", blocks.ContextTaskValidation))

	// Should not be available in other contexts
	assert.False(t, blocks.CanBlockBeUsedInContext("qr_code", blocks.ContextLocationContent))
	assert.False(t, blocks.CanBlockBeUsedInContext("qr_code", blocks.ContextLocationClues))
	assert.False(t, blocks.CanBlockBeUsedInContext("qr_code", blocks.ContextCheckpoint))
	assert.False(t, blocks.CanBlockBeUsedInContext("qr_code", blocks.ContextStart))
	assert.False(t, blocks.CanBlockBeUsedInContext("qr_code", blocks.ContextFinish))
}

func TestNewQRCodeBlock(t *testing.T) {
	base := blocks.BaseBlock{
		ID:         "test-id",
		LocationID: "location-123",
		Type:       "qr_code",
		Order:      1,
		Points:     0,
	}

	block := blocks.NewQRCodeBlock(base)

	assert.Equal(t, base, block.BaseBlock)
	assert.Empty(t, block.Instructions)
}
