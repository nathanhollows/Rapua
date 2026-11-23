package blocks

import (
	"fmt"

	"github.com/google/uuid"
)

// IDAssigner is implemented by blocks whose items are told apart by an id. The
// editor makes one for each item, but a game document may leave them out, and
// items sharing an empty id collapse into one when played.
type IDAssigner interface {
	AssignMissingIDs() error
}

func (b *SortingBlock) AssignMissingIDs() error {
	for i := range b.Items {
		if b.Items[i].ID != "" {
			continue
		}
		id, err := uuid.NewRandom()
		if err != nil {
			return fmt.Errorf("generating sorting item id: %w", err)
		}
		b.Items[i].ID = id.String()
	}
	return nil
}

func (b *ChecklistBlock) AssignMissingIDs() error {
	for i := range b.List {
		if b.List[i].ID != "" {
			continue
		}
		id, err := uuid.NewRandom()
		if err != nil {
			return fmt.Errorf("generating checklist item id: %w", err)
		}
		b.List[i].ID = id.String()
	}
	return nil
}

// AssignMissingIDs names an option by its place, as the editor does, because a
// player's answer is recorded against that name.
func (b *QuizBlock) AssignMissingIDs() error {
	for i := range b.Options {
		if b.Options[i].ID == "" {
			b.Options[i].ID = fmt.Sprintf("option_%d", i)
		}
	}
	return nil
}
