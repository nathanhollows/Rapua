package navigation_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/navigation"
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

func TestGetNextGroup_HasNextSibling(t *testing.T) {
	structure := makeTestStructure()
	// GetNextGroup no longer checks completion - it just finds next in sequence
	next, shouldAdvance, reason := navigation.GetNextGroup(structure, "group1", []string{})

	assert.True(t, shouldAdvance)
	assert.Equal(t, navigation.ReasonNextSibling, reason)
	assert.Equal(t, "group2", next.ID)
}

func TestGetNextGroup_SecondToThird(t *testing.T) {
	structure := makeTestStructure()

	next, shouldAdvance, reason := navigation.GetNextGroup(structure, "group2", []string{})

	assert.True(t, shouldAdvance)
	assert.Equal(t, navigation.ReasonNextSibling, reason)
	assert.Equal(t, "group3", next.ID)
}

func TestGetNextGroup_LastGroupNoNext(t *testing.T) {
	structure := makeTestStructure()
	// group3 is the last sibling, so there's no next group
	next, shouldAdvance, reason := navigation.GetNextGroup(structure, "group3", []string{})

	assert.False(t, shouldAdvance)
	assert.Equal(t, navigation.ReasonAllComplete, reason)
	assert.Nil(t, next)
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

// === ComputeCurrentGroup Tests ===

func TestComputeCurrentGroup_NoCompletions(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{}

	groupID := navigation.ComputeCurrentGroup(structure, completed, nil)
	assert.Equal(t, "group1", groupID, "should start at first visible group")
}

func TestComputeCurrentGroup_FirstGroupIncomplete(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc1"} // Only 1 of 2 locations in group1

	groupID := navigation.ComputeCurrentGroup(structure, completed, nil)
	assert.Equal(t, "group1", groupID, "should stay in incomplete group")
}

func TestComputeCurrentGroup_AutoAdvanceToGroup2(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc1", "loc2"} // group1 100% complete (all 2 locations)

	groupID := navigation.ComputeCurrentGroup(structure, completed, nil)
	assert.Equal(t, "group2", groupID, "should auto-advance to group2 (100% complete)")
}

func TestComputeCurrentGroup_Group2Incomplete(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc1", "loc2", "loc3"} // group1 complete, group2 only 1 of 3 (needs 2)

	groupID := navigation.ComputeCurrentGroup(structure, completed, nil)
	assert.Equal(t, "group2", groupID, "should stay in group2 (needs minimum 2)")
}

func TestComputeCurrentGroup_AutoAdvanceToGroup3(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc1", "loc2", "loc3", "loc4"} // group1 100%, group2 minimum (2 of 3) with AutoAdvance=true

	groupID := navigation.ComputeCurrentGroup(structure, completed, nil)
	assert.Equal(t, "group3", groupID, "should auto-advance to group3 (minimum met with AutoAdvance=true)")
}

func TestComputeCurrentGroup_Group3CompleteNoAutoAdvance(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc1", "loc2", "loc3", "loc4", "loc6"} // All groups 100% complete

	groupID := navigation.ComputeCurrentGroup(structure, completed, nil)
	assert.Equal(t, "group3", groupID, "should stay in group3 (last group, 100% complete)")
}

func TestComputeCurrentGroup_MinimumMetButAutoAdvanceFalse(t *testing.T) {
	// Create structure where group2 has AutoAdvance=false
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             "group1",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc1"},
			},
			{
				ID:              "group2",
				CompletionType:  models.CompletionMinimum,
				MinimumRequired: 2,
				AutoAdvance:     false, // Let players complete remaining locations
				LocationIDs:     []string{"loc2", "loc3", "loc4"},
			},
			{
				ID:             "group3",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc5"},
			},
		},
	}

	// Complete group1 fully, complete minimum for group2 (2 of 3)
	completed := []string{"loc1", "loc2", "loc3"}

	groupID := navigation.ComputeCurrentGroup(structure, completed, nil)
	assert.Equal(t, "group2", groupID, "should stay in group2 (minimum met but AutoAdvance=false)")
}

