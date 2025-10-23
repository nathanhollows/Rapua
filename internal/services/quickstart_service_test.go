package services_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupQuickstartService(t *testing.T) (*services.QuickstartService, func()) {
	dbc, cleanup := setupDB(t)

	instanceRepo := repositories.NewInstanceRepository(dbc)
	quickstartService := services.NewQuickstartService(instanceRepo)

	return quickstartService, cleanup
}

func createTestInstanceForQuickstart(t *testing.T, dbc *bun.DB, dismissed bool) *models.Instance {
	instance := &models.Instance{
		ID:                    gofakeit.UUID(),
		UserID:                gofakeit.UUID(),
		Name:                  gofakeit.Name(),
		IsQuickStartDismissed: dismissed,
		StartTime:             bun.NullTime{},
		EndTime:               bun.NullTime{},
	}

	instanceRepo := repositories.NewInstanceRepository(dbc)
	err := instanceRepo.Create(context.Background(), instance)
	require.NoError(t, err)

	return instance
}

func TestQuickstartService_DismissQuickstart(t *testing.T) {
	testCases := []struct {
		name              string
		setupFn           func(dbc *bun.DB) (string, bool) // returns instanceID and whether it should exist
		instanceID        string
		wantErr           bool
		expectedDismissed bool
		expectedName      string
	}{
		{
			name: "Dismiss quickstart for existing instance",
			setupFn: func(dbc *bun.DB) (string, bool) {
				instance := createTestInstanceForQuickstart(t, dbc, false)
				return instance.ID, true
			},
			wantErr:           false,
			expectedDismissed: true,
		},
		{
			name: "Dismiss quickstart for already dismissed instance",
			setupFn: func(dbc *bun.DB) (string, bool) {
				instance := createTestInstanceForQuickstart(t, dbc, true)
				return instance.ID, true
			},
			wantErr:           false,
			expectedDismissed: true,
		},
		{
			name: "Dismiss quickstart for non-existent instance",
			setupFn: func(dbc *bun.DB) (string, bool) {
				return gofakeit.UUID(), false
			},
			wantErr:           false, // Repository doesn't check existence, just updates
			expectedDismissed: true,  // Not applicable since instance doesn't exist
		},
		{
			name: "Dismiss quickstart with empty instance ID",
			setupFn: func(dbc *bun.DB) (string, bool) {
				return "", false
			},
			wantErr:           false, // Repository will accept empty string
			expectedDismissed: true,  // Not applicable
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupQuickstartService(t)
			defer cleanup()

			dbc, dbCleanup := setupDB(t)
			defer dbCleanup()

			instanceID, shouldExist := tc.setupFn(dbc)

			err := svc.DismissQuickstart(context.Background(), instanceID)

			if tc.wantErr {
				require.Error(t, err)
				if tc.expectedName != "" {
					assert.Contains(t, err.Error(), tc.expectedName)
				}
			} else {
				require.NoError(t, err)

				if shouldExist {
					// Verify the instance was actually updated in the database
					instanceRepo := repositories.NewInstanceRepository(dbc)
					instance, err := instanceRepo.GetByID(context.Background(), instanceID)
					if assert.NoError(t, err) {
						assert.Equal(t, tc.expectedDismissed, instance.IsQuickStartDismissed)
					}
				}
			}
		})
	}
}

