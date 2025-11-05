package navigation_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v5/models"
	"github.com/nathanhollows/Rapua/v5/navigation"
	"github.com/stretchr/testify/assert"
)

// Test helpers.
func makeTestStructure() *models.GameStructure {
	return &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             "group1",
				Name:           "First Group",
				Color:          "blue",
				Routing:        models.RouteStrategyOrdered,
				Navigation:     models.NavigationDisplayNames,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc1", "loc2"},
			},
			{
				ID:              "group2",
				Name:            "Second Group",
				Color:           "red",
				Routing:         models.RouteStrategyFreeRoam,
				Navigation:      models.NavigationDisplayNames,
				CompletionType:  models.CompletionMinimum,
				MinimumRequired: 2,
				AutoAdvance:     true,
				LocationIDs:     []string{"loc3", "loc4", "loc5"},
			},
			{
				ID:             "group3",
				Name:           "Third Group",
				Color:          "green",
				Routing:        models.RouteStrategyRandom,
				Navigation:     models.NavigationDisplayNames,
				CompletionType: models.CompletionAll,
				AutoAdvance:    false, // No auto-advance
				MaxNext:        1,     // Random routing requires MaxNext > 0
				LocationIDs:    []string{"loc6"},
			},
		},
	}
}

// === FindGroupByID Tests ===

func TestFindGroupByID_Root(t *testing.T) {
	structure := makeTestStructure()
	found := navigation.FindGroupByID(structure, "root")
	assert.NotNil(t, found)
	assert.Equal(t, "root", found.ID)
}

func TestFindGroupByID_DirectChild(t *testing.T) {
	structure := makeTestStructure()
	found := navigation.FindGroupByID(structure, "group1")
	assert.NotNil(t, found)
	assert.Equal(t, "group1", found.ID)
	assert.Equal(t, "First Group", found.Name)
}

func TestFindGroupByID_NotFound(t *testing.T) {
	structure := makeTestStructure()
	found := navigation.FindGroupByID(structure, "nonexistent")
	assert.Nil(t, found)
}

func TestFindGroupByID_NilStructure(t *testing.T) {
	found := navigation.FindGroupByID(nil, "any")
	assert.Nil(t, found)
}

// === IsGroupCompleted Tests ===

func TestIsGroupCompleted_CompletionAll_Complete(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc1", "loc2"}

	result := navigation.IsGroupCompleted(structure, "group1", completed)
	assert.True(t, result)
}

func TestIsGroupCompleted_CompletionAll_Incomplete(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc1"}

	result := navigation.IsGroupCompleted(structure, "group1", completed)
	assert.False(t, result)
}

func TestIsGroupCompleted_CompletionMinimum_Complete(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc3", "loc4"} // 2 out of 3, minimum is 2

	result := navigation.IsGroupCompleted(structure, "group2", completed)
	assert.True(t, result)
}

func TestIsGroupCompleted_CompletionMinimum_Incomplete(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc3"} // 1 out of 3, minimum is 2

	result := navigation.IsGroupCompleted(structure, "group2", completed)
	assert.False(t, result)
}

func TestIsGroupCompleted_GroupNotFound(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc1"}

	result := navigation.IsGroupCompleted(structure, "nonexistent", completed)
	assert.False(t, result)
}

func TestIsGroupCompleted_EmptyLocationIDs(t *testing.T) {
	structure := &models.GameStructure{
		ID:             "empty",
		CompletionType: models.CompletionAll,
		LocationIDs:    []string{},
	}

	result := navigation.IsGroupCompleted(structure, "empty", []string{})
	assert.False(t, result)
}

// === GetNextGroup Tests ===

func TestGetNextGroup_Incomplete(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc1"} // Only 1 of 2 locations

	next, shouldAdvance, reason := navigation.GetNextGroup(structure, "group1", completed)

	assert.False(t, shouldAdvance)
	assert.Equal(t, navigation.ReasonGroupIncomplete, reason)
	assert.Equal(t, "group1", next.ID)
}

