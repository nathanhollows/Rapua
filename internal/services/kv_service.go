package services

import (
	"context"
	"errors"

	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
)

var (
	ErrKVKeyNotFound   = errors.New("key not found")
	ErrKVEmptyKey      = errors.New("key cannot be empty")
	ErrKVEmptyInstance = errors.New("instance ID cannot be empty")
	ErrKVEmptyEntityID = errors.New("entity ID cannot be empty for this scope")
)

// KVService provides scoped key-value storage operations.
type KVService struct {
	kvRepo repositories.KVRepository
}

// NewKVService creates a new KV service.
func NewKVService(kvRepo repositories.KVRepository) *KVService {
	return &KVService{kvRepo: kvRepo}
}

// --- Core Operations ---

// GetState retrieves state for any scope.
func (s *KVService) GetState(
	ctx context.Context,
	instanceID string,
	scope models.KVScope,
	entityID string,
) (*models.GameState, error) {
	if instanceID == "" {
		return nil, ErrKVEmptyInstance
	}
	return s.kvRepo.GetState(ctx, instanceID, scope, entityID)
}

// SaveState persists state with optimistic locking.
func (s *KVService) SaveState(ctx context.Context, state *models.GameState) error {
	if state.InstanceID == "" {
		return ErrKVEmptyInstance
	}
	return s.kvRepo.SaveState(ctx, state)
}

// DeleteState removes state for a specific scope.
func (s *KVService) DeleteState(ctx context.Context, instanceID string, scope models.KVScope, entityID string) error {
	if instanceID == "" {
		return ErrKVEmptyInstance
	}
	return s.kvRepo.DeleteState(ctx, instanceID, scope, entityID)
}

// --- Convenience Methods ---

// GetGameState retrieves game-scoped state.
func (s *KVService) GetGameState(ctx context.Context, instanceID string) (*models.GameState, error) {
	return s.GetState(ctx, instanceID, models.KVScopeGame, "")
}

// GetTeamState retrieves team-scoped state.
func (s *KVService) GetTeamState(ctx context.Context, instanceID, teamCode string) (*models.GameState, error) {
	if teamCode == "" {
		return nil, ErrKVEmptyEntityID
	}
	return s.GetState(ctx, instanceID, models.KVScopeTeam, teamCode)
}

// --- Bulk Cleanup ---

// DeleteByInstanceID removes all state for an instance.
func (s *KVService) DeleteByInstanceID(ctx context.Context, instanceID string) error {
	if instanceID == "" {
		return ErrKVEmptyInstance
	}
	return s.kvRepo.DeleteByInstanceID(ctx, nil, instanceID)
}

// DeleteTeamState removes a team's state (used when resetting team).
func (s *KVService) DeleteTeamState(ctx context.Context, instanceID, teamCode string) error {
	if instanceID == "" {
		return ErrKVEmptyInstance
	}
	if teamCode == "" {
		return ErrKVEmptyEntityID
	}
	return s.kvRepo.DeleteByScope(ctx, nil, instanceID, models.KVScopeTeam, teamCode)
}

// --- KVStore (for block/template access) ---

// KVStore provides a scoped view of game state for a specific context.
// Load once at handler level, use throughout request for block rendering.
type KVStore struct {
	service    *KVService
	instanceID string
	teamCode   string

	// Cached state - loaded on first access
	gameState *models.GameState
	teamState *models.GameState
	gameDirty bool
	teamDirty bool
}

// NewKVStore creates a KV store scoped to a specific team and instance.
func (s *KVService) NewKVStore(instanceID, teamCode string) *KVStore {
	return &KVStore{
		service:    s,
		instanceID: instanceID,
		teamCode:   teamCode,
	}
}

// Load fetches both game and team state. Call once at start of request.
func (store *KVStore) Load(ctx context.Context) error {
	var err error

	store.gameState, err = store.service.GetGameState(ctx, store.instanceID)
	if err != nil {
		return err
	}

	if store.teamCode != "" {
		store.teamState, err = store.service.GetTeamState(ctx, store.instanceID, store.teamCode)
		if err != nil {
			return err
		}
	}

	return nil
}