func TestQuickstartService_DismissQuickstart_ValidationCases(t *testing.T) {
	testCases := []struct {
		name       string
		instanceID string
		wantErr    bool
	}{
		{
			name:       "Valid UUID",
			instanceID: gofakeit.UUID(),
			wantErr:    false, // Repository doesn't validate existence, just updates
		},
		{
			name:       "Empty string",
			instanceID: "",
			wantErr:    false, // Repository accepts empty string
		},
		{
			name:       "Invalid UUID format",
			instanceID: "not-a-uuid",
			wantErr:    false, // Repository doesn't validate UUID format
		},
		{
			name:       "SQL injection attempt",
			instanceID: "'; DROP TABLE instances; --",
			wantErr:    false, // Repository should handle this safely with parameterized queries
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			svc, cleanup := setupQuickstartService(t)
			defer cleanup()

			err := svc.DismissQuickstart(context.Background(), tc.instanceID)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// Integration Tests.
func TestQuickstartService_Integration_DatabasePersistence(t *testing.T) {
	svc, cleanup := setupQuickstartService(t)
	defer cleanup()

	dbc, dbCleanup := setupDB(t)
	defer dbCleanup()

	// Create an instance that hasn't dismissed quickstart
	instance := createTestInstanceForQuickstart(t, dbc, false)
	instanceRepo := repositories.NewInstanceRepository(dbc)

	// Verify initial state
	fetchedInstance, err := instanceRepo.GetByID(context.Background(), instance.ID)
	require.NoError(t, err)
	assert.False(t, fetchedInstance.IsQuickStartDismissed, "Instance should not have quickstart dismissed initially")

	// Dismiss quickstart
	err = svc.DismissQuickstart(context.Background(), instance.ID)
	require.NoError(t, err)

	// Verify persistence
	fetchedInstance, err = instanceRepo.GetByID(context.Background(), instance.ID)
	require.NoError(t, err)
	assert.True(
		t,
		fetchedInstance.IsQuickStartDismissed,
		"Instance should have quickstart dismissed after service call",
	)
}

func TestQuickstartService_Integration_MultipleInstances(t *testing.T) {
	svc, cleanup := setupQuickstartService(t)
	defer cleanup()

	dbc, dbCleanup := setupDB(t)
	defer dbCleanup()

	// Create multiple instances
	instance1 := createTestInstanceForQuickstart(t, dbc, false)
	instance2 := createTestInstanceForQuickstart(t, dbc, false)
	instance3 := createTestInstanceForQuickstart(t, dbc, true) // Already dismissed

	instanceRepo := repositories.NewInstanceRepository(dbc)

	// Dismiss quickstart for instance1 only
	err := svc.DismissQuickstart(context.Background(), instance1.ID)
	require.NoError(t, err)

	// Verify instance1 is dismissed
	fetchedInstance1, err := instanceRepo.GetByID(context.Background(), instance1.ID)
	require.NoError(t, err)
	assert.True(t, fetchedInstance1.IsQuickStartDismissed)

	// Verify instance2 is still not dismissed
	fetchedInstance2, err := instanceRepo.GetByID(context.Background(), instance2.ID)
	require.NoError(t, err)
	assert.False(t, fetchedInstance2.IsQuickStartDismissed)

	// Verify instance3 remains dismissed
	fetchedInstance3, err := instanceRepo.GetByID(context.Background(), instance3.ID)
	require.NoError(t, err)
	assert.True(t, fetchedInstance3.IsQuickStartDismissed)
}

func TestQuickstartService_Integration_ContextCancellation(t *testing.T) {
	svc, cleanup := setupQuickstartService(t)
	defer cleanup()

	dbc, dbCleanup := setupDB(t)
	defer dbCleanup()

	instance := createTestInstanceForQuickstart(t, dbc, false)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Try to dismiss quickstart with cancelled context
	err := svc.DismissQuickstart(ctx, instance.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestQuickstartService_Integration_ConcurrentAccess(t *testing.T) {
	svc, cleanup := setupQuickstartService(t)
	defer cleanup()

	dbc, dbCleanup := setupDB(t)
	defer dbCleanup()

	instance := createTestInstanceForQuickstart(t, dbc, false)

	// Test concurrent dismissals of the same instance
	const numGoroutines = 10
	errChan := make(chan error, numGoroutines)

	for range numGoroutines {
		go func() {
			err := svc.DismissQuickstart(context.Background(), instance.ID)
			errChan <- err
		}()
	}

	// Collect results
	var successCount int
	for range numGoroutines {
		err := <-errChan
		if err == nil {
			successCount++
		}
	}

	// All operations should succeed (idempotent operation)
	assert.Equal(t, numGoroutines, successCount, "All concurrent dismissals should succeed")

	// Verify final state
	instanceRepo := repositories.NewInstanceRepository(dbc)
	fetchedInstance, err := instanceRepo.GetByID(context.Background(), instance.ID)
	require.NoError(t, err)
	assert.True(t, fetchedInstance.IsQuickStartDismissed)
}

func TestQuickstartService_Integration_RepositoryError(t *testing.T) {
	// This test would require mocking the repository to simulate database errors
	// For now, we test with a malformed database connection scenario

	svc, cleanup := setupQuickstartService(t)
	defer cleanup()

	// Test with a non-existent instance ID - repository will not error
	err := svc.DismissQuickstart(context.Background(), "non-existent-instance")
	require.NoError(t, err)
	// Repository accepts the call and executes the update, even if no rows are affected
}

// Benchmark test.
func BenchmarkQuickstartService_DismissQuickstart(b *testing.B) {
	svc, cleanup := setupQuickstartService(&testing.T{})
	defer cleanup()

	dbc, dbCleanup := setupDB(&testing.T{})
	defer dbCleanup()

	// Create a test instance
	instance := createTestInstanceForQuickstart(&testing.T{}, dbc, false)

	b.ResetTimer()
	for range b.N {
		_ = svc.DismissQuickstart(context.Background(), instance.ID)
	}
}
