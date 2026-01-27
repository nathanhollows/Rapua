package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v6/db"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Default group constants used in grouping logic.
	defaultUngroupedName  = "Other"
	defaultUngroupedColor = "base-content"

	// Test fixture base time for deterministic timestamps.
	baseTime = "2024-01-15T10:00:00Z"
)

func setupTeamsService(t *testing.T) (services.TeamService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)

	checkinRepo := repositories.NewCheckInRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	teamRepo := repositories.NewTeamRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	creditRepo := repositories.NewCreditRepository(dbc)
	teamStartLogRepo := repositories.NewTeamStartLogRepository(dbc)
	creditService := services.NewCreditService(transactor, creditRepo, teamStartLogRepo, nil)
	teamService := services.NewTeamService(
		transactor,
		teamRepo,
		checkinRepo,
		creditService,
		blockStateRepo,
		locationRepo,
	)

	return *teamService, cleanup
}

// getBaseTime returns a fixed time for deterministic testing.
func getBaseTime() time.Time {
	t, _ := time.Parse(time.RFC3339, baseTime)
	return t
}

func TestTeamService_AddTeams(t *testing.T) {
	teamService, cleanup := setupTeamsService(t)
	defer cleanup()

	tests := []struct {
		name       string
		instanceID string
		count      int
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "add teams successfully",
			instanceID: gofakeit.UUID(),
			count:      3,
			wantCount:  3,
			wantErr:    false,
		},
		{
			name:       "add single team",
			instanceID: gofakeit.UUID(),
			count:      1,
			wantCount:  1,
			wantErr:    false,
		},
		{
			name:       "add many teams",
			instanceID: gofakeit.UUID(),
			count:      10,
			wantCount:  10,
			wantErr:    false,
		},
		{
			name:       "zero count should create no teams",
			instanceID: gofakeit.UUID(),
			count:      0,
			wantCount:  0,
			wantErr:    false,
		},
		{
			name:       "negative count should create no teams",
			instanceID: gofakeit.UUID(),
			count:      -1,
			wantCount:  0,
			wantErr:    false,
		},
		{
			name:       "empty instance ID still creates teams",
			instanceID: "",
			count:      3,
			wantCount:  3,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := teamService.AddTeams(context.Background(), tt.instanceID, tt.count)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, result, tt.wantCount)

			// Verify each team has proper values
			for _, team := range result {
				assert.NotEmpty(t, team.Code, "team code should not be empty")
				assert.Equal(t, tt.instanceID, team.InstanceID, "instance ID should match")
			}
		})
	}
}

func TestTeamService_FindAll(t *testing.T) {
	teamService, cleanup := setupTeamsService(t)
	defer cleanup()

	tests := []struct {
		name       string
		setupTeams int
		instanceID string
		wantCount  int
		wantErr    bool
	}{
		{
			name:       "find all teams for instance",
			setupTeams: 5,
			instanceID: gofakeit.UUID(),
			wantCount:  5,
			wantErr:    false,
		},
		{
			name:       "find no teams for empty instance",
			setupTeams: 0,
			instanceID: gofakeit.UUID(),
			wantCount:  0,
			wantErr:    false,
		},
		{
			name:       "find teams with special characters in instance ID",
			setupTeams: 3,
			instanceID: "test-instance-" + gofakeit.LetterN(10),
			wantCount:  3,
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: Create teams for this instance
			if tt.setupTeams > 0 {
				teams, err := teamService.AddTeams(context.Background(), tt.instanceID, tt.setupTeams)
				require.NoError(t, err)
				require.Len(t, teams, tt.setupTeams, "setup should create expected number of teams")
			}

			// Test: Find all teams
			result, err := teamService.FindAll(context.Background(), tt.instanceID)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Len(t, result, tt.wantCount)

			// Verify all teams belong to correct instance
			for _, team := range result {
				assert.Equal(t, tt.instanceID, team.InstanceID)
			}
		})
	}
}

