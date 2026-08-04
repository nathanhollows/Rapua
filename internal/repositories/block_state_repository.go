package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/uptrace/bun"
)

type BlockStateRepository interface {
	// Create creates a new player state
	Create(ctx context.Context, state blocks.PlayerState) (blocks.PlayerState, error)
	// NewBlockState creates a new block state for a specified block, run, and quest
	NewBlockState(ctx context.Context, blockID, runCode, questID string) (blocks.PlayerState, error)

	// GetByBlockAndRun gets a player state by block ID, run code, and quest ID
	GetByBlockAndRun(ctx context.Context, blockID, runCode, questID string) (blocks.PlayerState, error)

	// Update updates an existing player state
	Update(ctx context.Context, block blocks.PlayerState) (blocks.PlayerState, error)

	// Delete deletes a player state by block ID, run code, and quest ID
	Delete(ctx context.Context, blockID, runCode, questID string) error
	// DeleteByBlockID deletes all player states for a block
	DeleteByBlockID(ctx context.Context, tx *bun.Tx, blockID string) error
}

type blockStateRepository struct {
	db *bun.DB
}

func NewBlockStateRepository(db *bun.DB) BlockStateRepository {
	return &blockStateRepository{
		db: db,
	}
}

// PlayerStateData struct implements the PlayerState interface.
type PlayerStateData struct {
	blockID       string
	playerID      string
	questID       string
	playerData    json.RawMessage
	isComplete    bool
	pointsAwarded int
}

func (p *PlayerStateData) GetQuestID() string {
	return p.questID
}

func (p *PlayerStateData) GetBlockID() string {
	return p.blockID
}

func (p *PlayerStateData) GetPlayerData() json.RawMessage {
	return p.playerData
}

func (p *PlayerStateData) GetPlayerID() string {
	return p.playerID
}

func (p *PlayerStateData) SetPlayerData(data json.RawMessage) {
	p.playerData = data
}

func (p *PlayerStateData) IsComplete() bool {
	return p.isComplete
}

func (p *PlayerStateData) SetComplete(complete bool) {
	p.isComplete = complete
}

func (p *PlayerStateData) GetPointsAwarded() int {
	return p.pointsAwarded
}

func (p *PlayerStateData) SetPointsAwarded(points int) {
	p.pointsAwarded = points
}

// Convert model state to PlayerState.
func convertModelToPlayerStateData(state models.RunBlockState) blocks.PlayerState {
	return &PlayerStateData{
		blockID:       state.BlockID,
		playerID:      state.RunCode,
		questID:       state.QuestID,
		playerData:    state.PlayerData,
		isComplete:    state.IsComplete,
		pointsAwarded: state.PointsAwarded,
	}
}

// Convert PlayerState to model state.
func convertPlayerStateToModelData(state blocks.PlayerState) models.RunBlockState {
	return models.RunBlockState{
		RunCode:       state.GetPlayerID(),
		BlockID:       state.GetBlockID(),
		QuestID:       state.GetQuestID(),
		PlayerData:    state.GetPlayerData(),
		IsComplete:    state.IsComplete(),
		PointsAwarded: state.GetPointsAwarded(),
	}
}

// GetByBlockAndRun fetches a specific run block state for a block, run, and quest.
func (r *blockStateRepository) GetByBlockAndRun(
	ctx context.Context,
	blockID, runCode, questID string,
) (blocks.PlayerState, error) {
	var modelState models.RunBlockState
	err := r.db.NewSelect().
		Model(&modelState).
		Where("block_id = ?", blockID).
		Where("run_code = ?", runCode).
		Where("quest_id = ?", questID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return convertModelToPlayerStateData(modelState), nil
}

// Create inserts a new run block state into the database.
func (r *blockStateRepository) Create(ctx context.Context, state blocks.PlayerState) (blocks.PlayerState, error) {
	modelState := convertPlayerStateToModelData(state)

	_, err := r.db.NewInsert().
		Model(&modelState).
		Exec(ctx)
	if err != nil {
		return nil, err
	}

	return convertModelToPlayerStateData(modelState), nil
}

// Update modifies an existing run block state in the database.
func (r *blockStateRepository) Update(ctx context.Context, state blocks.PlayerState) (blocks.PlayerState, error) {
	modelState := convertPlayerStateToModelData(state)
	if state.GetBlockID() == "" || state.GetPlayerID() == "" {
		return nil, errors.New("block_id and run_code must be set")
	}

	_, err := r.db.NewUpdate().
		Model(&modelState).
		Set("player_data = ?", modelState.PlayerData).
		Set("is_complete = ?", modelState.IsComplete).
		Set("points_awarded = ?", modelState.PointsAwarded).
		Set("updated_at = ?", time.Now()).
		Where("block_id = ?", state.GetBlockID()).
		Where("run_code = ?", state.GetPlayerID()).
		Where("quest_id = ?", modelState.QuestID).
		Exec(ctx)

	return state, err
}

// NewBlockState creates a new block state.
func (r *blockStateRepository) NewBlockState(
	_ context.Context,
	blockID, runCode, questID string,
) (blocks.PlayerState, error) {
	state := &PlayerStateData{
		blockID:       blockID,
		playerID:      runCode,
		questID:       questID,
		playerData:    nil,
		isComplete:    false,
		pointsAwarded: 0,
	}
	return state, nil
}

// Delete removes a run block state from the database by its key.
func (r *blockStateRepository) Delete(ctx context.Context, blockID, runCode, questID string) error {
	_, err := r.db.NewDelete().
		Model(&models.RunBlockState{}).
		Where("block_id = ?", blockID).
		Where("run_code = ?", runCode).
		Where("quest_id = ?", questID).
		Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

// DeleteByBlockID removes all run block states for a block from the database.
func (r *blockStateRepository) DeleteByBlockID(ctx context.Context, tx *bun.Tx, blockID string) error {
	_, err := tx.NewDelete().
		Model(&models.RunBlockState{}).
		Where("block_id = ?", blockID).
		Exec(ctx)
	return err
}
