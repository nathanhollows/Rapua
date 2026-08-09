package models

import (
	"github.com/nathanhollows/Rapua/v8/game"
)

// Space owns the physical facts so objectives can reference it instead of
// restating coordinates.
type Space struct {
	baseModel

	ID       string              `bun:"id,pk,notnull"`
	QuestID  string              `bun:"quest_id,notnull"`
	Slug     string              `bun:"slug,type:varchar(255)"`
	Name     string              `bun:"name,type:varchar(255)"`
	Kind     game.SpaceKind      `bun:"kind,type:varchar(32),notnull"`
	Geometry *game.SpaceGeometry `bun:"geometry,type:text,nullzero"`
	Payload  string              `bun:"payload,type:varchar(255)"`
	Mobile   bool                `bun:"mobile,type:boolean"`

	Quest Quest `bun:"rel:has-one,join:quest_id=id"`
}

func (s *Space) HasCoordinates() bool {
	return s.Geometry.HasCoordinates()
}

// ProofMethods narrows the kind's baseline: a space that moves loses GPS,
// because the coordinates it was registered with say nothing about where it is now.
func (s *Space) ProofMethods() []game.ProofMethod {
	methods := game.ProofMethodsForKind(s.Kind)
	if !s.Mobile {
		return methods
	}
	kept := make([]game.ProofMethod, 0, len(methods))
	for _, m := range methods {
		if m != game.ProofMethodGPS {
			kept = append(kept, m)
		}
	}
	return kept
}

func (s *Space) AllowsMethod(method game.ProofMethod) bool {
	for _, m := range s.ProofMethods() {
		if m == method {
			return true
		}
	}
	return false
}
