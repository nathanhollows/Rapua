// Package specgen generates the block specification for AI-assisted game authoring.
//
//go:generate go run ../../cmd/genspec
package specgen

import (
	"encoding/json"
	"sort"

	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/game"
)

// GenerateBlockSpecs returns the BlockSpec for every registered block type, sorted by type name.
func GenerateBlockSpecs() []game.BlockSpec {
	registered := blocks.GetRegisteredBlocks()

	specs := make([]game.BlockSpec, 0, len(registered))
	for _, reg := range registered {
		if sp, ok := reg.Instance.(game.SpecProvider); ok {
			specs = append(specs, sp.GetSpec())
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
