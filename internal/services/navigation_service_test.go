package services_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupNavigationService(t *testing.T) (*services.NavigationService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	locationRepo := repositories.NewLocationRepository(dbc)
	teamRepo := repositories.NewTeamRepository(dbc)
	navigationService := services.NewNavigationService(locationRepo, teamRepo)

	return navigationService, cleanup
}

func createTestTeamWithInstance(t *testing.T, navMode models.RouteStrategy, maxNextLocs int) *models.Team {
	t.Helper()

	instanceID := gofakeit.UUID()

	// Create locations with simple marker IDs for testing
	locations := make([]models.Location, 3)
	for i := range 3 {
		locations[i] = models.Location{
			ID:         gofakeit.UUID(),
			Name:       gofakeit.Name(),
			InstanceID: instanceID,
			MarkerID:   fmt.Sprintf("MARKER%d", i), // Simple predictable marker IDs
			Points:     gofakeit.Number(10, 100),
			Order:      i, // 0, 1, 2 for ordered testing
		}
	}

	return &models.Team{
		ID:         gofakeit.UUID(),
		Code:       gofakeit.Word(),
		InstanceID: instanceID,
		CheckIns:   []models.CheckIn{}, // Start with no check-ins
		Instance: models.Instance{
			ID:        instanceID,
			Locations: locations,
			Settings: models.InstanceSettings{
				InstanceID:       instanceID,
				RouteStrategy:    navMode,
				MaxNextLocations: maxNextLocs,
			},
		},
	}
}

