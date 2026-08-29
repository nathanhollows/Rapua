package navigation_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
	"github.com/stretchr/testify/assert"
)

// Objective-ID mirror of makeTestStructure: same shape, ObjectiveIDs instead of LocationIDs.
func makeObjectiveTestStructure() *models.GameStructure {
	return &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{
				ID:             "group1",
				Name:           "First Group",
				Color:          "blue",
				Routing:        models.RouteStrategyOrdered,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				ObjectiveIDs:   []string{"obj1", "obj2"},
			},
			{
				ID:              "group2",
				Name:            "Second Group",
				Color:           "red",
				Routing:         models.RouteStrategyFreeRoam,
				CompletionType:  models.CompletionMinimum,
				MinimumRequired: 2,
				AutoAdvance:     true,
				ObjectiveIDs:    []string{"obj3", "obj4", "obj5"},
			},
			{
				ID:             "group3",
				Name:           "Third Group",
				Color:          "green",
				Routing:        models.RouteStrategyRandomised,
				CompletionType: models.CompletionAll,
				AutoAdvance:    false,
				MaxNext:        1,
				ObjectiveIDs:   []string{"obj6"},
			},
		},
	}
}

// === GetAvailableObjectiveIDs Tests ===

func TestGetAvailableObjectiveIDs_Ordered(t *testing.T) {
	structure := makeObjectiveTestStructure()

	objectiveIDs := navigation.GetAvailableObjectiveIDs(structure, "group1", []string{}, "TEAM1")

	assert.Len(t, objectiveIDs, 1)
	assert.Equal(t, "obj1", objectiveIDs[0])
}

func TestGetAvailableObjectiveIDs_OrderedPartialComplete(t *testing.T) {
	structure := makeObjectiveTestStructure()

	objectiveIDs := navigation.GetAvailableObjectiveIDs(structure, "group1", []string{"obj1"}, "TEAM1")

	assert.Len(t, objectiveIDs, 1)
	assert.Equal(t, "obj2", objectiveIDs[0])
}

func TestGetAvailableObjectiveIDs_FreeRoam(t *testing.T) {
	structure := makeObjectiveTestStructure()

	objectiveIDs := navigation.GetAvailableObjectiveIDs(structure, "group2", []string{}, "TEAM1")

	assert.Len(t, objectiveIDs, 3)
}

func TestGetAvailableObjectiveIDs_FreeRoamPartialComplete(t *testing.T) {
	structure := makeObjectiveTestStructure()

	objectiveIDs := navigation.GetAvailableObjectiveIDs(structure, "group2", []string{"obj3"}, "TEAM1")

	assert.Len(t, objectiveIDs, 2)
	assert.Contains(t, objectiveIDs, "obj4")
	assert.Contains(t, objectiveIDs, "obj5")
}

func TestGetAvailableObjectiveIDs_RandomDeterministic(t *testing.T) {
	structure := makeObjectiveTestStructure()

	ids1 := navigation.GetAvailableObjectiveIDs(structure, "group2", []string{}, "TEAM1")
	structure.SubGroups[1].Routing = models.RouteStrategyRandomised
	structure.SubGroups[1].MaxNext = 3
	ids2 := navigation.GetAvailableObjectiveIDs(structure, "group2", []string{}, "TEAM1")

	assert.Len(t, ids1, 3, "free-roam returns all unvisited")
	assert.Len(t, ids2, 3, "randomised with maxNext >= total returns all unvisited")
}

func TestGetAvailableObjectiveIDs_RandomWithMaxNext(t *testing.T) {
	structure := makeObjectiveTestStructure()

	objectiveIDs := navigation.GetAvailableObjectiveIDs(structure, "group3", []string{}, "TEAM1")
	assert.Len(t, objectiveIDs, 1, "group3 only has 1 objective")

	structure.SubGroups[1].Routing = models.RouteStrategyRandomised
	structure.SubGroups[1].MaxNext = 2
	objectiveIDs = navigation.GetAvailableObjectiveIDs(structure, "group2", []string{}, "TEAM1")
	assert.Len(t, objectiveIDs, 2)
}

func TestGetAvailableObjectiveIDs_NoObjectiveIDs(t *testing.T) {
	structure := makeObjectiveTestStructure()
	structure.SubGroups[0].ObjectiveIDs = []string{}

	objectiveIDs := navigation.GetAvailableObjectiveIDs(structure, "group1", []string{}, "TEAM1")

	assert.Empty(t, objectiveIDs)
}

