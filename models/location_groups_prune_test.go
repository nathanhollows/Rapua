package models_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
)

func TestRemoveLocationID_TopLevel(t *testing.T) {
	gs := models.GameStructure{LocationIDs: []string{"a", "b", "c"}}

	assert.True(t, gs.RemoveLocationID("b"))
	assert.Equal(t, []string{"a", "c"}, gs.LocationIDs, "order of survivors is preserved")
}

func TestRemoveLocationID_NestedSubGroup(t *testing.T) {
	gs := models.GameStructure{
		LocationIDs: []string{"a"},
		SubGroups: []models.GameStructure{
			{
				LocationIDs: []string{"b"},
				SubGroups: []models.GameStructure{
					{LocationIDs: []string{"c", "target"}},
				},
			},
		},
	}

	assert.True(t, gs.RemoveLocationID("target"))
	assert.Equal(t, []string{"c"}, gs.SubGroups[0].SubGroups[0].LocationIDs)
	assert.Equal(t, []string{"a"}, gs.LocationIDs, "other groups untouched")
	assert.Equal(t, []string{"b"}, gs.SubGroups[0].LocationIDs)
}

func TestRemoveLocationID_RemovesEveryOccurrence(t *testing.T) {
	gs := models.GameStructure{
		LocationIDs: []string{"dup", "a"},
		SubGroups:   []models.GameStructure{{LocationIDs: []string{"dup", "b"}}},
	}

	assert.True(t, gs.RemoveLocationID("dup"))
	assert.Equal(t, []string{"a"}, gs.LocationIDs)
	assert.Equal(t, []string{"b"}, gs.SubGroups[0].LocationIDs)
}

func TestRemoveLocationID_AbsentReportsFalse(t *testing.T) {
	gs := models.GameStructure{
		LocationIDs: []string{"a"},
		SubGroups:   []models.GameStructure{{LocationIDs: []string{"b"}}},
	}

	assert.False(t, gs.RemoveLocationID("missing"), "no write should be issued when nothing changed")
	assert.Equal(t, []string{"a"}, gs.LocationIDs)
	assert.Equal(t, []string{"b"}, gs.SubGroups[0].LocationIDs)
}

func TestRemoveLocationID_EmptyStructure(t *testing.T) {
	gs := models.GameStructure{}
	assert.False(t, gs.RemoveLocationID("anything"))
	assert.Empty(t, gs.LocationIDs)
}
