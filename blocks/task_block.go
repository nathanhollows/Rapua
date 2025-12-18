package blocks

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
)

const (
	defaultTaskIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-list-todo-icon lucide-list-todo"><path d="M13 5h8"/><path d="M13 12h8"/><path d="M13 19h8"/><path d="m3 17 2 2 4-4"/><rect x="3" y="4" width="6" height="6" rx="1"/></svg>`
	innerBlockIDSuffix = "_inner"
	maxTaskNameLength  = 200
)

// TaskBlock wraps another block (photo, quiz, password, etc.) as its inner validation mechanism.
type TaskBlock struct {
	BaseBlock
	TaskName  string          `json:"task_name"`
	InnerType string          `json:"inner_type"` // "photo", "quiz_block", etc.
	InnerData json.RawMessage `json:"inner_data"` // Embedded block config

	innerBlock Block `json:"-"` // Hydrated at runtime
}

func (t *TaskBlock) GetID() string {
	return t.ID
}

func (t *TaskBlock) GetType() string {
	return "task"
}

func (t *TaskBlock) GetLocationID() string {
	return t.LocationID
}

func (t *TaskBlock) GetName() string {
	return "Task"
}

func (t *TaskBlock) GetDescription() string {
	return "Task with validation requirement"
}

func (t *TaskBlock) GetOrder() int {
	return t.Order
}

func (t *TaskBlock) GetIconSVG() string {
	if t.innerBlock != nil {
		return t.innerBlock.GetIconSVG()
	}
	return defaultTaskIconSVG
}

func (t *TaskBlock) GetPoints() int {
	return t.Points // Points live on task, not inner
}

func (t *TaskBlock) GetData() json.RawMessage {
	data, err := json.Marshal(t)
	if err != nil {
		// Return empty JSON object to prevent downstream nil issues
		return json.RawMessage("{}")
	}
	return data
}

func (t *TaskBlock) ParseData() error {
	if err := json.Unmarshal(t.Data, t); err != nil {
		return err
	}

	// Create and parse inner block
	innerBase := BaseBlock{
		ID:         t.ID + innerBlockIDSuffix,
		LocationID: t.LocationID,
		Type:       t.InnerType,
		Data:       t.InnerData,
		Order:      0,
		Points:     t.Points, // Inner block handles points
	}

	inner, err := CreateFromBaseBlock(innerBase)
	if err != nil {
		return fmt.Errorf("creating inner block: %w", err)
	}

	// Validate that inner block type is suitable for task validation
	if !CanBlockBeUsedInContext(t.InnerType, ContextTaskValidation) {
		return fmt.Errorf("block type %s cannot be used for task validation", t.InnerType)
	}

	if err := inner.ParseData(); err != nil {
		return fmt.Errorf("parsing inner block: %w", err)
	}

	t.innerBlock = inner
	return nil
}

func (t *TaskBlock) RequiresValidation() bool {
	return true
}

func (t *TaskBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	if t.innerBlock == nil {
		return state, errors.New("this task is not properly configured - please contact the game administrator")
	}

	// Delegate to inner block - it handles points based on its configuration
	newState, err := t.innerBlock.ValidatePlayerInput(state, input)
	if err != nil {
		// Wrap inner block errors with task context for better user feedback
		return state, fmt.Errorf("validation failed for task '%s': %w", t.TaskName, err)
	}

	return newState, nil
}

