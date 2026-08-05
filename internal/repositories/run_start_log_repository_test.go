package repositories_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v7/internal/repositories"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupRunStartLogRepo(t *testing.T) (*repositories.RunStartLogRepository, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	runStartLogRepo := repositories.NewRunStartLogRepository(dbc)

	return runStartLogRepo, dbc, cleanup
}

func createTestRunStartLog(
	t *testing.T,
	db *bun.DB,
	userID,
	teamID,
	questID string,
	createdAt time.Time,
) models.RunStartLog {
	t.Helper()

	// Ensure the referenced user exists (run_start_logs.user_id FK).
	insertUserWithID(t, db, userID)

	log := models.RunStartLog{
		ID:        gofakeit.UUID(),
		UserID:    userID,
		RunID:     teamID,
		QuestID:   questID,
		CreatedAt: createdAt,
	}

	// Insert log directly into database for testing
	_, err := db.NewInsert().
		Model(&log).
		Exec(context.Background())
	require.NoError(t, err)

	return log
}

func TestRunStartLogRepo_CreateWithTx(t *testing.T) {
	repo, db, cleanup := setupRunStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)
	defer tx.Rollback()

	log := &models.RunStartLog{
		ID:        gofakeit.UUID(),
		UserID:    gofakeit.UUID(),
		RunID:     gofakeit.UUID(),
		QuestID:   gofakeit.UUID(),
		CreatedAt: time.Now(),
	}
	insertUserWithID(t, db, log.UserID)

	// Create log within transaction
	err = repo.CreateWithTx(ctx, &tx, log)
	require.NoError(t, err)

	// Commit transaction
	err = tx.Commit()
	require.NoError(t, err)

	// Verify log was created
	var createdLog models.RunStartLog
	err = db.NewSelect().
		Model(&createdLog).
		Where("id = ?", log.ID).
		Scan(ctx)
	require.NoError(t, err)

	assert.Equal(t, log.UserID, createdLog.UserID)
	assert.Equal(t, log.RunID, createdLog.RunID)
	assert.Equal(t, log.QuestID, createdLog.QuestID)
}

