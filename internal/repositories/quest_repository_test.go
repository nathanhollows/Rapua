package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupInstanceRepo(t *testing.T) (repositories.QuestRepository, db.Transactor, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	instanceRepo := repositories.NewQuestRepository(dbc)
	return instanceRepo, transactor, dbc, cleanup
}

func TestQuestRepository_Create(t *testing.T) {
	testCases := []struct {
		name       string
		instanceFn func() *models.Quest
		wantErr    bool
	}{
		{
			name: "Valid instance",
			instanceFn: func() *models.Quest {
				return &models.Quest{
					Name:   gofakeit.Word(),
					UserID: gofakeit.UUID(),
				}
			},
			wantErr: false,
		},
		{
			name: "Missing UserID",
			instanceFn: func() *models.Quest {
				return &models.Quest{
					Name: gofakeit.Word(),
					// No UserID
				}
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo, _, dbc, cleanup := setupInstanceRepo(t)
			defer cleanup()

			inst := tc.instanceFn()
			if inst.UserID != "" {
				insertUserWithID(t, dbc, inst.UserID)
			}
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

func TestQuestRepository_FindByID(t *testing.T) {
	testCases := []struct {
		name    string
		setupFn func(context.Context, *testing.T, repositories.QuestRepository, *bun.DB, models.Quest)
		questID string
		wantErr bool
	}{
		{
			name: "Existing instance",
			setupFn: func(
				ctx context.Context, t *testing.T,
				repo repositories.QuestRepository, dbc *bun.DB, inst models.Quest,
			) {
				t.Helper()
				insertUserWithID(t, dbc, inst.UserID)
				require.NoError(t, repo.Create(ctx, &inst))
			},
			wantErr: false,
		},
		{
			name: "Non-existent instance",
			setupFn: func(
				_ context.Context, _ *testing.T,
				_ repositories.QuestRepository, _ *bun.DB, _ models.Quest,
			) {
				// Intentionally do not save
			},
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo, _, dbc, cleanup := setupInstanceRepo(t)
			defer cleanup()

			ctx := context.Background()
			inst := models.Quest{
				ID:     gofakeit.UUID(),
				Name:   gofakeit.Word(),
				UserID: gofakeit.UUID(),
			}
			tc.setupFn(ctx, t, repo, dbc, inst)

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

func TestQuestRepository_FindByUserID(t *testing.T) {
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
			repo, _, dbc, cleanup := setupInstanceRepo(t)
			defer cleanup()

			ctx := context.Background()
			insertUserWithID(t, dbc, tc.userID)
			// Create instances for the given user
			for range tc.count {
				inst := &models.Quest{
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

func TestQuestRepository_FindTemplates(t *testing.T) {
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
			repo, _, dbc, cleanup := setupInstanceRepo(t)
			defer cleanup()

			ctx := context.Background()
			insertUserWithID(t, dbc, tc.userID)
			// Create instances for the given user
			for range tc.templateCount {
				inst := &models.Quest{
					ID:         gofakeit.UUID(),
					Name:       gofakeit.Word(),
					UserID:     tc.userID,
					IsTemplate: true,
				}
				err := repo.Create(ctx, inst)
				require.NoError(t, err)
			}
			for range tc.instanceCount {
				inst := &models.Quest{
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

func TestQuestRepository_Update(t *testing.T) {
	testCases := []struct {
		name       string
		setupFn    func(context.Context, testing.TB) (models.Quest, error)
		updateName string
		wantErr    bool
	}{
		{
			name: "Update existing instance",
			setupFn: func(ctx context.Context, tb testing.TB) (models.Quest, error) {
				innerRepo, _, innerDbc, _ := setupInstanceRepo(tb.(*testing.T))
				inst := models.Quest{
					ID:     gofakeit.UUID(),
					Name:   gofakeit.Word(),
					UserID: gofakeit.UUID(),
				}
				insertUserWithID(tb.(*testing.T), innerDbc, inst.UserID)
				err := innerRepo.Create(ctx, &inst)
				return inst, err
			},
			updateName: gofakeit.BeerName(),
			wantErr:    false,
		},
		{
			name: "Update non-existent instance",
			setupFn: func(_ context.Context, _ testing.TB) (models.Quest, error) {
				// Return an instance that hasn't been created
				inst := models.Quest{
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
			repo, _, _, cleanup := setupInstanceRepo(t)
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

func TestQuestRepository_Delete(t *testing.T) {
	repo, transactor, dbc, cleanup := setupInstanceRepo(t)
	defer cleanup()

	testCases := []struct {
		name    string
		setupFn func(ctx context.Context, t *testing.T) (models.Quest, *bun.Tx, error)
		wantErr bool
	}{
		{
			name: "Delete existing instance",
			setupFn: func(ctx context.Context, _ *testing.T) (models.Quest, *bun.Tx, error) {
				tx, err := transactor.BeginTx(ctx, nil)
				if err != nil {
					return models.Quest{}, nil, err
				}

				inst := models.Quest{
					ID:     gofakeit.UUID(),
					Name:   gofakeit.Word(),
					UserID: gofakeit.UUID(),
				}
				insertUserWithID(t, dbc, inst.UserID)
				err = repo.Create(ctx, &inst)
				return inst, tx, err
			},
			wantErr: false,
		},
		{
			name: "Delete non-existent instance",
			setupFn: func(ctx context.Context, _ *testing.T) (models.Quest, *bun.Tx, error) {
				// For a non-existent instance, just return an instance that wasn't saved
				tx, err := transactor.BeginTx(ctx, nil)
				if err != nil {
					return models.Quest{}, nil, err
				}

				inst := models.Quest{
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

func TestQuestRepository_Delete_CascadesObjectives(t *testing.T) {
	repo, transactor, dbc, cleanup := setupInstanceRepo(t)
	defer cleanup()

	ctx := context.Background()
	inst := models.Quest{
		ID:     gofakeit.UUID(),
		Name:   gofakeit.Word(),
		UserID: gofakeit.UUID(),
	}
	insertUserWithID(t, dbc, inst.UserID)
	require.NoError(t, repo.Create(ctx, &inst))

	objective := &models.Objective{
		ID:      gofakeit.UUID(),
		QuestID: inst.ID,
		Slug:    gofakeit.Word() + "-objective",
		Title:   gofakeit.Sentence(3),
	}
	_, err := dbc.NewInsert().Model(objective).Exec(ctx)
	require.NoError(t, err)

	tx, err := transactor.BeginTx(ctx, nil)
	require.NoError(t, err)
	require.NoError(t, repo.Delete(ctx, tx, inst.ID))
	require.NoError(t, tx.Commit())

	count, err := dbc.NewSelect().Model((*models.Objective)(nil)).Where("id = ?", objective.ID).Count(ctx)
	require.NoError(t, err)
	assert.Zero(t, count, "objective should be deleted when its quest is deleted")
}

func TestQuestRepository_DeleteByUser(t *testing.T) {
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
			repo, transactor, dbc, cleanup := setupInstanceRepo(t)
			defer cleanup()

			tx, err := transactor.BeginTx(context.Background(), nil)
			require.NoError(t, err)

			ctx := context.Background()
			insertUserWithID(t, dbc, tc.userID)
			// Create some instances for this user
			for range tc.count {
				inst := models.Quest{
					ID:     gofakeit.UUID(),
					Name:   gofakeit.Word(),
					UserID: tc.userID,
				}
				createErr := repo.Create(ctx, &inst)
				require.NoError(t, createErr)
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
				found, findErr := repo.FindByUserID(ctx, tc.userID)
				require.NoError(t, findErr)
				assert.Empty(t, found)
			}
		})
	}
}

func TestQuestRepository_DismissQuickstart(t *testing.T) {
	testCases := []struct {
		name    string
		questID string
		setupFn func(context.Context, testing.TB, string) error
		wantErr bool
	}{
		{
			name:    "Dismiss quickstart for existing user",
			questID: gofakeit.UUID(),
			setupFn: func(ctx context.Context, tb testing.TB, questID string) error {
				innerRepo, _, innerDbc, _ := setupInstanceRepo(tb.(*testing.T))
				inst := models.Quest{
					ID:                    questID,
					Name:                  gofakeit.Word(),
					UserID:                gofakeit.UUID(),
					IsQuickStartDismissed: false,
				}
				insertUserWithID(tb.(*testing.T), innerDbc, inst.UserID)
				return innerRepo.Create(ctx, &inst)
			},
			wantErr: false,
		},
		{
			name:    "Dismiss quickstart for user who has no instances",
			questID: gofakeit.UUID(),
			setupFn: func(_ context.Context, _ testing.TB, _ string) error {
				return nil
			},
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			repo, _, _, cleanup := setupInstanceRepo(t)
			defer cleanup()

			ctx := context.Background()
			err := tc.setupFn(ctx, t, tc.questID)
			require.NoError(t, err)

			err = repo.DismissQuickstart(ctx, tc.questID)
			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				instances, findErr := repo.FindByUserID(ctx, tc.questID)
				require.NoError(t, findErr)
				for _, inst := range instances {
					assert.True(t, inst.IsQuickStartDismissed, "quickstart should be dismissed")
				}
			}
		})
	}
}

func TestQuestRepository_CreateTx(t *testing.T) {
	repo, transactor, dbc, cleanup := setupInstanceRepo(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("creates instance within transaction", func(t *testing.T) {
		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		defer tx.Rollback()

		instance := &models.Quest{
			Name:   gofakeit.Word(),
			UserID: gofakeit.UUID(),
		}
		insertUserWithID(t, dbc, instance.UserID)

		err = repo.CreateTx(ctx, tx, instance)
		require.NoError(t, err)
		assert.NotEmpty(t, instance.ID, "ID should be generated")

		err = tx.Commit()
		require.NoError(t, err)

		// Verify instance was created
		found, err := repo.GetByID(ctx, instance.ID)
		require.NoError(t, err)
		assert.Equal(t, instance.Name, found.Name)
		assert.Equal(t, instance.UserID, found.UserID)
	})

	t.Run("rolls back on transaction failure", func(t *testing.T) {
		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)

		instance := &models.Quest{
			Name:   gofakeit.Word(),
			UserID: gofakeit.UUID(),
		}
		insertUserWithID(t, dbc, instance.UserID)

		err = repo.CreateTx(ctx, tx, instance)
		require.NoError(t, err)

		// Rollback transaction
		err = tx.Rollback()
		require.NoError(t, err)

		// Verify instance was NOT created
		_, err = repo.GetByID(ctx, instance.ID)
		require.Error(t, err)
	})

	t.Run("validates required fields", func(t *testing.T) {
		tx, err := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, err)
		defer tx.Rollback()

		instance := &models.Quest{
			Name: gofakeit.Word(),
			// Missing UserID
		}

		err = repo.CreateTx(ctx, tx, instance)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "UserID is required")
	})
}
