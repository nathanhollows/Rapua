package repositories_test

import (
	"context"
	"database/sql"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v6/db"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupKVRepo(t *testing.T) (repositories.KVRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)
	transactor := db.NewTransactor(dbc)
	kvRepository := repositories.NewKVRepository(dbc)
	return kvRepository, transactor, cleanup
}

func TestKVRepository_GameState(t *testing.T) {
	repo, _, cleanup := setupKVRepo(t)
	defer cleanup()

	ctx := context.Background()
	instanceID := gofakeit.UUID()

	t.Run("GetState returns empty for non-existent", func(t *testing.T) {
		state, err := repo.GetState(ctx, instanceID, models.KVScopeGame, "")
		require.NoError(t, err)
		assert.NotNil(t, state)
		assert.Equal(t, instanceID, state.InstanceID)
		assert.Equal(t, models.KVScopeGame, state.Scope)
		assert.Empty(t, state.EntityID)
		assert.NotNil(t, state.Data)
		assert.Empty(t, state.Data)
		assert.Equal(t, 0, state.Version) // Version 0 = new record
	})

	t.Run("SaveState creates new record", func(t *testing.T) {
		state := &models.GameState{
			InstanceID: instanceID,
			Scope:      models.KVScopeGame,
			EntityID:   "",
			Data: map[string]any{
				"phase":   1,
				"started": true,
			},
			Version: 0,
		}

		err := repo.SaveState(ctx, state)
		require.NoError(t, err)
		assert.Equal(t, 1, state.Version)
	})

	t.Run("GetState retrieves saved data", func(t *testing.T) {
		state, err := repo.GetGameState(ctx, instanceID)
		require.NoError(t, err)
		assert.Equal(t, 1, state.Version)
		assert.InEpsilon(t, float64(1), state.Data["phase"], 0.0001)
		assert.True(t, state.Data["started"].(bool))
	})

	t.Run("SaveState updates existing record", func(t *testing.T) {
		state, err := repo.GetGameState(ctx, instanceID)
		require.NoError(t, err)

		state.Data["phase"] = 2
		state.Data["winner"] = "TeamA"

		err = repo.SaveState(ctx, state)
		require.NoError(t, err)
		assert.Equal(t, 2, state.Version)

		// Verify
		state2, err := repo.GetGameState(ctx, instanceID)
		require.NoError(t, err)
		assert.InEpsilon(t, float64(2), state2.Data["phase"], 0.0001)
		assert.Equal(t, "TeamA", state2.Data["winner"])
	})

	t.Run("Optimistic locking detects conflict", func(t *testing.T) {
		state1, err := repo.GetGameState(ctx, instanceID)
		require.NoError(t, err)

		state2, err := repo.GetGameState(ctx, instanceID)
		require.NoError(t, err)

		// Update first
		state1.Data["phase"] = 3
		err = repo.SaveState(ctx, state1)
		require.NoError(t, err)

		// Second should conflict
		state2.Data["phase"] = 4
		err = repo.SaveState(ctx, state2)
		assert.ErrorIs(t, err, repositories.ErrKVVersionConflict)
	})

	t.Run("DeleteState removes record", func(t *testing.T) {
		err := repo.DeleteState(ctx, instanceID, models.KVScopeGame, "")
		require.NoError(t, err)

		state, err := repo.GetGameState(ctx, instanceID)
		require.NoError(t, err)
		assert.Equal(t, 0, state.Version)
		assert.Empty(t, state.Data)
	})
}

func TestKVRepository_TeamState(t *testing.T) {
	repo, _, cleanup := setupKVRepo(t)
	defer cleanup()

	ctx := context.Background()
	instanceID := gofakeit.UUID()
	teamCode := gofakeit.LetterN(4)

	t.Run("GetTeamState returns empty for non-existent", func(t *testing.T) {
		state, err := repo.GetTeamState(ctx, instanceID, teamCode)
		require.NoError(t, err)
		assert.Equal(t, models.KVScopeTeam, state.Scope)
		assert.Equal(t, teamCode, state.EntityID)
		assert.Equal(t, 0, state.Version)
	})

	t.Run("SaveState creates team record", func(t *testing.T) {
		state := &models.GameState{
			InstanceID: instanceID,
			Scope:      models.KVScopeTeam,
			EntityID:   teamCode,
			Data: map[string]any{
				"score":  100,
				"items":  []any{"key", "sword"},
				"active": true,
			},
			Version: 0,
		}

		err := repo.SaveState(ctx, state)
		require.NoError(t, err)
		assert.Equal(t, 1, state.Version)
	})

	t.Run("GetTeamState retrieves saved data", func(t *testing.T) {
		state, err := repo.GetTeamState(ctx, instanceID, teamCode)
		require.NoError(t, err)
		assert.Equal(t, 1, state.Version)

		score, ok := state.Data["score"].(float64)
		assert.True(t, ok)
		assert.InEpsilon(t, float64(100), score, 0.0001)

		items, ok := state.Data["items"].([]any)
		assert.True(t, ok)
		assert.Len(t, items, 2)
	})
}