func TestComputeCurrentGroup_AllLocationsCompleteAlwaysAdvances(t *testing.T) {
	// Even with AutoAdvance=false, should advance when ALL locations complete
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             "group1",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc1"},
			},
			{
				ID:              "group2",
				CompletionType:  models.CompletionMinimum,
				MinimumRequired: 2,
				AutoAdvance:     false, // Normally would stay
				LocationIDs:     []string{"loc2", "loc3", "loc4"},
			},
			{
				ID:             "group3",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc5"},
			},
		},
	}

	// Complete ALL locations in group2 (not just minimum)
	completed := []string{"loc1", "loc2", "loc3", "loc4"}

	groupID := navigation.ComputeCurrentGroup(structure, completed, nil)
	assert.Equal(t, "group3", groupID, "should advance to group3 (100% complete overrides AutoAdvance=false)")
}

func TestComputeCurrentGroup_NoVisibleGroups(t *testing.T) {
	structure := &models.GameStructure{
		ID:        "root",
		IsRoot:    true,
		SubGroups: []models.GameStructure{}, // No visible groups
	}
	completed := []string{}

	groupID := navigation.ComputeCurrentGroup(structure, completed, nil)
	assert.Empty(t, groupID, "should return empty string when no visible groups")
}

func TestComputeCurrentGroup_Deterministic(t *testing.T) {
	structure := makeTestStructure()
	completed := []string{"loc1", "loc2", "loc3"}

	// Call multiple times with same inputs
	groupID1 := navigation.ComputeCurrentGroup(structure, completed, nil)
	groupID2 := navigation.ComputeCurrentGroup(structure, completed, nil)
	groupID3 := navigation.ComputeCurrentGroup(structure, completed, nil)

	assert.Equal(t, groupID1, groupID2, "should be deterministic")
	assert.Equal(t, groupID2, groupID3, "should be deterministic")
}

func TestComputeCurrentGroup_NestedGroups(t *testing.T) {
	// Test with nested subgroups
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             "parent",
				Name:           "Parent Group",
				Color:          "blue",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{},
				SubGroups: []models.GameStructure{
					{
						ID:             "child1",
						Name:           "Child 1",
						Color:          "red",
						CompletionType: models.CompletionAll,
						AutoAdvance:    true,
						LocationIDs:    []string{"loc1", "loc2"},
					},
					{
						ID:             "child2",
						Name:           "Child 2",
						Color:          "green",
						CompletionType: models.CompletionAll,
						AutoAdvance:    false,
						LocationIDs:    []string{"loc3"},
					},
				},
			},
		},
	}

	// No completions - should start at first visible subgroup
	groupID := navigation.ComputeCurrentGroup(structure, []string{}, nil)
	assert.Equal(t, "parent", groupID, "should start at parent group")
}

func TestComputeCurrentGroup_DifferentCompletionOrder(t *testing.T) {
	structure := makeTestStructure()

	// Same locations completed, different order
	completed1 := []string{"loc2", "loc1"}
	completed2 := []string{"loc1", "loc2"}

	groupID1 := navigation.ComputeCurrentGroup(structure, completed1, nil)
	groupID2 := navigation.ComputeCurrentGroup(structure, completed2, nil)

	assert.Equal(t, groupID1, groupID2, "order of completions should not matter")
}

func TestComputeCurrentGroup_SkippedGroupAdvances(t *testing.T) {
	// Create structure: group1 → group2 (AutoAdvance=false) → group3
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             "group1",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc1"},
			},
			{
				ID:              "group2",
				CompletionType:  models.CompletionMinimum,
				MinimumRequired: 2,
				AutoAdvance:     false, // Normally would stay here
				LocationIDs:     []string{"loc2", "loc3", "loc4"},
			},
			{
				ID:             "group3",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc5"},
			},
		},
	}

	// Complete group1, complete minimum for group2
	completed := []string{"loc1", "loc2", "loc3"}

	// Without skipping, should stay in group2 (AutoAdvance=false)
	groupID := navigation.ComputeCurrentGroup(structure, completed, nil)
	assert.Equal(t, "group2", groupID, "should stay in group2 without skip")

	// With group2 skipped, should advance to group3
	skipped := []string{"group2"}
	groupID = navigation.ComputeCurrentGroup(structure, completed, skipped)
	assert.Equal(t, "group3", groupID, "should advance to group3 when group2 skipped")
}

