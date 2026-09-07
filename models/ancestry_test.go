package models_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

// The whole point of the helper: a stored cycle must not spin. Every plain
// parent walk in this codebase met one and hung.
func TestWalkAncestors_TerminatesOnACycle(t *testing.T) {
	parentOf := map[string]string{"a": "b", "b": "a"}

	var visited []string
	models.WalkAncestors("a", parentOf, func(id string) bool {
		visited = append(visited, id)
		return true
	})

	assert.Equal(t, []string{"a", "b"}, visited, "each row once, then stop")
}

func TestWalkAncestors_TerminatesBelowACycle(t *testing.T) {
	parentOf := map[string]string{"below": "a", "a": "b", "b": "a"}

	count := 0
	models.WalkAncestors("below", parentOf, func(string) bool {
		count++
		return true
	})

	assert.Equal(t, 3, count, "the row, then the loop once")
}

func TestHasAncestor(t *testing.T) {
	parentOf := map[string]string{"child": "parent", "parent": "root", "root": ""}

	assert.True(t, models.HasAncestor("child", "root", parentOf))
	assert.True(t, models.HasAncestor("child", "child", parentOf), "a row is its own ancestor here")
	assert.False(t, models.HasAncestor("parent", "child", parentOf))
	assert.False(t, models.HasAncestor("child", "absent", parentOf))
}

// A cycle must not make an unrelated question answer true.
func TestHasAncestor_CycleDoesNotInventAnAncestor(t *testing.T) {
	parentOf := map[string]string{"a": "b", "b": "a"}
	assert.False(t, models.HasAncestor("a", "elsewhere", parentOf))
}
