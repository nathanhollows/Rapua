package blocks_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v6/blocks"
	templates "github.com/nathanhollows/Rapua/v6/internal/templates/blocks"
	"github.com/nathanhollows/Rapua/v6/models"
)

func TestBlocks_MethodsExist(t *testing.T) {
	instanceSettings := models.InstanceSettings{}
	// This tests that all blocks have matching views
	// This does *not* test for correctness of the views
	for _, block := range blocks.GetBlocksForContext(blocks.ContextLocationContent) {
		template := templates.RenderAdminEdit(instanceSettings, block)
		if template == nil {
			t.Errorf("Block %s is missing a RenderAdminEdit view", block.GetName())
		}

		template = templates.RenderAdminBlock(instanceSettings, block, true)
		if template == nil {
			t.Errorf("Block %s is missing a RenderAdminBlock view", block.GetName())
		}

		template = templates.RenderPlayerView(instanceSettings, block, nil)
		if template == nil {
			t.Errorf("Block %s is missing a RenderPlayerView view", block.GetName())
		}

		template = templates.RenderPlayerUpdate(instanceSettings, block, nil)
		if template == nil {
			t.Errorf("Block %s is missing a RenderPlayerUpdate view", block.GetName())
		}
	}
}
