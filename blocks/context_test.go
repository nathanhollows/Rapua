package blocks_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v6/blocks"
)

func TestBlockContextFiltering(t *testing.T) {
	tests := []struct {
		name             string
		context          blocks.BlockContext
		expectedBlocks   []string
		unexpectedBlocks []string
	}{
		{
			name:             "Content context should include most blocks",
			context:          blocks.ContextLocationContent,
			expectedBlocks:   []string{"markdown", "alert", "button", "image", "broker", "checklist"},
			unexpectedBlocks: []string{}, // All current blocks support content
		},
		// {
		// 	name:             "Navigation context should be limited",
		// 	context:          blocks.ContextNavigation,
		// 	expectedBlocks:   []string{"markdown", "image", "youtube", "clue"},
		// 	unexpectedBlocks: []string{"broker", "checklist", "pincode", "quiz_block", "button"},
		// },
		// {
		// 	name:             "Start page context should exclude interactive blocks",
		// 	context:          blocks.ContextStart,
		// 	expectedBlocks:   []string{"markdown", "alert", "button", "divider", "image", "youtube"},
		// 	unexpectedBlocks: []string{"broker", "checklist", "pincode", "quiz_block", "sorting"},
		// },
		// {
		// 	name:             "End page context should exclude interactive blocks",
		// 	context:          blocks.ContextEnd,
		// 	expectedBlocks:   []string{"markdown", "alert", "button", "divider", "image", "youtube"},
		// 	unexpectedBlocks: []string{"broker", "checklist", "pincode", "quiz_block", "sorting"},
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			availableBlocks := blocks.GetBlocksForContext(tt.context)
			checkExpectedBlocks(t, availableBlocks, tt.expectedBlocks, tt.context)
			checkUnexpectedBlocks(t, availableBlocks, tt.unexpectedBlocks, tt.context)
		})
	}
}

func checkExpectedBlocks(
	t *testing.T,
	availableBlocks blocks.Blocks,
	expectedBlocks []string,
	context blocks.BlockContext,
) {
	t.Helper()
	for _, expectedBlock := range expectedBlocks {
		if !blockTypeExists(availableBlocks, expectedBlock) {
			t.Errorf("Expected block '%s' not found in context '%s'", expectedBlock, context)
		}
	}
}

func checkUnexpectedBlocks(
	t *testing.T,
	availableBlocks blocks.Blocks,
	unexpectedBlocks []string,
	context blocks.BlockContext,
) {
	t.Helper()
	for _, unexpectedBlock := range unexpectedBlocks {
		if blockTypeExists(availableBlocks, unexpectedBlock) {
			t.Errorf("Unexpected block '%s' found in context '%s'", unexpectedBlock, context)
		}
	}
}

func blockTypeExists(blks blocks.Blocks, blockType string) bool {
	for _, block := range blks {
		if block.GetType() == blockType {
			return true
		}
	}
	return false
}

func TestCanBlockBeUsedInContext(t *testing.T) {
	tests := []struct {
		blockType string
		context   blocks.BlockContext
		expected  bool
	}{
		{"markdown", blocks.ContextLocationContent, true},
		// {"markdown", blocks.ContextNavigation, true},
		// {"markdown", blocks.ContextStart, true},
		{"broker", blocks.ContextLocationContent, true},
		// {"broker", blocks.ContextNavigation, false},
		// {"broker", blocks.ContextStart, false},
		{"clue", blocks.ContextLocationContent, true},
		// {"clue", blocks.ContextNavigation, true},
		// {"clue", blocks.ContextStart, false},
		{"nonexistent", blocks.ContextLocationContent, false},
	}

	for _, tt := range tests {
		t.Run(tt.blockType+"_in_"+string(tt.context), func(t *testing.T) {
			result := blocks.CanBlockBeUsedInContext(tt.blockType, tt.context)
			if result != tt.expected {
				t.Errorf("CanBlockBeUsedInContext(%s, %s) = %v, expected %v",
					tt.blockType, tt.context, result, tt.expected)
			}
		})
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that GetRegisteredBlocks still works
	blks := blocks.GetBlocksForContext(blocks.ContextLocationContent)
	if len(blks) == 0 {
		t.Error("GetRegisteredBlocks() returned empty slice")
	}

	// Test that all registered block types can be created
	expectedTypes := []string{
		"markdown",
		"alert",
		"button",
		"divider",
		"image",
		"youtube",
		"broker",
		"checklist",
		"clue",
		"answer",
		"pincode",
		"quiz_block",
		"sorting",
	}

	for _, blockType := range expectedTypes {
		baseBlock := blocks.BaseBlock{
			Type: blockType,
			ID:   "test-id",
		}

		block, err := blocks.CreateFromBaseBlock(baseBlock)
		if err != nil {
			t.Errorf("Failed to create block of type %s: %v", blockType, err)
			continue
		}

		if block.GetType() != blockType {
			t.Errorf("Created block has wrong type: expected %s, got %s", blockType, block.GetType())
		}
	}
}
