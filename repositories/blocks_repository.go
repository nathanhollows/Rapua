package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v4/blocks"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/uptrace/bun"
)

type BlockRepository interface {
	// Create creates a new block for an owner with specific context
	Create(
		ctx context.Context,
		block blocks.Block,
		ownerID string,
		blockContext blocks.BlockContext,
	) (blocks.Block, error)

	// GetByID fetches a block by its ID
	GetByID(ctx context.Context, blockID string) (blocks.Block, error)
	// GetBlockAndStateByBlockIDAndTeamCode fetches a block and its state by block ID and team code
	GetBlockAndStateByBlockIDAndTeamCode(
		ctx context.Context,
		blockID string,
		teamCode string,
	) (blocks.Block, blocks.PlayerState, error)
	// FindByOwnerID fetches all blocks for an owner (context agnostic)
	FindByOwnerID(ctx context.Context, ownerID string) (blocks.Blocks, error)
	// FindByOwnerIDAndContext fetches all blocks for an owner with specific context
	FindByOwnerIDAndContext(
		ctx context.Context,
		ownerID string,
		blockContext blocks.BlockContext,
	) (blocks.Blocks, error)
	// FindByLocationID fetches all blocks for a location (legacy method)
	FindByLocationID(ctx context.Context, locationID string) (blocks.Blocks, error)
	// FindBlocksAndStatesByOwnerIDAndTeamCode fetches blocks and their states by owner and team code
	FindBlocksAndStatesByOwnerIDAndTeamCode(
		ctx context.Context,
		ownerID string,
		teamCode string,
	) ([]blocks.Block, []blocks.PlayerState, error)
	// FindBlocksAndStatesByLocationIDAndTeamCode fetches blocks and their states by location and team code (legacy method)
	FindBlocksAndStatesByLocationIDAndTeamCode(
		ctx context.Context,
		locationID string,
		teamCode string,
	) ([]blocks.Block, []blocks.PlayerState, error)

	// Update updates an existing block
	Update(ctx context.Context, block blocks.Block) (blocks.Block, error)

	// Delete deletes a block by its ID
	// Requires a transaction as related data will also need to be deleted
	Delete(ctx context.Context, tx *bun.Tx, blockID string) error
	// DeleteByOwnerID deletes all blocks associated with an owner ID
	// Requires a transaction as related data will also need to be deleted
	DeleteByOwnerID(ctx context.Context, tx *bun.Tx, ownerID string) error
	// DeleteByLocationID deletes all blocks associated with a location ID (legacy method)
	// Requires a transaction as related data will also need to be deleted
	DeleteByLocationID(ctx context.Context, tx *bun.Tx, locationID string) error

	// Reorder reorders the blocks for a specific location
	Reorder(ctx context.Context, blockIDs []string) error
}

type blockRepository struct {
	db        *bun.DB
	stateRepo BlockStateRepository
}

func NewBlockRepository(db *bun.DB, stateRepo BlockStateRepository) BlockRepository {
	return &blockRepository{
		db:        db,
		stateRepo: stateRepo,
	}
}