func TestGetNextGroup_CompleteWithAutoAdvance(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc1", "loc2"} // All locations complete

	next, shouldAdvance, reason := navigation.GetNextGroup(structure, "group1", completed)

	assert.True(t, shouldAdvance)
	assert.Equal(t, navigation.ReasonNextSibling, reason)
	assert.Equal(t, "group2", next.ID)
}

func TestGetNextGroup_CompleteNoAutoAdvance(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc6"} // Group3 complete but AutoAdvance = false

	next, shouldAdvance, reason := navigation.GetNextGroup(structure, "group3", completed)

	assert.False(t, shouldAdvance)
	assert.Equal(t, navigation.ReasonAutoAdvanceDisabled, reason)
	assert.Equal(t, "group3", next.ID)
}

func TestGetNextGroup_LastGroup(t *testing.T) {
	structure := makeTestStructure()
	// Complete group2 (last group with AutoAdvance)
	completed := []string{"loc3", "loc4"}

	next, shouldAdvance, reason := navigation.GetNextGroup(structure, "group2", completed)

	// Should advance to group3 (next sibling)
	assert.True(t, shouldAdvance)
	assert.Equal(t, navigation.ReasonNextSibling, reason)
	assert.Equal(t, "group3", next.ID)
}

func TestGetNextGroup_GroupNotFound(t *testing.T) {
	structure := makeTestStructure()

	next, shouldAdvance, reason := navigation.GetNextGroup(structure, "nonexistent", []string{})

	assert.False(t, shouldAdvance)
	assert.Equal(t, navigation.ReasonGroupNotFound, reason)
	assert.Nil(t, next)
}

// === GetAvailableLocationIDs Tests ===

func TestGetAvailableLocationIDs_Ordered(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{}

	locationIDs := navigation.GetAvailableLocationIDs(structure, "group1", completed, "TEAM1")

	// Should return only the first location ID (lowest order)
	assert.Len(t, locationIDs, 1)
	assert.Equal(t, "loc1", locationIDs[0])
}

func TestGetAvailableLocationIDs_OrderedPartialComplete(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc1"}

	locationIDs := navigation.GetAvailableLocationIDs(structure, "group1", completed, "TEAM1")

	// Should return next location ID in order
	assert.Len(t, locationIDs, 1)
	assert.Equal(t, "loc2", locationIDs[0])
}

func TestGetAvailableLocationIDs_FreeRoam(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{}

	locationIDs := navigation.GetAvailableLocationIDs(structure, "group2", completed, "TEAM1")

	// Should return all unvisited location IDs
	assert.Len(t, locationIDs, 3)
}

func TestGetAvailableLocationIDs_FreeRoamPartialComplete(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc3"}

	locationIDs := navigation.GetAvailableLocationIDs(structure, "group2", completed, "TEAM1")

	// Should return remaining unvisited location IDs
	assert.Len(t, locationIDs, 2)
	assert.Contains(t, locationIDs, "loc4")
	assert.Contains(t, locationIDs, "loc5")
}

func TestGetAvailableLocationIDs_RandomDeterministic(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{}

	// Same team code should produce same shuffle
	locationIDs1 := navigation.GetAvailableLocationIDs(structure, "group2", completed, "TEAM1")
	locationIDs2 := navigation.GetAvailableLocationIDs(structure, "group2", completed, "TEAM1")

	assert.Len(t, locationIDs1, 3)
	assert.Len(t, locationIDs2, 3)

	// Order should be identical
	for i := range locationIDs1 {
		assert.Equal(t, locationIDs1[i], locationIDs2[i])
	}
}

