package repositories_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupTeamStartLogRepo(t *testing.T) (*repositories.TeamStartLogRepository, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	teamStartLogRepo := repositories.NewTeamStartLogRepository(dbc)

	return teamStartLogRepo, dbc, cleanup
}

func createTestTeamStartLog(t *testing.T, db *bun.DB, userID, teamID, instanceID string, createdAt time.Time) models.TeamStartLog {
	t.Helper()

	log := models.TeamStartLog{
		ID:         gofakeit.UUID(),
		UserID:     userID,
		TeamID:     teamID,
		InstanceID: instanceID,
		CreatedAt:  createdAt,
	}

	// Insert log directly into database for testing
	_, err := db.NewInsert().
		Model(&log).
		Exec(context.Background())
	require.NoError(t, err)

	return log
}

func TestTeamStartLogRepo_CreateWithTx(t *testing.T) {
	repo, db, cleanup := setupTeamStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	log := &models.TeamStartLog{
		ID:         gofakeit.UUID(),
		UserID:     gofakeit.UUID(),
		TeamID:     gofakeit.UUID(),
		InstanceID: gofakeit.UUID(),
		CreatedAt:  time.Now(),
	}

	// Create log within transaction
	err = repo.CreateWithTx(ctx, &tx, log)
	assert.NoError(t, err)

	// Commit transaction
	err = tx.Commit()
	assert.NoError(t, err)

	// Verify log was created
	var createdLog models.TeamStartLog
	err = db.NewSelect().
		Model(&createdLog).
		Where("id = ?", log.ID).
		Scan(ctx)
	require.NoError(t, err)

	assert.Equal(t, log.UserID, createdLog.UserID)
	assert.Equal(t, log.TeamID, createdLog.TeamID)
	assert.Equal(t, log.InstanceID, createdLog.InstanceID)
}

