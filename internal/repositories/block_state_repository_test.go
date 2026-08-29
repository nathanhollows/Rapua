package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
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