func TestKVRepository_ScopeIsolation(t *testing.T) {
	repo, _, cleanup := setupKVRepo(t)
	defer cleanup()

	ctx := context.Background()
	instanceID := gofakeit.UUID()
	team1 := gofakeit.LetterN(4)
	team2 := gofakeit.LetterN(4)

	// Create game state
	gameState := &models.GameState{
		InstanceID: instanceID,
		Scope:      models.KVScopeGame,
		EntityID:   "",
		Data:       map[string]any{"global": "value"},
		Version:    0,
	}
	err := repo.SaveState(ctx, gameState)
	require.NoError(t, err)

	// Create team1 state
	team1State := &models.GameState{
		InstanceID: instanceID,
		Scope:      models.KVScopeTeam,
		EntityID:   team1,
		Data:       map[string]any{"score": 100},
		Version:    0,
	}
	err = repo.SaveState(ctx, team1State)
	require.NoError(t, err)

	// Create team2 state
	team2State := &models.GameState{
		InstanceID: instanceID,
		Scope:      models.KVScopeTeam,
		EntityID:   team2,
		Data:       map[string]any{"score": 200},
		Version:    0,
	}
	err = repo.SaveState(ctx, team2State)
	require.NoError(t, err)

	// Verify isolation
	loadedGame, err := repo.GetGameState(ctx, instanceID)
	require.NoError(t, err)
	assert.Equal(t, "value", loadedGame.Data["global"])

	loadedTeam1, err := repo.GetTeamState(ctx, instanceID, team1)
	require.NoError(t, err)
	assert.InEpsilon(t, float64(100), loadedTeam1.Data["score"], 0.0001)

	loadedTeam2, err := repo.GetTeamState(ctx, instanceID, team2)
	require.NoError(t, err)
	assert.InEpsilon(t, float64(200), loadedTeam2.Data["score"], 0.0001)
}

func TestKVRepository_InstanceIsolation(t *testing.T) {
	repo, _, cleanup := setupKVRepo(t)
	defer cleanup()

	ctx := context.Background()
	instance1 := gofakeit.UUID()
	instance2 := gofakeit.UUID()

	// Create state for instance1
	state1 := &models.GameState{
		InstanceID: instance1,
		Scope:      models.KVScopeGame,
		EntityID:   "",
		Data:       map[string]any{"status": "running"},
		Version:    0,
	}
	err := repo.SaveState(ctx, state1)
	require.NoError(t, err)

	// Create state for instance2
	state2 := &models.GameState{
		InstanceID: instance2,
		Scope:      models.KVScopeGame,
		EntityID:   "",
		Data:       map[string]any{"status": "paused"},
		Version:    0,
	}
	err = repo.SaveState(ctx, state2)
	require.NoError(t, err)

	// Verify isolation
	loaded1, err := repo.GetGameState(ctx, instance1)
	require.NoError(t, err)
	assert.Equal(t, "running", loaded1.Data["status"])

	loaded2, err := repo.GetGameState(ctx, instance2)
	require.NoError(t, err)
	assert.Equal(t, "paused", loaded2.Data["status"])
}

func TestKVRepository_BulkDelete(t *testing.T) {
	repo, transactor, cleanup := setupKVRepo(t)
	defer cleanup()

	ctx := context.Background()
	instanceID := gofakeit.UUID()

	// Create game and team state
	gameState := &models.GameState{
		InstanceID: instanceID,
		Scope:      models.KVScopeGame,
		EntityID:   "",
		Data:       map[string]any{"status": "active"},
		Version:    0,
	}
	err := repo.SaveState(ctx, gameState)
	require.NoError(t, err)

	team1State := &models.GameState{
		InstanceID: instanceID,
		Scope:      models.KVScopeTeam,
		EntityID:   "TEAM1",
		Data:       map[string]any{"score": 100},
		Version:    0,
	}
	err = repo.SaveState(ctx, team1State)
	require.NoError(t, err)

	team2State := &models.GameState{
		InstanceID: instanceID,
		Scope:      models.KVScopeTeam,
		EntityID:   "TEAM2",
		Data:       map[string]any{"score": 200},
		Version:    0,
	}
	err = repo.SaveState(ctx, team2State)
	require.NoError(t, err)

	t.Run("DeleteByScope only removes that scope/entity", func(t *testing.T) {
		tx, txErr := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, txErr)

		delErr := repo.DeleteByScope(ctx, tx, instanceID, models.KVScopeTeam, "TEAM1")
		require.NoError(t, delErr)

		commitErr := tx.Commit()
		require.NoError(t, commitErr)

		// TEAM1 should be gone
		state1, err1 := repo.GetTeamState(ctx, instanceID, "TEAM1")
		require.NoError(t, err1)
		assert.Equal(t, 0, state1.Version)

		// TEAM2 should still exist
		state2, err2 := repo.GetTeamState(ctx, instanceID, "TEAM2")
		require.NoError(t, err2)
		assert.Equal(t, 1, state2.Version)

		// Game state should still exist
		gameLoaded, err3 := repo.GetGameState(ctx, instanceID)
		require.NoError(t, err3)
		assert.Equal(t, 1, gameLoaded.Version)
	})

	t.Run("DeleteByInstanceID removes all state for instance", func(t *testing.T) {
		tx, txErr := transactor.BeginTx(ctx, &sql.TxOptions{})
		require.NoError(t, txErr)

		delErr := repo.DeleteByInstanceID(ctx, tx, instanceID)
		require.NoError(t, delErr)

		commitErr := tx.Commit()
		require.NoError(t, commitErr)

		// All should be gone
		team2, err1 := repo.GetTeamState(ctx, instanceID, "TEAM2")
		require.NoError(t, err1)
		assert.Equal(t, 0, team2.Version)

		game, err2 := repo.GetGameState(ctx, instanceID)
		require.NoError(t, err2)
		assert.Equal(t, 0, game.Version)
	})
}

