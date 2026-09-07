package admin

import (
	"context"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	templates "github.com/nathanhollows/Rapua/v8/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/require"
)

// randomRows builds a quest whose parent pointers are deliberately arbitrary:
// cycles, orphans, several roots, chains, and every mixture of them.
//
// This is the shape no hand-written fixture reaches. Four rounds of review
// found four different unbounded walks over exactly this state, each written
// as though the rows formed a tree because every test gave them one.
func randomRows(rng *rand.Rand, count int) []models.Objective {
	rows := make([]models.Objective, count)
	for i := range rows {
		rows[i] = models.Objective{
			ID:       fmt.Sprintf("row-%d", i),
			Slug:     fmt.Sprintf("row-%d", i),
			Title:    fmt.Sprintf("Row %d", i),
			Position: i,
			Draft:    rng.Intn(4) == 0,
		}
	}
	for i := range rows {
		switch rng.Intn(4) {
		case 0:
			// No parent: a root, or one of several.
		case 1:
			// A parent that is not in this quest.
			rows[i].ParentID = "absent-" + strconv.Itoa(rng.Intn(count))
		default:
			// Any row at all, itself included: this is what makes cycles.
			rows[i].ParentID = rows[rng.Intn(count)].ID
		}
	}
	return rows
}

// The builder and the renderer must terminate on any arrangement of rows, and
// draw each row at most once. Storage holds shapes a tree cannot, the editor
// exists to repair them, and it cannot repair what it cannot draw.
func TestBuildObjectiveTree_TerminatesOnAnyShape(t *testing.T) {
	for seed := range int64(200) {
		rng := rand.New(rand.NewSource(seed))
		rows := randomRows(rng, 1+rng.Intn(12))

		root, nodes := buildObjectiveTree(rows, nil, services.QuestLint{})

		var out strings.Builder
		require.NoError(t, templates.ObjectiveTree(
			services.QuestLint{}, root, nodes, false).Render(context.Background(), &out),
			"seed %d", seed)

		rendered := out.String()
		for _, row := range rows {
			drawn := strings.Count(rendered, fmt.Sprintf(`data-objective-id="%s"`, row.ID))
			require.LessOrEqual(t, drawn, 1,
				"seed %d: %s drawn %d times", seed, row.ID, drawn)
		}
	}
}

// Every row a walk can start from has to be reachable in the drawn tree, or
// the author is told about damage they cannot touch.
func TestBuildObjectiveTree_EveryRowIsReachable(t *testing.T) {
	for seed := range int64(200) {
		rng := rand.New(rand.NewSource(seed))
		rows := randomRows(rng, 1+rng.Intn(12))

		root, nodes := buildObjectiveTree(rows, nil, services.QuestLint{})

		drawn := make(map[string]bool, len(rows))
		var walk func(node *templates.ObjectiveTreeNode)
		walk = func(node *templates.ObjectiveTreeNode) {
			if drawn[node.Objective.ID] {
				require.Fail(t, "a row is drawn twice", "seed %d: %s", seed, node.Objective.ID)
			}
			drawn[node.Objective.ID] = true
			for _, child := range node.Children {
				walk(child)
			}
		}
		for _, node := range nodes {
			walk(node)
		}

		for _, row := range rows {
			if row.ID == root.ID {
				continue
			}
			require.True(t, drawn[row.ID],
				"seed %d: %s is in the quest but nowhere in the tree", seed, row.ID)
		}
	}
}
