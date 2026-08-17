// Package specgen generates the game specification for AI-assisted game authoring.
//
//go:generate go run ../../cmd/rapua genspec
package specgen

import (
	"encoding/json"
	"sort"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
)

// GenerateBlockSpecs returns the BlockSpec for every registered block type, sorted by type name.
// All block specs receive a `when` field. Interactive block specs also receive a `sets` field.
func GenerateBlockSpecs() []game.BlockSpec {
	registered := blocks.GetRegisteredBlocks()

	specs := make([]game.BlockSpec, 0, len(registered))
	for _, reg := range registered {
		if sp, ok := reg.Prototype.(game.SpecProvider); ok {
			spec := sp.GetSpec()
			spec.SharedFields = []string{"when"}
			// Asked of the block rather than listed here, so a new interactive
			// block cannot ship with the spec hiding a field the runtime honours.
			if reg.Prototype.SupportsVariableSets() {
				spec.SharedFields = append(spec.SharedFields, "sets")
			}
			specs = append(specs, spec)
		}
	}

	sort.Slice(specs, func(i, j int) bool {
		return specs[i].Type < specs[j].Type
	})

	return specs
}

// GenerateJSON returns the complete v7 spec serialised as indented JSON.
func GenerateJSON() ([]byte, error) {
	return json.MarshalIndent(GenerateFullSpec(), "", "  ")
}