func TestComputeCurrentGroup_SkippedGroupBeforeCompletion(t *testing.T) {
	// Test that skipping a group works even if minimum not yet met
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:              "group1",
				CompletionType:  models.CompletionMinimum,
				MinimumRequired: 2,
				AutoAdvance:     false,
				LocationIDs:     []string{"loc1", "loc2", "loc3"},
			},
			{
				ID:             "group2",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc4"},
			},
		},
	}

	// Complete only 1 location in group1 (minimum not met)
	completed := []string{"loc1"}

	// Without skipping, should stay in group1
	groupID := navigation.ComputeCurrentGroup(structure, completed, nil)
	assert.Equal(t, "group1", groupID, "should stay in group1 without skip")

	// With group1 skipped, should advance to group2 even though minimum not met
	skipped := []string{"group1"}
	groupID = navigation.ComputeCurrentGroup(structure, completed, skipped)
	assert.Equal(t, "group2", groupID, "should advance to group2 when group1 skipped")
}

func TestComputeCurrentGroup_SkippedLastGroup(t *testing.T) {
	// Test that skipping the last group keeps you there (nowhere to go)
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             "group1",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc1"},
			},
			{
				ID:             "group2",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc2"},
			},
		},
	}

	// Complete group1
	completed := []string{"loc1"}

	// Skip group2 (the last group)
	skipped := []string{"group2"}
	groupID := navigation.ComputeCurrentGroup(structure, completed, skipped)

	// Should stay at group2 since there's nowhere to advance to
	assert.Equal(t, "group2", groupID, "should stay at group2 (last group) even when skipped")
}

func TestComputeCurrentGroup_MultipleSkippedGroups(t *testing.T) {
	// Test skipping multiple groups in sequence
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             "group1",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc1"},
			},
			{
				ID:             "group2",
				CompletionType: models.CompletionAll,
				AutoAdvance:    false,
				LocationIDs:    []string{"loc2"},
			},
			{
				ID:             "group3",
				CompletionType: models.CompletionAll,
				AutoAdvance:    false,
				LocationIDs:    []string{"loc3"},
			},
			{
				ID:             "group4",
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc4"},
			},
		},
	}

	// Complete group1
	completed := []string{"loc1"}

	// Skip both group2 and group3
	skipped := []string{"group2", "group3"}
	groupID := navigation.ComputeCurrentGroup(structure, completed, skipped)

	// Should advance through both skipped groups to group4
	assert.Equal(t, "group4", groupID, "should advance through multiple skipped groups to group4")
}

// Benchmark ComputeCurrentGroup.
func BenchmarkComputeCurrentGroup(b *testing.B) {
	structure := makeTestStructure()
	completed := []string{"loc1", "loc2", "loc3"}

	b.ResetTimer()
	for range b.N {
		navigation.ComputeCurrentGroup(structure, completed, nil)
	}
}

// Test GetAccessibleSecretLocationIDs - sibling secret groups.
func TestGetAccessibleSecretLocationIDs_SiblingSecrets(t *testing.T) {
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
				LocationIDs:    []string{"loc1", "loc2", "loc3"},
			},
			{
				ID:             "secret_group",
				Name:           "Secret Group",
				Color:          "red",
				Routing:        models.RouteStrategySecret,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc4", "loc5"},
			},
		},
	}

	// Player is in group1
	secretIDs := navigation.GetAccessibleSecretLocationIDs(structure, "group1", []string{})

	assert.Len(t, secretIDs, 2, "should have 2 accessible secret locations")
	assert.Contains(t, secretIDs, "loc4")
	assert.Contains(t, secretIDs, "loc5")
}

