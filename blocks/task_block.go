package blocks

import (
	"encoding/json"
	"errors"
)

type TaskBlock struct {
	BaseBlock
	Task        string `json:"task"`
	Icon        string `json:"icon"`
	LinkThrough bool   `json:"link_through"`
}

// Basic Attributes Getters

func (b *TaskBlock) GetID() string         { return b.ID }
func (b *TaskBlock) GetType() string       { return "task" }
func (b *TaskBlock) GetLocationID() string { return b.LocationID }
func (b *TaskBlock) GetName() string       { return "Task" }
func (b *TaskBlock) GetDescription() string {
	return "Give players a task to complete."
}
func (b *TaskBlock) GetOrder() int  { return b.Order }
func (b *TaskBlock) GetPoints() int { return b.Points }
func (b *TaskBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-square-check-big-icon lucide-square-check-big"><path d="M21 10.656V19a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h12.344"/><path d="m9 11 3 3L22 4"/></svg>`
}
func (b *TaskBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(b)
	return data
}

// Data Operations

func (b *TaskBlock) ParseData() error {
	return json.Unmarshal(b.Data, b)
}

func (b *TaskBlock) UpdateBlockData(input map[string][]string) error {
	task, exists := input["task"]
	if !exists || len(task) == 0 {
		return errors.New("task is a required field")
	}
	b.Task = task[0]

	icon, exists := input["icon"]
	if exists && len(icon) > 0 {
		b.Icon = icon[0]
	} else {
		b.Icon = ""
	}

	linkThrough, exists := input["link_through"]
	if exists && len(linkThrough) > 0 && (linkThrough[0] == "on" || linkThrough[0] == "1") {
		b.LinkThrough = true
	} else {
		b.LinkThrough = false
	}

	return nil
}

// RequiresValidation returns whether this block requires player input validation.
func (b *TaskBlock) RequiresValidation() bool {
	return false
}

func (b *TaskBlock) ValidatePlayerInput(state PlayerState, _ map[string][]string) (PlayerState, error) {
	// No validation required for TaskBlock; mark as complete
	state.SetComplete(true)
	return state, nil
}