func TestGetAvailableLocationIDs_RandomWithMaxNext(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{}

	// Use group3 which has Random routing
	locationIDs := navigation.GetAvailableLocationIDs(structure, "group3", completed, "TEAM1")

	// group3 only has 1 location, so maxNext doesn't limit it
	assert.Len(t, locationIDs, 1)

	// Test with group2 if we give it Random routing with MaxNext limit
	structure.SubGroups[1].Routing = models.RouteStrategyRandom
	structure.SubGroups[1].MaxNext = 2
	locationIDs = navigation.GetAvailableLocationIDs(structure, "group2", completed, "TEAM1")
	assert.Len(t, locationIDs, 2)
}

// TestGetAvailableLocationIDs_RandomWithReplacement tests that random mode
// maintains consistent order as locations are completed (random with replacement).
func TestGetAvailableLocationIDs_RandomWithReplacement(t *testing.T) {
	// Create a group with 5 locations for random routing
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:          "random-group",
				Name:        "Random Group",
				Color:       "blue",
				Routing:     models.RouteStrategyRandom,
				Navigation:  models.NavigationDisplayNames,
				MaxNext:     3, // Show 3 random locations at a time
				LocationIDs: []string{"loc1", "loc2", "loc3", "loc4", "loc5"},
			},
		},
	}

	teamCode := "TEAM1"

	// Get initial random order (no locations completed)
	completed := []string{}
	round1 := navigation.GetAvailableLocationIDs(structure, "random-group", completed, teamCode)
	assert.Len(t, round1, 3)

	// Record the IDs in order
	order1 := []string{round1[0], round1[1], round1[2]}

	// Complete the first location from round 1
	completed = []string{round1[0]}
	round2 := navigation.GetAvailableLocationIDs(structure, "random-group", completed, teamCode)
	assert.Len(t, round2, 3)

	// The first element should now be what was second in round1
	assert.Equal(t, order1[1], round2[0], "After completing first, second should become first")
	assert.Equal(t, order1[2], round2[1], "After completing first, third should become second")

	// Complete the second location (first from round2)
	completed = []string{round1[0], round2[0]}
	round3 := navigation.GetAvailableLocationIDs(structure, "random-group", completed, teamCode)
	assert.Len(t, round3, 3)

	// The first element should be what was third in round1
	assert.Equal(t, order1[2], round3[0], "Order should shift consistently")

	// Verify the shuffle is deterministic - same team code produces same order
	completedEmpty := []string{}
	verifyRound1 := navigation.GetAvailableLocationIDs(structure, "random-group", completedEmpty, teamCode)
	assert.Equal(t, order1[0], verifyRound1[0], "Shuffle should be deterministic")
	assert.Equal(t, order1[1], verifyRound1[1], "Shuffle should be deterministic")
	assert.Equal(t, order1[2], verifyRound1[2], "Shuffle should be deterministic")
}

// TestGetAvailableLocationIDs_RandomDifferentTeams verifies different teams get different shuffles.
func TestGetAvailableLocationIDs_RandomDifferentTeams(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:          "random-group",
				Name:        "Random Group",
				Color:       "blue",
				Routing:     models.RouteStrategyRandom,
				Navigation:  models.NavigationDisplayNames,
				MaxNext:     5, // Show all 5 locations
				LocationIDs: []string{"loc1", "loc2", "loc3", "loc4", "loc5"},
			},
		},
	}

	completed := []string{}

	// Get shuffles for different teams
	team1IDs := navigation.GetAvailableLocationIDs(structure, "random-group", completed, "TEAM1")
	team2IDs := navigation.GetAvailableLocationIDs(structure, "random-group", completed, "TEAM2")
	team3IDs := navigation.GetAvailableLocationIDs(structure, "random-group", completed, "TEAM3")

	// Teams should have different orders (probabilistically)
	// At least one team should have a different first location
	sameAsTeam1 := (team2IDs[0] == team1IDs[0]) && (team3IDs[0] == team1IDs[0])
	assert.False(t, sameAsTeam1, "Different teams should get different random orders")

	// But each team's order should be consistent
	verifyTeam1 := navigation.GetAvailableLocationIDs(structure, "random-group", completed, "TEAM1")
	assert.Equal(t, team1IDs[0], verifyTeam1[0], "Same team should get same shuffle")
	assert.Equal(t, team1IDs[1], verifyTeam1[1], "Same team should get same shuffle")
}