func TestTeamService_FindTeamByCode(t *testing.T) {
	teamService, cleanup := setupTeamsService(t)
	defer cleanup()

	t.Run("find existing team by code", func(t *testing.T) {
		instanceID := gofakeit.UUID()
		teams, err := teamService.AddTeams(context.Background(), instanceID, 1)
		require.NoError(t, err)
		require.Len(t, teams, 1)

		team, err := teamService.GetTeamByCode(context.Background(), teams[0].Code)
		require.NoError(t, err)
		assert.Equal(t, teams[0].Code, team.Code)
		assert.Equal(t, instanceID, team.InstanceID)
	})

	t.Run("return error for non-existent code", func(t *testing.T) {
		nonExistentCode := gofakeit.LetterN(6)

		team, err := teamService.GetTeamByCode(context.Background(), nonExistentCode)
		require.Error(t, err, "should return error for non-existent code")
		assert.Nil(t, team, "team should be nil when not found")
	})

	t.Run("return error for empty code", func(t *testing.T) {
		team, err := teamService.GetTeamByCode(context.Background(), "")
		require.Error(t, err, "should return error for empty code")
		assert.Nil(t, team, "team should be nil for empty code")
	})

	t.Run("find correct team among multiple teams", func(t *testing.T) {
		instanceID := gofakeit.UUID()
		teams, err := teamService.AddTeams(context.Background(), instanceID, 5)
		require.NoError(t, err)
		require.Len(t, teams, 5)

		// Test finding each team
		for _, expectedTeam := range teams {
			foundTeam, err := teamService.GetTeamByCode(context.Background(), expectedTeam.Code)
			require.NoError(t, err)
			assert.Equal(t, expectedTeam.Code, foundTeam.Code)
			assert.Equal(t, expectedTeam.InstanceID, foundTeam.InstanceID)
		}
	})
}

