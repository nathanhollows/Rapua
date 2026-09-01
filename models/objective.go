package models

import (
	"github.com/nathanhollows/Rapua/v8/game"
)

// Objective is a single thing to accomplish, rendered with two contexts (proof,
// then reveal) rather than the freeform block canvas a Location uses.
type Objective struct {
	baseModel

	ID      string `bun:"id,pk,notnull"`
	QuestID string `bun:"quest_id,notnull"`
	// ParentID is empty for the quest's root objective, which is the only row
	// without one. Every other row names the objective it sits beneath.
	ParentID string `bun:"parent_id,type:varchar(36),nullzero"`
	// Position orders an objective among its siblings. Meaningful only within
	// one parent.
	Position   int               `bun:"position,type:int"`
	Slug       string            `bun:"slug,type:varchar(255)"`
	Title      string            `bun:"title,type:varchar(255)"`
	Color      string            `bun:"color,type:varchar(255)"`
	Depends    game.DependsField `bun:"depends,type:text,nullzero" json:"depends,omitempty"`
	ProofSets  game.SetsField    `bun:"proof_sets,type:text,nullzero" json:"proof_sets,omitempty"`
	RevealSets game.SetsField    `bun:"reveal_sets,type:text,nullzero" json:"reveal_sets,omitempty"`

	// Routing, the band, MaxNext and FinishLabel all govern children, and mean
	// nothing on an objective without any.
	Routing game.RouteStrategy `bun:"routing,type:varchar(32)"`
	// ChildrenMin and ChildrenMax are the completion band. Nullable because an
	// explicit 0 is a different node from an omitted bound: see game.FillBand.
	ChildrenMin *int   `bun:"children_min,type:int,nullzero"`
	ChildrenMax *int   `bun:"children_max,type:int,nullzero"`
	MaxNext     int    `bun:"max_next,type:int"`
	FinishLabel string `bun:"finish_label,type:varchar(255)"`

	Quest  Quest   `bun:"rel:has-one,join:quest_id=id"`
	Blocks []Block `bun:"rel:has-many,join:id=owner_id"`
}

func (o *Objective) HasProofContext() bool {
	for i := range o.Blocks {
		if o.Blocks[i].Context == game.ContextObjectiveProof {
			return true
		}
	}
	return false
}

func (o *Objective) HasRevealContext() bool {
	for i := range o.Blocks {
		if o.Blocks[i].Context == game.ContextObjectiveReveal {
			return true
		}
	}
	return false
}

// TotalPoints is the objective's point value: the sum of its blocks' points,
// not a field of its own. Requires Blocks to be loaded (e.g. via
// GameStructureService.LoadBlocksForStructure or ObjectiveRepository.LoadBlocks).
func (o *Objective) TotalPoints() int {
	total := 0
	for i := range o.Blocks {
		total += o.Blocks[i].Points
	}
	return total
}
