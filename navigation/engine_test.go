package navigation_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
	"github.com/stretchr/testify/assert"
)

// The structure used throughout this file is makeObjectiveTestStructure, defined
// in engine_objectives_test.go: the Objective-ID mirror of the old
// Location-built shape, which is all the generic functions below need.

// === FindGroupByID Tests ===

func TestFindGroupByID_Root(t *testing.T) {
	structure := makeObjectiveTestStructure()
	found := navigation.FindGroupByID(structure, "root")
	assert.NotNil(t, found)
	assert.Equal(t, "root", found.ID)
}

func TestFindGroupByID_DirectChild(t *testing.T) {
	structure := makeObjectiveTestStructure()
	found := navigation.FindGroupByID(structure, "group1")
	assert.NotNil(t, found)
	assert.Equal(t, "group1", found.ID)
	assert.Equal(t, "First Group", found.Name)
}

func TestFindGroupByID_NotFound(t *testing.T) {
	structure := makeObjectiveTestStructure()
	found := navigation.FindGroupByID(structure, "nonexistent")
	assert.Nil(t, found)
}

func TestFindGroupByID_NilStructure(t *testing.T) {
	found := navigation.FindGroupByID(nil, "any")
	assert.Nil(t, found)
}

// === GetNextGroup Tests ===

func TestGetNextGroup_HasNextSibling(t *testing.T) {
	structure := makeObjectiveTestStructure()
	// GetNextGroup no longer checks completion - it just finds next in sequence
	next, shouldAdvance, reason := navigation.GetNextGroup(structure, "group1", []string{})

	assert.True(t, shouldAdvance)
	assert.Equal(t, navigation.ReasonNextSibling, reason)
	assert.Equal(t, "group2", next.ID)
}

func TestGetNextGroup_SecondToThird(t *testing.T) {
	structure := makeObjectiveTestStructure()

	next, shouldAdvance, reason := navigation.GetNextGroup(structure, "group2", []string{})

	assert.True(t, shouldAdvance)
	assert.Equal(t, navigation.ReasonNextSibling, reason)
	assert.Equal(t, "group3", next.ID)
}

func TestGetNextGroup_LastGroupNoNext(t *testing.T) {
	structure := makeObjectiveTestStructure()
	// group3 is the last sibling, so there's no next group
	next, shouldAdvance, reason := navigation.GetNextGroup(structure, "group3", []string{})

	assert.False(t, shouldAdvance)
	assert.Equal(t, navigation.ReasonAllComplete, reason)
	assert.Nil(t, next)
}

func TestGetNextGroup_GroupNotFound(t *testing.T) {
	structure := makeObjectiveTestStructure()

	next, shouldAdvance, reason := navigation.GetNextGroup(structure, "nonexistent", []string{})

	assert.False(t, shouldAdvance)
	assert.Equal(t, navigation.ReasonGroupNotFound, reason)
	assert.Nil(t, next)
}

// TestGetNextGroup_SkipsSecretGroups verifies secret groups are never returned
// as the next group when advancing.
func TestGetNextGroup_SkipsSecretGroups(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             "group1",
				Name:           "Group 1",
				Color:          "blue",
				Routing:        models.RouteStrategyOrdered,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				ObjectiveIDs:   []string{"obj1"},
			},
			{
				ID:             "secret_group",
				Name:           "Secret Group",
				Color:          "red",
				Routing:        models.RouteStrategySecret,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				ObjectiveIDs:   []string{"secret1"},
			},
			{
				ID:             "group2",
				Name:           "Group 2",
				Color:          "green",
				Routing:        models.RouteStrategyFreeRoam,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				ObjectiveIDs:   []string{"obj2"},
			},
		},
	}

	// Get next group from group1 - should skip secret_group and return group2.
	nextGroup, shouldAdvance, reason := navigation.GetNextGroup(structure, "group1", []string{"obj1"})

	assert.True(t, shouldAdvance, "should advance to next non-secret group")
	assert.NotNil(t, nextGroup, "should return a group")
	assert.Equal(t, "group2", nextGroup.ID, "should skip secret_group and return group2")
	assert.Equal(t, navigation.ReasonNextSibling, reason, "should report correct reason")
}

// === GetFirstVisibleGroup Tests ===

func TestGetFirstVisibleGroup_Success(t *testing.T) {
	structure := makeObjectiveTestStructure()

	first := navigation.GetFirstVisibleGroup(structure)

	assert.NotNil(t, first)
	assert.Equal(t, "group1", first.ID)
}

func TestGetFirstVisibleGroup_NoSubGroups(t *testing.T) {
	structure := &models.GameStructure{
		ID:        "root",
		IsRoot:    true,
		SubGroups: []models.GameStructure{},
	}

	first := navigation.GetFirstVisibleGroup(structure)

	assert.Nil(t, first)
}