func TestGetAvailableObjectiveIDs_AllComplete(t *testing.T) {
	structure := makeObjectiveTestStructure()

	objectiveIDs := navigation.GetAvailableObjectiveIDs(structure, "group1", []string{"obj1", "obj2"}, "TEAM1")

	assert.Empty(t, objectiveIDs)
}

func TestGetAvailableObjectiveIDs_GroupNotFound(t *testing.T) {
	structure := makeObjectiveTestStructure()

	objectiveIDs := navigation.GetAvailableObjectiveIDs(structure, "nonexistent", []string{}, "TEAM1")

	assert.Empty(t, objectiveIDs)
}

func TestGetAvailableObjectiveIDs_SecretRoutingReturnsEmpty(t *testing.T) {
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
				ObjectiveIDs:   []string{"obj1", "obj2"},
			},
		},
	}

	objectiveIDs := navigation.GetAvailableObjectiveIDs(structure, "secret_group", []string{}, "TEAM1")

	assert.Empty(t, objectiveIDs, "secret groups should never return objectives in GetAvailableObjectiveIDs")
}

// === ComputeCurrentGroupForObjectives Tests ===

func TestComputeCurrentGroupForObjectives_NoCompletions(t *testing.T) {
	structure := makeObjectiveTestStructure()

	groupID := navigation.ComputeCurrentGroupForObjectives(structure, []string{}, nil)
	assert.Equal(t, "group1", groupID)
}

func TestComputeCurrentGroupForObjectives_FirstGroupIncomplete(t *testing.T) {
	structure := makeObjectiveTestStructure()

	groupID := navigation.ComputeCurrentGroupForObjectives(structure, []string{"obj1"}, nil)
	assert.Equal(t, "group1", groupID)
}

func TestComputeCurrentGroupForObjectives_AutoAdvanceToGroup2(t *testing.T) {
	structure := makeObjectiveTestStructure()

	groupID := navigation.ComputeCurrentGroupForObjectives(structure, []string{"obj1", "obj2"}, nil)
	assert.Equal(t, "group2", groupID)
}

func TestComputeCurrentGroupForObjectives_Group2Incomplete(t *testing.T) {
	structure := makeObjectiveTestStructure()

	groupID := navigation.ComputeCurrentGroupForObjectives(structure, []string{"obj1", "obj2", "obj3"}, nil)
	assert.Equal(t, "group2", groupID, "needs minimum 2 of 3")
}

func TestComputeCurrentGroupForObjectives_AutoAdvanceToGroup3(t *testing.T) {
	structure := makeObjectiveTestStructure()

	groupID := navigation.ComputeCurrentGroupForObjectives(
		structure, []string{"obj1", "obj2", "obj3", "obj4"}, nil,
	)
	assert.Equal(t, "group3", groupID)
}

func TestComputeCurrentGroupForObjectives_Group3CompleteNoAutoAdvance(t *testing.T) {
	structure := makeObjectiveTestStructure()

	groupID := navigation.ComputeCurrentGroupForObjectives(
		structure, []string{"obj1", "obj2", "obj3", "obj4", "obj6"}, nil,
	)
	assert.Equal(t, "group3", groupID, "last group, 100% complete, stays put")
}

func TestComputeCurrentGroupForObjectives_MinimumMetButAutoAdvanceFalse(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{ID: "group1", CompletionType: models.CompletionAll, AutoAdvance: true, ObjectiveIDs: []string{"obj1"}},
			{
				ID:              "group2",
				CompletionType:  models.CompletionMinimum,
				MinimumRequired: 2,
				AutoAdvance:     false,
				ObjectiveIDs:    []string{"obj2", "obj3", "obj4"},
			},
			{ID: "group3", CompletionType: models.CompletionAll, AutoAdvance: true, ObjectiveIDs: []string{"obj5"}},
		},
	}

	groupID := navigation.ComputeCurrentGroupForObjectives(structure, []string{"obj1", "obj2", "obj3"}, nil)
	assert.Equal(t, "group2", groupID, "minimum met but AutoAdvance=false")
}

