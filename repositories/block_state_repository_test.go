package repositories_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v4/blocks"
	"github.com/nathanhollows/Rapua/v4/db"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
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
		action      func(state blocks.PlayerState) (interface{}, error)
		assertion   func(result interface{}, err error)
		cleanupFunc func(state blocks.PlayerState)
	}{
		{
			name: "Create new player state",
			setup: func() (blocks.PlayerState, error) {
				return repo.NewBlockState(context.Background(), gofakeit.UUID(), gofakeit.UUID())
			},
			action: func(state blocks.PlayerState) (interface{}, error) {
				return repo.Create(context.Background(), state)
			},
			assertion: func(result interface{}, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			},
			cleanupFunc: func(state blocks.PlayerState) {
				err := repo.Delete(context.Background(), state.GetBlockID(), state.GetPlayerID())
				assert.NoError(t, err)
			},
		},
		{
			name: "Get player state by block and team",
			setup: func() (blocks.PlayerState, error) {
				state, _ := repo.NewBlockState(context.Background(), gofakeit.UUID(), gofakeit.UUID())
				return repo.Create(context.Background(), state)
			},
			action: func(state blocks.PlayerState) (interface{}, error) {
				return repo.GetByBlockAndTeam(context.Background(), state.GetBlockID(), state.GetPlayerID())
			},
			assertion: func(result interface{}, err error) {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			},
			cleanupFunc: func(state blocks.PlayerState) {
				err := repo.Delete(context.Background(), state.GetBlockID(), state.GetPlayerID())
				assert.NoError(t, err)
			},
		},
		{
			name: "Update player state",
			setup: func() (blocks.PlayerState, error) {
				state, _ := repo.NewBlockState(context.Background(), gofakeit.UUID(), gofakeit.UUID())
				createdState, _ := repo.Create(context.Background(), state)
				return createdState, nil
			},
			action: func(state blocks.PlayerState) (interface{}, error) {
				state.SetPlayerData([]byte(`{"key":"value"}`))
				state.SetComplete(true)
				state.SetPointsAwarded(100)
				return repo.Update(context.Background(), state)
			},
			assertion: func(result interface{}, err error) {
				assert.NoError(t, err)
				updatedState := result.(blocks.PlayerState)
				assert.Equal(t, true, updatedState.IsComplete())
				assert.Equal(t, 100, updatedState.GetPointsAwarded())
			},
			cleanupFunc: func(state blocks.PlayerState) {
				err := repo.Delete(context.Background(), state.GetBlockID(), state.GetPlayerID())
				assert.NoError(t, err)
			},
		},
		{
			name: "Delete player state",
			setup: func() (blocks.PlayerState, error) {
				state, _ := repo.NewBlockState(context.Background(), gofakeit.UUID(), gofakeit.UUID())
				return repo.Create(context.Background(), state)
			},
			action: func(state blocks.PlayerState) (interface{}, error) {
				return nil, repo.Delete(context.Background(), state.GetBlockID(), state.GetPlayerID())
			},
			assertion: func(result interface{}, err error) {
				assert.NoError(t, err)
			},
			cleanupFunc: func(state blocks.PlayerState) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := tt.setup()
			assert.NoError(t, err)
			result, err := tt.action(state)
			tt.assertion(result, err)
			if tt.cleanupFunc != nil {
				tt.cleanupFunc(state)
			}
		})
	}
}

func TestBlockStateRepository_Bulk(t *testing.T) {
	repo, transactor, cleanup := setupBlockStateRepo(t)
	defer cleanup()

	tests := []struct {
		name        string
		setup       func() ([]blocks.PlayerState, error)
		action      func(state []blocks.PlayerState) (interface{}, error)
		assertion   func(result interface{}, err error)
		cleanupFunc func(state []blocks.PlayerState)
	}{
		{
			name: "Delete player states by block ID",
			setup: func() ([]blocks.PlayerState, error) {
				blockID := gofakeit.UUID()
				playerStates := make([]blocks.PlayerState, 3)
				for i := 0; i < 3; i++ {
					state, _ := repo.NewBlockState(context.Background(), blockID, gofakeit.UUID())
					ps, err := repo.Create(context.Background(), state)
					playerStates[i] = ps
					if err != nil {
						return nil, err
					}
				}
				return playerStates, nil
			},
			action: func(state []blocks.PlayerState) (interface{}, error) {
				tx, err := transactor.BeginTx(context.Background(), &sql.TxOptions{})
				if err != nil {
					return nil, err
				}

				err = repo.DeleteByBlockID(context.Background(), tx, state[0].GetBlockID())
				if err == nil {
					err := tx.Commit()
					if err != nil {
						return nil, err
					}
				} else {
					err2 := tx.Rollback()
					if err2 != nil {
						return nil, err2
					}
					return nil, err
				}

				// Check that the states have been deleted
				for _, s := range state {
					_, err := repo.GetByBlockAndTeam(context.Background(), s.GetBlockID(), s.GetPlayerID())
					if err.Error() != "sql: no rows in result set" {
						return nil, err
					}
				}

				return nil, nil
			},
			assertion: func(result interface{}, err error) {
				assert.NoError(t, err)
			},
			// cleanup is what we're testing
			cleanupFunc: func(state []blocks.PlayerState) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, err := tt.setup()
			assert.NoError(t, err)
			result, err := tt.action(state)
			tt.assertion(result, err)
			if tt.cleanupFunc != nil {
				tt.cleanupFunc(state)
			}
		})
	}
}

func TestBlockStateRepository_DeleteByTeamCodes(t *testing.T) {
	repo, transactor, cleanup := setupBlockStateRepo(t)
	defer cleanup()

	tests := []struct {
		name        string
		setup       func() ([]blocks.PlayerState, []string, error)
		action      func(state []blocks.PlayerState, teamCodes []string) (interface{}, error)
		assertion   func(result interface{}, err error)
		cleanupFunc func(state []blocks.PlayerState)
	}{
		{
			name: "Delete player states by team codes",
			setup: func() ([]blocks.PlayerState, []string, error) {
				teamCodes := []string{gofakeit.UUID(), gofakeit.UUID()}
				playerStates := make([]blocks.PlayerState, 4)
				for i := 0; i < 4; i++ {
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
			action: func(state []blocks.PlayerState, teamCodes []string) (interface{}, error) {
				tx, err := transactor.BeginTx(context.Background(), &sql.TxOptions{})
				if err != nil {
					return nil, err
				}

				err = repo.DeleteByTeamCodes(context.Background(), tx, teamCodes)
				if err != nil {
					err2 := tx.Rollback()
					if err2 != nil {
						return nil, err2
					}
					return nil, err
				} else {
					err := tx.Commit()
					if err != nil {
						return nil, err
					}
				}

				// Check that the states have been deleted
				for _, s := range state {
					_, err := repo.GetByBlockAndTeam(context.Background(), s.GetBlockID(), s.GetPlayerID())
					if err == nil || !errors.Is(err, sql.ErrNoRows) {
						return nil, errors.New("player state was not deleted")
					}
				}

				return nil, nil
			},
			assertion: func(result interface{}, err error) {
				assert.NoError(t, err)
			},
			cleanupFunc: func(state []blocks.PlayerState) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, teamCodes, err := tt.setup()
			assert.NoError(t, err)
			result, err := tt.action(state, teamCodes)
			tt.assertion(result, err)
			if tt.cleanupFunc != nil {
				tt.cleanupFunc(state)
			}
		})
	}
}