func (t *TaskBlock) UpdateBlockData(input map[string][]string) error {
	// Update task-level fields
	if name := input["task_name"]; len(name) > 0 {
		taskName := name[0]
		if len(taskName) > maxTaskNameLength {
			return fmt.Errorf("task name exceeds maximum length of %d characters", maxTaskNameLength)
		}
		t.TaskName = taskName
	}

	if points := input["points"]; len(points) > 0 {
		pts, err := strconv.Atoi(points[0])
		if err != nil {
			return fmt.Errorf("invalid points value: %w", err)
		}
		t.Points = pts
	}

	// Handle inner type change
	if newType := input["inner_type"]; len(newType) > 0 && newType[0] != t.InnerType {
		t.InnerType = newType[0]

		// Validate that inner block type is suitable for task validation
		if !CanBlockBeUsedInContext(t.InnerType, ContextTaskValidation) {
			return fmt.Errorf("block type %s cannot be used for task validation", t.InnerType)
		}

		// Create new default inner block with task's points
		innerBase := BaseBlock{
			Type:   t.InnerType,
			Points: t.Points,
		}
		inner, err := CreateFromBaseBlock(innerBase)
		if err != nil {
			return fmt.Errorf("creating new inner block: %w", err)
		}

		// Set default values for specific block types
		if err := setDefaultTaskValidationValues(inner); err != nil {
			return fmt.Errorf("setting default values: %w", err)
		}

		t.innerBlock = inner

		// Serialize the new default block immediately
		innerData, err := json.Marshal(inner)
		if err != nil {
			return fmt.Errorf("serializing new inner block: %w", err)
		}
		t.InnerData = innerData
	} else {
		// Ensure inner block is hydrated if it hasn't been already
		if t.innerBlock == nil && t.InnerType != "" {
			if err := t.hydrateInnerBlock(); err != nil {
				return fmt.Errorf("hydrating inner block: %w", err)
			}
		}

		// Type is stable - filter input for inner block fields only
		innerInput := make(map[string][]string)
		for key, value := range input {
			// Skip task-level fields (task_* prefix and special fields)
			if key == "task_name" || key == "points" || key == "inner_type" {
				continue
			}
			// Pass everything else to inner block
			innerInput[key] = value
		}

		// Only delegate if we have inner fields AND an inner block
		if t.innerBlock != nil && len(innerInput) > 0 {
			if err := t.innerBlock.UpdateBlockData(innerInput); err != nil {
				return fmt.Errorf("updating inner block: %w", err)
			}
			// Serialize updated inner block
			innerData, err := json.Marshal(t.innerBlock)
			if err != nil {
				return fmt.Errorf("serializing inner block: %w", err)
			}
			t.InnerData = innerData
		}
	}

	// Serialize updated task data
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	t.Data = data

	return nil
}

// hydrateInnerBlock creates and parses the inner block from InnerData if not already hydrated
func (t *TaskBlock) hydrateInnerBlock() error {
	if t.innerBlock != nil {
		return nil // Already hydrated
	}

	if t.InnerType == "" {
		return errors.New("cannot hydrate inner block: no inner type specified")
	}

	innerBase := BaseBlock{
		ID:         t.ID + innerBlockIDSuffix,
		LocationID: t.LocationID,
		Type:       t.InnerType,
		Data:       t.InnerData,
		Order:      0,
		Points:     t.Points,
	}

	inner, err := CreateFromBaseBlock(innerBase)
	if err != nil {
		return fmt.Errorf("creating inner block: %w", err)
	}

	if !CanBlockBeUsedInContext(t.InnerType, ContextTaskValidation) {
		return fmt.Errorf("block type %s cannot be used for task validation", t.InnerType)
	}

	if len(t.InnerData) > 0 {
		if err := inner.ParseData(); err != nil {
			return fmt.Errorf("parsing inner block: %w", err)
		}
	}

	t.innerBlock = inner
	return nil
}

// GetInnerBlock returns the hydrated inner block for template access.
func (t *TaskBlock) GetInnerBlock() Block {
	return t.innerBlock
}

// SetInnerBlock sets the inner block directly (used for temporary blocks in handlers).
func (t *TaskBlock) SetInnerBlock(inner Block) {
	t.innerBlock = inner
}

// GetTaskValidationTypes returns the list of block types that can be used for task validation.
func GetTaskValidationTypes() []string {
	blocks := GetBlocksForContext(ContextTaskValidation)
	types := make([]string, 0, len(blocks))
	for _, block := range blocks {
		types = append(types, block.GetType())
	}
	return types
}

// setDefaultTaskValidationValues sets sensible defaults for blocks used in task validation
func setDefaultTaskValidationValues(block Block) error {
	switch b := block.(type) {
	case *QRCodeBlock:
		b.Instructions = "Find and scan the QR code to complete this task"
	}
	return nil
}