func TestComputeCurrentGroupForObjectives_AllObjectivesCompleteAlwaysAdvances(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{ID: "group1", CompletionType: models.CompletionAll, AutoAdvance: true, ObjectiveIDs: []string{"obj1"}},
			{
				ID:              "group2",
				CompletionType:  models.CompletionMinimum,
				MinimumRequired: 2,
				AutoAdvance:     false,
				ObjectiveIDs:    []string{"obj2", "obj3", "obj4"},
			},
			{ID: "group3", CompletionType: models.CompletionAll, AutoAdvance: true, ObjectiveIDs: []string{"obj5"}},
		},
	}

	groupID := navigation.ComputeCurrentGroupForObjectives(
		structure, []string{"obj1", "obj2", "obj3", "obj4"}, nil,
	)
	assert.Equal(t, "group3", groupID, "100% complete overrides AutoAdvance=false")
}

func TestComputeCurrentGroupForObjectives_NoVisibleGroups(t *testing.T) {
	structure := &models.GameStructure{ID: "root", IsRoot: true, SubGroups: []models.GameStructure{}}

	groupID := navigation.ComputeCurrentGroupForObjectives(structure, []string{}, nil)
	assert.Empty(t, groupID)
}

func TestComputeCurrentGroupForObjectives_SkippedGroupAdvances(t *testing.T) {
	structure := &models.GameStructure{
		ID:     "root",
		IsRoot: true,
		SubGroups: []models.GameStructure{
			{ID: "group1", CompletionType: models.CompletionAll, AutoAdvance: true, ObjectiveIDs: []string{"obj1"}},
			{
				ID:              "group2",
				CompletionType:  models.CompletionMinimum,
				MinimumRequired: 2,
				AutoAdvance:     false,
				ObjectiveIDs:    []string{"obj2", "obj3", "obj4"},
			},
			{ID: "group3", CompletionType: models.CompletionAll, AutoAdvance: true, ObjectiveIDs: []string{"obj5"}},
		},
	}

	completed := []string{"obj1", "obj2", "obj3"}

	groupID := navigation.ComputeCurrentGroupForObjectives(structure, completed, nil)
	assert.Equal(t, "group2", groupID, "stays without skip")

	groupID = navigation.ComputeCurrentGroupForObjectives(structure, completed, []string{"group2"})
	assert.Equal(t, "group3", groupID, "advances when group2 skipped")
}

func TestComputeCurrentGroupForObjectives_SecretGroupsNeverCurrent(t *testing.T) {
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
				ObjectiveIDs:   []string{"secretobj1", "secretobj2"},
			},
			{
				ID:             "group1",
				Name:           "Group 1",
				Color:          "blue",
				Routing:        models.RouteStrategyOrdered,
				CompletionType: models.CompletionAll,
				AutoAdvance:    true,
				ObjectiveIDs:   []string{"obj1", "obj2"},
			},
		},
	}

	currentGroupID := navigation.ComputeCurrentGroupForObjectives(structure, []string{}, nil)
	assert.Equal(t, "group1", currentGroupID, "should skip secret groups and start at group1")

	currentGroupID = navigation.ComputeCurrentGroupForObjectives(structure, []string{"secretobj1"}, nil)
	assert.Equal(t, "group1", currentGroupID, "completing secret objectives should not affect current group")
}

// === FindGroupContainingObjective Tests ===

func TestFindGroupContainingObjective_FirstGroup(t *testing.T) {
	structure := makeObjectiveTestStructure()
	found := navigation.FindGroupContainingObjective(structure, "obj1")
	assert.NotNil(t, found)
	assert.Equal(t, "group1", found.ID)
}

func TestFindGroupContainingObjective_SecondGroup(t *testing.T) {
	structure := makeObjectiveTestStructure()
	found := navigation.FindGroupContainingObjective(structure, "obj4")
	assert.NotNil(t, found)
	assert.Equal(t, "group2", found.ID)
}

func TestFindGroupContainingObjective_ThirdGroup(t *testing.T) {
	structure := makeObjectiveTestStructure()
	found := navigation.FindGroupContainingObjective(structure, "obj6")
	assert.NotNil(t, found)
	assert.Equal(t, "group3", found.ID)
}

func TestFindGroupContainingObjective_NotFound(t *testing.T) {
	structure := makeObjectiveTestStructure()
	found := navigation.FindGroupContainingObjective(structure, "nonexistent")
	assert.Nil(t, found)
}

func TestFindGroupContainingObjective_NilStructure(t *testing.T) {
	found := navigation.FindGroupContainingObjective(nil, "obj1")
	assert.Nil(t, found)
}
