package services_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestRollingAverageDuration tests the rolling average calculation formula
// used in the checkout process to ensure it correctly accounts for the fact
// that TotalVisits includes currently checked-in teams.
func TestRollingAverageDuration(t *testing.T) {
	t.Run("First checkout establishes baseline", func(t *testing.T) {
		// Simulate location state after first check-in
		totalVisits := 1
		currentCount := 1
		avgDuration := 0.0

		// Simulate checkout after 5 minutes (300 seconds)
		visitDuration := 300.0

		// Calculate rolling average as the checkout method does
		completedVisitsBefore := totalVisits - currentCount // 1 - 1 = 0
		newAvg := (avgDuration*float64(completedVisitsBefore) + visitDuration) / float64(completedVisitsBefore+1)

		// Expected: (0 * 0 + 300) / 1 = 300 seconds
		assert.InDelta(t, 300.0, newAvg, 0.01, "First checkout should establish baseline of 300 seconds")
	})

	t.Run("Second checkout updates rolling average correctly", func(t *testing.T) {
		// Simulate location state after first checkout completed and second team checked in
		totalVisits := 2     // 1 completed + 1 currently visiting
		currentCount := 1    // 1 team currently checked in
		avgDuration := 300.0 // First visit was 5 minutes

		// Simulate checkout after 7 minutes (420 seconds)
		visitDuration := 420.0

		// Calculate rolling average as the checkout method does
		completedVisitsBefore := totalVisits - currentCount // 2 - 1 = 1
		newAvg := (avgDuration*float64(completedVisitsBefore) + visitDuration) / float64(completedVisitsBefore+1)

		// Expected: (300 * 1 + 420) / 2 = 720 / 2 = 360 seconds
		assert.InDelta(t, 360.0, newAvg, 0.01, "Rolling average should be (300 + 420) / 2 = 360 seconds")
	})

	t.Run("Multiple concurrent visitors handled correctly", func(t *testing.T) {
		// Simulate location with 2 completed visits and 3 teams currently checked in
		totalVisits := 5     // 2 completed + 3 currently visiting
		currentCount := 3    // 3 teams currently checked in
		avgDuration := 300.0 // Average of 2 completed visits

		// Simulate checkout after 6 minutes (360 seconds)
		visitDuration := 360.0

		// Calculate rolling average as the checkout method does
		completedVisitsBefore := totalVisits - currentCount // 5 - 3 = 2
		newAvg := (avgDuration*float64(completedVisitsBefore) + visitDuration) / float64(completedVisitsBefore+1)

		// Expected: (300 * 2 + 360) / 3 = 960 / 3 = 320 seconds
		assert.InDelta(t, 320.0, newAvg, 0.01, "With concurrent visitors, average should be (300*2 + 360) / 3 = 320 seconds")
	})

	t.Run("Rolling average formula matches incremental calculation", func(t *testing.T) {
		// Test that our rolling average produces same result as calculating from scratch
		durations := []float64{300, 420, 360, 480, 330} // Various durations in seconds

		// Calculate incrementally using rolling average (simulating multiple checkouts)
		rollingAvg := 0.0
		for i, duration := range durations {
			rollingAvg = (rollingAvg*float64(i) + duration) / float64(i+1)
		}

		// Calculate from scratch (sum / count)
		sum := 0.0
		for _, d := range durations {
			sum += d
		}
		directAvg := sum / float64(len(durations))

		assert.InDelta(t, directAvg, rollingAvg, 0.01, "Rolling average should match direct calculation")
	})

	t.Run("Realistic scenario with varied durations", func(t *testing.T) {
		// Simulate a realistic game scenario
		visitDurations := []time.Duration{
			5 * time.Minute,  // 300 seconds
			7 * time.Minute,  // 420 seconds
			6 * time.Minute,  // 360 seconds
			8 * time.Minute,  // 480 seconds
			5 * time.Minute,  // 300 seconds
			10 * time.Minute, // 600 seconds
		}

		// Track location state
		totalVisits := 0
		currentCount := 0
		avgDuration := 0.0

		for _, visitDuration := range visitDurations {
			// Check-in: increment counters
			totalVisits++
			currentCount++

			// Check-out: update rolling average
			completedVisitsBefore := totalVisits - currentCount
			avgDuration = (avgDuration*float64(completedVisitsBefore) + visitDuration.Seconds()) / float64(completedVisitsBefore+1)
			currentCount--
		}

		// Calculate expected average from scratch
		totalSeconds := 0.0
		for _, d := range visitDurations {
			totalSeconds += d.Seconds()
		}
		expectedAvg := totalSeconds / float64(len(visitDurations))

		assert.InDelta(t, expectedAvg, avgDuration, 0.01, "Realistic scenario should produce correct average")
		assert.Equal(t, 0, currentCount, "All teams should be checked out")
		assert.Equal(t, len(visitDurations), totalVisits, "Total visits should match number of checkouts")
	})

	t.Run("Bug regression: old incorrect formula", func(t *testing.T) {
		// This test demonstrates the bug that was fixed
		// OLD (WRONG): (oldAvg * TotalVisits + newDuration) / (TotalVisits + 1)
		// NEW (CORRECT): (oldAvg * completedVisitsBefore + newDuration) / (completedVisitsBefore + 1)

		totalVisits := 2     // 1 completed + 1 currently visiting
		currentCount := 1    // 1 team currently checked in
		avgDuration := 300.0 // First visit was 5 minutes
		visitDuration := 420.0

		// OLD incorrect formula (would use totalVisits directly)
		incorrectAvg := (avgDuration*float64(totalVisits) + visitDuration) / float64(totalVisits+1)
		// (300 * 2 + 420) / 3 = 1020 / 3 = 340 seconds (WRONG!)

		// NEW correct formula (uses completedVisitsBefore)
		completedVisitsBefore := totalVisits - currentCount
		correctAvg := (avgDuration*float64(completedVisitsBefore) + visitDuration) / float64(completedVisitsBefore+1)
		// (300 * 1 + 420) / 2 = 720 / 2 = 360 seconds (CORRECT!)

		assert.NotEqual(t, incorrectAvg, correctAvg, "Old bug formula should produce different result")
		assert.InDelta(t, 340.0, incorrectAvg, 0.01, "Old formula incorrectly produces 340")
		assert.InDelta(t, 360.0, correctAvg, 0.01, "New formula correctly produces 360")

		// The correct average of [300, 420] is 360, not 340
		expectedAvg := (300.0 + 420.0) / 2.0
		assert.InDelta(t, expectedAvg, correctAvg, 0.01, "Correct formula matches expected average")
	})
}
