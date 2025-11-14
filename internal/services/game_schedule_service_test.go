package services_test

import (
	"context"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupGameScheduleService(t *testing.T) (*services.GameScheduleService, func()) {
	dbc, cleanup := setupDB(t)

	instanceRepo := repositories.NewInstanceRepository(dbc)

	gameScheduleService := services.NewGameScheduleService(instanceRepo)

	return gameScheduleService, cleanup
}

func createTestInstance(t *testing.T, dbc *bun.DB) *models.Instance {
	instance := &models.Instance{
		ID:        gofakeit.UUID(),
		UserID:    gofakeit.UUID(),
		Name:      gofakeit.Name(),
		StartTime: bun.NullTime{},
		EndTime:   bun.NullTime{},
	}

	instanceRepo := repositories.NewInstanceRepository(dbc)
	err := instanceRepo.Create(context.Background(), instance)
	require.NoError(t, err)

	return instance
}

func TestGameScheduleService_Start(t *testing.T) {
	testCases := []struct {
		name         string
		setupFn      func(dbc *bun.DB) *models.Instance
		wantErr      bool
		expectedName string
	}{
		{
			name: "Start inactive game",
			setupFn: func(dbc *bun.DB) *models.Instance {
				return createTestInstance(t, dbc)
			},
			wantErr: false,
		},
		{
			name: "Start already active game",
			setupFn: func(dbc *bun.DB) *models.Instance {
				instance := createTestInstance(t, dbc)
				// Set start time to make it active
				instance.StartTime = bun.NullTime{Time: time.Now().Add(-1 * time.Hour)}
				instanceRepo := repositories.NewInstanceRepository(dbc)
				err := instanceRepo.Update(context.Background(), instance)
				require.NoError(t, err)
				return instance
			},
			wantErr:      true,
			expectedName: "game is already active",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupGameScheduleService(t)
			defer cleanup()

			// Use the same DB connection for setup
			dbc, dbCleanup := setupDB(t)
			defer dbCleanup()

			instance := tc.setupFn(dbc)
			startTime := time.Now()

			err := svc.Start(context.Background(), instance)

			if tc.wantErr {
				require.Error(t, err)
				if tc.expectedName != "" {
					assert.Contains(t, err.Error(), tc.expectedName)
				}
			} else {
				require.NoError(t, err)
				assert.True(t, instance.StartTime.Time.After(startTime.Add(-1*time.Second)))
				assert.True(t, instance.StartTime.Time.Before(startTime.Add(1*time.Second)))
			}
		})
	}
}

func TestGameScheduleService_Stop(t *testing.T) {
	testCases := []struct {
		name         string
		setupFn      func(dbc *bun.DB) *models.Instance
		wantErr      bool
		expectedName string
	}{
		{
			name: "Stop active game",
			setupFn: func(dbc *bun.DB) *models.Instance {
				instance := createTestInstance(t, dbc)
				instance.StartTime = bun.NullTime{Time: time.Now().Add(-1 * time.Hour)}
				instanceRepo := repositories.NewInstanceRepository(dbc)
				err := instanceRepo.Update(context.Background(), instance)
				require.NoError(t, err)
				return instance
			},
			wantErr: false,
		},
		{
			name: "Stop already closed game",
			setupFn: func(dbc *bun.DB) *models.Instance {
				instance := createTestInstance(t, dbc)
				// Set both start and end time to make it closed
				instance.StartTime = bun.NullTime{Time: time.Now().Add(-2 * time.Hour)}
				instance.EndTime = bun.NullTime{Time: time.Now().Add(-1 * time.Hour)}
				instanceRepo := repositories.NewInstanceRepository(dbc)
				err := instanceRepo.Update(context.Background(), instance)
				require.NoError(t, err)
				return instance
			},
			wantErr:      true,
			expectedName: "game is already closed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupGameScheduleService(t)
			defer cleanup()

			dbc, dbCleanup := setupDB(t)
			defer dbCleanup()

			instance := tc.setupFn(dbc)
			stopTime := time.Now()

			err := svc.Stop(context.Background(), instance)

			if tc.wantErr {
				require.Error(t, err)
				if tc.expectedName != "" {
					assert.Contains(t, err.Error(), tc.expectedName)
				}
			} else {
				require.NoError(t, err)
				assert.True(t, instance.EndTime.Time.After(stopTime.Add(-1*time.Second)))
				assert.True(t, instance.EndTime.Time.Before(stopTime.Add(1*time.Second)))
			}
		})
	}
}

