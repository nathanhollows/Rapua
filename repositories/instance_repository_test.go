package repositories_test

import (
	"context"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupInstanceRepo(t *testing.T) (repositories.InstanceRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	instanceRepo := repositories.NewInstanceRepository(dbc)
	return instanceRepo, transactor, cleanup
}

func TestInstanceRepository_Create(t *testing.T) {
	testCases := []struct {
		name       string
		instanceFn func() *models.Instance
		wantErr    bool
	}{
		{
			name: "Valid instance",
			instanceFn: func() *models.Instance {
				return &models.Instance{
					Name:   gofakeit.Word(),
					UserID: gofakeit.UUID(),
				}
			},
			wantErr: false,
		},
		{
			name: "Missing UserID",
			instanceFn: func() *models.Instance {
				return &models.Instance{
					Name: gofakeit.Word(),
					// No UserID
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo, _, cleanup := setupInstanceRepo(t)
			defer cleanup()

			inst := tc.instanceFn()
			err := repo.Create(context.Background(), inst)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, inst.ID)
			}
		})
	}
}

func TestInstanceRepository_FindByID(t *testing.T) {
	testCases := []struct {
		name       string
		setupFn    func(context.Context, testing.TB, models.Instance, *testing.T)
		instanceID string
		wantErr    bool
	}{
		{
			name: "Existing instance",
			setupFn: func(ctx context.Context, tb testing.TB, inst models.Instance, t *testing.T) {
				repo, _, cleanup := setupInstanceRepo(t)
				defer cleanup()
				err := repo.Create(ctx, &inst)
				require.NoError(t, err)
			},
			wantErr: false,
		},
		{
			name: "Non-existent instance",
			setupFn: func(ctx context.Context, tb testing.TB, inst models.Instance, t *testing.T) {
				// Intentionally do not save
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo, _, cleanup := setupInstanceRepo(t)
			defer cleanup()

			ctx := context.Background()
			inst := models.Instance{
				ID:     gofakeit.UUID(),
				Name:   gofakeit.Word(),
				UserID: gofakeit.UUID(),
			}
			tc.setupFn(ctx, t, inst, t)

			found, err := repo.GetByID(ctx, inst.ID)
			if tc.wantErr {
				require.Error(t, err)
				assert.Nil(t, found)
			} else {
				require.NoError(t, err)
				assert.NotNil(t, found)
				assert.Equal(t, inst.ID, found.ID)
				assert.Equal(t, inst.Name, found.Name)
			}
		})
	}
}

func TestInstanceRepository_FindByUserID(t *testing.T) {
	testCases := []struct {
		name    string
		userID  string
		count   int
		wantErr bool
	}{
		{
			name:   "Multiple instances for one user",
			userID: gofakeit.UUID(),
			count:  3,
		},
		{
			name:   "No instances for this user",
			userID: gofakeit.UUID(),
			count:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo, _, cleanup := setupInstanceRepo(t)
			defer cleanup()

			ctx := context.Background()
			// Create instances for the given user
			for range tc.count {
				inst := &models.Instance{
					ID:     gofakeit.UUID(),
					Name:   gofakeit.Word(),
					UserID: tc.userID,
				}
				err := repo.Create(ctx, inst)
				require.NoError(t, err)
			}

			instances, err := repo.FindByUserID(ctx, tc.userID)
			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, instances)
			} else {
				require.NoError(t, err)
				assert.Len(t, instances, tc.count)
			}
		})
	}
}