// FindByOwnerID fetches all blocks for an owner (context agnostic).
func (r *blockRepository) FindByOwnerID(ctx context.Context, ownerID string) (blocks.Blocks, error) {
	modelBlocks := []models.Block{}
	err := r.db.NewSelect().
		Model(&modelBlocks).
		Where("owner_id = ?", ownerID).
		Order("ordering ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return convertModelsToBlocks(modelBlocks)
}

// FindByOwnerIDAndContext fetches all blocks for an owner with specific context.
func (r *blockRepository) FindByOwnerIDAndContext(
	ctx context.Context,
	ownerID string,
	blockContext blocks.BlockContext,
) (blocks.Blocks, error) {
	modelBlocks := []models.Block{}
	err := r.db.NewSelect().
		Model(&modelBlocks).
		Where("owner_id = ? AND context = ?", ownerID, blockContext).
		Order("ordering ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return convertModelsToBlocks(modelBlocks)
}

// FindByLocationID fetches all blocks for a location (legacy method).
func (r *blockRepository) FindByLocationID(ctx context.Context, locationID string) (blocks.Blocks, error) {
	modelBlocks := []models.Block{}
	err := r.db.NewSelect().
		Model(&modelBlocks).
		Where("owner_id = ?", locationID).
		Order("ordering ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return convertModelsToBlocks(modelBlocks)
}

// GetByID fetches a block by its ID.
func (r *blockRepository) GetByID(ctx context.Context, blockID string) (blocks.Block, error) {
	modelBlock := &models.Block{}
	err := r.db.NewSelect().
		Model(modelBlock).
		Where("id = ?", blockID).
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return convertModelToBlock(modelBlock)
}

// Create saves a new block to the database.
func (r *blockRepository) Create(
	ctx context.Context,
	block blocks.Block,
	ownerID string,
	blockContext blocks.BlockContext,
) (blocks.Block, error) {
	modelBlock := models.Block{
		ID:                 uuid.New().String(),
		OwnerID:            ownerID,
		Type:               block.GetType(),
		Context:            blockContext,
		Data:               block.GetData(),
		Ordering:           block.GetOrder(),
		Points:             block.GetPoints(),
		ValidationRequired: block.RequiresValidation(),
	}
	_, err := r.db.NewInsert().Model(&modelBlock).Exec(ctx)
	if err != nil {
		return nil, err
	}
	// Convert back to block and return
	createdBlock, err := convertModelToBlock(&modelBlock)
	if err != nil {
		return nil, err
	}
	return createdBlock, nil
}

// Update saves an existing block to the database.
func (r *blockRepository) Update(ctx context.Context, block blocks.Block) (blocks.Block, error) {
	modelBlock := convertBlockToModel(block)
	_, err := r.db.NewUpdate().Model(&modelBlock).WherePK().Exec(ctx)
	if err != nil {
		return nil, err
	}
	// Convert back to block and return
	updatedBlock, err := convertModelToBlock(&modelBlock)
	if err != nil {
		return nil, err
	}
	return updatedBlock, nil
}

// Convert block to model.
func convertBlockToModel(block blocks.Block) models.Block {
	return models.Block{
		ID:                 block.GetID(),
		OwnerID:            block.GetLocationID(), // Use GetLocationID as OwnerID for backward compatibility
		Type:               block.GetType(),
		Context:            blocks.ContextLocationContent, // Set context for polymorphic relation
		Ordering:           block.GetOrder(),
		Data:               block.GetData(),
		Points:             block.GetPoints(),
		ValidationRequired: block.RequiresValidation(),
	}
}

func convertModelsToBlocks(modelBlocks []models.Block) (blocks.Blocks, error) {
	b := make(blocks.Blocks, len(modelBlocks))
	for i, modelBlock := range modelBlocks {
		block, err := convertModelToBlock(&modelBlock)
		if err != nil {
			return nil, err
		}
		b[i] = block
	}
	return b, nil
}

func convertModelToBlock(model *models.Block) (blocks.Block, error) {
	// Convert model to block
	newBlock, err := blocks.CreateFromBaseBlock(blocks.BaseBlock{
		ID:         model.ID,
		LocationID: model.OwnerID, // Map OwnerID to LocationID for backward compatibility
		Type:       model.Type,
		Data:       model.Data,
		Order:      model.Ordering,
		Points:     model.Points,
	})
	if err != nil {
		return nil, err
	}
	err = newBlock.ParseData()
	if err != nil {
		return nil, err
	}
	return newBlock, nil
}

// Delete deletes a block from the database.
func (r *blockRepository) Delete(ctx context.Context, tx *bun.Tx, blockID string) error {
	_, err := tx.NewDelete().Model(&models.Block{}).Where("id = ?", blockID).Exec(ctx)
	return err
}

// DeleteByOwnerID deletes all blocks for an owner.
func (r *blockRepository) DeleteByOwnerID(ctx context.Context, tx *bun.Tx, ownerID string) error {
	_, err := tx.NewDelete().Model(&models.Block{}).Where("owner_id = ?", ownerID).Exec(ctx)
	return err
}

// DeleteByLocationID deletes all blocks for a location (legacy method).
func (r *blockRepository) DeleteByLocationID(ctx context.Context, tx *bun.Tx, locationID string) error {
	_, err := tx.NewDelete().Model(&models.Block{}).Where("owner_id = ?", locationID).Exec(ctx)
	return err
}

// Reorder reorders the blocks.
func (r *blockRepository) Reorder(ctx context.Context, blockIDs []string) error {
	for i, blockID := range blockIDs {
		_, err := r.db.NewUpdate().
			Model(&models.Block{}).
			Set("ordering = ?", i).
			Where("id = ?", blockID).
			Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// FindBlocksAndStatesByOwnerIDAndTeamCode fetches all blocks for an owner with their player states.
func (r *blockRepository) FindBlocksAndStatesByOwnerIDAndTeamCode(
	ctx context.Context,
	ownerID string,
	teamCode string,
) ([]blocks.Block, []blocks.PlayerState, error) {
	if teamCode == "" {
		return nil, nil, errors.New("team code must be set")
	}

	modelBlocks := []models.Block{}
	states := []models.TeamBlockState{}

	err := r.db.NewSelect().
		Model(&modelBlocks).
		Where("owner_id = ?", ownerID).
		Order("ordering ASC").
		Scan(ctx)
	if err != nil {
		return nil, nil, err
	}

	err = r.db.NewSelect().
		Model(&states).
		Where("block_id IN (?)", r.db.NewSelect().Model((*models.Block)(nil)).Column("id").Where("owner_id = ?", ownerID)).
		Where("team_code = ?", teamCode).
		Scan(ctx)
	if err != nil {
		return nil, nil, err
	}

	foundBlocks, err := convertModelsToBlocks(modelBlocks)
	if err != nil {
		return nil, nil, err
	}

	playerStates := make([]blocks.PlayerState, len(states))
	for i, state := range states {
		playerStates[i] = convertModelToPlayerStateData(state)
	}

	// Populate playerStates with empty states for blocks without a state
	for _, block := range foundBlocks {
		found := false
		for _, state := range playerStates {
			if state.GetBlockID() == block.GetID() {
				found = true
				break
			}
		}
		if !found {
			if block.RequiresValidation() && teamCode != "" {
				newState, err := r.stateRepo.NewBlockState(ctx, block.GetID(), teamCode)
				if err != nil {
					return nil, nil, err
				}
				newState, err = r.stateRepo.Create(ctx, newState)
				if err != nil {
					return nil, nil, err
				}
				playerStates = append(playerStates, newState)
			} else {
				newState, err := r.stateRepo.NewBlockState(ctx, block.GetID(), "")
				if err != nil {
					return nil, nil, err
				}
				playerStates = append(playerStates, newState)
			}
		}
	}

	return foundBlocks, playerStates, nil
}

// FindBlocksAndStatesByLocationIDAndTeamCode fetches all blocks for a location with their player states (legacy method).
func (r *blockRepository) FindBlocksAndStatesByLocationIDAndTeamCode(
	ctx context.Context,
	locationID string,
	teamCode string,
) ([]blocks.Block, []blocks.PlayerState, error) {
	if teamCode == "" {
		return nil, nil, errors.New("team code must be set")
	}

	modelBlocks := []models.Block{}
	states := []models.TeamBlockState{}

	err := r.db.NewSelect().
		Model(&modelBlocks).
		Where("owner_id = ?", locationID).
		Order("ordering ASC").
		Scan(ctx)
	if err != nil {
		return nil, nil, err
	}

	err = r.db.NewSelect().
		Model(&states).
		Where("block_id IN (?)", r.db.NewSelect().Model((*models.Block)(nil)).Column("id").Where("owner_id = ?", locationID)).
		Where("team_code = ?", teamCode).
		Scan(ctx)
	if err != nil {
		return nil, nil, err
	}

	foundBlocks, err := convertModelsToBlocks(modelBlocks)
	if err != nil {
		return nil, nil, err
	}

	playerStates := make([]blocks.PlayerState, len(states))
	for i, state := range states {
		playerStates[i] = convertModelToPlayerStateData(state)
	}

	// Populate playerStates with empty states for blocks without a state
	for _, block := range foundBlocks {
		found := false
		for _, state := range playerStates {
			if state.GetBlockID() == block.GetID() {
				found = true
				break
			}
		}
		if !found {
			if block.RequiresValidation() && teamCode != "" {
				newState, err := r.stateRepo.NewBlockState(ctx, block.GetID(), teamCode)
				if err != nil {
					return nil, nil, err
				}
				newState, err = r.stateRepo.Create(ctx, newState)
				if err != nil {
					return nil, nil, err
				}
				playerStates = append(playerStates, newState)
			} else {
				newState, err := r.stateRepo.NewBlockState(ctx, block.GetID(), "")
				if err != nil {
					return nil, nil, err
				}
				playerStates = append(playerStates, newState)
			}
		}
	}

	return foundBlocks, playerStates, nil
}

// GetBlockAndStateByBlockIDAndTeamCode fetches a block by its ID with the player state for a given team.
func (r *blockRepository) GetBlockAndStateByBlockIDAndTeamCode(
	ctx context.Context,
	blockID string,
	teamCode string,
) (blocks.Block, blocks.PlayerState, error) {
	modelBlock := models.Block{}
	err := r.db.NewSelect().
		Model(&modelBlock).
		Where("id = ?", blockID).
		Scan(ctx)
	if err != nil {
		return nil, nil, err
	}

	state, err := r.stateRepo.GetByBlockAndTeam(ctx, blockID, teamCode)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return nil, nil, err
	} else if err != nil {
		state, err = r.stateRepo.NewBlockState(ctx, blockID, teamCode)
		if err != nil {
			return nil, nil, err
		}
	}

	block, err := convertModelToBlock(&modelBlock)
	if err != nil {
		return nil, nil, err
	}

	return block, state, nil
}
