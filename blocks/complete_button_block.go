package blocks

import (
	"encoding/json"
)

// CompleteButtonBlock represents a simple button that marks a task as complete when clicked
type CompleteButtonBlock struct {
	BaseBlock
	Text string `json:"text"` // Button text
}

// GetID returns the block's unique identifier
func (c *CompleteButtonBlock) GetID() string {
	return c.ID
}

// GetType returns the block type
func (c *CompleteButtonBlock) GetType() string {
	return "complete_button"
}

// GetLocationID returns the location ID this block belongs to
func (c *CompleteButtonBlock) GetLocationID() string {
	return c.LocationID
}

// GetName returns the human-readable name of the block type
func (c *CompleteButtonBlock) GetName() string {
	return "Complete Button"
}

// GetDescription returns a description of what this block does
func (c *CompleteButtonBlock) GetDescription() string {
	return "A simple button that marks a task as complete when clicked"
}

// GetOrder returns the display order of the block
func (c *CompleteButtonBlock) GetOrder() int {
	return c.Order
}

// GetPoints returns the points awarded for completing this block
func (c *CompleteButtonBlock) GetPoints() int {
	return c.Points
}

// GetData returns the serialized block data
func (c *CompleteButtonBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(c)
	return data
}

// NewCompleteButtonBlock creates a new complete button block with default values
func NewCompleteButtonBlock() *CompleteButtonBlock {
	return &CompleteButtonBlock{
		BaseBlock: BaseBlock{
			Type: "complete_button",
		},
		Text: "Mark as complete",
	}
}

// NewCompleteButtonBlockFromBase creates a new complete button block from a base block
func NewCompleteButtonBlockFromBase(base BaseBlock) *CompleteButtonBlock {
	return &CompleteButtonBlock{
		BaseBlock: base,
		Text:      "Mark as complete",
	}
}

// ParseData parses the block's data field into the CompleteButtonBlock struct
func (c *CompleteButtonBlock) ParseData() error {
	if len(c.Data) == 0 {
		return nil
	}
	return json.Unmarshal(c.Data, c)
}

// UpdateBlockData updates the complete button block data from form values
func (c *CompleteButtonBlock) UpdateBlockData(data map[string][]string) error {
	// Update text if provided
	if text, ok := data["text"]; ok && len(text) > 0 {
		c.Text = text[0]
	}

	// Serialize the updated data back to the Data field
	updatedData, err := json.Marshal(c)
	if err != nil {
		return err
	}
	c.Data = updatedData

	return nil
}

// RequiresValidation returns true as this block validates task completion
func (c *CompleteButtonBlock) RequiresValidation() bool {
	return true
}

// ValidatePlayerInput validates the player's input and marks the task as complete
// This block always succeeds when submitted, marking the task complete
func (c *CompleteButtonBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	// Mark as complete - this is the sole purpose of this block
	state.SetComplete(true)
	return state, nil
}

// GetIconSVG returns the SVG icon for the complete button block (checkmark icon)
func (c *CompleteButtonBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" stroke-width="1.5" stroke="currentColor" class="size-6">
  <path stroke-linecap="round" stroke-linejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
</svg>`
}
