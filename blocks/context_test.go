package blocks

import (
	"testing"
)

func TestBlockContextFiltering(t *testing.T) {
	tests := []struct {
		name             string
		context          BlockContext
		expectedBlocks   []string
		unexpectedBlocks []string
	}{
		{
			name:             "Content context should include most blocks",
			context:          ContextLocationContent,
			expectedBlocks:   []string{"markdown", "alert", "button", "image", "broker", "checklist"},
			unexpectedBlocks: []string{}, // All current blocks support content
		},
		// {
		// 	name:             "Navigation context should be limited",
		// 	context:          ContextNavigation,
		// 	expectedBlocks:   []string{"markdown", "image", "youtube", "clue"},
		// 	unexpectedBlocks: []string{"broker", "checklist", "pincode", "quiz_block", "button"},
		// },
		// {
		// 	name:             "Start page context should exclude interactive blocks",
		// 	context:          ContextStart,
		// 	expectedBlocks:   []string{"markdown", "alert", "button", "divider", "image", "youtube"},
		// 	unexpectedBlocks: []string{"broker", "checklist", "pincode", "quiz_block", "sorting"},
		// },
		// {
		// 	name:             "End page context should exclude interactive blocks",
		// 	context:          ContextEnd,
		// 	expectedBlocks:   []string{"markdown", "alert", "button", "divider", "image", "youtube"},
		// 	unexpectedBlocks: []string{"broker", "checklist", "pincode", "quiz_block", "sorting"},
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			availableBlocks := GetBlocksForContext(tt.context)

			// Check that expected blocks are present
			for _, expectedBlock := range tt.expectedBlocks {
				found := false
				for _, availableBlock := range availableBlocks {
					if availableBlock.GetType() == expectedBlock {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected block '%s' not found in context '%s'", expectedBlock, tt.context)
				}
			}

			// Check that unexpected blocks are not present
			for _, unexpectedBlock := range tt.unexpectedBlocks {
				for _, availableBlock := range availableBlocks {
					if availableBlock.GetType() == unexpectedBlock {
						t.Errorf("Unexpected block '%s' found in context '%s'", unexpectedBlock, tt.context)
					}
				}
			}
		})
	}
}

func TestCanBlockBeUsedInContext(t *testing.T) {
	tests := []struct {
		blockType string
		context   BlockContext
		expected  bool
	}{
		{"markdown", ContextLocationContent, true},
		// {"markdown", ContextNavigation, true},
		// {"markdown", ContextStart, true},
		{"broker", ContextLocationContent, true},
		// {"broker", ContextNavigation, false},
		// {"broker", ContextStart, false},
		{"clue", ContextLocationContent, true},
		// {"clue", ContextNavigation, true},
		// {"clue", ContextStart, false},
		{"nonexistent", ContextLocationContent, false},
	}

	for _, tt := range tests {
		t.Run(tt.blockType+"_in_"+string(tt.context), func(t *testing.T) {
			result := CanBlockBeUsedInContext(tt.blockType, tt.context)
			if result != tt.expected {
				t.Errorf("CanBlockBeUsedInContext(%s, %s) = %v, expected %v",
					tt.blockType, tt.context, result, tt.expected)
			}
		})
	}
}

func TestBackwardCompatibility(t *testing.T) {
	// Test that GetRegisteredBlocks still works
	blocks := GetBlocksForContext(ContextLocationContent)
	if len(blocks) == 0 {
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
		baseBlock := BaseBlock{
			Type: blockType,
			ID:   "test-id",
		}

		block, err := CreateFromBaseBlock(baseBlock)
		if err != nil {
			t.Errorf("Failed to create block of type %s: %v", blockType, err)
			continue
		}

		if block.GetType() != blockType {
			t.Errorf("Created block has wrong type: expected %s, got %s", blockType, block.GetType())
		}
	}
}
