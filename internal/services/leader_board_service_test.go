package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
)

func setupLeaderboardService(t *testing.T) (*services.LeaderBoardService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	teamRepo := repositories.NewTeamRepository(dbc)

	leaderboardService := services.NewLeaderBoardService(teamRepo)

	return leaderboardService, cleanup
}

// Helper function to create test teams with various states.
func createTestTeams() []models.Team {
	baseTime := time.Now().Add(-time.Hour * 2)

	return []models.Team{
		{
			ID:         "team1",
			Code:       "T001",
			Name:       "Alpha Team",
			Points:     100,
			HasStarted: true,
			CheckIns: []models.CheckIn{
				{TimeIn: baseTime, TimeOut: baseTime.Add(time.Minute * 30)},
				{TimeIn: baseTime.Add(time.Hour), TimeOut: baseTime.Add(time.Hour).Add(time.Minute * 20)},
			},
		},
		{
			ID:           "team2",
			Code:         "T002",
			Name:         "Beta Team",
			Points:       150,
			HasStarted:   true,
			MustCheckOut: "location1",
			CheckIns: []models.CheckIn{
				{TimeIn: baseTime.Add(time.Minute * 10), TimeOut: baseTime.Add(time.Minute * 40)},
			},
		},
		{
			ID:         "team3",
			Code:       "T003",
			Name:       "Gamma Team",
			Points:     75,
			HasStarted: true,
			CheckIns: []models.CheckIn{
				{TimeIn: baseTime.Add(time.Minute * 5), TimeOut: baseTime.Add(time.Minute * 35)},
				{
					TimeIn:  baseTime.Add(time.Hour).Add(time.Minute * 5),
					TimeOut: baseTime.Add(time.Hour).Add(time.Minute * 25),
				},
				{TimeIn: baseTime.Add(time.Hour * 2), TimeOut: time.Time{}}, // Currently checked in
			},
		},
		{
			ID:         "team4",
			Code:       "T004",
			Name:       "Delta Team",
			Points:     200,
			HasStarted: false, // This team should be excluded from leaderboard
			CheckIns:   []models.CheckIn{},
		},
		{
			ID:         "team5",
			Code:       "T005",
			Name:       "Echo Team",
			Points:     125,
			HasStarted: true,
			CheckIns: []models.CheckIn{
				{TimeIn: baseTime.Add(time.Minute * 15), TimeOut: baseTime.Add(time.Minute * 45)},
				{
					TimeIn:  baseTime.Add(time.Hour).Add(time.Minute * 15),
					TimeOut: baseTime.Add(time.Hour).Add(time.Minute * 35),
				},
			},
		},
	}
}

func TestLeaderBoardService_GetLeaderBoardData(t *testing.T) {
	service, cleanup := setupLeaderboardService(t)
	defer cleanup()

	ctx := context.Background()
	teams := createTestTeams()
	locationCount := 3

	t.Run("RankByProgress", func(t *testing.T) {
		result, err := service.GetLeaderBoardData(ctx, teams, locationCount, "progress", "rank", "asc")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Should exclude team4 (not started)
		if len(result) != 4 {
			t.Errorf("Expected 4 teams, got %d", len(result))
		}

		// Check that teams are ranked by progress (check-in count)
		// Team3 has 3 check-ins, should be rank 1
		found := false
		for _, team := range result {
			if team.Code == "T003" && team.Rank == 1 {
				found = true
				if team.Progress != 3 {
					t.Errorf("Expected team T003 to have progress 3, got %d", team.Progress)
				}
				break
			}
		}
		if !found {
			t.Error("Expected team T003 to be ranked 1st")
		}
	})

	t.Run("RankByPoints", func(t *testing.T) {
		result, err := service.GetLeaderBoardData(ctx, teams, locationCount, "points", "rank", "asc")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Team4 excluded (not started), Team2 should be rank 1 (150 points)
		found := false
		for _, team := range result {
			if team.Code == "T002" && team.Rank == 1 {
				found = true
				if team.Points != 150 {
					t.Errorf("Expected team T002 to have 150 points, got %d", team.Points)
				}
				break
			}
		}
		if !found {
			t.Error("Expected team T002 to be ranked 1st by points")
		}
	})

	t.Run("RankByCompletion", func(t *testing.T) {
		result, err := service.GetLeaderBoardData(ctx, teams, locationCount, "completion", "rank", "asc")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Team3 has 3 check-ins == locationCount, should be finished and rank 1
		found := false
		for _, team := range result {
			if team.Code == "T003" && team.Rank == 1 {
				found = true
				if team.Status != services.StatusFinished {
					t.Errorf("Expected team T003 to have status finished, got %s", team.Status)
				}
				break
			}
		}
		if !found {
			t.Error("Expected team T003 to be ranked 1st by completion")
		}
	})

	t.Run("SortByName", func(t *testing.T) {
		result, err := service.GetLeaderBoardData(ctx, teams, locationCount, "progress", "name", "asc")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// First team should be "Alpha Team" when sorted alphabetically
		if result[0].Name != "Alpha Team" {
			t.Errorf("Expected first team to be 'Alpha Team', got '%s'", result[0].Name)
		}
	})

	t.Run("SortDescending", func(t *testing.T) {
		result, err := service.GetLeaderBoardData(ctx, teams, locationCount, "progress", "points", "desc")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// First team should have the highest points
		if result[0].Points != 150 {
			t.Errorf("Expected first team to have 150 points, got %d", result[0].Points)
		}
	})
}

