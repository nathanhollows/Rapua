package models

import (
	"github.com/nathanhollows/Rapua/v8/game"
)

// Objective is a single thing to accomplish, rendered with two contexts (proof,
// then reveal) rather than the freeform block canvas a Location uses.
type Objective struct {
	baseModel

	ID         string           `bun:"id,pk,notnull"`
	QuestID    string           `bun:"quest_id,notnull"`
	Slug       string           `bun:"slug,type:varchar(255)"`
	Title      string           `bun:"title,type:varchar(255)"`
	When       *game.WhenClause `bun:"when_clause,type:text,nullzero" json:"when,omitempty"`
	Order      int              `bun:"order,type:int"`
	ProofSets  game.SetsField   `bun:"proof_sets,type:text,nullzero" json:"proof_sets,omitempty"`
	RevealSets game.SetsField   `bun:"reveal_sets,type:text,nullzero" json:"reveal_sets,omitempty"`

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