func TestTeamStartLogRepo_CreateWithTx_Rollback(t *testing.T) {
	repo, db, cleanup := setupTeamStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	log := &models.TeamStartLog{
		ID:         gofakeit.UUID(),
		UserID:     gofakeit.UUID(),
		TeamID:     gofakeit.UUID(),
		InstanceID: gofakeit.UUID(),
		CreatedAt:  time.Now(),
	}

	// Create log within transaction
	err = repo.CreateWithTx(ctx, &tx, log)
	assert.NoError(t, err)

	// Rollback transaction
	err = tx.Rollback()
	assert.NoError(t, err)

	// Verify log was not created (due to rollback)
	count, err := db.NewSelect().
		Model(&models.TeamStartLog{}).
		Where("id = ?", log.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestTeamStartLogRepo_GetByUserID(t *testing.T) {
	repo, db, cleanup := setupTeamStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()

	// Create test logs
	now := time.Now()
	log1 := createTestTeamStartLog(t, db, userID, "team-1", "instance-1", now.Add(-2*time.Hour))
	log2 := createTestTeamStartLog(t, db, userID, "team-2", "instance-1", now.Add(-1*time.Hour))
	log3 := createTestTeamStartLog(t, db, "other-user", "team-3", "instance-1", now)

	// Get logs for user
	logs, err := repo.GetByUserID(ctx, userID)
	require.NoError(t, err)

	// Should get 2 logs for the user, in descending order by created_at
	assert.Len(t, logs, 2)
	assert.Equal(t, log2.ID, logs[0].ID) // More recent first
	assert.Equal(t, log1.ID, logs[1].ID)

	// Should not include log3 (different user)
	for _, log := range logs {
		assert.NotEqual(t, log3.ID, log.ID)
	}
}

func TestTeamStartLogRepo_GetByUserIDWithTimeframe(t *testing.T) {
	repo, db, cleanup := setupTeamStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()

	// Create test logs with different timestamps
	now := time.Now()
	log1 := createTestTeamStartLog(t, db, userID, "team-1", "instance-1", now.Add(-3*time.Hour)) // Outside timeframe
	log2 := createTestTeamStartLog(t, db, userID, "team-2", "instance-1", now.Add(-1*time.Hour)) // Inside timeframe
	log3 := createTestTeamStartLog(t, db, userID, "team-3", "instance-1", now.Add(-30*time.Minute)) // Inside timeframe

	// Define timeframe (last 2 hours)
	startTime := now.Add(-2 * time.Hour)
	endTime := now

	// Get logs within timeframe
	logs, err := repo.GetByUserIDWithTimeframe(ctx, userID, startTime, endTime)
	require.NoError(t, err)

	// Should get 2 logs within timeframe, in descending order
	assert.Len(t, logs, 2)
	assert.Equal(t, log3.ID, logs[0].ID) // More recent first
	assert.Equal(t, log2.ID, logs[1].ID)

	// Should not include log1 (outside timeframe)
	for _, log := range logs {
		assert.NotEqual(t, log1.ID, log.ID)
	}
}

func TestTeamStartLogRepo_GetByUserIDAndInstanceID(t *testing.T) {
	repo, db, cleanup := setupTeamStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	instanceID := gofakeit.UUID()

	// Create test logs
	now := time.Now()
	log1 := createTestTeamStartLog(t, db, userID, "team-1", instanceID, now.Add(-2*time.Hour))
	log2 := createTestTeamStartLog(t, db, userID, "team-2", instanceID, now.Add(-1*time.Hour))
	log3 := createTestTeamStartLog(t, db, userID, "team-3", "other-instance", now) // Different instance

	// Get logs for user and instance
	logs, err := repo.GetByUserIDAndInstanceID(ctx, userID, instanceID)
	require.NoError(t, err)

	// Should get 2 logs for the user and instance, in descending order
	assert.Len(t, logs, 2)
	assert.Equal(t, log2.ID, logs[0].ID) // More recent first
	assert.Equal(t, log1.ID, logs[1].ID)

	// Should not include log3 (different instance)
	for _, log := range logs {
		assert.NotEqual(t, log3.ID, log.ID)
	}
}

func TestTeamStartLogRepo_GetByUserIDAndInstanceIDWithTimeframe(t *testing.T) {
	repo, db, cleanup := setupTeamStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	instanceID := gofakeit.UUID()

	// Create test logs
	now := time.Now()
	log1 := createTestTeamStartLog(t, db, userID, "team-1", instanceID, now.Add(-3*time.Hour)) // Outside timeframe
	log2 := createTestTeamStartLog(t, db, userID, "team-2", instanceID, now.Add(-1*time.Hour)) // Inside timeframe
	log3 := createTestTeamStartLog(t, db, userID, "team-3", instanceID, now.Add(-30*time.Minute)) // Inside timeframe
	log4 := createTestTeamStartLog(t, db, userID, "team-4", "other-instance", now.Add(-45*time.Minute)) // Different instance

	// Define timeframe (last 2 hours)
	startTime := now.Add(-2 * time.Hour)
	endTime := now

	// Get logs for user, instance, and timeframe
	logs, err := repo.GetByUserIDAndInstanceIDWithTimeframe(ctx, userID, instanceID, startTime, endTime)
	require.NoError(t, err)

	// Should get 2 logs within criteria, in descending order
	assert.Len(t, logs, 2)
	assert.Equal(t, log3.ID, logs[0].ID) // More recent first
	assert.Equal(t, log2.ID, logs[1].ID)

	// Should not include log1 (outside timeframe) or log4 (different instance)
	for _, log := range logs {
		assert.NotEqual(t, log1.ID, log.ID)
		assert.NotEqual(t, log4.ID, log.ID)
	}
}

func TestTeamStartLogRepo_EmptyResults(t *testing.T) {
	repo, _, cleanup := setupTeamStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()
	nonExistentUserID := gofakeit.UUID()

	tests := []struct {
		name string
		fn   func() ([]models.TeamStartLog, error)
	}{
		{
			name: "GetByUserID with non-existent user",
			fn: func() ([]models.TeamStartLog, error) {
				return repo.GetByUserID(ctx, nonExistentUserID)
			},
		},
		{
			name: "GetByUserIDWithTimeframe with non-existent user",
			fn: func() ([]models.TeamStartLog, error) {
				return repo.GetByUserIDWithTimeframe(ctx, nonExistentUserID, time.Now().Add(-1*time.Hour), time.Now())
			},
		},
		{
			name: "GetByUserIDAndInstanceID with non-existent user",
			fn: func() ([]models.TeamStartLog, error) {
				return repo.GetByUserIDAndInstanceID(ctx, nonExistentUserID, gofakeit.UUID())
			},
		},
		{
			name: "GetByUserIDAndInstanceIDWithTimeframe with non-existent user",
			fn: func() ([]models.TeamStartLog, error) {
				return repo.GetByUserIDAndInstanceIDWithTimeframe(ctx, nonExistentUserID, gofakeit.UUID(), time.Now().Add(-1*time.Hour), time.Now())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs, err := tt.fn()
			assert.NoError(t, err)
			assert.Empty(t, logs)
		})
	}
}

func TestTeamStartLogRepo_OrderingConsistency(t *testing.T) {
	repo, db, cleanup := setupTeamStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()

	// Create logs with precise timestamps to test ordering
	now := time.Now()
	timestamps := []time.Time{
		now.Add(-5 * time.Minute),
		now.Add(-3 * time.Minute),
		now.Add(-1 * time.Minute),
		now.Add(-4 * time.Minute),
		now.Add(-2 * time.Minute),
	}

	var expectedOrder []string
	for i, ts := range timestamps {
		log := createTestTeamStartLog(t, db, userID, fmt.Sprintf("team-%d", i), "instance-1", ts)
		expectedOrder = append(expectedOrder, log.ID)
	}

	// Sort expected order by timestamp (descending - most recent first)
	// Index 2 (-1min), Index 4 (-2min), Index 1 (-3min), Index 3 (-4min), Index 0 (-5min)
	expectedSortedOrder := []string{
		expectedOrder[2], // -1 min
		expectedOrder[4], // -2 min
		expectedOrder[1], // -3 min
		expectedOrder[3], // -4 min
		expectedOrder[0], // -5 min
	}

	// Get logs and verify order
	logs, err := repo.GetByUserID(ctx, userID)
	require.NoError(t, err)
	assert.Len(t, logs, 5)

	for i, log := range logs {
		assert.Equal(t, expectedSortedOrder[i], log.ID, "Log at index %d should be %s but was %s", i, expectedSortedOrder[i], log.ID)
	}
}