func TestKVRepository_ComplexData(t *testing.T) {
	repo, _, cleanup := setupKVRepo(t)
	defer cleanup()

	ctx := context.Background()
	instanceID := gofakeit.UUID()
	teamCode := gofakeit.LetterN(4)

	// Test complex nested data structures
	state := &models.GameState{
		InstanceID: instanceID,
		Scope:      models.KVScopeTeam,
		EntityID:   teamCode,
		Data: map[string]any{
			"inventory": map[string]any{
				"items": []any{
					map[string]any{"id": "key", "count": 1},
					map[string]any{"id": "sword", "count": 2},
				},
				"gold": 500,
			},
			"quests": map[string]any{
				"main":      "find_treasure",
				"completed": []any{"intro", "tutorial"},
			},
			"flags": map[string]any{
				"met_npc_bob":      true,
				"unlocked_passage": false,
			},
		},
		Version: 0,
	}

	err := repo.SaveState(ctx, state)
	require.NoError(t, err)

	// Retrieve and verify nested structure
	loaded, err := repo.GetTeamState(ctx, instanceID, teamCode)
	require.NoError(t, err)

	inventory, ok := loaded.Data["inventory"].(map[string]any)
	require.True(t, ok)

	gold, ok := inventory["gold"].(float64)
	require.True(t, ok)
	assert.InEpsilon(t, float64(500), gold, 0.0001)

	items, ok := inventory["items"].([]any)
	require.True(t, ok)
	assert.Len(t, items, 2)

	item1, ok := items[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "key", item1["id"])

	flags, ok := loaded.Data["flags"].(map[string]any)
	require.True(t, ok)
	assert.True(t, flags["met_npc_bob"].(bool))
	assert.False(t, flags["unlocked_passage"].(bool))
}

func TestKVRepository_FutureScopes(t *testing.T) {
	repo, _, cleanup := setupKVRepo(t)
	defer cleanup()

	ctx := context.Background()
	instanceID := gofakeit.UUID()
	playerID := gofakeit.UUID()

	// Test player scope (future use)
	playerState := &models.GameState{
		InstanceID: instanceID,
		Scope:      models.KVScopePlayer,
		EntityID:   playerID,
		Data: map[string]any{
			"preferences": map[string]any{
				"theme": "dark",
			},
		},
		Version: 0,
	}

	err := repo.SaveState(ctx, playerState)
	require.NoError(t, err)

	loaded, err := repo.GetState(ctx, instanceID, models.KVScopePlayer, playerID)
	require.NoError(t, err)
	assert.Equal(t, 1, loaded.Version)

	prefs, ok := loaded.Data["preferences"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "dark", prefs["theme"])
}

func TestKVRepository_EdgeCases(t *testing.T) {
	repo, _, cleanup := setupKVRepo(t)
	defer cleanup()

	ctx := context.Background()
	instanceID := gofakeit.UUID()

	t.Run("Empty data map", func(t *testing.T) {
		state := &models.GameState{
			InstanceID: instanceID,
			Scope:      models.KVScopeGame,
			EntityID:   "",
			Data:       map[string]any{},
			Version:    0,
		}

		err := repo.SaveState(ctx, state)
		require.NoError(t, err)

		loaded, err := repo.GetGameState(ctx, instanceID)
		require.NoError(t, err)
		assert.NotNil(t, loaded.Data)
		assert.Empty(t, loaded.Data)

		_ = repo.DeleteState(ctx, instanceID, models.KVScopeGame, "")
	})

	t.Run("Nil values in data", func(t *testing.T) {
		state := &models.GameState{
			InstanceID: instanceID,
			Scope:      models.KVScopeGame,
			EntityID:   "",
			Data: map[string]any{
				"null_field": nil,
				"string":     "value",
			},
			Version: 0,
		}

		err := repo.SaveState(ctx, state)
		require.NoError(t, err)

		loaded, err := repo.GetGameState(ctx, instanceID)
		require.NoError(t, err)
		assert.Nil(t, loaded.Data["null_field"])
		assert.Equal(t, "value", loaded.Data["string"])

		_ = repo.DeleteState(ctx, instanceID, models.KVScopeGame, "")
	})

	t.Run("Delete non-existent does not error", func(t *testing.T) {
		err := repo.DeleteState(ctx, "non-existent", models.KVScopeGame, "")
		require.NoError(t, err)
	})
}