func TestGameScheduleService_SetStartTime(t *testing.T) {
	testCases := []struct {
		name         string
		setupFn      func(dbc *bun.DB) *models.Instance
		startTime    time.Time
		wantErr      bool
		expectedName string
	}{
		{
			name: "Set start time for inactive game",
			setupFn: func(dbc *bun.DB) *models.Instance {
				return createTestInstance(t, dbc)
			},
			startTime: time.Now().Add(1 * time.Hour),
			wantErr:   false,
		},
		{
			name: "Set start time for already active game",
			setupFn: func(dbc *bun.DB) *models.Instance {
				instance := createTestInstance(t, dbc)
				instance.StartTime = bun.NullTime{Time: time.Now().Add(-1 * time.Hour)}
				instanceRepo := repositories.NewInstanceRepository(dbc)
				err := instanceRepo.Update(context.Background(), instance)
				require.NoError(t, err)
				return instance
			},
			startTime:    time.Now().Add(1 * time.Hour),
			wantErr:      true,
			expectedName: "game is already active",
		},
		{
			name: "Set start time after existing end time - should clear end time",
			setupFn: func(dbc *bun.DB) *models.Instance {
				instance := createTestInstance(t, dbc)
				instance.EndTime = bun.NullTime{Time: time.Now().Add(1 * time.Hour)}
				instanceRepo := repositories.NewInstanceRepository(dbc)
				err := instanceRepo.Update(context.Background(), instance)
				require.NoError(t, err)
				return instance
			},
			startTime: time.Now().Add(2 * time.Hour),
			wantErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupGameScheduleService(t)
			defer cleanup()

			dbc, dbCleanup := setupDB(t)
			defer dbCleanup()

			instance := tc.setupFn(dbc)
			originalEndTime := instance.EndTime

			err := svc.SetStartTime(context.Background(), instance, tc.startTime)

			if tc.wantErr {
				require.Error(t, err)
				if tc.expectedName != "" {
					assert.Contains(t, err.Error(), tc.expectedName)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.startTime.Unix(), instance.StartTime.Time.Unix())

				// Check if end time was cleared when start time is after it
				if !originalEndTime.IsZero() && tc.startTime.After(originalEndTime.Time) {
					assert.True(t, instance.EndTime.IsZero(), "End time should be cleared when start time is after it")
				}
			}
		})
	}
}

