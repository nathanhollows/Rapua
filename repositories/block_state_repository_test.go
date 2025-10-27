package repositories_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v5/blocks"
	"github.com/nathanhollows/Rapua/v5/db"
	"github.com/nathanhollows/Rapua/v5/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupBlockStateRepo(t *testing.T) (repositories.BlockStateRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	blockStateRepository := repositories.NewBlockStateRepository(dbc)
	return blockStateRepository, transactor, cleanup
}

func TestBlockStateRepository(t *testing.T) {
	repo, _, cleanup := setupBlockStateRepo(t)
	defer cleanup()

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
				return repo.NewBlockState(context.Background(), gofakeit.UUID(), gofakeit.UUID())
			},
			action: func(state blocks.PlayerState) (any, error) {
				return repo.Create(context.Background(), state)
			},
			assertion: func(result any, err error) {
				require.NoError(t, err)
				assert.NotNil(t, result)
			},
			cleanupFunc: func(state blocks.PlayerState) {
				err := repo.Delete(context.Background(), state.GetBlockID(), state.GetPlayerID())
				require.NoError(t, err)
			},
		},
		{
			name: "Get player state by block and team",
			setup: func() (blocks.PlayerState, error) {
				state, _ := repo.NewBlockState(context.Background(), gofakeit.UUID(), gofakeit.UUID())
				return repo.Create(context.Background(), state)
			},
			action: func(state blocks.PlayerState) (any, error) {
				return repo.GetByBlockAndTeam(context.Background(), state.GetBlockID(), state.GetPlayerID())
			},
			assertion: func(result any, err error) {
				require.NoError(t, err)
				assert.NotNil(t, result)
			},
			cleanupFunc: func(state blocks.PlayerState) {
				err := repo.Delete(context.Background(), state.GetBlockID(), state.GetPlayerID())
				require.NoError(t, err)
			},
		},
		{
			name: "Update player state",
			setup: func() (blocks.PlayerState, error) {
				state, _ := repo.NewBlockState(context.Background(), gofakeit.UUID(), gofakeit.UUID())
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
				err := repo.Delete(context.Background(), state.GetBlockID(), state.GetPlayerID())
				require.NoError(t, err)
			},
		},
		{
			name: "Delete player state",
			setup: func() (blocks.PlayerState, error) {
				state, _ := repo.NewBlockState(context.Background(), gofakeit.UUID(), gofakeit.UUID())
				return repo.Create(context.Background(), state)
			},
			action: func(state blocks.PlayerState) (any, error) {
				return nil, repo.Delete(context.Background(), state.GetBlockID(), state.GetPlayerID())
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

func verifyStatesDeleted(repo repositories.BlockStateRepository, states []blocks.PlayerState) error {
	for _, s := range states {
		_, getErr := repo.GetByBlockAndTeam(context.Background(), s.GetBlockID(), s.GetPlayerID())
		if getErr.Error() != "sql: no rows in result set" {
			return getErr
		}
	}
	return nil
}

func TestBlockStateRepository_Bulk(t *testing.T) {
	repo, transactor, cleanup := setupBlockStateRepo(t)
	defer cleanup()

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
				blockID := gofakeit.UUID()
				playerStates := make([]blocks.PlayerState, 3)
				for i := range 3 {
					state, _ := repo.NewBlockState(context.Background(), blockID, gofakeit.UUID())
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

				if verifyErr := verifyStatesDeleted(repo, state); verifyErr != nil {
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

func verifyStatesDeletedWithCheck(repo repositories.BlockStateRepository, states []blocks.PlayerState) error {
	for _, s := range states {
		_, getErr := repo.GetByBlockAndTeam(context.Background(), s.GetBlockID(), s.GetPlayerID())
		if getErr == nil || !errors.Is(getErr, sql.ErrNoRows) {
			return errors.New("player state was not deleted")
		}
	}
	return nil
}

func TestBlockStateRepository_DeleteByTeamCodes(t *testing.T) {
	repo, transactor, cleanup := setupBlockStateRepo(t)
	defer cleanup()

	tests := []struct {
		name        string
		setup       func() ([]blocks.PlayerState, []string, error)
		action      func(state []blocks.PlayerState, teamCodes []string) (any, error)
		assertion   func(result any, err error)
		cleanupFunc func(state []blocks.PlayerState)
	}{
		{
			name: "Delete player states by team codes",
			setup: func() ([]blocks.PlayerState, []string, error) {
				teamCodes := []string{gofakeit.UUID(), gofakeit.UUID()}
				playerStates := make([]blocks.PlayerState, 4)
				for i := range 4 {
					blockID := gofakeit.UUID()
					state, _ := repo.NewBlockState(context.Background(), blockID, teamCodes[i%2])
					ps, err := repo.Create(context.Background(), state)
					playerStates[i] = ps
					if err != nil {
						return nil, nil, err
					}
				}
				return playerStates, teamCodes, nil
			},
			action: func(state []blocks.PlayerState, teamCodes []string) (any, error) {
				tx, err := transactor.BeginTx(context.Background(), &sql.TxOptions{})
				if err != nil {
					return nil, err
				}

				err = repo.DeleteByTeamCodes(context.Background(), tx, teamCodes)
				if err != nil {
					if err2 := tx.Rollback(); err2 != nil {
						return nil, err2
					}
					return nil, err
				}

				if commitErr := tx.Commit(); commitErr != nil {
					return nil, commitErr
				}

				if verifyErr := verifyStatesDeletedWithCheck(repo, state); verifyErr != nil {
					return nil, verifyErr
				}

				return "deletion verified", nil
			},
			assertion: func(_ any, err error) {
				require.NoError(t, err)
			},
			cleanupFunc: func(_ []blocks.PlayerState) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, teamCodes, err := tt.setup()
			require.NoError(t, err)
			result, err := tt.action(state, teamCodes)
			tt.assertion(result, err)
			if tt.cleanupFunc != nil {
				tt.cleanupFunc(state)
			}
		})
	}
}
