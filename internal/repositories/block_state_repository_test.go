package repositories_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupBlockStateRepo(t *testing.T) (repositories.BlockStateRepository, db.Transactor, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	blockStateRepository := repositories.NewBlockStateRepository(dbc)
	return blockStateRepository, transactor, dbc, cleanup
}

func TestBlockStateRepository(t *testing.T) {
	repo, _, dbc, cleanup := setupBlockStateRepo(t)
	defer cleanup()

	// Create one parent chain; each test case uses its own block+team drawn from these parents.
	parents := createTestParents(t, dbc)

	tests := []struct {
		name        string
		setup       func() (blocks.PlayerState, error)
		action      func(state blocks.PlayerState) (any, error)
		assertion   func(result any, err error)
		cleanupFunc func(state blocks.PlayerState)
	}{
		{
			name: "Create new player state",
			setup: func() (blocks.PlayerState, error) {
				blockID := createTestBlock(t, dbc, gofakeit.UUID())
				runCode := createTestTeam(t, dbc, parents.QuestID)
				return repo.NewBlockState(context.Background(), blockID, runCode, parents.QuestID)
			},
			action: func(state blocks.PlayerState) (any, error) {
				return repo.Create(context.Background(), state)
			},
			assertion: func(result any, err error) {
				require.NoError(t, err)
				assert.NotNil(t, result)
			},
			cleanupFunc: func(state blocks.PlayerState) {
				err := repo.Delete(context.Background(), state.GetBlockID(), state.GetPlayerID(), parents.QuestID)
				require.NoError(t, err)
			},
		},
		{
			name: "Get player state by block and team",
			setup: func() (blocks.PlayerState, error) {
				blockID := createTestBlock(t, dbc, gofakeit.UUID())
				runCode := createTestTeam(t, dbc, parents.QuestID)
				state, _ := repo.NewBlockState(context.Background(), blockID, runCode, parents.QuestID)
				return repo.Create(context.Background(), state)
			},
			action: func(state blocks.PlayerState) (any, error) {
				return repo.GetByBlockAndRun(
					context.Background(), state.GetBlockID(), state.GetPlayerID(), parents.QuestID,
				)
			},
			assertion: func(result any, err error) {
				require.NoError(t, err)
				assert.NotNil(t, result)
			},
			cleanupFunc: func(state blocks.PlayerState) {
				err := repo.Delete(context.Background(), state.GetBlockID(), state.GetPlayerID(), parents.QuestID)
				require.NoError(t, err)
			},
		},
		{
			name: "Update player state",
			setup: func() (blocks.PlayerState, error) {
				blockID := createTestBlock(t, dbc, gofakeit.UUID())
				runCode := createTestTeam(t, dbc, parents.QuestID)
				state, _ := repo.NewBlockState(context.Background(), blockID, runCode, parents.QuestID)
				createdState, _ := repo.Create(context.Background(), state)
				return createdState, nil
			},
			action: func(state blocks.PlayerState) (any, error) {
				state.SetPlayerData([]byte(`{"key":"value"}`))
				state.SetComplete(true)
				state.SetPointsAwarded(100)
				return repo.Update(context.Background(), state)
			},
			assertion: func(result any, err error) {
				require.NoError(t, err)
				updatedState := result.(blocks.PlayerState)
				assert.True(t, updatedState.IsComplete())
				assert.Equal(t, 100, updatedState.GetPointsAwarded())
			},
			cleanupFunc: func(state blocks.PlayerState) {
				err := repo.Delete(context.Background(), state.GetBlockID(), state.GetPlayerID(), parents.QuestID)
				require.NoError(t, err)
			},
		},
		{
			name: "Delete player state",
			setup: func() (blocks.PlayerState, error) {
				blockID := createTestBlock(t, dbc, gofakeit.UUID())
				runCode := createTestTeam(t, dbc, parents.QuestID)
				state, _ := repo.NewBlockState(context.Background(), blockID, runCode, parents.QuestID)
				return repo.Create(context.Background(), state)
			},
			action: func(state blocks.PlayerState) (any, error) {
				return nil, repo.Delete(context.Background(), state.GetBlockID(), state.GetPlayerID(), parents.QuestID)
			},
			assertion: func(_ any, err error) {
				require.NoError(t, err)
			},
			cleanupFunc: func(_ blocks.PlayerState) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := tt.setup()
			require.NoError(t, err)
			result, err := tt.action(state)
			tt.assertion(result, err)
			if tt.cleanupFunc != nil {
				tt.cleanupFunc(state)
			}
		})
	}
}

