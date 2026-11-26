package navigation_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
	"github.com/stretchr/testify/assert"
)

// chain builds root > s1 > s2 > ... > sN > leaf and indexes it by ID, flagging
// the sections named in passThrough.
func chain(depth int, passThrough ...string) (map[string]models.Objective, models.Objective) {
	flagged := map[string]bool{}
	for _, id := range passThrough {
		flagged[id] = true
	}
	byID := map[string]models.Objective{"root": {ID: "root"}}
	parent := "root"
	for i := 1; i <= depth; i++ {
		id := "s" + string(rune('0'+i))
		byID[id] = models.Objective{ID: id, ParentID: parent, PassThrough: flagged[id]}
		parent = id
	}
	leaf := models.Objective{ID: "leaf", ParentID: parent}
	byID["leaf"] = leaf
	return byID, leaf
}

func ids(sections []models.Objective) []string {
	out := make([]string, 0, len(sections))
	for _, s := range sections {
		out = append(out, s.ID)
	}
	return out
}

func TestPlaceFor_CutsAtTheDrawnDepth(t *testing.T) {
	byID, leaf := chain(4)

	place, path := navigation.PlaceFor(byID, leaf)

	assert.Equal(t, "s2", place.ID, "the third and fourth sections pass through")
	assert.Equal(t, []string{"s1"}, ids(path))
}

func TestPlaceFor_AFlaggedSectionAllowsOneMoreLevel(t *testing.T) {
	byID, leaf := chain(4, "s2")

	place, path := navigation.PlaceFor(byID, leaf)

	assert.Equal(t, "s3", place.ID)
	assert.Equal(t, []string{"s1"}, ids(path), "s2 is not drawn, so s3 takes its place in the path")
}

func TestPlaceFor_FlaggedSectionsDoNotCountAnywhere(t *testing.T) {
	byID, leaf := chain(5, "s1", "s3")

	place, path := navigation.PlaceFor(byID, leaf)

	assert.Equal(t, "s4", place.ID)
	assert.Equal(t, []string{"s2"}, ids(path))
}

func TestPlaceFor_EverySectionFlaggedLeavesTheRowStandingAlone(t *testing.T) {
	byID, leaf := chain(2, "s1", "s2")

	place, path := navigation.PlaceFor(byID, leaf)

	assert.Empty(t, place.ID)
	assert.Empty(t, path)
}

func TestPlaceFor_RowUnderTheRootStandsAlone(t *testing.T) {
	byID := map[string]models.Objective{"root": {ID: "root"}}
	row := models.Objective{ID: "row", ParentID: "root"}

	place, _ := navigation.PlaceFor(byID, row)

	assert.Empty(t, place.ID)
}

func TestPlaceFor_SurvivesACycle(t *testing.T) {
	byID := map[string]models.Objective{
		"a": {ID: "a", ParentID: "b"},
		"b": {ID: "b", ParentID: "a"},
	}

	assert.NotPanics(t, func() { navigation.PlaceFor(byID, byID["a"]) })
}

func TestFinishPlaceFor(t *testing.T) {
	t.Run("a drawn section is its own place", func(t *testing.T) {
		byID, _ := chain(2)

		place, path := navigation.FinishPlaceFor(byID, byID["s2"])

		assert.Equal(t, "s2", place.ID)
		assert.Equal(t, []string{"s1"}, ids(path))
	})

	t.Run("a flagged section lands on the nearest drawn ancestor", func(t *testing.T) {
		byID, _ := chain(2, "s2")

		place, _ := navigation.FinishPlaceFor(byID, byID["s2"])

		assert.Equal(t, "s1", place.ID)
	})

	t.Run("a section past the cut lands on the last drawn one", func(t *testing.T) {
		byID, _ := chain(3)

		place, _ := navigation.FinishPlaceFor(byID, byID["s3"])

		assert.Equal(t, "s2", place.ID)
	})

	t.Run("the root has no place", func(t *testing.T) {
		byID, _ := chain(1)

		place, _ := navigation.FinishPlaceFor(byID, byID["root"])

		assert.Empty(t, place.ID)
	})
}

func TestKindOf(t *testing.T) {
	byID, _ := chain(4, "s2")

	assert.Equal(t, navigation.SectionDrawn, navigation.KindOf(byID, byID["s1"]))
	assert.Equal(t, navigation.SectionPassThrough, navigation.KindOf(byID, byID["s2"]))
	assert.Equal(t, navigation.SectionDrawn, navigation.KindOf(byID, byID["s3"]), "flagging s2 made room for s3")
	assert.Equal(t, navigation.SectionForced, navigation.KindOf(byID, byID["s4"]))
	assert.True(t, navigation.KindOf(byID, byID["s4"]).Hidden())
	assert.False(t, navigation.KindOf(byID, byID["s1"]).Hidden())
}