func TestInstanceRepository_FindTemplates(t *testing.T) {
	testCases := []struct {
		name          string
		userID        string
		templateCount int
		instanceCount int
		wantErr       bool
	}{
		{
			name:          "Multiple templates for one user",
			userID:        gofakeit.UUID(),
			templateCount: 3,
			instanceCount: 2,
			wantErr:       false,
		},
		{
			name:          "No templates for this user",
			userID:        gofakeit.UUID(),
			templateCount: 0,
			instanceCount: 2,
			wantErr:       false,
		},
		{
			name:          "No instances for this user",
			userID:        gofakeit.UUID(),
			templateCount: 2,
			instanceCount: 0,
			wantErr:       false,
		},
		{
			name:          "No instances or templates for this user",
			userID:        gofakeit.UUID(),
			templateCount: 0,
			instanceCount: 0,
			wantErr:       false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo, _, cleanup := setupInstanceRepo(t)
			defer cleanup()

			ctx := context.Background()
			// Create instances for the given user
			for range tc.templateCount {
				inst := &models.Instance{
					ID:         gofakeit.UUID(),
					Name:       gofakeit.Word(),
					UserID:     tc.userID,
					IsTemplate: true,
				}
				err := repo.Create(ctx, inst)
				require.NoError(t, err)
			}
			for range tc.instanceCount {
				inst := &models.Instance{
					ID:     gofakeit.UUID(),
					Name:   gofakeit.Word(),
					UserID: tc.userID,
				}
				err := repo.Create(ctx, inst)
				require.NoError(t, err)
			}

			instances, err := repo.FindTemplates(ctx, tc.userID)
			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, instances)
			} else {
				require.NoError(t, err)
				assert.Len(t, instances, tc.templateCount)
			}
		})
	}
}

func TestInstanceRepository_Update(t *testing.T) {
	testCases := []struct {
		name       string
		setupFn    func(context.Context, testing.TB) (models.Instance, error)
		updateName string
		wantErr    bool
	}{
		{
			name: "Update existing instance",
			setupFn: func(ctx context.Context, tb testing.TB) (models.Instance, error) {
				repo, _, _ := setupInstanceRepo(tb.(*testing.T))
				defer func() {}()
				inst := models.Instance{
					ID:     gofakeit.UUID(),
					Name:   gofakeit.Word(),
					UserID: gofakeit.UUID(),
				}
				err := repo.Create(ctx, &inst)
				return inst, err
			},
			updateName: gofakeit.BeerName(),
			wantErr:    false,
		},
		{
			name: "Update non-existent instance",
			setupFn: func(ctx context.Context, tb testing.TB) (models.Instance, error) {
				// Return an instance that hasn't been created
				inst := models.Instance{
					ID:     gofakeit.UUID(),
					Name:   gofakeit.Word(),
					UserID: gofakeit.UUID(),
				}
				return inst, nil
			},
			updateName: gofakeit.BeerName(),
			wantErr:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo, _, cleanup := setupInstanceRepo(t)
			defer cleanup()

			ctx := context.Background()
			inst, err := tc.setupFn(ctx, t)
			require.NoError(t, err)

			// Now attempt to update
			inst.Name = tc.updateName
			err = repo.Update(ctx, &inst)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				// Check that it's updated
				updated, getErr := repo.GetByID(ctx, inst.ID)
				require.NoError(t, getErr)
				assert.Equal(t, tc.updateName, updated.Name)
			}
		})
	}
}