func TestNavigationService_IsValidLocation(t *testing.T) {
	service, cleanup := setupNavigationService(t)
	defer cleanup()

	t.Run("Valid location for free roam", func(t *testing.T) {
		team := createTestTeamWithInstance(t, models.RouteStrategyFreeRoam, 3)
		validMarkerID := "MARKER0" // First location

		valid, err := service.IsValidLocation(context.Background(), team, validMarkerID)
		require.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("Invalid location code", func(t *testing.T) {
		team := createTestTeamWithInstance(t, models.RouteStrategyFreeRoam, 3)

		valid, err := service.IsValidLocation(context.Background(), team, "INVALID")
		require.Error(t, err)
		assert.False(t, valid)
		assert.Contains(t, err.Error(), "not a valid next location")
	})

	t.Run("Ordered navigation - only first location valid", func(t *testing.T) {
		team := createTestTeamWithInstance(t, models.RouteStrategyOrdered, 1)

		// First location should be valid
		valid, err := service.IsValidLocation(context.Background(), team, "MARKER0")
		require.NoError(t, err)
		assert.True(t, valid)

		// Second location should be invalid
		valid, err = service.IsValidLocation(context.Background(), team, "MARKER1")
		require.Error(t, err)
		assert.False(t, valid)
	})

	t.Run("Team with missing instance", func(t *testing.T) {
		team := &models.Team{
			ID:       gofakeit.UUID(),
			Code:     gofakeit.Word(),
			Instance: models.Instance{}, // Empty instance
		}

		valid, err := service.IsValidLocation(context.Background(), team, "ANY")
		require.Error(t, err)
		assert.False(t, valid)
	})

	t.Run("Normalize marker ID - case insensitive and trimmed", func(t *testing.T) {
		team := createTestTeamWithInstance(t, models.RouteStrategyFreeRoam, 3)

		// Test with lowercase and spaces
		valid, err := service.IsValidLocation(context.Background(), team, "  marker0  ")
		require.NoError(t, err)
		assert.True(t, valid)
	})
}

// Test the core navigation logic using reflection to access private methods
// Since GetNextLocations tries to load database relations, we'll test the core logic directly.
func TestNavigationService_NavigationLogic(t *testing.T) {
	service, cleanup := setupNavigationService(t)
	defer cleanup()

	t.Run("Free roam navigation logic", func(t *testing.T) {
		team := createTestTeamWithInstance(t, models.RouteStrategyFreeRoam, 3)

		// Test using IsValidLocation which uses the core logic without loading relations
		valid, err := service.IsValidLocation(context.Background(), team, "MARKER0")
		require.NoError(t, err)
		assert.True(t, valid)

		valid, err = service.IsValidLocation(context.Background(), team, "MARKER1")
		require.NoError(t, err)
		assert.True(t, valid)

		valid, err = service.IsValidLocation(context.Background(), team, "MARKER2")
		require.NoError(t, err)
		assert.True(t, valid)
	})

	t.Run("Ordered navigation logic", func(t *testing.T) {
		team := createTestTeamWithInstance(t, models.RouteStrategyOrdered, 1)

		// Only first location should be valid
		valid, err := service.IsValidLocation(context.Background(), team, "MARKER0")
		require.NoError(t, err)
		assert.True(t, valid)

		// Second location should not be valid yet
		valid, err = service.IsValidLocation(context.Background(), team, "MARKER1")
		require.Error(t, err)
		assert.False(t, valid)
	})

	t.Run("Random navigation logic", func(t *testing.T) {
		team := createTestTeamWithInstance(t, models.RouteStrategyRandom, 2)

		// At least some locations should be valid
		validCount := 0
		for i := range 3 {
			valid, _ := service.IsValidLocation(context.Background(), team, fmt.Sprintf("MARKER%d", i))
			if valid {
				validCount++
			}
		}
		assert.Positive(t, validCount)
		assert.LessOrEqual(t, validCount, 2) // Limited by MaxNextLocations
	})

	t.Run("All locations visited", func(t *testing.T) {
		team := createTestTeamWithInstance(t, models.RouteStrategyFreeRoam, 3)

		// Add check-ins for all locations
		for _, location := range team.Instance.Locations {
			team.CheckIns = append(team.CheckIns, models.CheckIn{
				LocationID: location.ID,
				TeamID:     team.Code,
			})
		}

		// No location should be valid when all are visited
		valid, err := service.IsValidLocation(context.Background(), team, "MARKER0")
		require.Error(t, err)
		assert.False(t, valid)
		assert.Contains(t, err.Error(), "all locations visited")
	})
}

func TestNavigationService_HasVisited(t *testing.T) {
	service, cleanup := setupNavigationService(t)
	defer cleanup()

	locationID := gofakeit.UUID()
	checkIns := []models.CheckIn{
		{
			LocationID: locationID,
			TeamID:     gofakeit.Word(),
		},
		{
			LocationID: gofakeit.UUID(), // Different location
			TeamID:     gofakeit.Word(),
		},
	}

	t.Run("Location visited", func(t *testing.T) {
		visited := service.HasVisited(checkIns, locationID)
		assert.True(t, visited)
	})

	t.Run("Location not visited", func(t *testing.T) {
		visited := service.HasVisited(checkIns, gofakeit.UUID())
		assert.False(t, visited)
	})

	t.Run("Empty check-ins", func(t *testing.T) {
		visited := service.HasVisited([]models.CheckIn{}, locationID)
		assert.False(t, visited)
	})
}

func TestNavigationService_OrderedNavigation(t *testing.T) {
	service, cleanup := setupNavigationService(t)
	defer cleanup()

	t.Run("Returns location with lowest order via validation", func(t *testing.T) {
		team := createTestTeamWithInstance(t, models.RouteStrategyOrdered, 1)

		// Shuffle the locations to test order selection
		team.Instance.Locations[0].Order = 5
		team.Instance.Locations[1].Order = 1 // This should be the only valid one
		team.Instance.Locations[2].Order = 3

		// Only the location with order 1 should be valid
		valid, err := service.IsValidLocation(context.Background(), team, "MARKER1")
		require.NoError(t, err)
		assert.True(t, valid)

		// Other locations should not be valid
		valid, err = service.IsValidLocation(context.Background(), team, "MARKER0")
		require.Error(t, err)
		assert.False(t, valid)

		valid, err = service.IsValidLocation(context.Background(), team, "MARKER2")
		require.Error(t, err)
		assert.False(t, valid)
	})

	t.Run("Progress through ordered locations", func(t *testing.T) {
		team := createTestTeamWithInstance(t, models.RouteStrategyOrdered, 1)

		// First location (order 0) should be valid
		valid, err := service.IsValidLocation(context.Background(), team, "MARKER0")
		require.NoError(t, err)
		assert.True(t, valid)

		// Visit first location
		team.CheckIns = append(team.CheckIns, models.CheckIn{
			LocationID: team.Instance.Locations[0].ID,
			TeamID:     team.Code,
		})

		// Second location (order 1) should now be valid
		valid, err = service.IsValidLocation(context.Background(), team, "MARKER1")
		require.NoError(t, err)
		assert.True(t, valid)

		// First location should no longer be valid (already visited)
		valid, err = service.IsValidLocation(context.Background(), team, "MARKER0")
		require.Error(t, err)
		assert.False(t, valid)
	})
}

func TestNavigationService_RandomNavigation(t *testing.T) {
	service, cleanup := setupNavigationService(t)
	defer cleanup()

	t.Run("Deterministic randomness with same team code", func(t *testing.T) {
		team1 := createTestTeamWithInstance(t, models.RouteStrategyRandom, 2)
		team1.Code = "TESTTEAM"

		team2 := createTestTeamWithInstance(t, models.RouteStrategyRandom, 2)
		team2.Code = "TESTTEAM"                             // Same code
		team2.Instance.Locations = team1.Instance.Locations // Same locations

		// Test that both teams get the same validity results due to deterministic seeding
		results1 := make([]bool, 3)
		results2 := make([]bool, 3)

		for i := range 3 {
			valid1, _ := service.IsValidLocation(context.Background(), team1, fmt.Sprintf("MARKER%d", i))
			valid2, _ := service.IsValidLocation(context.Background(), team2, fmt.Sprintf("MARKER%d", i))
			results1[i] = valid1
			results2[i] = valid2
		}

		// Should get same results due to deterministic seeding
		assert.Equal(t, results1, results2)
	})

	t.Run("Respects MaxNextLocations limit", func(t *testing.T) {
		team := createTestTeamWithInstance(t, models.RouteStrategyRandom, 1)

		// Count how many locations are valid
		validCount := 0
		for i := range 3 {
			valid, _ := service.IsValidLocation(context.Background(), team, fmt.Sprintf("MARKER%d", i))
			if valid {
				validCount++
			}
		}
		assert.LessOrEqual(t, validCount, 1)
		assert.Positive(t, validCount) // Should have at least one valid location
	})
}

func TestNavigationService_EdgeCases(t *testing.T) {
	service, cleanup := setupNavigationService(t)
	defer cleanup()

	t.Run("Invalid navigation mode", func(t *testing.T) {
		team := createTestTeamWithInstance(t, 999, 3) // Invalid mode

		valid, err := service.IsValidLocation(context.Background(), team, "MARKER0")
		require.Error(t, err)
		assert.False(t, valid)
		assert.Contains(t, err.Error(), "invalid navigation mode")
	})

	t.Run("Team with no locations", func(t *testing.T) {
		team := &models.Team{
			ID:   gofakeit.UUID(),
			Code: gofakeit.Word(),
			Instance: models.Instance{
				ID:        gofakeit.UUID(),
				Locations: []models.Location{}, // No locations
				Settings: models.InstanceSettings{
					InstanceID:    gofakeit.UUID(),
					RouteStrategy: models.RouteStrategyFreeRoam,
				},
			},
		}

		valid, err := service.IsValidLocation(context.Background(), team, "MARKER0")
		require.Error(t, err)
		assert.False(t, valid)
	})
}
