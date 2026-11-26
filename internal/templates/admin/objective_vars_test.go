package templates

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

// An objective's own gate is named on the row; what its blocks set or wait on
// is an icon. Each tooltip says in a sentence what the mark means for a team,
// naming the block and the variable, and what the hover outlines.
func TestObjectiveTree_BadgesVariables(t *testing.T) {
	gated := &ObjectiveTreeNode{
		Objective: models.Objective{ID: "door", Slug: "door", Title: "Door", Depends: game.DependsField{"key", "lamp"}},
		BlockVars: services.VarUsage{
			Gated: []services.VarRef{{BlockName: "Text", Vars: []string{"met_guide"}}},
			Sets: []services.VarRef{
				{BlockName: "Quiz", Vars: []string{"door_open"}},
				{BlockName: "Choice", Vars: []string{"left", "right"}},
			},
		},
	}
	html := renderRow(t, gated)
	mono := func(name string) string { return `<span class="font-mono">` + name + `</span>` }
	assert.Contains(t, html, "key, lamp")
	assert.Contains(t, html, "Teams can&#39;t start this until "+mono("key")+" and "+mono("lamp")+" are both set.")
	assert.Contains(t, html, "The Text block stays hidden until "+mono("met_guide")+" is set.")
	assert.Contains(t, html, "Completing the Quiz block sets "+mono("door_open")+".")
	assert.Contains(t, html,
		"The Choice block sets "+mono("left")+" or "+mono("right")+", whichever option the team picks.")
	assert.Contains(t, html, "Rows with a block that sets them are outlined.")
	assert.Contains(t, html, "Rows with a block that sets it are outlined.")
	assert.Contains(t, html, "Rows waiting on them are outlined.")

	plain := renderRow(t, &ObjectiveTreeNode{Objective: models.Objective{ID: "hall", Slug: "hall", Title: "Hall"}})
	assert.NotContains(t, plain, "can&#39;t start this")
	assert.NotContains(t, plain, "stays hidden")
	assert.NotContains(t, plain, "Sets a variable")
}
