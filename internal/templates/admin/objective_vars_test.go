package templates

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

// An objective's own gate is named on the row; what its blocks set or wait on
// is an icon whose tooltip names it.
func TestObjectiveTree_BadgesVariables(t *testing.T) {
	gated := &ObjectiveTreeNode{
		Objective: models.Objective{ID: "door", Slug: "door", Title: "Door", Depends: game.DependsField{"key", "lamp"}},
		BlockVars: services.VarUsage{
			Gated: []services.VarRef{{BlockName: "Text", Vars: []string{"met_guide"}}},
			Sets:  []services.VarRef{{BlockName: "Quiz", Vars: []string{"door_open"}}},
		},
	}
	html := renderRow(t, gated)
	assert.Contains(t, html, "key, lamp")
	assert.Contains(t, html, "Locked until these are set")
	assert.Contains(t, html, "Hidden until set")
	assert.Contains(t, html, "met_guide")
	assert.Contains(t, html, "door_open")
	assert.Contains(t, html, `font-semibold mb-1">Sets`)

	plain := renderRow(t, &ObjectiveTreeNode{Objective: models.Objective{ID: "hall", Slug: "hall", Title: "Hall"}})
	assert.NotContains(t, plain, "Locked until")
	assert.NotContains(t, plain, "Hidden until set")
	assert.NotContains(t, plain, `font-semibold mb-1">Sets`)
}
