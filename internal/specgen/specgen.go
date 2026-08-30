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
// All block specs receive a `when` field. Interactive block specs (RequiresValidation) that
// actually honour it (SupportsPoints) also receive `points`; those supporting creator vars
// also receive `sets`.
func GenerateBlockSpecs() []game.BlockSpec {
	registered := blocks.GetRegisteredBlocks()

	specs := make([]game.BlockSpec, 0, len(registered))
	for _, reg := range registered {
		if sp, ok := reg.Prototype.(game.SpecProvider); ok {
			spec := sp.GetSpec()
			// Contexts comes from the registry, not the block's own GetSpec
			// literal: SupportedContexts is what BlockCreate actually enforces,
			// so it can't drift out of sync with reality the way a
			// hand-maintained duplicate list already had.
			contexts := make([]string, len(reg.SupportedContexts))
			for i, c := range reg.SupportedContexts {
				contexts[i] = string(c)
			}
			spec.Contexts = contexts
			spec.SharedFields = []string{"when"}
			// Asked of the block rather than listed here, so a new interactive
			// block cannot ship with the spec hiding a field the runtime honours.
			if reg.Prototype.RequiresValidation() && reg.Prototype.SupportsPoints() {
				spec.SharedFields = append(spec.SharedFields, "points")
			}
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

// GenerateJSON returns the complete v8 spec serialised as indented JSON.
func GenerateJSON() ([]byte, error) {
	return json.MarshalIndent(GenerateFullSpec(), "", "  ")
}