func TestGetFirstVisibleGroup_NotRoot(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "notroot",
		IsRoot: false,
	}

	first := navigation.GetFirstVisibleGroup(structure)

	assert.Nil(t, first)
}

func TestGetFirstVisibleGroup_Nil(t *testing.T) {
	first := navigation.GetFirstVisibleGroup(nil)
	assert.Nil(t, first)
}

// === ValidateStructure Tests ===

func TestValidateStructure_Valid(t *testing.T) {
	structure := makeObjectiveTestStructure()

	err := navigation.ValidateStructure(structure)

	assert.NoError(t, err)
}

func TestValidateStructure_NoSubGroups(t *testing.T) {
	structure := &models.GameStructure{
		ID:        "root",
		IsRoot:    true,
		SubGroups: []models.GameStructure{},
	}

	err := navigation.ValidateStructure(structure)

	assert.ErrorIs(t, err, navigation.ErrNoVisibleGroups)
}

func TestValidateStructure_DuplicateGroupID(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{ID: "duplicate", Name: "Group 1", Color: "blue"},
			{ID: "duplicate", Name: "Group 2", Color: "red"},
		},
	}

	err := navigation.ValidateStructure(structure)

	assert.ErrorIs(t, err, navigation.ErrDuplicateGroupID)
}

func TestValidateStructure_MissingGroupName(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             "group1",
				Name:           "", // Missing name
				Color:          "blue",
				Routing:        models.RouteStrategyOrdered,
				CompletionType: models.CompletionAll,
			},
		},
	}

	err := navigation.ValidateStructure(structure)

	assert.ErrorIs(t, err, navigation.ErrMissingGroupName)
}

func TestValidateStructure_MissingGroupColor(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             "group1",
				Name:           "Group 1",
				Color:          "", // Missing color
				Routing:        models.RouteStrategyOrdered,
				CompletionType: models.CompletionAll,
			},
		},
	}

	err := navigation.ValidateStructure(structure)

	assert.ErrorIs(t, err, navigation.ErrMissingGroupColor)
}

func TestValidateStructure_RootCanHaveEmptyNameAndColor(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		Name:   "", // Root can have empty name
		Color:  "", // Root can have empty color
		SubGroups: []models.GameStructure{
			{
				ID:             "group1",
				Name:           "Group 1",
				Color:          "blue",
				Routing:        models.RouteStrategyOrdered,
				CompletionType: models.CompletionAll,
			},
		},
	}

	err := navigation.ValidateStructure(structure)

	assert.NoError(t, err)
}

func TestValidateStructure_Nil(t *testing.T) {
	err := navigation.ValidateStructure(nil)
	assert.NoError(t, err)
}

func TestValidateStructure_NonRootIsRoot(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:     "child1",
				Name:   "Child 1",
				Color:  "blue",
				IsRoot: true, // ERROR: child cannot be root
			},
		},
	}

	err := navigation.ValidateStructure(structure)

	assert.ErrorIs(t, err, navigation.ErrNonRootIsRoot)
}

func TestValidateStructure_RandomWithMaxNextZero(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:           "group1",
				Name:         "Group 1",
				Color:        "blue",
				Routing:      models.RouteStrategyRandomised,
				MaxNext:      0, // ERROR: random routing must have MaxNext > 0.
				ObjectiveIDs: []string{"obj1", "obj2"},
			},
		},
	}

	err := navigation.ValidateStructure(structure)

	assert.ErrorIs(t, err, navigation.ErrInvalidMaxNext)
}

func TestValidateStructure_RandomWithValidMaxNext(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:           "group1",
				Name:         "Group 1",
				Color:        "blue",
				Routing:      models.RouteStrategyRandomised,
				MaxNext:      3, // Valid: MaxNext > 0.
				ObjectiveIDs: []string{"obj1", "obj2", "obj3"},
			},
		},
	}

	err := navigation.ValidateStructure(structure)

	assert.NoError(t, err)
}

func TestValidateStructure_NestedNonRootIsRoot(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:    "child1",
				Name:  "Child 1",
				Color: "blue",
				SubGroups: []models.GameStructure{
					{
						ID:     "grandchild1",
						Name:   "Grandchild 1",
						Color:  "red",
						IsRoot: true, // ERROR: nested child cannot be root
					},
				},
			},
		},
	}

	err := navigation.ValidateStructure(structure)

	assert.ErrorIs(t, err, navigation.ErrNonRootIsRoot)
}

// === Performance Tests ===

func BenchmarkGetNextGroup(b *testing.B) {
	structure := makeObjectiveTestStructure()
	completed := []string{"obj1", "obj2"}

	b.ResetTimer()
	for range b.N {
		navigation.GetNextGroup(structure, "group1", completed)
	}
}

func BenchmarkFindGroupByID(b *testing.B) {
	structure := makeObjectiveTestStructure()

	b.ResetTimer()
	for range b.N {
		navigation.FindGroupByID(structure, "group2")
	}
}