func TestRunStartLogRepo_CreateWithTx_Rollback(t *testing.T) {
	repo, db, cleanup := setupRunStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	require.NoError(t, err)

	log := &models.RunStartLog{
		ID:        gofakeit.UUID(),
		UserID:    gofakeit.UUID(),
		RunID:     gofakeit.UUID(),
		QuestID:   gofakeit.UUID(),
		CreatedAt: time.Now(),
	}
	insertUserWithID(t, db, log.UserID)

	// Create log within transaction
	err = repo.CreateWithTx(ctx, &tx, log)
	require.NoError(t, err)

	// Rollback transaction
	err = tx.Rollback()
	require.NoError(t, err)

	// Verify log was not created (due to rollback)
	count, err := db.NewSelect().
		Model(&models.RunStartLog{}).
		Where("id = ?", log.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestRunStartLogRepo_GetByUserID(t *testing.T) {
	repo, db, cleanup := setupRunStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()

	// Create test logs
	now := time.Now()
	log1 := createTestRunStartLog(t, db, userID, "team-1", "instance-1", now.Add(-2*time.Hour))
	log2 := createTestRunStartLog(t, db, userID, "team-2", "instance-1", now.Add(-1*time.Hour))
	log3 := createTestRunStartLog(t, db, "other-user", "team-3", "instance-1", now)

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

func TestRunStartLogRepo_GetByUserIDWithTimeframe(t *testing.T) {
	repo, db, cleanup := setupRunStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()

	// Create test logs with different timestamps
	now := time.Now()
	log1 := createTestRunStartLog(t, db, userID, "team-1", "instance-1", now.Add(-3*time.Hour))    // Outside timeframe
	log2 := createTestRunStartLog(t, db, userID, "team-2", "instance-1", now.Add(-1*time.Hour))    // Inside timeframe
	log3 := createTestRunStartLog(t, db, userID, "team-3", "instance-1", now.Add(-30*time.Minute)) // Inside timeframe

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

func TestRunStartLogRepo_GetByUserIDAndQuestID(t *testing.T) {
	repo, db, cleanup := setupRunStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	questID := gofakeit.UUID()

	// Create test logs
	now := time.Now()
	log1 := createTestRunStartLog(t, db, userID, "team-1", questID, now.Add(-2*time.Hour))
	log2 := createTestRunStartLog(t, db, userID, "team-2", questID, now.Add(-1*time.Hour))
	log3 := createTestRunStartLog(t, db, userID, "team-3", "other-instance", now) // Different instance

	// Get logs for user and instance
	logs, err := repo.GetByUserIDAndQuestID(ctx, userID, questID)
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

func TestRunStartLogRepo_GetByUserIDAndQuestIDWithTimeframe(t *testing.T) {
	repo, db, cleanup := setupRunStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()
	userID := gofakeit.UUID()
	questID := gofakeit.UUID()

	// Create test logs
	now := time.Now()
	log1 := createTestRunStartLog(t, db, userID, "team-1",
		questID, now.Add(-3*time.Hour)) // Outside timeframe
	log2 := createTestRunStartLog(t, db, userID, "team-2",
		questID, now.Add(-1*time.Hour)) // Inside timeframe
	log3 := createTestRunStartLog(t, db, userID, "team-3",
		questID, now.Add(-30*time.Minute)) // Inside timeframe
	log4 := createTestRunStartLog(t, db, userID, "team-4",
		"other-instance", now.Add(-45*time.Minute)) // Different instance

	// Define timeframe (last 2 hours)
	startTime := now.Add(-2 * time.Hour)
	endTime := now

	// Get logs for user, instance, and timeframe
	logs, err := repo.GetByUserIDAndQuestIDWithTimeframe(ctx, userID, questID, startTime, endTime)
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

func TestRunStartLogRepo_EmptyResults(t *testing.T) {
	repo, _, cleanup := setupRunStartLogRepo(t)
	defer cleanup()

	ctx := context.Background()
	nonExistentUserID := gofakeit.UUID()

	tests := []struct {
		name string
		fn   func() ([]models.RunStartLog, error)
	}{
		{
			name: "GetByUserID with non-existent user",
			fn: func() ([]models.RunStartLog, error) {
				return repo.GetByUserID(ctx, nonExistentUserID)
			},
		},
		{
			name: "GetByUserIDWithTimeframe with non-existent user",
			fn: func() ([]models.RunStartLog, error) {
				return repo.GetByUserIDWithTimeframe(ctx, nonExistentUserID, time.Now().Add(-1*time.Hour), time.Now())
			},
		},
		{
			name: "GetByUserIDAndQuestID with non-existent user",
			fn: func() ([]models.RunStartLog, error) {
				return repo.GetByUserIDAndQuestID(ctx, nonExistentUserID, gofakeit.UUID())
			},
		},
		{
			name: "GetByUserIDAndQuestIDWithTimeframe with non-existent user",
			fn: func() ([]models.RunStartLog, error) {
				return repo.GetByUserIDAndQuestIDWithTimeframe(
					ctx,
					nonExistentUserID,
					gofakeit.UUID(),
					time.Now().Add(-1*time.Hour),
					time.Now(),
				)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logs, err := tt.fn()
			require.NoError(t, err)
			assert.Empty(t, logs)
		})
	}
}

func TestRunStartLogRepo_OrderingConsistency(t *testing.T) {
	repo, db, cleanup := setupRunStartLogRepo(t)
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
		log := createTestRunStartLog(t, db, userID, fmt.Sprintf("team-%d", i), "instance-1", ts)
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
		assert.Equal(t, expectedSortedOrder[i], log.ID,
			"Log at index %d should be %s but was %s",
			i, expectedSortedOrder[i], log.ID)
	}
}
