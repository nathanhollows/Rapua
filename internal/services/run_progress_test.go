package services_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

// progressTree is a root, a lone leaf and a section of three, ordered the way
// the repository returns a tree: parents before children.
func progressTree() []models.Objective {
	return []models.Objective{
		{ID: "root", Title: "The quest"},
		{ID: "intro", ParentID: "root", Title: "Arrival briefing"},
		{ID: "campus", ParentID: "root", Title: "Campus"},
		{ID: "c1", ParentID: "campus", Title: "The library"},
		{ID: "c2", ParentID: "campus", Title: "The cafe"},
		{ID: "c3", ParentID: "campus", Title: "The quad"},
	}
}

// Leaves read done from their proofs, started from the stream's touch, and
// sections from their children; the counts are of in-play children.
func TestBuildRunProgress_LeavesAndSections(t *testing.T) {
	progress := services.BuildRunProgress(
		progressTree(),
		map[string]bool{"intro": true, "c1": true},
		services.RunActivity{Touched: map[string]bool{"c2": true}},
		map[string]bool{"c3": true},
	)

	assert.Equal(t, 2, progress.Done)
	assert.Equal(t, 4, progress.Total, "the leaves: intro and the three on campus")
	assert.False(t, progress.Finished)

	byID := make(map[string]services.RunProgressNode)
	for _, node := range progress.Nodes {
		byID[node.Objective.ID] = node
	}
	assert.Equal(t, 0, byID["root"].Depth)
	assert.Equal(t, 1, byID["campus"].Depth)
	assert.Equal(t, 2, byID["c1"].Depth)
	assert.True(t, byID["intro"].Done)
	assert.False(t, byID["campus"].Done, "one of three is not the band")
	assert.Equal(t, 1, byID["campus"].Found)
	assert.Equal(t, 3, byID["campus"].Children)
	assert.False(t, byID["c2"].RuledOut, "an unfinished run owes what is open")
	assert.True(t, byID["c2"].Started, "the run opened it, so it is started")
	assert.False(t, byID["c3"].Started, "nothing has touched it")
	assert.True(t, byID["c3"].Locked, "the frontier says the run cannot reach it yet")
	assert.False(t, byID["c2"].Locked, "reachable and merely not done is its own state")
	assert.True(t, byID["campus"].Started, "a section is started when anything under it is")
}

// A ranged band that reached its maximum is met without a press; a band with a
// range needs the press even at its minimum, and the press is what closes it.
func TestBuildRunProgress_BandsAndThePress(t *testing.T) {
	tree := progressTree()
	// A pick-two band on campus: met at two.
	two := 2
	tree[2].ChildrenMin = &two
	tree[2].ChildrenMax = &two

	met := services.BuildRunProgress(tree, map[string]bool{"c1": true, "c2": true}, services.RunActivity{}, nil)
	for _, node := range met.Nodes {
		if node.Objective.ID == "campus" {
			assert.True(t, node.Done, "the band's maximum was reached")
			assert.Equal(t, 2, node.Found)
			assert.Equal(t, 2, node.Children, "the leftover is ruled out, not owed")
		}
		if node.Objective.ID == "c3" {
			assert.True(t, node.RuledOut, "a met band cuts its leftovers out of the game")
		}
	}

	// A ranged band, minimum met, not pressed: open, with its children still
	// owed. The press closes it and rules the leftovers out.
	ranged := progressTree()
	one := 1
	three := 3
	ranged[2].ChildrenMin = &one
	ranged[2].ChildrenMax = &three

	unpressed := services.BuildRunProgress(ranged, map[string]bool{"c1": true}, services.RunActivity{}, nil)
	for _, node := range unpressed.Nodes {
		if node.Objective.ID == "campus" {
			assert.False(t, node.Done, "reaching the minimum only offers the button")
			assert.Equal(t, 3, node.Children, "nothing is ruled out while the section is open")
		}
	}

	pressed := services.BuildRunProgress(
		ranged, map[string]bool{"c1": true}, services.RunActivity{Closed: map[string]bool{"campus": true}}, nil,
	)
	assert.Equal(t, 1, pressed.Done)
	assert.Equal(t, 2, pressed.Total, "the cut children leave the denominator with the section")
	for _, node := range pressed.Nodes {
		if node.Objective.ID == "campus" {
			assert.True(t, node.Done, "the press is what closes a ranged band")
			assert.Equal(t, 1, node.Found, "closed with children still open")
			assert.Equal(t, 1, node.Children, "and those children are no longer owed")
		}
	}
}

// A finished run owes nothing: every section reads settled, a leaf it never
// proved reads ruled out rather than open, and the count lands on 100 per
// cent rather than the share of leaves that happened to be proved.
func TestBuildRunProgress_FinishedRunRulesOutTheUnvisited(t *testing.T) {
	progress := services.BuildRunProgress(
		progressTree(),
		map[string]bool{"intro": true},
		services.RunActivity{Closed: map[string]bool{"campus": true}, Finished: true},
		nil,
	)

	assert.True(t, progress.Finished)
	assert.Equal(t, 1, progress.Done)
	assert.Equal(t, 1, progress.Total, "a completed run is not part-done")

	byID := make(map[string]services.RunProgressNode)
	for _, node := range progress.Nodes {
		byID[node.Objective.ID] = node
	}
	assert.True(t, byID["root"].Done)
	assert.True(t, byID["c2"].RuledOut, "the run finished without it")
	assert.False(t, byID["intro"].RuledOut, "it was proved")
}