func TestGameScheduleService_SetEndTime(t *testing.T) {
	testCases := []struct {
		name         string
		setupFn      func(dbc *bun.DB) *models.Instance
		endTime      time.Time
		wantErr      bool
		expectedName string
	}{
		{
			name: "Set end time for active game",
			setupFn: func(dbc *bun.DB) *models.Instance {
				instance := createTestInstance(t, dbc)
				instance.StartTime = bun.NullTime{Time: time.Now().Add(-1 * time.Hour)}
				instanceRepo := repositories.NewInstanceRepository(dbc)
				err := instanceRepo.Update(context.Background(), instance)
				require.NoError(t, err)
				return instance
			},
			endTime: time.Now().Add(1 * time.Hour),
			wantErr: false,
		},
		{
			name: "Set end time for already closed game",
			setupFn: func(dbc *bun.DB) *models.Instance {
				instance := createTestInstance(t, dbc)
				instance.StartTime = bun.NullTime{Time: time.Now().Add(-2 * time.Hour)}
				instance.EndTime = bun.NullTime{Time: time.Now().Add(-1 * time.Hour)}
				instanceRepo := repositories.NewInstanceRepository(dbc)
				err := instanceRepo.Update(context.Background(), instance)
				require.NoError(t, err)
				return instance
			},
			endTime:      time.Now().Add(1 * time.Hour),
			wantErr:      true,
			expectedName: "game is already closed",
		},
		{
			name: "Set end time before start time",
			setupFn: func(dbc *bun.DB) *models.Instance {
				instance := createTestInstance(t, dbc)
				instance.StartTime = bun.NullTime{Time: time.Now().Add(2 * time.Hour)}
				instanceRepo := repositories.NewInstanceRepository(dbc)
				err := instanceRepo.Update(context.Background(), instance)
				require.NoError(t, err)
				return instance
			},
			endTime:      time.Now().Add(1 * time.Hour),
			wantErr:      true,
			expectedName: "end time cannot be before start time",
		},
		{
			name: "Set end time for game without start time",
			setupFn: func(dbc *bun.DB) *models.Instance {
				return createTestInstance(t, dbc)
			},
			endTime:      time.Now().Add(1 * time.Hour),
			wantErr:      true,
			expectedName: "game is already closed",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupGameScheduleService(t)
			defer cleanup()

			dbc, dbCleanup := setupDB(t)
			defer dbCleanup()

			instance := tc.setupFn(dbc)

			err := svc.SetEndTime(context.Background(), instance, tc.endTime)

			if tc.wantErr {
				require.Error(t, err)
				if tc.expectedName != "" {
					assert.Contains(t, err.Error(), tc.expectedName)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.endTime.Unix(), instance.EndTime.Time.Unix())
			}
		})
	}
}

func TestGameScheduleService_ScheduleGame(t *testing.T) {
	testCases := []struct {
		name         string
		setupFn      func(dbc *bun.DB) *models.Instance
		startTime    time.Time
		endTime      time.Time
		wantErr      bool
		expectedName string
	}{
		{
			name: "Schedule game with valid times",
			setupFn: func(dbc *bun.DB) *models.Instance {
				return createTestInstance(t, dbc)
			},
			startTime: time.Now().Add(1 * time.Hour),
			endTime:   time.Now().Add(3 * time.Hour),
			wantErr:   false,
		},
		{
			name: "Schedule game with start time after end time",
			setupFn: func(dbc *bun.DB) *models.Instance {
				return createTestInstance(t, dbc)
			},
			startTime:    time.Now().Add(3 * time.Hour),
			endTime:      time.Now().Add(1 * time.Hour),
			wantErr:      true,
			expectedName: "start time cannot be after end time",
		},
		{
			name: "Schedule game with same start and end time",
			setupFn: func(dbc *bun.DB) *models.Instance {
				return createTestInstance(t, dbc)
			},
			startTime: time.Now().Add(1 * time.Hour),
			endTime:   time.Now().Add(1 * time.Hour),
			wantErr:   false,
		},
		{
			name: "Reschedule existing game",
			setupFn: func(dbc *bun.DB) *models.Instance {
				instance := createTestInstance(t, dbc)
				instance.StartTime = bun.NullTime{Time: time.Now().Add(-1 * time.Hour)}
				instance.EndTime = bun.NullTime{Time: time.Now().Add(1 * time.Hour)}
				instanceRepo := repositories.NewInstanceRepository(dbc)
				err := instanceRepo.Update(context.Background(), instance)
				require.NoError(t, err)
				return instance
			},
			startTime: time.Now().Add(2 * time.Hour),
			endTime:   time.Now().Add(4 * time.Hour),
			wantErr:   false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupGameScheduleService(t)
			defer cleanup()

			dbc, dbCleanup := setupDB(t)
			defer dbCleanup()

			instance := tc.setupFn(dbc)

			err := svc.ScheduleGame(context.Background(), instance, tc.startTime, tc.endTime)

			if tc.wantErr {
				require.Error(t, err)
				if tc.expectedName != "" {
					assert.Contains(t, err.Error(), tc.expectedName)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.startTime.Unix(), instance.StartTime.Time.Unix())
				assert.Equal(t, tc.endTime.Unix(), instance.EndTime.Time.Unix())
			}
		})
	}
}

