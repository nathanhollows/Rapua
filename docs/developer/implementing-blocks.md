---
title: "Implementing Blocks"
sidebar: true
order: 7
---

# Implementing a New Block Type

Blocks are the core content elements in Rapua that players interact with. Each block type provides different functionality, from displaying text to interactive elements that require player input. This guide walks through the process of implementing a new block type in Rapua.

## Block Architecture Overview

Blocks in Rapua follow a consistent pattern:

1. **Block Interface Implementation** - Each block must implement the `Block` interface defined in `blocks/block.go`
2. **Block Registration** - Blocks must be registered in the system to be available for use
3. **Templates** - Each block needs admin and player view templates
4. **Block-specific Logic** - Implement validation, player input handling, and scoring

## Block Implementation Steps

### 1. Create the Block Struct

Start by creating a new Go file in the `/blocks` directory. Name it according to your block type, e.g., `sorting_block.go`.

Define your block struct, extending the BaseBlock:

```go
// YourBlock is a description of what your block does
type YourBlock struct {
    BaseBlock
    // Add block-specific fields here
    Content string `json:"content"`
    // ... other fields
}
```

### 2. Implement the Block Interface

Every block must implement the [Block interface](https://github.com/nathanhollows/Rapua/blob/main/blocks/block.go). Here are the key methods to implement:

#### Basic Attribute Getters

```go
// Basic Attributes Getters
func (b *YourBlock) GetName() string { return "Your Block Name" }

func (b *YourBlock) GetDescription() string {
    return "Description of what your block does."
}

func (b *YourBlock) GetIconSVG() string {
    return `<svg>...</svg>` // SVG icon markup for your block
}

func (b *YourBlock) GetType() string { return "your_block_type" }

func (b *YourBlock) GetID() string { return b.ID }

func (b *YourBlock) GetLocationID() string { return b.LocationID }

func (b *YourBlock) GetOrder() int { return b.Order }

func (b *YourBlock) GetPoints() int { return b.Points }

func (b *YourBlock) GetData() json.RawMessage {
    data, _ := json.Marshal(b)
    return data
}
```

#### Data Operations

```go
// Data operations
func (b *YourBlock) ParseData() error {
    return json.Unmarshal(b.Data, b)
}

func (b *YourBlock) UpdateBlockData(input map[string][]string) error {
    // Parse points
    pointsInput, ok := input["points"]
    if ok && len(pointsInput[0]) > 0 {
        points, err := strconv.Atoi(pointsInput[0])
        if err != nil {
            return errors.New("points must be an integer")
        }
        b.Points = points
    } else {
        b.Points = 0
    }

    // Update block-specific fields
    if content, exists := input["content"]; exists && len(content) > 0 {
        b.Content = content[0]
    }
    
    // Process other fields...
    
    return nil
}
```

#### Validation and Points Calculation

```go
// Validation and Points Calculation
func (b *YourBlock) RequiresValidation() bool { 
    // Return true if the block needs player input validation
    return true 
}

func (b *YourBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
    newState := state

    // Parse current player data if it exists
    var playerData YourBlockPlayerData
    if state.GetPlayerData() != nil {
        if err := json.Unmarshal(state.GetPlayerData(), &playerData); err != nil {
            return state, fmt.Errorf("failed to parse player data: %w", err)
        }
    }

    // Process player input
    // ...

    // Update player data
    newPlayerData, err := json.Marshal(playerData)
    if err != nil {
        return state, fmt.Errorf("failed to save player data: %w", err)
    }
    newState.SetPlayerData(newPlayerData)

    // Determine if the block is complete and calculate points
    // ...
    newState.SetComplete(true)
    newState.SetPointsAwarded(calculatedPoints)

    return newState, nil
}
```

### 3. Create Player Data Structure

If your block needs to track player progress or state, define a player data structure:

```go
// YourBlockPlayerData stores player progress
type YourBlockPlayerData struct {
    Attempts  int      `json:"attempts"`     // Number of attempts made so far
    IsCorrect bool     `json:"is_correct"`   // Whether the current answer is correct
    // ... other fields specific to your block
}
```

### 4. Register the Block

Add your block to the registered blocks list in `blocks/block.go`:

```go
var registeredBlocks = Blocks{
    &MarkdownBlock{},
    &DividerBlock{},
    // ... other blocks
    &YourBlock{},
}
```

Also add a constructor function:

```go
func NewYourBlock(base BaseBlock) *YourBlock {
    return &YourBlock{
        BaseBlock: base,
    }
}
```

And register it in the `CreateFromBaseBlock` function:

```go
func CreateFromBaseBlock(baseBlock BaseBlock) (Block, error) {
    switch baseBlock.Type {
    // ... other cases
    case "your_block_type":
        return NewYourBlock(baseBlock), nil
    default:
        return nil, fmt.Errorf("block type %s not found", baseBlock.Type)
    }
}
```

### 5. Create Block Templates

Create a new template file in `/internal/templates/blocks/your_block.templ` with admin and player views:

```html
package blocks

import (
    "fmt"
    "github.com/nathanhollows/Rapua/v4/blocks"
    "github.com/nathanhollows/Rapua/v4/models"
)

// Admin view
templ yourBlockAdmin(settings models.InstanceSettings, block blocks.YourBlock) {
    <form
        id={ fmt.Sprintf("form-%s", block.ID) }
        hx-post={ fmt.Sprint("/admin/locations/", block.LocationID, "/blocks/", block.ID, "/update") }
        hx-trigger={ fmt.Sprintf("keyup changed from:(#form-%s textarea) delay:500ms, change from:(#form-%s input) delay:500ms", block.ID, block.ID) }
        hx-swap="none"
    >
        <!-- Admin settings form -->
        <label class="form-control w-full">
            <div class="label">
                <span class="label-text font-bold">Content</span>
            </div>
            <textarea
                name="content"
                rows="3"
                class="textarea textarea-bordered w-full"
                placeholder="Block content here..."
            >{ block.Content }</textarea>
        </label>
        
        <!-- Points settings if enabled -->
        if settings.EnablePoints {
            <label class="form-control w-full mt-4">
                <div class="label">
                    <span class="label-text font-bold">Points</span>
                </div>
                <input 
                    name="points" 
                    type="number" 
                    class="input input-bordered w-full" 
                    placeholder="Points" 
                    value={ fmt.Sprint(block.Points) }
                />
            </label>
        }
        
        <!-- Other block-specific settings -->
    </form>
}

// Player view
templ yourBlockPlayer(settings models.InstanceSettings, block blocks.YourBlock, data blocks.PlayerState) {
    <div
        id={ fmt.Sprintf("player-block-%s", block.ID) }
        class="indicator w-full"
    >
        <!-- Points badge -->
        if settings.EnablePoints && block.Points > 0 {
            <span class="indicator-item indicator-top indicator-center badge badge-info">{ fmt.Sprint(block.GetPoints()) } pts</span>
        }
        
        <!-- Completion badge -->
        @completionBadge(data)
        
        <div class="card prose p-5 bg-base-200 shadow-lg w-full">
            <!-- Block content -->
            @templ.Raw(stringToMarkdown(block.Content))
            
            <!-- Interactive elements -->
            if !data.IsComplete() {
                <form
                    id={ fmt.Sprintf("form-%s", block.ID) }
                    hx-post={ fmt.Sprint("/blocks/validate") }
                    hx-target={ fmt.Sprintf("#player-block-%s", block.ID) }
                >
                    <input type="hidden" name="block" value={ block.ID }/>
                    
                    <!-- Player interaction elements -->
                    
                    <div class="flex justify-end mt-4">
                        <button class="btn btn-primary">Submit</button>
                    </div>
                </form>
            } else {
                <div class="alert alert-success">
                    <span>Success message here...</span>
                </div>
            }
        </div>
    </div>
}
```

### 6. Update the Block Rendering

Add your block to the rendering functions in `/internal/templates/blocks/blocks.templ`:

```go
func RenderAdminEdit(settings models.InstanceSettings, block blocks.Block) templ.Component {
    switch block.GetType() {
    // ... other cases
    case "your_block_type":
        b := block.(*blocks.YourBlock)
        return yourBlockAdmin(settings, *b)
    }
    return nil
}

func RenderPlayerView(settings models.InstanceSettings, block blocks.Block, state blocks.PlayerState) templ.Component {
    switch block.GetType() {
    // ... other cases
    case "your_block_type":
        b := block.(*blocks.YourBlock)
        return yourBlockPlayer(settings, *b, state)
    }
    return nil
}

func RenderPlayerUpdate(settings models.InstanceSettings, block blocks.Block, state blocks.PlayerState) templ.Component {
    switch block.GetType() {
    // ... other cases
    case "your_block_type":
        b := block.(*blocks.YourBlock)
        return yourBlockPlayer(settings, *b, state)
    }
    return nil
}
```

### 7. Write Tests

Create a test file (e.g., `/blocks/your_block_test.go`) to test your block implementation:

```go
package blocks

import (
    "encoding/json"
    "testing"
    
    "github.com/stretchr/testify/assert"
)

func TestYourBlock_UpdateBlockData(t *testing.T) {
    block := &YourBlock{
        BaseBlock: BaseBlock{
            ID:   "test-id",
            Type: "your_block_type",
        },
    }
    
    input := map[string][]string{
        "content": {"Test content"},
        "points":  {"100"},
        // Other test inputs
    }
    
    err := block.UpdateBlockData(input)
    assert.NoError(t, err)
    assert.Equal(t, "Test content", block.Content)
    assert.Equal(t, 100, block.Points)
    // Other assertions
}

func TestYourBlock_ValidatePlayerInput(t *testing.T) {
    block := &YourBlock{
        BaseBlock: BaseBlock{
            ID:     "block-id",
            Type:   "your_block_type",
            Points: 100,
        },
        // Initialize with test data
    }
    
    // Test correct input
    state := &mockPlayerState{blockID: "block-id", playerID: "player-id"}
    input := map[string][]string{
        "your_input_field": {"correct_value"},
    }
    
    newState, err := block.ValidatePlayerInput(state, input)
    assert.NoError(t, err)
    assert.True(t, newState.IsComplete())
    assert.Equal(t, 100, newState.GetPointsAwarded())
    
    // Test incorrect input
    // ...
}
```

### 8. Build and Test

Run the following commands to build and test your block:

```bash
# Generate templ files
make templ-generate

# Build the application
make build

# Run tests
make test
```

## Example: Sorting Block Implementation

Here's a simplified example of the sorting block implementation:

### 1. Block Struct (sorting_block.go)

```go
// SortingBlock allows players to sort items in the correct order
type SortingBlock struct {
    BaseBlock
    Content        string        `json:"content"`
    Items          []SortingItem `json:"items"`
    ScoringScheme  string        `json:"scoring_scheme"`
    ScoringPercent int           `json:"scoring_percent"`
}

// SortingItem represents an individual item to be sorted
type SortingItem struct {
    ID          string `json:"id"`
    Description string `json:"description"`
    Position    int    `json:"position"` // The correct position (1-based)
}

// SortingPlayerData stores player progress
type SortingPlayerData struct {
    PlayerOrder  []string `json:"player_order"`  // List of item IDs in player's submitted order
    ShuffleOrder []string `json:"shuffle_order"` // Shuffled order shown to player initially
    Attempts     int      `json:"attempts"`      // Number of attempts made so far
    IsCorrect    bool     `json:"is_correct"`    // Whether the current order is correct
}
```

### 2. Player Input Validation

```go
func (b *SortingBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
    newState := state
    
    // Parse player data from the existing state
    var playerData SortingPlayerData
    if state.GetPlayerData() != nil {
        if err := json.Unmarshal(state.GetPlayerData(), &playerData); err != nil {
            return state, fmt.Errorf("failed to parse player data: %w", err)
        }
    }
    
    // Get player's ordering from input
    itemOrder, exists := input["sorting-item-order"]
    if !exists || len(itemOrder) == 0 {
        return state, errors.New("sorting order is required")
    }
    
    // Store the player's order and increment attempts
    playerData.PlayerOrder = itemOrder
    playerData.Attempts++
    
    // Calculate points based on scoring scheme
    points := b.calculatePoints(playerData.PlayerOrder)
    
    // Check if order is correct
    isOrderCorrect := points == b.Points
    playerData.IsCorrect = isOrderCorrect
    
    // Marshal the updated player data
    newPlayerData, err := json.Marshal(playerData)
    if err != nil {
        return state, fmt.Errorf("failed to save player data: %w", err)
    }
    newState.SetPlayerData(newPlayerData)
    
    // Handle different scoring schemes for completion status
    switch b.ScoringScheme {
    case "retry_until_correct":
        // Only mark as complete when correct
        if isOrderCorrect {
            newState.SetComplete(true)
            newState.SetPointsAwarded(points)
        } else {
            newState.SetComplete(false)
            newState.SetPointsAwarded(0)
        }
    default:
        // For other schemes, mark as complete and award proportional points
        newState.SetComplete(true)
        newState.SetPointsAwarded(points)
    }
    
    return newState, nil
}
```

### 3. Templates (sorting.templ)

Admin view:
```html
templ sortingAdmin(settings models.InstanceSettings, block blocks.SortingBlock) {
    <form
        id={ fmt.Sprintf("form-%s", block.ID) }
        hx-post={ fmt.Sprint("/admin/locations/", block.LocationID, "/blocks/", block.ID, "/update") }
        hx-trigger={ fmt.Sprintf("keyup changed from:(#form-%s textarea) delay:500ms, click from:(#form-%s button) delay:100ms", block.ID, block.ID) }
        hx-swap="none"
    >
        <!-- Points setting -->
        if settings.EnablePoints {
            <label class="form-control w-full">
                <div class="label">
                    <span class="label-text font-bold">Points</span>
                </div>
                <input name="points" type="number" class="input input-bordered w-full" value={ fmt.Sprint(block.Points) }/>
            </label>
        }
        
        <!-- Scoring scheme selection -->
        <label class="form-control w-full mt-5">
            <div class="label">
                <span class="label-text font-bold">Scoring Scheme</span>
            </div>
            <select name="scoring_scheme" class="select select-bordered w-full">
                <option value="all_or_nothing" selected?={ block.ScoringScheme == "all_or_nothing" }>All or Nothing</option>
                <option value="correct_item_correct_place" selected?={ block.ScoringScheme == "correct_item_correct_place" }>Correct Item, Correct Place</option>
                <option value="runs_percentage" selected?={ block.ScoringScheme == "runs_percentage" }>Percentage of Correct Items</option>
                <option value="retry_until_correct" selected?={ block.ScoringScheme == "retry_until_correct" }>Retry Until Correct</option>
            </select>
        </label>
        
        <!-- Content textarea -->
        <label class="form-control w-full mt-5">
            <div class="label">
                <span class="label-text font-bold">Instructions</span>
            </div>
            <textarea
                name="content"
                rows="2"
                class="textarea textarea-bordered w-full"
                placeholder="Markdown content here..."
            >{ block.Content }</textarea>
        </label>
        
        <!-- Sorting items -->
        <div class="form-control w-full">
            <div class="label font-bold flex justify-between">
                Sorting Items
                <button class="btn btn-outline btn-sm" type="button" onclick="addSortingItem(event)">
                    Add Item
                </button>
            </div>
            <div id="sorting-items" class="joining join-vertical">
                <!-- Existing items -->
                for _, item := range block.Items {
                    @sortingItem(item)
                }
                
                <!-- Empty slots for new items -->
                for i := 0; i < (2 - len(block.Items)); i++ {
                    @sortingItem(blocks.SortingItem{})
                }
            </div>
        </div>
    </form>
}
```

## Best Practices

1. **Consistent Interface**: Follow the established patterns for blocks.
2. **Error Handling**: Provide clear error messages when input validation fails.
3. **Responsive UI**: Ensure your block looks good on both desktop and mobile.
4. **Test Coverage**: Write comprehensive tests for your block.
5. **Player Experience**: Consider the player experience and how feedback is provided.
6. **Documentation**: Document your block's functionality and configuration options.

## Common Patterns

- **Player State**: Use the PlayerState interface to track player progress.
- **HTMX Integration**: Use HTMX attributes for dynamic updates without page reloads.
- **Form Validation**: Validate input on both client and server sides.
- **Points Calculation**: Implement a clear scoring system based on player performance.

## Conclusion

Creating new block types is a powerful way to extend Rapua's functionality. By following this guide, you can create interactive, engaging blocks that enhance the user experience. Remember to test thoroughly and maintain a consistent user interface.

The block system in Rapua is designed to be extensible, allowing for a wide variety of interactive elements to be created. If you have any questions or need further guidance, please reach out on GitHub or contact the maintainers.