// Save persists any modified state. Call at end of request if changes were made.
func (store *KVStore) Save(ctx context.Context) error {
	if store.gameDirty && store.gameState != nil {
		if err := store.service.SaveState(ctx, store.gameState); err != nil {
			return err
		}
		store.gameDirty = false
	}

	if store.teamDirty && store.teamState != nil {
		if err := store.service.SaveState(ctx, store.teamState); err != nil {
			return err
		}
		store.teamDirty = false
	}

	return nil
}

// IsDirty returns true if any changes need to be saved.
func (store *KVStore) IsDirty() bool {
	return store.gameDirty || store.teamDirty
}

// --- Game Scope Access ---

// GameData returns the raw game state map for template access.
func (store *KVStore) GameData() map[string]any {
	if store.gameState == nil {
		return make(map[string]any)
	}
	return store.gameState.Data
}

// GetGame retrieves a game-scoped value.
func (store *KVStore) GetGame(key string) (any, bool) {
	if store.gameState == nil {
		return nil, false
	}
	v, ok := store.gameState.Data[key]
	return v, ok
}

// SetGame sets a game-scoped value.
func (store *KVStore) SetGame(key string, value any) {
	if store.gameState == nil {
		return
	}
	store.gameState.Data[key] = value
	store.gameDirty = true
}

// DeleteGame removes a game-scoped value.
func (store *KVStore) DeleteGame(key string) {
	if store.gameState == nil {
		return
	}
	delete(store.gameState.Data, key)
	store.gameDirty = true
}

// GetGameString retrieves a game-scoped string value.
func (store *KVStore) GetGameString(key string) (string, bool) {
	v, ok := store.GetGame(key)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// GetGameInt retrieves a game-scoped integer value.
func (store *KVStore) GetGameInt(key string) (int, bool) {
	v, ok := store.GetGame(key)
	if !ok {
		return 0, false
	}
	// JSON unmarshals numbers as float64
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	default:
		return 0, false
	}
}

// GetGameBool retrieves a game-scoped boolean value.
func (store *KVStore) GetGameBool(key string) (bool, bool) {
	v, ok := store.GetGame(key)
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// IncrementGame increments an integer value in game scope. Returns new value.
func (store *KVStore) IncrementGame(key string, amount int) int {
	current, _ := store.GetGameInt(key)
	newValue := current + amount
	store.SetGame(key, newValue)
	return newValue
}

// --- Team Scope Access ---

// TeamData returns the raw team state map for template access.
func (store *KVStore) TeamData() map[string]any {
	if store.teamState == nil {
		return make(map[string]any)
	}
	return store.teamState.Data
}

// GetTeam retrieves a team-scoped value.
func (store *KVStore) GetTeam(key string) (any, bool) {
	if store.teamState == nil {
		return nil, false
	}
	v, ok := store.teamState.Data[key]
	return v, ok
}

// SetTeam sets a team-scoped value.
func (store *KVStore) SetTeam(key string, value any) {
	if store.teamState == nil {
		return
	}
	store.teamState.Data[key] = value
	store.teamDirty = true
}

// DeleteTeam removes a team-scoped value.
func (store *KVStore) DeleteTeam(key string) {
	if store.teamState == nil {
		return
	}
	delete(store.teamState.Data, key)
	store.teamDirty = true
}

// GetTeamString retrieves a team-scoped string value.
func (store *KVStore) GetTeamString(key string) (string, bool) {
	v, ok := store.GetTeam(key)
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// GetTeamInt retrieves a team-scoped integer value.
func (store *KVStore) GetTeamInt(key string) (int, bool) {
	v, ok := store.GetTeam(key)
	if !ok {
		return 0, false
	}
	// JSON unmarshals numbers as float64
	switch n := v.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	default:
		return 0, false
	}
}

// GetTeamBool retrieves a team-scoped boolean value.
func (store *KVStore) GetTeamBool(key string) (bool, bool) {
	v, ok := store.GetTeam(key)
	if !ok {
		return false, false
	}
	b, ok := v.(bool)
	return b, ok
}

// IncrementTeam increments an integer value in team scope. Returns new value.
func (store *KVStore) IncrementTeam(key string, amount int) int {
	current, _ := store.GetTeamInt(key)
	newValue := current + amount
	store.SetTeam(key, newValue)
	return newValue
}

// --- Context Accessors ---

// GetInstanceID returns the instance ID for this store.
func (store *KVStore) GetInstanceID() string {
	return store.instanceID
}

// GetTeamCode returns the team code for this store.
func (store *KVStore) GetTeamCode() string {
	return store.teamCode
}
