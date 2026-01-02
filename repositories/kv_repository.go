package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/uptrace/bun"
)

var (
	ErrKVNotFound        = errors.New("kv not found")
	ErrKVVersionConflict = errors.New("version conflict - data was modified")
)

// KVRepository handles persistence for scoped game state (JSON blob approach).
type KVRepository interface {
	// Core operations - work with any scope
	GetState(ctx context.Context, instanceID string, scope models.KVScope, entityID string) (*models.GameState, error)
	SaveState(ctx context.Context, state *models.GameState) error
	DeleteState(ctx context.Context, instanceID string, scope models.KVScope, entityID string) error

	// Convenience methods for common scopes
	GetGameState(ctx context.Context, instanceID string) (*models.GameState, error)
	GetTeamState(ctx context.Context, instanceID, teamCode string) (*models.GameState, error)

	// Bulk cleanup
	DeleteByInstanceID(ctx context.Context, tx bun.IDB, instanceID string) error
	DeleteByScope(ctx context.Context, tx bun.IDB, instanceID string, scope models.KVScope, entityID string) error
}

type kvRepository struct {
	db *bun.DB
}

// NewKVRepository creates a new KV repository.
func NewKVRepository(db *bun.DB) KVRepository {
	return &kvRepository{db: db}
}

// GetState retrieves state for any scope.
// Returns an empty GameState if not found - caller can check Version == 0.
func (r *kvRepository) GetState(
	ctx context.Context,
	instanceID string,
	scope models.KVScope,
	entityID string,
) (*models.GameState, error) {
	var state models.GameState
	err := r.db.NewSelect().
		Model(&state).
		Where("instance_id = ?", instanceID).
		Where("scope = ?", scope).
		Where("entity_id = ?", entityID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Return empty struct - Version 0 indicates new record
			return &models.GameState{
				InstanceID: instanceID,
				Scope:      scope,
				EntityID:   entityID,
				Data:       make(map[string]any),
				Version:    0,
			}, nil
		}
		return nil, err
	}
	// Ensure Data is not nil
	if state.Data == nil {
		state.Data = make(map[string]any)
	}
	return &state, nil
}

// SaveState persists state with optimistic locking.
func (r *kvRepository) SaveState(ctx context.Context, state *models.GameState) error {
	state.UpdatedAt = time.Now()

	if state.Version == 0 {
		// New record - insert
		state.Version = 1
		_, err := r.db.NewInsert().
			Model(state).
			Exec(ctx)
		return err
	}

	// Existing record - update with version check
	oldVersion := state.Version
	state.Version++

	res, err := r.db.NewUpdate().
		Model(state).
		Where("instance_id = ?", state.InstanceID).
		Where("scope = ?", state.Scope).
		Where("entity_id = ?", state.EntityID).
		Where("version = ?", oldVersion).
		Exec(ctx)
	if err != nil {
		return err
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrKVVersionConflict
	}

	return nil
}

// DeleteState removes state for a specific scope.
func (r *kvRepository) DeleteState(
	ctx context.Context,
	instanceID string,
	scope models.KVScope,
	entityID string,
) error {
	_, err := r.db.NewDelete().
		Model((*models.GameState)(nil)).
		Where("instance_id = ?", instanceID).
		Where("scope = ?", scope).
		Where("entity_id = ?", entityID).
		Exec(ctx)
	return err
}

// GetGameState is a convenience method for game scope (entityID = "").
func (r *kvRepository) GetGameState(ctx context.Context, instanceID string) (*models.GameState, error) {
	return r.GetState(ctx, instanceID, models.KVScopeGame, "")
}

// GetTeamState is a convenience method for team scope.
func (r *kvRepository) GetTeamState(ctx context.Context, instanceID, teamCode string) (*models.GameState, error) {
	return r.GetState(ctx, instanceID, models.KVScopeTeam, teamCode)
}

// DeleteByInstanceID removes all state for an instance (all scopes).
func (r *kvRepository) DeleteByInstanceID(ctx context.Context, tx bun.IDB, instanceID string) error {
	if tx == nil {
		tx = r.db
	}

	_, err := tx.NewDelete().
		Model((*models.GameState)(nil)).
		Where("instance_id = ?", instanceID).
		Exec(ctx)
	return err
}

// DeleteByScope removes state for a specific scope and entity.
func (r *kvRepository) DeleteByScope(
	ctx context.Context,
	tx bun.IDB,
	instanceID string,
	scope models.KVScope,
	entityID string,
) error {
	if tx == nil {
		tx = r.db
	}

	_, err := tx.NewDelete().
		Model((*models.GameState)(nil)).
		Where("instance_id = ?", instanceID).
		Where("scope = ?", scope).
		Where("entity_id = ?", entityID).
		Exec(ctx)
	return err
}
