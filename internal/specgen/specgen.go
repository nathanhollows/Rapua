// Package specgen generates the game specification for AI-assisted game authoring.
//
//go:generate go run ../../cmd/rapua genspec
package specgen

import (
	"encoding/json"
	"sort"

	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/game"
)

// interactiveBlockTypes is the set of block types that support the `sets` field.
// Content-only blocks (text, alert, image, etc.) do not support sets. Never mutated after init.
//
//nolint:gochecknoglobals // lookup table initialised once, never written after init
var interactiveBlockTypes = map[string]bool{
	"quiz":      true,
	"password":  true,
	"pincode":   true,
	"broker":    true,
	"sorting":   true,
	"photo":     true,
	"checklist": true,
	"rating":    true,
	"free_text": true,
}

// GenerateBlockSpecs returns the BlockSpec for every registered block type, sorted by type name.
// All block specs receive a `when` field. Interactive block specs also receive a `sets` field.
func GenerateBlockSpecs() []game.BlockSpec {
	registered := blocks.GetRegisteredBlocks()

	specs := make([]game.BlockSpec, 0, len(registered))
	for _, reg := range registered {
		if sp, ok := reg.Prototype.(game.SpecProvider); ok {
			spec := sp.GetSpec()
			spec.SharedFields = []string{"when"}
			if interactiveBlockTypes[spec.Type] {
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
