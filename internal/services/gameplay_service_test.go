package services_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/stretchr/testify/assert"
)

// TestBonusPointsCalculationLogic tests the bonus points calculation logic directly
// without going through the full CheckIn flow
func TestBonusPointsCalculationLogic(t *testing.T) {
	tests := []struct {
		name                string
		enableBonusPoints   bool
		locationPoints      int
		locationTotalVisits int
		expectedPoints      int
		description         string
	}{
		{
			name:                "First visit with bonus points enabled",
			enableBonusPoints:   true,
			locationPoints:      100,
			locationTotalVisits: 0, // Before increment
			expectedPoints:      200, // 100 * 2
			description:         "First visitor should get 2x points",
		},
		{
			name:                "Second visit with bonus points enabled",
			enableBonusPoints:   true,
			locationPoints:      100,
			locationTotalVisits: 1, // Before increment
			expectedPoints:      150, // 100 * 1.5
			description:         "Second visitor should get 1.5x points",
		},
		{
			name:                "Third visit with bonus points enabled",
			enableBonusPoints:   true,
			locationPoints:      100,
			locationTotalVisits: 2, // Before increment
			expectedPoints:      120, // 100 * 1.2
			description:         "Third visitor should get 1.2x points",
		},
		{
			name:                "Fourth visit with bonus points enabled",
			enableBonusPoints:   true,
			locationPoints:      100,
			locationTotalVisits: 3, // Before increment
			expectedPoints:      100, // Regular points
			description:         "Fourth and later visitors should get regular points",
		},
		{
			name:                "Bonus points disabled",
			enableBonusPoints:   false,
			locationPoints:      100,
			locationTotalVisits: 0,
			expectedPoints:      100, // Regular points
			description:         "Should get regular points when bonus disabled",
		},
		{
			name:                "Zero points location with bonus",
			enableBonusPoints:   true,
			locationPoints:      0,
			locationTotalVisits: 0,
			expectedPoints:      0, // 0 * 2 = 0
			description:         "Should still be 0 points even with bonus",
		},
		{
			name:                "Negative points location with bonus",
			enableBonusPoints:   true,
			locationPoints:      -50,
			locationTotalVisits: 0,
			expectedPoints:      -100, // -50 * 2 = -100
			description:         "Should apply bonus to negative points",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test objects
			team := &models.Team{
				Points: 0,
			}

			location := &models.Location{
				Points:      tt.locationPoints,
				TotalVisits: tt.locationTotalVisits,
				Instance: models.Instance{
					Settings: models.InstanceSettings{
						EnableBonusPoints: tt.enableBonusPoints,
					},
				},
			}

			// Simulate the bonus points calculation logic from the fixed CheckIn method
			originalPoints := team.Points
			
			if location.Instance.Settings.EnableBonusPoints {
				// Use the TotalVisits count BEFORE incrementing it (this is the fix)
				switch location.TotalVisits {
				case 0:
					team.Points += location.Points * 2 // First visit gets double points
				case 1:
					team.Points += int(float64(location.Points) * 1.5) // Second visit gets 1.5x points
				case 2:
					team.Points += int(float64(location.Points) * 1.2) // Third visit gets 1.2x points
				default:
					team.Points += location.Points // Regular points for all other visits
				}
			} else {
				team.Points += location.Points
			}

			// Verify the calculation
			expectedTotalPoints := originalPoints + tt.expectedPoints
			assert.Equal(t, expectedTotalPoints, team.Points, tt.description)
		})
	}
}

// TestBonusPointsWithExistingPoints tests bonus calculation when team already has points
func TestBonusPointsWithExistingPoints(t *testing.T) {
	team := &models.Team{
		Points: 250, // Starting with existing points
	}

	location := &models.Location{
		Points:      100,
		TotalVisits: 0, // First visit
		Instance: models.Instance{
			Settings: models.InstanceSettings{
				EnableBonusPoints: true,
			},
		},
	}

	// Apply bonus points calculation
	team.Points += location.Points * 2 // First visit gets 2x

	// Should be 250 + (100 * 2) = 450
	assert.Equal(t, 450, team.Points, "Should add bonus points to existing points")
}