func TestLeaderBoardService_TeamStatus(t *testing.T) {
	service, cleanup := setupLeaderboardService(t)
	defer cleanup()

	teams := createTestTeams()
	locationCount := 3

	testCases := []struct {
		teamCode       string
		expectedStatus services.TeamStatus
	}{
		{"T001", services.StatusTransit},  // Has check-ins but not finished
		{"T002", services.StatusOnsite},   // Has MustCheckOut
		{"T003", services.StatusFinished}, // Has 3 check-ins (== locationCount)
		{"T005", services.StatusTransit},  // Has check-ins but not finished
	}

	ctx := context.Background()
	result, err := service.GetLeaderBoardData(ctx, teams, locationCount, "progress", "rank", "asc")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	for _, tc := range testCases {
		found := false
		for _, team := range result {
			if team.Code == tc.teamCode {
				found = true
				if team.Status != tc.expectedStatus {
					t.Errorf("Expected team %s to have status %s, got %s", tc.teamCode, tc.expectedStatus, team.Status)
				}
				break
			}
		}
		if !found {
			t.Errorf("Team %s not found in results", tc.teamCode)
		}
	}
}

func TestLeaderBoardService_ParseFunctions(t *testing.T) {
	testCases := []struct {
		name     string
		function func(string) interface{}
		input    string
		expected interface{}
	}{
		{
			"ParseSortField_Valid",
			func(s string) interface{} { return services.ParseSortField(s) },
			"points",
			services.SortByPoints,
		},
		{
			"ParseSortField_Invalid",
			func(s string) interface{} { return services.ParseSortField(s) },
			"invalid",
			services.SortByRank,
		},
		{
			"ParseSortOrder_Asc",
			func(s string) interface{} { return services.ParseSortOrder(s) },
			"asc",
			services.SortAsc,
		},
		{
			"ParseSortOrder_Desc",
			func(s string) interface{} { return services.ParseSortOrder(s) },
			"desc",
			services.SortDesc,
		},
		{
			"ParseSortOrder_Invalid",
			func(s string) interface{} { return services.ParseSortOrder(s) },
			"invalid",
			services.SortAsc,
		},
		{
			"ParseRankingScheme_Progress",
			func(s string) interface{} { return services.ParseRankingScheme(s) },
			"progress",
			services.RankByProgress,
		},
		{
			"ParseRankingScheme_Points",
			func(s string) interface{} { return services.ParseRankingScheme(s) },
			"points",
			services.RankByPoints,
		},
		{
			"ParseRankingScheme_Invalid",
			func(s string) interface{} { return services.ParseRankingScheme(s) },
			"invalid",
			services.RankByProgress,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tc.function(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %v, got %v", tc.expected, result)
			}
		})
	}
}

func TestLeaderBoardService_GetSupportedValues(t *testing.T) {
	service, cleanup := setupLeaderboardService(t)
	defer cleanup()

	t.Run("GetSupportedRankingSchemes", func(t *testing.T) {
		schemes := service.GetSupportedRankingSchemes()
		expected := []string{"progress", "points", "completion", "time_to_first", "time_to_last"}

		if len(schemes) != len(expected) {
			t.Errorf("Expected %d schemes, got %d", len(expected), len(schemes))
		}

		for i, scheme := range expected {
			if schemes[i] != scheme {
				t.Errorf("Expected scheme %s at index %d, got %s", scheme, i, schemes[i])
			}
		}
	})

	t.Run("GetSupportedSortFields", func(t *testing.T) {
		fields := service.GetSupportedSortFields()
		expected := []string{"rank", "code", "name", "points", "last_seen", "progress", "status"}

		if len(fields) != len(expected) {
			t.Errorf("Expected %d fields, got %d", len(expected), len(fields))
		}

		for i, field := range expected {
			if fields[i] != field {
				t.Errorf("Expected field %s at index %d, got %s", field, i, fields[i])
			}
		}
	})

	t.Run("GetSupportedSortOrders", func(t *testing.T) {
		orders := service.GetSupportedSortOrders()
		expected := []string{"asc", "desc"}

		if len(orders) != len(expected) {
			t.Errorf("Expected %d orders, got %d", len(expected), len(orders))
		}

		for i, order := range expected {
			if orders[i] != order {
				t.Errorf("Expected order %s at index %d, got %s", order, i, orders[i])
			}
		}
	})
}