// Test GetAccessibleSecretLocationIDs - uncle secret groups.
func TestGetAccessibleSecretLocationIDs_UncleSecrets(t *testing.T) {
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
				SubGroups: []models.GameStructure{
					{
						ID:             "secret_group2",
						Name:           "Secret Subgroup",
						Color:          "purple",
						Routing:        models.RouteStrategySecret,
						CompletionType: models.CompletionAll,
						AutoAdvance:    true,
						LocationIDs:    []string{"loc6"},
					},
					{
						ID:             "subgroup1",
						Name:           "Subgroup 1",
						Color:          "green",
						Routing:        models.RouteStrategyOrdered,
						CompletionType: models.CompletionAll,
						AutoAdvance:    true,
						LocationIDs:    []string{"loc1", "loc2", "loc3"},
					},
				},
			},
			{
				ID:             "group2",
				Name:           "Group 2",
				Color:          "yellow",
				Routing:        models.RouteStrategyOrdered,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc7", "loc8"},
			},
			{
				ID:             "secret_group",
				Name:           "Secret Root Group",
				Color:          "red",
				Routing:        models.RouteStrategySecret,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc4", "loc5"},
			},
		},
	}

	// Player is in subgroup1 (child of group1)
	// Should have access to:
	// - secret_group2 (sibling)
	// - secret_group (uncle - sibling of parent group1)
	secretIDs := navigation.GetAccessibleSecretLocationIDs(structure, "subgroup1", []string{})

	assert.Len(t, secretIDs, 3, "should have 3 accessible secret locations")
	assert.Contains(t, secretIDs, "loc6", "should access sibling secret_group2")
	assert.Contains(t, secretIDs, "loc4", "should access uncle secret_group")
	assert.Contains(t, secretIDs, "loc5", "should access uncle secret_group")
}

// Test GetAccessibleSecretLocationIDs - filters completed locations.
func TestGetAccessibleSecretLocationIDs_FilterCompleted(t *testing.T) {
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
				LocationIDs:    []string{"loc1", "loc2"},
			},
			{
				ID:             "secret_group",
				Name:           "Secret Group",
				Color:          "red",
				Routing:        models.RouteStrategySecret,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc4", "loc5", "loc6"},
			},
		},
	}

	// Player has completed loc4
	secretIDs := navigation.GetAccessibleSecretLocationIDs(structure, "group1", []string{"loc4"})

	assert.Len(t, secretIDs, 2, "should only return unvisited secret locations")
	assert.Contains(t, secretIDs, "loc5")
	assert.Contains(t, secretIDs, "loc6")
	assert.NotContains(t, secretIDs, "loc4", "should not include completed location")
}

// Test GetAccessibleSecretLocationIDs - no secret groups.
func TestGetAccessibleSecretLocationIDs_NoSecrets(t *testing.T) {
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
				LocationIDs:    []string{"loc1", "loc2"},
			},
			{
				ID:             "group2",
				Name:           "Group 2",
				Color:          "red",
				Routing:        models.RouteStrategyFreeRoam,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc3", "loc4"},
			},
		},
	}

	secretIDs := navigation.GetAccessibleSecretLocationIDs(structure, "group1", []string{})

	assert.Empty(t, secretIDs, "should return empty slice when no secret groups exist")
}

// Test GetAccessibleSecretLocationIDs - invalid group ID.
func TestGetAccessibleSecretLocationIDs_InvalidGroupID(t *testing.T) {
	structure := makeTestStructure()

	secretIDs := navigation.GetAccessibleSecretLocationIDs(structure, "nonexistent", []string{})

	assert.Empty(t, secretIDs, "should return empty slice for invalid group ID")
}

// Test GetAvailableLocationIDs - secret routing strategy returns empty.
func TestGetAvailableLocationIDs_SecretRoutingReturnsEmpty(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             "secret_group",
				Name:           "Secret Group",
				Color:          "red",
				Routing:        models.RouteStrategySecret,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc1", "loc2"},
			},
		},
	}

	locationIDs := navigation.GetAvailableLocationIDs(structure, "secret_group", []string{}, "TEAM1")

	assert.Empty(t, locationIDs, "secret groups should never return locations in GetAvailableLocationIDs")
}

// Test ComputeCurrentGroup - secret groups are never current group.
func TestComputeCurrentGroup_SecretGroupsNeverCurrent(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             "secret_group",
				Name:           "Secret Group",
				Color:          "red",
				Routing:        models.RouteStrategySecret,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"secret1", "secret2"},
			},
			{
				ID:             "group1",
				Name:           "Group 1",
				Color:          "blue",
				Routing:        models.RouteStrategyOrdered,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc1", "loc2"},
			},
		},
	}

	// Start of game - should be group1, not secret_group
	currentGroupID := navigation.ComputeCurrentGroup(structure, []string{}, nil)
	assert.Equal(t, "group1", currentGroupID, "should skip secret groups and start at group1")

	// Complete a secret location - current group should still be group1
	currentGroupID = navigation.ComputeCurrentGroup(structure, []string{"secret1"}, nil)
	assert.Equal(t, "group1", currentGroupID, "completing secret locations should not affect current group")
}

