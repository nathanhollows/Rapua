package models_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

func TestObjective_TotalPoints(t *testing.T) {
	t.Run("sums points across all loaded blocks", func(t *testing.T) {
		obj := models.Objective{
			Blocks: []models.Block{
				{Context: game.ContextObjectiveProof, Points: 10},
				{Context: game.ContextObjectiveProof, Points: 5},
				{Context: game.ContextObjectiveReveal, Points: 20},
			},
		}
		assert.Equal(t, 35, obj.TotalPoints())
	})

	t.Run("negative points (e.g. a clue's cost) reduce the total", func(t *testing.T) {
		obj := models.Objective{
			Blocks: []models.Block{
				{Points: 10},
				{Points: -15},
			},
		}
		assert.Equal(t, -5, obj.TotalPoints())
	})

	t.Run("no blocks loaded is zero, not an error", func(t *testing.T) {
		obj := models.Objective{}
		assert.Equal(t, 0, obj.TotalPoints())
	})
}