func TestLeaderBoardService_GetDefaultSortForRankingScheme(t *testing.T) {
	service, cleanup := setupLeaderboardService(t)
	defer cleanup()

	testCases := []struct {
		scheme   string
		expected string
	}{
		{"points", "points"},
		{"progress", "progress"},
		{"completion", "rank"},
		{"invalid", "progress"}, // Invalid schemes default to progress
	}

	for _, tc := range testCases {
		t.Run("Scheme_"+tc.scheme, func(t *testing.T) {
			result := service.GetDefaultSortForRankingScheme(tc.scheme)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

func TestLeaderBoardService_LastSeenCalculation(t *testing.T) {
	service, cleanup := setupLeaderboardService(t)
	defer cleanup()

	ctx := context.Background()

	// Create a team with specific check-in times
	baseTime := time.Now().Add(-time.Hour * 3)
	teams := []models.Team{
		{
			ID:         "team1",
			Code:       "T001",
			Name:       "Test Team",
			Points:     100,
			HasStarted: true,
			CheckIns: []models.CheckIn{
				{TimeIn: baseTime.Add(time.Hour), TimeOut: baseTime.Add(time.Hour).Add(time.Minute * 30)},
				{TimeIn: baseTime.Add(time.Hour * 2), TimeOut: time.Time{}}, // Currently checked in, no TimeOut
			},
		},
	}
	// Set the UpdatedAt field after creating the struct
	teams[0].UpdatedAt = baseTime

	result, err := service.GetLeaderBoardData(ctx, teams, 3, "progress", "rank", "asc")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 team, got %d", len(result))
	}

	// LastSeen should be the latest check-in TimeIn (since TimeOut is zero)
	expectedLastSeen := baseTime.Add(time.Hour * 2)
	if !result[0].LastSeen.Equal(expectedLastSeen) {
		t.Errorf("Expected LastSeen to be %v, got %v", expectedLastSeen, result[0].LastSeen)
	}
}

func TestLeaderBoardService_EmptyTeamsList(t *testing.T) {
	service, cleanup := setupLeaderboardService(t)
	defer cleanup()

	ctx := context.Background()
	teams := []models.Team{}

	result, err := service.GetLeaderBoardData(ctx, teams, 3, "progress", "rank", "asc")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d teams", len(result))
	}
}

func TestLeaderBoardService_TieBreaker(t *testing.T) {
	service, cleanup := setupLeaderboardService(t)
	defer cleanup()

	ctx := context.Background()
	baseTime := time.Now().Add(-time.Hour)

	// Create teams with same progress but different last seen times
	teams := []models.Team{
		{
			ID:         "team1",
			Code:       "T001",
			Name:       "Team A",
			Points:     100,
			HasStarted: true,
			CheckIns: []models.CheckIn{
				{TimeIn: baseTime.Add(time.Minute * 10), TimeOut: baseTime.Add(time.Minute * 40)}, // Later check-in
			},
		},
		{
			ID:         "team2",
			Code:       "T002",
			Name:       "Team B",
			Points:     100,
			HasStarted: true,
			CheckIns: []models.CheckIn{
				{TimeIn: baseTime, TimeOut: baseTime.Add(time.Minute * 30)}, // Earlier check-in
			},
		},
	}

	result, err := service.GetLeaderBoardData(ctx, teams, 3, "progress", "rank", "asc")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 teams, got %d", len(result))
	}

	// Team with earlier last seen time should be ranked higher (rank 1)
	if result[0].Code != "T002" {
		t.Errorf("Expected T002 to be ranked first (earlier last seen), got %s", result[0].Code)
	}

	if result[1].Code != "T001" {
		t.Errorf("Expected T001 to be ranked second (later last seen), got %s", result[1].Code)
	}

	// Ensure no tied ranks
	if result[0].Rank == result[1].Rank {
		t.Error("Expected no tied ranks, but ranks are the same")
	}
}