func TestTeamService_BuildLocationGroupMap(t *testing.T) {
	teamService, cleanup := setupTeamsService(t)
	defer cleanup()

	tests := []struct {
		name      string
		structure *models.GameStructure
		want      map[string]services.LocationGroupInfo
	}{
		{
			name: "empty root structure",
			structure: &models.GameStructure{
				IsRoot:      true,
				LocationIDs: []string{},
				SubGroups:   []models.GameStructure{},
			},
			want: map[string]services.LocationGroupInfo{},
		},
		{
			name: "root with locations only",
			structure: &models.GameStructure{
				IsRoot:      true,
				LocationIDs: []string{"loc1", "loc2"},
				SubGroups:   []models.GameStructure{},
			},
			want: map[string]services.LocationGroupInfo{},
		},
		{
			name: "single level with one group",
			structure: &models.GameStructure{
				IsRoot:      true,
				LocationIDs: []string{},
				SubGroups: []models.GameStructure{
					{
						Name:        "Museum Tour",
						Color:       "primary",
						LocationIDs: []string{"loc1", "loc2"},
					},
				},
			},
			want: map[string]services.LocationGroupInfo{
				"loc1": {GroupName: "Museum Tour", GroupColor: "primary"},
				"loc2": {GroupName: "Museum Tour", GroupColor: "primary"},
			},
		},
		{
			name: "nested groups",
			structure: &models.GameStructure{
				IsRoot:      true,
				LocationIDs: []string{},
				SubGroups: []models.GameStructure{
					{
						Name:        "Zone A",
						Color:       "primary",
						LocationIDs: []string{"loc1"},
						SubGroups: []models.GameStructure{
							{
								Name:        "Subzone A1",
								Color:       "secondary",
								LocationIDs: []string{"loc2", "loc3"},
							},
						},
					},
					{
						Name:        "Zone B",
						Color:       "accent",
						LocationIDs: []string{"loc4"},
					},
				},
			},
			want: map[string]services.LocationGroupInfo{
				"loc1": {GroupName: "Zone A", GroupColor: "primary"},
				"loc2": {GroupName: "Subzone A1", GroupColor: "secondary"},
				"loc3": {GroupName: "Subzone A1", GroupColor: "secondary"},
				"loc4": {GroupName: "Zone B", GroupColor: "accent"},
			},
		},
		{
			name: "multiple locations in same group",
			structure: &models.GameStructure{
				IsRoot:      true,
				LocationIDs: []string{},
				SubGroups: []models.GameStructure{
					{
						Name:        gofakeit.Company(),
						Color:       "primary",
						LocationIDs: []string{"loc1", "loc2", "loc3", "loc4", "loc5"},
					},
				},
			},
			want: map[string]services.LocationGroupInfo{
				"loc1": {GroupName: gofakeit.Company(), GroupColor: "primary"},
				"loc2": {GroupName: gofakeit.Company(), GroupColor: "primary"},
				"loc3": {GroupName: gofakeit.Company(), GroupColor: "primary"},
				"loc4": {GroupName: gofakeit.Company(), GroupColor: "primary"},
				"loc5": {GroupName: gofakeit.Company(), GroupColor: "primary"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Handle dynamic gofakeit values for specific test
			if tt.name == "multiple locations in same group" {
				groupName := gofakeit.Company()
				tt.structure.SubGroups[0].Name = groupName
				for key := range tt.want {
					tt.want[key] = services.LocationGroupInfo{
						GroupName:  groupName,
						GroupColor: "primary",
					}
				}
			}

			got := teamService.BuildLocationGroupMap(tt.structure)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTeamService_BuildGroupOrder(t *testing.T) {
	teamService, cleanup := setupTeamsService(t)
	defer cleanup()

	tests := []struct {
		name      string
		structure *models.GameStructure
		want      map[string]int
	}{
		{
			name: "empty root structure",
			structure: &models.GameStructure{
				IsRoot:    true,
				SubGroups: []models.GameStructure{},
			},
			want: map[string]int{},
		},
		{
			name: "single level groups",
			structure: &models.GameStructure{
				IsRoot: true,
				SubGroups: []models.GameStructure{
					{Name: "First Group"},
					{Name: "Second Group"},
					{Name: "Third Group"},
				},
			},
			want: map[string]int{
				"First Group":  0,
				"Second Group": 1,
				"Third Group":  2,
			},
		},
		{
			name: "nested groups preserve depth-first order",
			structure: &models.GameStructure{
				IsRoot: true,
				SubGroups: []models.GameStructure{
					{
						Name: "Zone A",
						SubGroups: []models.GameStructure{
							{Name: "Subzone A1"},
							{Name: "Subzone A2"},
						},
					},
					{Name: "Zone B"},
					{
						Name: "Zone C",
						SubGroups: []models.GameStructure{
							{Name: "Subzone C1"},
						},
					},
				},
			},
			want: map[string]int{
				"Zone A":     0,
				"Subzone A1": 1,
				"Subzone A2": 2,
				"Zone B":     3,
				"Zone C":     4,
				"Subzone C1": 5,
			},
		},
		{
			name: "deeply nested structure",
			structure: &models.GameStructure{
				IsRoot: true,
				SubGroups: []models.GameStructure{
					{
						Name: "Level 1",
						SubGroups: []models.GameStructure{
							{
								Name: "Level 2",
								SubGroups: []models.GameStructure{
									{Name: "Level 3"},
								},
							},
						},
					},
				},
			},
			want: map[string]int{
				"Level 1": 0,
				"Level 2": 1,
				"Level 3": 2,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := teamService.BuildGroupOrder(tt.structure)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTeamService_GroupCheckInsByGroup(t *testing.T) {
	teamService, cleanup := setupTeamsService(t)
	defer cleanup()

	// Helper to create check-ins with deterministic times relative to base time
	makeCheckIn := func(locationID string, minutesAfterBase int, mustCheckOut bool) models.CheckIn {
		checkIn := models.CheckIn{
			LocationID:   locationID,
			MustCheckOut: mustCheckOut,
			Location: models.Location{
				ID: locationID,
			},
		}
		// Use deterministic base time instead of time.Now()
		checkIn.CreatedAt = getBaseTime().Add(time.Duration(minutesAfterBase) * time.Minute)
		return checkIn
	}

	tests := []struct {
		name           string
		checkIns       []models.CheckIn
		locationGroups map[string]services.LocationGroupInfo
		groupOrder     map[string]int
		wantGroupCount int
		validate       func(t *testing.T, result []services.GroupedCheckIns)
	}{
		{
			name:           "empty check-ins",
			checkIns:       []models.CheckIn{},
			locationGroups: map[string]services.LocationGroupInfo{},
			groupOrder:     map[string]int{},
			wantGroupCount: 0,
		},
		{
			name: "excludes check-ins with MustCheckOut true",
			checkIns: []models.CheckIn{
				makeCheckIn("loc1", 10, true),
				makeCheckIn("loc2", 5, false),
			},
			locationGroups: map[string]services.LocationGroupInfo{
				"loc1": {GroupName: "Group A", GroupColor: "primary"},
				"loc2": {GroupName: "Group A", GroupColor: "primary"},
			},
			groupOrder:     map[string]int{"Group A": 0},
			wantGroupCount: 1,
			validate: func(t *testing.T, result []services.GroupedCheckIns) {
				assert.Len(t, result[0].CheckIns, 1, "should only have one check-in (MustCheckOut excluded)")
				assert.Equal(t, "loc2", result[0].CheckIns[0].LocationID)
			},
		},
		{
			name: "groups by location group",
			checkIns: []models.CheckIn{
				makeCheckIn("loc1", 10, false),
				makeCheckIn("loc2", 5, false),
				makeCheckIn("loc3", 3, false),
			},
			locationGroups: map[string]services.LocationGroupInfo{
				"loc1": {GroupName: "Group A", GroupColor: "primary"},
				"loc2": {GroupName: "Group B", GroupColor: "secondary"},
				"loc3": {GroupName: "Group A", GroupColor: "primary"},
			},
			groupOrder:     map[string]int{"Group A": 0, "Group B": 1},
			wantGroupCount: 2,
			validate: func(t *testing.T, result []services.GroupedCheckIns) {
				assert.Equal(t, "Group A", result[0].GroupInfo.GroupName)
				assert.Len(t, result[0].CheckIns, 2, "Group A should have 2 check-ins")
				assert.Equal(t, "Group B", result[1].GroupInfo.GroupName)
				assert.Len(t, result[1].CheckIns, 1, "Group B should have 1 check-in")
			},
		},
		{
			name: "sorts check-ins within group by creation time",
			checkIns: []models.CheckIn{
				makeCheckIn("loc1", 30, false), // Latest
				makeCheckIn("loc2", 10, false), // Earliest
				makeCheckIn("loc3", 20, false), // Middle
			},
			locationGroups: map[string]services.LocationGroupInfo{
				"loc1": {GroupName: "Group A", GroupColor: "primary"},
				"loc2": {GroupName: "Group A", GroupColor: "primary"},
				"loc3": {GroupName: "Group A", GroupColor: "primary"},
			},
			groupOrder:     map[string]int{"Group A": 0},
			wantGroupCount: 1,
			validate: func(t *testing.T, result []services.GroupedCheckIns) {
				assert.Len(t, result[0].CheckIns, 3)
				// Should be sorted by creation time (reverse chronological - latest to earliest)
				assert.Equal(t, "loc1", result[0].CheckIns[0].LocationID, "latest should be first")
				assert.Equal(t, "loc3", result[0].CheckIns[1].LocationID, "middle should be second")
				assert.Equal(t, "loc2", result[0].CheckIns[2].LocationID, "earliest should be last")
			},
		},
		{
			name: "sorts groups by group order",
			checkIns: []models.CheckIn{
				makeCheckIn("loc1", 10, false),
				makeCheckIn("loc2", 10, false),
				makeCheckIn("loc3", 10, false),
			},
			locationGroups: map[string]services.LocationGroupInfo{
				"loc1": {GroupName: "Group C", GroupColor: "accent"},
				"loc2": {GroupName: "Group A", GroupColor: "primary"},
				"loc3": {GroupName: "Group B", GroupColor: "secondary"},
			},
			groupOrder:     map[string]int{"Group A": 0, "Group B": 1, "Group C": 2},
			wantGroupCount: 3,
			validate: func(t *testing.T, result []services.GroupedCheckIns) {
				assert.Equal(t, "Group A", result[0].GroupInfo.GroupName, "first by order")
				assert.Equal(t, "Group B", result[1].GroupInfo.GroupName, "second by order")
				assert.Equal(t, "Group C", result[2].GroupInfo.GroupName, "third by order")
			},
		},
		{
			name: "ungrouped locations added as Other group at end",
			checkIns: []models.CheckIn{
				makeCheckIn("loc1", 10, false),
				makeCheckIn("loc2", 5, false),
			},
			locationGroups: map[string]services.LocationGroupInfo{
				"loc1": {GroupName: "Group A", GroupColor: "primary"},
				// loc2 has no group mapping
			},
			groupOrder:     map[string]int{"Group A": 0},
			wantGroupCount: 2,
			validate: func(t *testing.T, result []services.GroupedCheckIns) {
				assert.Equal(t, "Group A", result[0].GroupInfo.GroupName)
				assert.Equal(t, defaultUngroupedName, result[1].GroupInfo.GroupName, "ungrouped locations at end")
				assert.Equal(t, defaultUngroupedColor, result[1].GroupInfo.GroupColor)
				assert.Len(t, result[1].CheckIns, 1)
				assert.Equal(t, "loc2", result[1].CheckIns[0].LocationID)
			},
		},
		{
			name: "groups without order come after groups with order",
			checkIns: []models.CheckIn{
				makeCheckIn("loc1", 10, false),
				makeCheckIn("loc2", 10, false),
				makeCheckIn("loc3", 10, false),
			},
			locationGroups: map[string]services.LocationGroupInfo{
				"loc1": {GroupName: "Group A", GroupColor: "primary"},
				"loc2": {GroupName: "Group B", GroupColor: "secondary"},
				"loc3": {GroupName: "Group C", GroupColor: "accent"},
			},
			groupOrder:     map[string]int{"Group A": 0}, // Only Group A has order
			wantGroupCount: 3,
			validate: func(t *testing.T, result []services.GroupedCheckIns) {
				assert.Equal(t, "Group A", result[0].GroupInfo.GroupName, "ordered group comes first")
				// Groups B and C don't have order, so order between them is stable but after A
			},
		},
		{
			name: "handles all check-ins with MustCheckOut true",
			checkIns: []models.CheckIn{
				makeCheckIn("loc1", 10, true),
				makeCheckIn("loc2", 5, true),
			},
			locationGroups: map[string]services.LocationGroupInfo{
				"loc1": {GroupName: "Group A", GroupColor: "primary"},
				"loc2": {GroupName: "Group A", GroupColor: "primary"},
			},
			groupOrder:     map[string]int{"Group A": 0},
			wantGroupCount: 0,
			validate: func(t *testing.T, result []services.GroupedCheckIns) {
				assert.Empty(t, result, "should have no groups when all check-ins require checkout")
			},
		},
		{
			name: "handles mixed grouped and ungrouped locations",
			checkIns: []models.CheckIn{
				makeCheckIn("loc1", 5, false),
				makeCheckIn("loc2", 10, false),
				makeCheckIn("loc3", 15, false),
				makeCheckIn("loc4", 20, false),
			},
			locationGroups: map[string]services.LocationGroupInfo{
				"loc1": {GroupName: "Group A", GroupColor: "primary"},
				"loc3": {GroupName: "Group B", GroupColor: "secondary"},
				// loc2 and loc4 are ungrouped
			},
			groupOrder:     map[string]int{"Group A": 0, "Group B": 1},
			wantGroupCount: 3,
			validate: func(t *testing.T, result []services.GroupedCheckIns) {
				assert.Equal(t, "Group A", result[0].GroupInfo.GroupName)
				assert.Len(t, result[0].CheckIns, 1)
				assert.Equal(t, "Group B", result[1].GroupInfo.GroupName)
				assert.Len(t, result[1].CheckIns, 1)
				assert.Equal(t, defaultUngroupedName, result[2].GroupInfo.GroupName)
				assert.Len(t, result[2].CheckIns, 2, "should have 2 ungrouped check-ins")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := teamService.GroupCheckInsByGroup(tt.checkIns, tt.locationGroups, tt.groupOrder)
			assert.Len(t, got, tt.wantGroupCount)
			if tt.validate != nil {
				tt.validate(t, got)
			}
		})
	}
}