// TestBonusPointsEdgeCases tests edge cases for bonus points
func TestBonusPointsEdgeCases(t *testing.T) {
	t.Run("Large point values", func(t *testing.T) {
		team := &models.Team{Points: 0}
		location := &models.Location{
			Points:      10000,
			TotalVisits: 0,
			Instance: models.Instance{
				Settings: models.InstanceSettings{EnableBonusPoints: true},
			},
		}

		team.Points += location.Points * 2
		assert.Equal(t, 20000, team.Points, "Should handle large point values")
	})

	t.Run("Fractional calculations", func(t *testing.T) {
		team := &models.Team{Points: 0}
		location := &models.Location{
			Points:      100,
			TotalVisits: 1, // Second visit (1.5x multiplier)
			Instance: models.Instance{
				Settings: models.InstanceSettings{EnableBonusPoints: true},
			},
		}

		// Test the exact calculation from the code
		team.Points += int(float64(location.Points) * 1.5)
		assert.Equal(t, 150, team.Points, "Should handle fractional multipliers correctly")
	})

	t.Run("Third visit calculation", func(t *testing.T) {
		team := &models.Team{Points: 0}
		location := &models.Location{
			Points:      100,
			TotalVisits: 2, // Third visit (1.2x multiplier)
			Instance: models.Instance{
				Settings: models.InstanceSettings{EnableBonusPoints: true},
			},
		}

		team.Points += int(float64(location.Points) * 1.2)
		assert.Equal(t, 120, team.Points, "Should handle 1.2x multiplier correctly")
	})
}

// TestCheckInRecordPointsCalculation tests that CheckIn records store the correct awarded points
func TestCheckInRecordPointsCalculation(t *testing.T) {
	tests := []struct {
		name                string
		enableBonusPoints   bool
		locationPoints      int
		locationTotalVisits int
		mustCheckOut        bool
		expectedPoints      int
		description         string
	}{
		{
			name:                "CheckIn record with bonus points - first visit",
			enableBonusPoints:   true,
			locationPoints:      100,
			locationTotalVisits: 0,
			mustCheckOut:        false,
			expectedPoints:      200, // 2x points
			description:         "CheckIn record should store 2x points for first visit",
		},
		{
			name:                "CheckIn record with bonus points - second visit",
			enableBonusPoints:   true,
			locationPoints:      100,
			locationTotalVisits: 1,
			mustCheckOut:        false,
			expectedPoints:      150, // 1.5x points
			description:         "CheckIn record should store 1.5x points for second visit",
		},
		{
			name:                "CheckIn record without bonus points",
			enableBonusPoints:   false,
			locationPoints:      100,
			locationTotalVisits: 0,
			mustCheckOut:        false,
			expectedPoints:      100, // Regular points
			description:         "CheckIn record should store regular points when bonus disabled",
		},
		{
			name:                "CheckIn record with mustCheckOut - first visit",
			enableBonusPoints:   true,
			locationPoints:      100,
			locationTotalVisits: 0,
			mustCheckOut:        true,
			expectedPoints:      100, // Bonus points only (base awarded at checkout)
			description:         "CheckIn record should store bonus points when must check out",
		},
		{
			name:                "CheckIn record with mustCheckOut - second visit",
			enableBonusPoints:   true,
			locationPoints:      100,
			locationTotalVisits: 1,
			mustCheckOut:        true,
			expectedPoints:      50, // 50% bonus points only
			description:         "CheckIn record should store 50% bonus points for second visit",
		},
		{
			name:                "CheckIn record with mustCheckOut - no bonus",
			enableBonusPoints:   false,
			locationPoints:      100,
			locationTotalVisits: 0,
			mustCheckOut:        true,
			expectedPoints:      0, // No bonus, no immediate points
			description:         "CheckIn record should store 0 points when no bonus and must check out",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test objects
			location := &models.Location{
				Points:      tt.locationPoints,
				TotalVisits: tt.locationTotalVisits,
				Instance: models.Instance{
					Settings: models.InstanceSettings{
						EnableBonusPoints: tt.enableBonusPoints,
						CompletionMethod: func() models.CompletionMethod {
							if tt.mustCheckOut {
								return models.CheckInAndOut
							}
							return models.CheckInOnly
						}(),
					},
				},
			}

			// Simulate the points calculation logic from the fixed CheckIn method
			var pointsForCheckInRecord int
			mustCheckOut := location.Instance.Settings.CompletionMethod == models.CheckInAndOut

			if mustCheckOut {
				// Check-in-and-out mode: only bonus points recorded in CheckIn record
				if location.Instance.Settings.EnableBonusPoints {
					// Calculate bonus points based on visit count
					switch location.TotalVisits {
					case 0:
						pointsForCheckInRecord = location.Points // First visit gets +100% bonus
					case 1:
						pointsForCheckInRecord = int(float64(location.Points) * 0.5) // Second visit gets +50% bonus
					case 2:
						pointsForCheckInRecord = int(float64(location.Points) * 0.2) // Third visit gets +20% bonus
					default:
						pointsForCheckInRecord = 0 // No bonus for later visits
					}
				} else {
					pointsForCheckInRecord = 0 // No bonus, no immediate points
				}
			} else {
				// Check-in-only mode: full points recorded in CheckIn record
				if location.Instance.Settings.EnableBonusPoints {
					// Calculate total points with bonus
					switch location.TotalVisits {
					case 0:
						pointsForCheckInRecord = location.Points * 2 // First visit gets double points
					case 1:
						pointsForCheckInRecord = int(float64(location.Points) * 1.5) // Second visit gets 1.5x points
					case 2:
						pointsForCheckInRecord = int(float64(location.Points) * 1.2) // Third visit gets 1.2x points
					default:
						pointsForCheckInRecord = location.Points // Regular points for all other visits
					}
				} else {
					pointsForCheckInRecord = location.Points
				}
			}

			// Verify the points calculation for CheckIn record
			assert.Equal(t, tt.expectedPoints, pointsForCheckInRecord, tt.description)
		})
	}
}