func TestGetAvailableLocationIDs_NoLocationIDs(t *testing.T) {
	structure := makeTestStructure()
	// Clear location IDs
	structure.SubGroups[0].LocationIDs = []string{}
	completed := []string{}

	locationIDs := navigation.GetAvailableLocationIDs(structure, "group1", completed, "TEAM1")

	assert.Empty(t, locationIDs)
}

func TestGetAvailableLocationIDs_AllComplete(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc1", "loc2"}

	locationIDs := navigation.GetAvailableLocationIDs(structure, "group1", completed, "TEAM1")

	assert.Empty(t, locationIDs)
}

func TestGetAvailableLocationIDs_GroupNotFound(t *testing.T) {
	structure := makeTestStructure()

	locationIDs := navigation.GetAvailableLocationIDs(structure, "nonexistent", []string{}, "TEAM1")

	assert.Empty(t, locationIDs)
}

// === GetFirstVisibleGroup Tests ===

func TestGetFirstVisibleGroup_Success(t *testing.T) {
	structure := makeTestStructure()

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
	structure := makeTestStructure()

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

func TestValidateStructure_DuplicateLocationID(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:          "group1",
				Name:        "Group 1",
				Color:       "blue",
				LocationIDs: []string{"loc1", "loc2"},
			},
			{
				ID:          "group2",
				Name:        "Group 2",
				Color:       "red",
				LocationIDs: []string{"loc2", "loc3"}, // loc2 duplicated
			},
		},
	}

	err := navigation.ValidateStructure(structure)

	assert.ErrorIs(t, err, navigation.ErrDuplicateLocationID)
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
				ID:          "group1",
				Name:        "Group 1",
				Color:       "blue",
				Routing:     models.RouteStrategyRandom,
				MaxNext:     0, // ERROR: random routing must have MaxNext > 0
				LocationIDs: []string{"loc1", "loc2"},
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
				ID:          "group1",
				Name:        "Group 1",
				Color:       "blue",
				Routing:     models.RouteStrategyRandom,
				MaxNext:     3, // Valid: MaxNext > 0
				LocationIDs: []string{"loc1", "loc2", "loc3"},
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

func BenchmarkIsGroupCompleted(b *testing.B) {
	structure := makeTestStructure()
	completed := []string{"loc1", "loc2"}

	b.ResetTimer()
	for range b.N {
		navigation.IsGroupCompleted(structure, "group1", completed)
	}
}

func BenchmarkGetNextGroup(b *testing.B) {
	structure := makeTestStructure()
	completed := []string{"loc1", "loc2"}

	b.ResetTimer()
	for range b.N {
		navigation.GetNextGroup(structure, "group1", completed)
	}
}

func BenchmarkGetAvailableLocationIDs_Ordered(b *testing.B) {
	structure := makeTestStructure()
	completed := []string{}

	b.ResetTimer()
	for range b.N {
		navigation.GetAvailableLocationIDs(structure, "group1", completed, "TEAM1")
	}
}

func BenchmarkGetAvailableLocationIDs_FreeRoam(b *testing.B) {
	structure := makeTestStructure()
	completed := []string{}

	b.ResetTimer()
	for range b.N {
		navigation.GetAvailableLocationIDs(structure, "group2", completed, "TEAM1")
	}
}

func BenchmarkFindGroupByID(b *testing.B) {
	structure := makeTestStructure()

	b.ResetTimer()
	for range b.N {
		navigation.FindGroupByID(structure, "group2")
	}
}
