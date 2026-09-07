package templates

import (
	"context"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
)

// The badge shipped on leaf rows only, while sections are where routing, the
// band and finish_label live: most row-attributable diagnostics had nowhere to
// appear, and the count above the tree pointed at rows that could not show
// anything.
func TestObjectiveTree_BadgeRendersOnSectionsAndLeaves(t *testing.T) {
	// Distinct messages: a section renders its children, so shared text would
	// let the child's badge satisfy an assertion about the parent's.
	leaf := &ObjectiveTreeNode{
		Objective: models.Objective{ID: "leaf-id", Slug: "leaf", Title: "Leaf"},
		Lint: game.LintResult{
			Errors: []game.LintDiag{{Code: "BAND_ON_LEAF", Message: "leaf band is inert"}},
		},
	}
	section := &ObjectiveTreeNode{
		Objective: models.Objective{ID: "section-id", Slug: "section", Title: "Section"},
		Children:  []*ObjectiveTreeNode{leaf},
		Lint: game.LintResult{
			Errors: []game.LintDiag{{Code: "BAND_OUT_OF_RANGE", Message: "section band is unmeetable"}},
		},
	}

	cases := []struct {
		name string
		node *ObjectiveTreeNode
		want string
	}{
		{"section", section, "section band is unmeetable"},
		{"leaf", leaf, "leaf band is inert"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out strings.Builder
			if err := objectiveNode(tc.node, false, 0).Render(context.Background(), &out); err != nil {
				t.Fatalf("rendering: %v", err)
			}
			if !strings.Contains(out.String(), tc.want) {
				t.Errorf("%s row renders no lint badge of its own", tc.name)
			}
		})
	}
}