func TestInstanceRepository_Delete(t *testing.T) {
	repo, transactor, cleanup := setupInstanceRepo(t)
	defer cleanup()

	testCases := []struct {
		name    string
		setupFn func(ctx context.Context, t *testing.T) (models.Instance, *bun.Tx, error)
		wantErr bool
	}{
		{
			name: "Delete existing instance",
			setupFn: func(ctx context.Context, t *testing.T) (models.Instance, *bun.Tx, error) {
				tx, err := transactor.BeginTx(ctx, nil)
				if err != nil {
					return models.Instance{}, nil, err
				}

				inst := models.Instance{
					ID:     gofakeit.UUID(),
					Name:   gofakeit.Word(),
					UserID: gofakeit.UUID(),
				}
				err = repo.Create(ctx, &inst)
				return inst, tx, err
			},
			wantErr: false,
		},
		{
			name: "Delete non-existent instance",
			setupFn: func(ctx context.Context, t *testing.T) (models.Instance, *bun.Tx, error) {
				// For a non-existent instance, just return an instance that wasn't saved
				tx, err := transactor.BeginTx(ctx, nil)
				if err != nil {
					return models.Instance{}, nil, err
				}

				inst := models.Instance{
					ID:   gofakeit.UUID(),
					Name: gofakeit.Word(),
				}
				return inst, tx, nil
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()

			inst, tx, err := tc.setupFn(ctx, t)
			if err != nil {
				t.Fatalf("setupFn failed: %v", err)
			}

			// Call Delete
			err = repo.Delete(ctx, tx, inst.ID)

			if tc.wantErr {
				rollbackErr := tx.Rollback()
				if rollbackErr != nil {
					t.Fatalf("failed to rollback transaction: %v", rollbackErr)
				}
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				err = tx.Commit()
				require.NoError(t, err)

				// Double-check that the instance no longer exists
				found, findErr := repo.GetByID(ctx, inst.ID)
				require.Error(t, findErr)
				assert.Nil(t, found)
			}
		})
	}
}

func TestInstanceRepository_DeleteByUser(t *testing.T) {
	testCases := []struct {
		name    string
		userID  string
		count   int
		wantErr bool
	}{
		{
			name:   "Delete multiple instances for user",
			userID: gofakeit.UUID(),
			count:  3,
		},
		{
			name:   "No instances for user, but no error expected",
			userID: gofakeit.UUID(),
			count:  0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo, transactor, cleanup := setupInstanceRepo(t)
			defer cleanup()

			tx, err := transactor.BeginTx(context.Background(), nil)
			require.NoError(t, err)

			ctx := context.Background()
			// Create some instances for this user
			for range tc.count {
				inst := models.Instance{
					ID:     gofakeit.UUID(),
					Name:   gofakeit.Word(),
					UserID: tc.userID,
				}
				err := repo.Create(ctx, &inst)
				require.NoError(t, err)
			}

			err = repo.DeleteByUser(ctx, tx, tc.userID)
			if tc.wantErr {
				rollbackErr := tx.Rollback()
				if rollbackErr != nil {
					t.Fatalf("failed to rollback transaction: %v", rollbackErr)
				}
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				err = tx.Commit()
				require.NoError(t, err)
				// Ensure none remain
				found, err := repo.FindByUserID(ctx, tc.userID)
				require.NoError(t, err)
				assert.Empty(t, found)
			}
		})
	}
}

func TestInstanceRepository_DismissQuickstart(t *testing.T) {
	testCases := []struct {
		name       string
		instanceID string
		setupFn    func(context.Context, testing.TB, string) error
		wantErr    bool
	}{
		{
			name:       "Dismiss quickstart for existing user",
			instanceID: gofakeit.UUID(),
			setupFn: func(ctx context.Context, tb testing.TB, instanceID string) error {
				// Optionally create some instances for this user
				repo, _, _ := setupInstanceRepo(tb.(*testing.T))
				defer func() {}()
				inst := models.Instance{
					ID:                    instanceID,
					Name:                  gofakeit.Word(),
					UserID:                gofakeit.UUID(),
					IsQuickStartDismissed: false,
				}
				return repo.Create(ctx, &inst)
			},
			wantErr: false,
		},
		{
			name:       "Dismiss quickstart for user who has no instances",
			instanceID: gofakeit.UUID(),
			setupFn: func(ctx context.Context, tb testing.TB, instanceID string) error {
				return nil
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo, _, cleanup := setupInstanceRepo(t)
			defer cleanup()

			ctx := context.Background()
			err := tc.setupFn(ctx, t, tc.instanceID)
			require.NoError(t, err)

			err = repo.DismissQuickstart(ctx, tc.instanceID)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				instances, findErr := repo.FindByUserID(ctx, tc.instanceID)
				require.NoError(t, findErr)
				for _, inst := range instances {
					assert.True(t, inst.IsQuickStartDismissed, "quickstart should be dismissed")
				}
			}
		})
	}
}