// LastSeenByRun is what the activity page's "last seen" reads, because
// runs.updated_at is stamped once at insert and never advanced.
func TestBlockStateRepository_LastSeenByRun(t *testing.T) {
	repo, _, dbc, cleanup := setupBlockStateRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)

	active := createTestTeam(t, dbc, parents.QuestID)
	quiet := createTestTeam(t, dbc, parents.QuestID)
	idle := createTestTeam(t, dbc, parents.QuestID)

	older := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	newer := time.Now().UTC().Add(-time.Minute).Truncate(time.Second)

	// Touched directly rather than through Create, which leaves updated_at to
	// the schema default and so cannot express "an hour ago".
	touch := func(runCode string, at time.Time) {
		t.Helper()
		state := &models.RunBlockState{
			RunCode: runCode,
			BlockID: createTestBlock(t, dbc, gofakeit.UUID()),
			QuestID: parents.QuestID,
		}
		state.UpdatedAt = at
		_, err := dbc.NewInsert().Model(state).Exec(ctx)
		require.NoError(t, err)
	}

	touch(active, older)
	touch(active, newer)
	touch(quiet, older)

	lastSeen, err := repo.LastSeenByRun(ctx, parents.QuestID)
	require.NoError(t, err)

	// The newer of a run's two touches is the one that answers "last seen".
	assert.WithinDuration(t, newer, lastSeen[active], time.Second)
	assert.WithinDuration(t, older, lastSeen[quiet], time.Second)

	_, ok := lastSeen[idle]
	assert.False(t, ok, "a run with no block state is absent from the map rather than reported as zero")
}

// The guard against two submissions completing one block: the loser matches no row
// and says so.
func TestBlockStateRepository_UpdateIfIncompleteOnlyWritesOnce(t *testing.T) {
	repo, transactor, dbc, cleanup := setupBlockStateRepo(t)
	defer cleanup()

	ctx := context.Background()
	parents := createTestParents(t, dbc)
	blockID := createTestBlock(t, dbc, gofakeit.UUID())
	runCode := createTestTeam(t, dbc, parents.QuestID)

	state, err := repo.NewBlockState(ctx, blockID, runCode, parents.QuestID)
	require.NoError(t, err)
	created, err := repo.Create(ctx, state)
	require.NoError(t, err)

	completeOnce := func() bool {
		t.Helper()
		tx, err := transactor.BeginTx(ctx, nil)
		require.NoError(t, err)
		created.SetComplete(true)
		won, err := repo.UpdateIfIncompleteWithTx(ctx, tx, created)
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
		return won
	}

	assert.True(t, completeOnce(), "the first submission is the one that completes the block")
	assert.False(t, completeOnce(), "a second submission matches no row and records nothing")
}

func verifyStatesDeleted(repo repositories.BlockStateRepository, states []blocks.PlayerState, questID string) error {
	for _, s := range states {
		_, getErr := repo.GetByBlockAndRun(context.Background(), s.GetBlockID(), s.GetPlayerID(), questID)
		if getErr.Error() != "sql: no rows in result set" {
			return getErr
		}
	}
	return nil
}

func TestBlockStateRepository_Bulk(t *testing.T) {
	repo, transactor, dbc, cleanup := setupBlockStateRepo(t)
	defer cleanup()

	parents := createTestParents(t, dbc)

	tests := []struct {
		name        string
		setup       func() ([]blocks.PlayerState, error)
		action      func(state []blocks.PlayerState) (any, error)
		assertion   func(result any, err error)
		cleanupFunc func(state []blocks.PlayerState)
	}{
		{
			name: "Delete player states by block ID",
			setup: func() ([]blocks.PlayerState, error) {
				blockID := createTestBlock(t, dbc, gofakeit.UUID())
				playerStates := make([]blocks.PlayerState, 3)
				for i := range 3 {
					runCode := createTestTeam(t, dbc, parents.QuestID)
					state, _ := repo.NewBlockState(context.Background(), blockID, runCode, parents.QuestID)
					ps, err := repo.Create(context.Background(), state)
					playerStates[i] = ps
					if err != nil {
						return nil, err
					}
				}
				return playerStates, nil
			},
			action: func(state []blocks.PlayerState) (any, error) {
				tx, err := transactor.BeginTx(context.Background(), &sql.TxOptions{})
				if err != nil {
					return nil, err
				}

				err = repo.DeleteByBlockID(context.Background(), tx, state[0].GetBlockID())
				if err != nil {
					if err2 := tx.Rollback(); err2 != nil {
						return nil, err2
					}
					return nil, err
				}

				if commitErr := tx.Commit(); commitErr != nil {
					return nil, commitErr
				}

				if verifyErr := verifyStatesDeleted(repo, state, parents.QuestID); verifyErr != nil {
					return nil, verifyErr
				}

				return "deletion verified", nil
			},
			assertion: func(_ any, err error) {
				require.NoError(t, err)
			},
			// cleanup is what we're testing
			cleanupFunc: func(_ []blocks.PlayerState) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := tt.setup()
			require.NoError(t, err)
			result, err := tt.action(state)
			tt.assertion(result, err)
			if tt.cleanupFunc != nil {
				tt.cleanupFunc(state)
			}
		})
	}
}