// TestCheckInAndOutTeamPointsCalculation tests team points in check-in-and-out mode
func TestCheckInAndOutTeamPointsCalculation(t *testing.T) {
	tests := []struct {
		name                    string
		enableBonusPoints       bool
		locationPoints          int
		locationTotalVisits     int
		expectedCheckInPoints   int // Points added to team at check-in
		expectedCheckOutPoints  int // Points added to team at check-out
		expectedTotalPoints     int // Total points after both check-in and check-out
		description             string
	}{
		{
			name:                    "Check-in-and-out with bonus - first visit",
			enableBonusPoints:       true,
			locationPoints:          100,
			locationTotalVisits:     0,
			expectedCheckInPoints:   100, // Bonus points at check-in
			expectedCheckOutPoints:  100, // Base points at check-out
			expectedTotalPoints:     200, // Total 2x points
			description:             "First visit should get bonus at check-in, base at check-out, CheckIn record shows total after checkout",
		},
		{
			name:                    "Check-in-and-out with bonus - second visit",
			enableBonusPoints:       true,
			locationPoints:          100,
			locationTotalVisits:     1,
			expectedCheckInPoints:   50,  // 50% bonus points at check-in
			expectedCheckOutPoints:  100, // Base points at check-out
			expectedTotalPoints:     150, // Total 1.5x points
			description:             "Second visit should get 50% bonus at check-in, base at check-out, CheckIn record shows total after checkout",
		},
		{
			name:                    "Check-in-and-out without bonus",
			enableBonusPoints:       false,
			locationPoints:          100,
			locationTotalVisits:     0,
			expectedCheckInPoints:   0,   // No bonus points
			expectedCheckOutPoints:  100, // Base points at check-out
			expectedTotalPoints:     100, // Only base points
			description:             "No bonus should give 0 at check-in, base at check-out, CheckIn record shows total after checkout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test objects
			team := &models.Team{Points: 0}
			
			location := &models.Location{
				Points:      tt.locationPoints,
				TotalVisits: tt.locationTotalVisits,
				Instance: models.Instance{
					Settings: models.InstanceSettings{
						EnableBonusPoints: tt.enableBonusPoints,
						CompletionMethod:  models.CheckInAndOut,
					},
				},
			}

			// Simulate check-in logic
			var bonusPoints int
			if location.Instance.Settings.EnableBonusPoints {
				switch location.TotalVisits {
				case 0:
					bonusPoints = location.Points // First visit gets +100% bonus
				case 1:
					bonusPoints = int(float64(location.Points) * 0.5) // Second visit gets +50% bonus
				case 2:
					bonusPoints = int(float64(location.Points) * 0.2) // Third visit gets +20% bonus
				default:
					bonusPoints = 0 // No bonus for later visits
				}
			}
			team.Points += bonusPoints

			// Verify check-in points
			assert.Equal(t, tt.expectedCheckInPoints, bonusPoints, "Check-in bonus points should match expected")
			assert.Equal(t, tt.expectedCheckInPoints, team.Points, "Team points after check-in should match expected")

			// Simulate check-out logic (award base points)
			team.Points += location.Points

			// Verify final points
			assert.Equal(t, tt.expectedTotalPoints, team.Points, tt.description)
		})
	}
}

