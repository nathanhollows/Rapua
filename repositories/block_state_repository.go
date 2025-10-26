package repositories

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/nathanhollows/Rapua/v5/blocks"
	"github.com/nathanhollows/Rapua/v5/models"
	"github.com/uptrace/bun"
)

type BlockStateRepository interface {
	// Create creates a new player state
	Create(ctx context.Context, state blocks.PlayerState) (blocks.PlayerState, error)
	// NewBlockState creates a new block state for a specified block and team
	NewBlockState(ctx context.Context, blockID, teamCode string) (blocks.PlayerState, error)

	// GetByBlockAndTeam gets a player state by block ID and team code
	GetByBlockAndTeam(ctx context.Context, blockID string, teamCode string) (blocks.PlayerState, error)

	// Update updates an existing player state
	Update(ctx context.Context, block blocks.PlayerState) (blocks.PlayerState, error)

	// Delete deletes a player state by block ID and team code
	Delete(ctx context.Context, blockID string, teamCode string) error
	// DeleteByBlockID deletes all player states for a block
	// Requires a transaction as this implies a cascade delete
	DeleteByBlockID(ctx context.Context, tx *bun.Tx, blockID string) error
	// DeleteByTeamCodes deletes all player states for a team
	DeleteByTeamCodes(ctx context.Context, tx *bun.Tx, teamCodes []string) error
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
	playerData    json.RawMessage
	isComplete    bool
	pointsAwarded int
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
func convertModelToPlayerStateData(state models.TeamBlockState) blocks.PlayerState {
	return &PlayerStateData{
		blockID:       state.BlockID,
		playerID:      state.TeamCode,
		playerData:    state.PlayerData,
		isComplete:    state.IsComplete,
		pointsAwarded: state.PointsAwarded,
	}
}

// Convert PlayerState to model state.
func convertPlayerStateToModelData(state blocks.PlayerState) models.TeamBlockState {
	return models.TeamBlockState{
		TeamCode:      state.GetPlayerID(),
		BlockID:       state.GetBlockID(),
		PlayerData:    state.GetPlayerData(),
		IsComplete:    state.IsComplete(),
		PointsAwarded: state.GetPointsAwarded(),
	}
}

// GetByBlockAndTeam fetches a specific team block state for a block and team.
func (r *blockStateRepository) GetByBlockAndTeam(
	ctx context.Context,
	blockID string,
	teamCode string,
) (blocks.PlayerState, error) {
	var modelState models.TeamBlockState
	err := r.db.NewSelect().
		Model(&modelState).
		Where("block_id = ?", blockID).
		Where("team_code = ?", teamCode).
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	return convertModelToPlayerStateData(modelState), nil
}

// Create inserts a new team block state into the database.
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

// Update modifies an existing team block state in the database.
func (r *blockStateRepository) Update(ctx context.Context, state blocks.PlayerState) (blocks.PlayerState, error) {
	modelState := convertPlayerStateToModelData(state)
	if state.GetBlockID() == "" || state.GetPlayerID() == "" {
		return nil, errors.New("block_id and team_code must be set")
	}

	_, err := r.db.NewUpdate().
		Model(&modelState).
		Set("player_data = ?", modelState.PlayerData).
		Set("is_complete = ?", modelState.IsComplete).
		Set("points_awarded = ?", modelState.PointsAwarded).
		Set("updated_at = ?", time.Now()).
		Where("block_id = ?", state.GetBlockID()).
		Where("team_code = ?", state.GetPlayerID()).
		Exec(ctx)

	return state, err
}

// NewBlockState creates a new block state.
func (r *blockStateRepository) NewBlockState(
	_ context.Context,
	blockID, teamCode string,
) (blocks.PlayerState, error) {
	state := &PlayerStateData{
		blockID:       blockID,
		playerID:      teamCode,
		playerData:    nil,
		isComplete:    false,
		pointsAwarded: 0,
	}
	return state, nil
}

// Delete removes a team block state from the database by its ID.
func (r *blockStateRepository) Delete(ctx context.Context, blockID string, teamCode string) error {
	_, err := r.db.NewDelete().
		Model(&models.TeamBlockState{}).
		Where("block_id = ?", blockID).
		Where("team_code = ?", teamCode).
		Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

// DeleteByBlockID removes all team block states for a block from the database.
func (r *blockStateRepository) DeleteByBlockID(ctx context.Context, tx *bun.Tx, blockID string) error {
	_, err := tx.NewDelete().
		Model(&models.TeamBlockState{}).
		Where("block_id = ?", blockID).
		Exec(ctx)
	return err
}

// DeleteByTeamCodes removes all team block states for a team from the database.
func (r *blockStateRepository) DeleteByTeamCodes(ctx context.Context, tx *bun.Tx, teamCodes []string) error {
	_, err := tx.NewDelete().
		Model(&models.TeamBlockState{}).
		Where("team_code IN (?)", bun.In(teamCodes)).
		Exec(ctx)
	return err
}
