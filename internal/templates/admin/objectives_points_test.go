package templates

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/stretchr/testify/assert"
)

func TestEditObjectiveData_TotalPoints(t *testing.T) {
	t.Run("sums points across proof and reveal blocks", func(t *testing.T) {
		data := EditObjectiveData{
			ProofBlocks: blocks.Blocks{
				blocks.NewMarkdownBlock(blocks.BaseBlock{Points: 10}),
			},
			RevealBlocks: blocks.Blocks{
				blocks.NewMarkdownBlock(blocks.BaseBlock{Points: 20}),
			},
		}
		assert.Equal(t, 30, data.TotalPoints())
	})

	t.Run("no blocks is zero", func(t *testing.T) {
		assert.Equal(t, 0, EditObjectiveData{}.TotalPoints())
	})
}