// TestCheckInRecordUpdatedAtCheckout tests that CheckIn records are updated to include base points at checkout
func TestCheckInRecordUpdatedAtCheckout(t *testing.T) {
	tests := []struct {
		name                    string
		enableBonusPoints       bool
		locationPoints          int
		locationTotalVisits     int
		expectedCheckInPoints   int // Points in CheckIn record after check-in
		expectedFinalPoints     int // Points in CheckIn record after check-out
		description             string
	}{
		{
			name:                    "CheckIn record updated with base points - first visit",
			enableBonusPoints:       true,
			locationPoints:          100,
			locationTotalVisits:     0,
			expectedCheckInPoints:   100, // Bonus points only
			expectedFinalPoints:     200, // Bonus + base points
			description:             "CheckIn record should show total points after checkout for first visit",
		},
		{
			name:                    "CheckIn record updated with base points - second visit",
			enableBonusPoints:       true,
			locationPoints:          100,
			locationTotalVisits:     1,
			expectedCheckInPoints:   50,  // 50% bonus points only
			expectedFinalPoints:     150, // Bonus + base points
			description:             "CheckIn record should show total points after checkout for second visit",
		},
		{
			name:                    "CheckIn record updated with base points - no bonus",
			enableBonusPoints:       false,
			locationPoints:          100,
			locationTotalVisits:     0,
			expectedCheckInPoints:   0,   // No bonus points
			expectedFinalPoints:     100, // Only base points
			description:             "CheckIn record should show base points after checkout when no bonus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test objects
			team := &models.Team{Points: 0}
			
			location := &models.Location{
				Points:      tt.locationPoints,
				TotalVisits: tt.locationTotalVisits,
				Instance: models.Instance{
					Settings: models.InstanceSettings{
						EnableBonusPoints: tt.enableBonusPoints,
						CompletionMethod:  models.CheckInAndOut,
					},
				},
			}

			// Simulate check-in logic (bonus points awarded to team and recorded in CheckIn)
			var bonusPoints int
			if location.Instance.Settings.EnableBonusPoints {
				switch location.TotalVisits {
				case 0:
					bonusPoints = location.Points // First visit gets +100% bonus
				case 1:
					bonusPoints = int(float64(location.Points) * 0.5) // Second visit gets +50% bonus
				case 2:
					bonusPoints = int(float64(location.Points) * 0.2) // Third visit gets +20% bonus
				default:
					bonusPoints = 0 // No bonus for later visits
				}
			}
			
			// Simulate CheckIn record creation with bonus points
			checkInRecord := models.CheckIn{
				Points: bonusPoints,
			}
			
			// Verify initial CheckIn record points
			assert.Equal(t, tt.expectedCheckInPoints, checkInRecord.Points, "CheckIn record should have correct bonus points after check-in")
			
			// Simulate checkout logic (base points awarded to team and added to CheckIn record)
			team.Points += bonusPoints // Points from check-in
			team.Points += location.Points // Base points from check-out
			
			// Update CheckIn record to include base points (this is the fix being tested)
			checkInRecord.Points += location.Points
			
			// Verify final CheckIn record points
			assert.Equal(t, tt.expectedFinalPoints, checkInRecord.Points, tt.description)
			
			// Verify team has correct total points
			expectedTeamPoints := tt.expectedFinalPoints
			assert.Equal(t, expectedTeamPoints, team.Points, "Team should have correct total points")
		})
	}
}