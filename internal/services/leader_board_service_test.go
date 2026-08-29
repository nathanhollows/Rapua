package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
)

func setupLeaderboardService(t *testing.T) *services.LeaderBoardService {
	t.Helper()
	return services.NewLeaderBoardService()
}

// completedCounts maps run codes to completed-objective counts, mirroring what
// production derives from ObjectiveContextCompletion rows.
func completedCounts() map[string]int {
	return map[string]int{"T001": 2, "T002": 1, "T003": 3, "T005": 2}
}

// Helper function to create test teams with various states.
func createTestTeams() []models.Run {
	return []models.Run{
		{
			ID:         "team1",
			Code:       "T001",
			Name:       "Alpha Team",
			Points:     100,
			HasStarted: true,
		},
		{
			ID:         "team2",
			Code:       "T002",
			Name:       "Beta Team",
			Points:     150,
			HasStarted: true,
		},
		{
			ID:         "team3",
			Code:       "T003",
			Name:       "Gamma Team",
			Points:     75,
			HasStarted: true,
		},
		{
			ID:         "team4",
			Code:       "T004",
			Name:       "Delta Team",
			Points:     200,
			HasStarted: false, // This team should be excluded from leaderboard
		},
		{
			ID:         "team5",
			Code:       "T005",
			Name:       "Echo Team",
			Points:     125,
			HasStarted: true,
		},
	}
}

func TestLeaderBoardService_GetLeaderBoardData(t *testing.T) {
	service := setupLeaderboardService(t)

	ctx := context.Background()
	teams := createTestTeams()
	locationCount := 3

	t.Run("RankByProgress", func(t *testing.T) {
		result, err := service.GetLeaderBoardData(
			ctx, teams, locationCount, completedCounts(), "progress", "rank", "asc",
		)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Should exclude team4 (not started)
		if len(result) != 4 {
			t.Errorf("Expected 4 teams, got %d", len(result))
		}

		// Check that teams are ranked by progress (completed objective count).
		// Team3 has completed 3 objectives, should be rank 1.
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
		result, err := service.GetLeaderBoardData(
			ctx, teams, locationCount, completedCounts(), "points", "rank", "asc",
		)
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
		result, err := service.GetLeaderBoardData(
			ctx, teams, locationCount, completedCounts(), "completion", "rank", "asc",
		)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// Team3 has completed 3 objectives == locationCount, should be finished and rank 1.
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
		result, err := service.GetLeaderBoardData(
			ctx, teams, locationCount, completedCounts(), "progress", "name", "asc",
		)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		// First team should be "Alpha Team" when sorted alphabetically
		if result[0].Name != "Alpha Team" {
			t.Errorf("Expected first team to be 'Alpha Team', got '%s'", result[0].Name)
		}
	})

	t.Run("SortDescending", func(t *testing.T) {
		result, err := service.GetLeaderBoardData(
			ctx, teams, locationCount, completedCounts(), "progress", "points", "desc",
		)
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
	service := setupLeaderboardService(t)

	teams := createTestTeams()
	locationCount := 3

	testCases := []struct {
		runCode        string
		expectedStatus services.TeamStatus
	}{
		{"T001", services.StatusTransit},  // Has 2 of 3 objectives complete.
		{"T002", services.StatusTransit},  // Has 1 of 3 objectives complete.
		{"T003", services.StatusFinished}, // Has 3 of 3 objectives complete.
		{"T005", services.StatusTransit},  // Has 2 of 3 objectives complete.
	}

	ctx := context.Background()
	result, err := service.GetLeaderBoardData(
		ctx, teams, locationCount, completedCounts(), "progress", "rank", "asc",
	)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	for _, tc := range testCases {
		found := false
		for _, team := range result {
			if team.Code == tc.runCode {
				found = true
				if team.Status != tc.expectedStatus {
					t.Errorf("Expected team %s to have status %s, got %s", tc.runCode, tc.expectedStatus, team.Status)
				}
				break
			}
		}
		if !found {
			t.Errorf("Team %s not found in results", tc.runCode)
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
	service := setupLeaderboardService(t)

	t.Run("GetSupportedRankingSchemes", func(t *testing.T) {
		schemes := service.GetSupportedRankingSchemes()
		expected := []string{"progress", "points", "completion"}

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
	service := setupLeaderboardService(t)

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
	service := setupLeaderboardService(t)

	ctx := context.Background()

	// LastSeen mirrors team.UpdatedAt: check-in history no longer feeds it.
	// UpdatedAt is promoted from the unexported baseModel, so it cannot be set
	// in a composite literal.
	baseTime := time.Now().Add(-time.Hour * 3)
	teams := []models.Run{
		{
			ID:         "team1",
			Code:       "T001",
			Name:       "Test Team",
			Points:     100,
			HasStarted: true,
		},
	}
	teams[0].UpdatedAt = baseTime

	result, err := service.GetLeaderBoardData(
		ctx, teams, 3, completedCounts(), "progress", "rank", "asc",
	)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("Expected 1 team, got %d", len(result))
	}

	if !result[0].LastSeen.Equal(baseTime) {
		t.Errorf("Expected LastSeen to be %v, got %v", baseTime, result[0].LastSeen)
	}
}

func TestLeaderBoardService_EmptyTeamsList(t *testing.T) {
	service := setupLeaderboardService(t)

	ctx := context.Background()
	teams := []models.Run{}

	result, err := service.GetLeaderBoardData(
		ctx, teams, 3, completedCounts(), "progress", "rank", "asc",
	)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(result) != 0 {
		t.Errorf("Expected empty result, got %d teams", len(result))
	}
}

func TestLeaderBoardService_TieBreaker(t *testing.T) {
	service := setupLeaderboardService(t)

	ctx := context.Background()
	baseTime := time.Now().Add(-time.Hour)

	// Create teams with same progress but different UpdatedAt (LastSeen) times.
	// UpdatedAt is promoted from the unexported baseModel, so it cannot be set
	// in a composite literal.
	teams := []models.Run{
		{
			ID:         "team1",
			Code:       "T001",
			Name:       "Team A",
			Points:     100,
			HasStarted: true,
		},
		{
			ID:         "team2",
			Code:       "T005",
			Name:       "Team B",
			Points:     100,
			HasStarted: true,
		},
	}
	// T001 and T005 both map to 2 completed objectives in completedCounts(), so
	// the tie must be broken by LastSeen alone.
	teams[0].UpdatedAt = baseTime.Add(time.Minute * 10) // Later last seen.
	teams[1].UpdatedAt = baseTime                       // Earlier last seen.

	result, err := service.GetLeaderBoardData(
		ctx, teams, 3, completedCounts(), "progress", "rank", "asc",
	)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("Expected 2 teams, got %d", len(result))
	}

	// Team with earlier last seen time should be ranked higher (rank 1)
	if result[0].Code != "T005" {
		t.Errorf("Expected T005 to be ranked first (earlier last seen), got %s", result[0].Code)
	}

	if result[1].Code != "T001" {
		t.Errorf("Expected T001 to be ranked second (later last seen), got %s", result[1].Code)
	}

	// Ensure no tied ranks
	if result[0].Rank == result[1].Rank {
		t.Error("Expected no tied ranks, but ranks are the same")
	}
}