// Test ComputeCurrentGroup - game completion ignores secret groups.
func TestComputeCurrentGroup_GameCompletionIgnoresSecrets(t *testing.T) {
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
				LocationIDs:    []string{"loc1", "loc2"},
			},
			{
				ID:             "secret_group",
				Name:           "Secret Group",
				Color:          "red",
				Routing:        models.RouteStrategySecret,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"secret1", "secret2"},
			},
			{
				ID:             "group2",
				Name:           "Group 2",
				Color:          "green",
				Routing:        models.RouteStrategyFreeRoam,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc3", "loc4"},
			},
		},
	}

	// Complete all non-secret locations
	completed := []string{"loc1", "loc2", "loc3", "loc4"}
	currentGroupID := navigation.ComputeCurrentGroup(structure, completed, nil)

	// Game should be complete (current group is last non-secret group)
	// When all locations are complete, we stay at the last group
	assert.Equal(
		t,
		"group2",
		currentGroupID,
		"should stay at last non-secret group when all non-secret locations complete",
	)

	// Even with some secret locations completed, game should still be complete
	completedWithSecrets := []string{"loc1", "loc2", "loc3", "loc4", "secret1"}
	currentGroupID = navigation.ComputeCurrentGroup(structure, completedWithSecrets, nil)
	assert.Equal(t, "group2", currentGroupID, "secret location completion should not affect game state")
}

// Test ComputeCurrentGroup - secret groups between regular groups.
func TestComputeCurrentGroup_SecretGroupsBetweenRegular(t *testing.T) {
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
				LocationIDs:    []string{"loc1", "loc2"},
			},
			{
				ID:             "secret_group",
				Name:           "Secret Group",
				Color:          "red",
				Routing:        models.RouteStrategySecret,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"secret1"},
			},
			{
				ID:             "group2",
				Name:           "Group 2",
				Color:          "green",
				Routing:        models.RouteStrategyFreeRoam,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc3", "loc4"},
			},
		},
	}

	// Start - should be group1
	currentGroupID := navigation.ComputeCurrentGroup(structure, []string{}, nil)
	assert.Equal(t, "group1", currentGroupID, "should start at group1")

	// Complete group1 - should skip secret_group and advance to group2
	completed := []string{"loc1", "loc2"}
	currentGroupID = navigation.ComputeCurrentGroup(structure, completed, nil)
	assert.Equal(t, "group2", currentGroupID, "should skip secret_group and advance to group2")

	// Complete secret while in group2 - should stay in group2
	completedWithSecret := []string{"loc1", "loc2", "secret1"}
	currentGroupID = navigation.ComputeCurrentGroup(structure, completedWithSecret, nil)
	assert.Equal(t, "group2", currentGroupID, "completing secret should not change current group")
}

// Test GetNextGroup - should skip secret groups when advancing.
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
				LocationIDs:    []string{"loc1"},
			},
			{
				ID:             "secret_group",
				Name:           "Secret Group",
				Color:          "red",
				Routing:        models.RouteStrategySecret,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"secret1"},
			},
			{
				ID:             "group2",
				Name:           "Group 2",
				Color:          "green",
				Routing:        models.RouteStrategyFreeRoam,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				LocationIDs:    []string{"loc2"},
			},
		},
	}

	// Get next group from group1 - should skip secret_group and return group2
	nextGroup, shouldAdvance, reason := navigation.GetNextGroup(structure, "group1", []string{"loc1"})

	assert.True(t, shouldAdvance, "should advance to next non-secret group")
	assert.NotNil(t, nextGroup, "should return a group")
	assert.Equal(t, "group2", nextGroup.ID, "should skip secret_group and return group2")
	assert.Equal(t, navigation.ReasonNextSibling, reason, "should report correct reason")
}