// Integration tests.
func TestGameScheduleService_Integration_CompleteWorkflow(t *testing.T) {
	svc, cleanup := setupGameScheduleService(t)
	defer cleanup()

	dbc, dbCleanup := setupDB(t)
	defer dbCleanup()

	// Create a fresh instance
	instance := createTestInstance(t, dbc)

	// Test the complete workflow: Schedule -> Start -> Stop
	ctx := context.Background()

	// 1. Schedule the game
	startTime := time.Now().Add(1 * time.Hour)
	endTime := time.Now().Add(3 * time.Hour)

	err := svc.ScheduleGame(ctx, instance, startTime, endTime)
	require.NoError(t, err)
	assert.Equal(t, startTime.Unix(), instance.StartTime.Time.Unix())
	assert.Equal(t, endTime.Unix(), instance.EndTime.Time.Unix())

	// 2. Start the game immediately (should update start time)
	err = svc.Start(ctx, instance)
	require.NoError(t, err)
	assert.True(t, instance.StartTime.Time.Before(time.Now().Add(1*time.Second)))

	// 3. Try to start again (should fail)
	err = svc.Start(ctx, instance)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "game is already active")

	// 4. Stop the game
	err = svc.Stop(ctx, instance)
	require.NoError(t, err)
	assert.True(t, instance.EndTime.Time.Before(time.Now().Add(1*time.Second)))

	// 5. Try to stop again (should fail)
	err = svc.Stop(ctx, instance)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "game is already closed")
}

func TestGameScheduleService_Integration_DatabasePersistence(t *testing.T) {
	svc, cleanup := setupGameScheduleService(t)
	defer cleanup()

	dbc, dbCleanup := setupDB(t)
	defer dbCleanup()

	instance := createTestInstance(t, dbc)
	instanceRepo := repositories.NewInstanceRepository(dbc)

	// Schedule a game
	startTime := time.Now().Add(1 * time.Hour)
	endTime := time.Now().Add(3 * time.Hour)

	err := svc.ScheduleGame(context.Background(), instance, startTime, endTime)
	require.NoError(t, err)

	// Verify persistence by fetching from database
	fetchedInstance, err := instanceRepo.GetByID(context.Background(), instance.ID)
	require.NoError(t, err)
	assert.Equal(t, startTime.Unix(), fetchedInstance.StartTime.Time.Unix())
	assert.Equal(t, endTime.Unix(), fetchedInstance.EndTime.Time.Unix())

	// Start the game and verify persistence
	err = svc.Start(context.Background(), instance)
	require.NoError(t, err)

	fetchedInstance, err = instanceRepo.GetByID(context.Background(), instance.ID)
	require.NoError(t, err)
	assert.True(t, fetchedInstance.StartTime.Time.Before(time.Now().Add(1*time.Second)))
}

func TestGameScheduleService_Integration_EdgeCases(t *testing.T) {
	svc, cleanup := setupGameScheduleService(t)
	defer cleanup()

	dbc, dbCleanup := setupDB(t)
	defer dbCleanup()

	instance := createTestInstance(t, dbc)

	// Test setting start time that clears end time
	instance.EndTime = bun.NullTime{Time: time.Now().Add(1 * time.Hour)}
	newStartTime := time.Now().Add(2 * time.Hour)

	err := svc.SetStartTime(context.Background(), instance, newStartTime)
	require.NoError(t, err)
	assert.Equal(t, newStartTime.Unix(), instance.StartTime.Time.Unix())
	assert.True(t, instance.EndTime.IsZero(), "End time should be cleared when start time is after it")

	// Test setting end time equal to start time
	equalTime := time.Now().Add(1 * time.Hour)
	instance.StartTime = bun.NullTime{Time: equalTime}

	err = svc.SetEndTime(context.Background(), instance, equalTime)
	require.NoError(t, err)
	assert.Equal(t, equalTime.Unix(), instance.EndTime.Time.Unix())
}
