package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v5/blocks"
	"github.com/nathanhollows/Rapua/v5/models"
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
	// FindBlocksAndStatesByOwnerIDAndTeamCode fetches blocks and their states by owner and team code
	FindBlocksAndStatesByOwnerIDAndTeamCode(
		ctx context.Context,
		ownerID string,
		teamCode string,
	) ([]blocks.Block, []blocks.PlayerState, error)
	// FindBlocksAndStatesByOwnerIDAndTeamCodeWithContext fetches blocks and their states by owner, team code, and context
	FindBlocksAndStatesByOwnerIDAndTeamCodeWithContext(
		ctx context.Context,
		ownerID string,
		teamCode string,
		blockContext blocks.BlockContext,
	) ([]blocks.Block, []blocks.PlayerState, error)

	// Update updates an existing block
	Update(ctx context.Context, block blocks.Block) (blocks.Block, error)

	// Delete deletes a block by its ID
	// Requires a transaction as related data will also need to be deleted
	Delete(ctx context.Context, tx *bun.Tx, blockID string) error
	// DeleteByOwnerID deletes all blocks associated with an owner ID
	// Requires a transaction as related data will also need to be deleted
	DeleteByOwnerID(ctx context.Context, tx *bun.Tx, ownerID string) error

	// Reorder reorders the blocks for a specific location
	Reorder(ctx context.Context, blockIDs []string) error

	// DuplicateBlocksByOwner duplicates all blocks from oldOwnerID to newOwnerID
	// Preserves all block properties including context, ordering, points, etc.
	DuplicateBlocksByOwner(ctx context.Context, oldOwnerID, newOwnerID string) error
	// DuplicateBlocksByOwnerTx duplicates all blocks within a transaction
	DuplicateBlocksByOwnerTx(ctx context.Context, tx *bun.Tx, oldOwnerID, newOwnerID string) error
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

	count, err := r.db.NewSelect().
		Model((*models.Block)(nil)).
		Where("owner_id = ? AND context = ?", ownerID, blockContext).
		Count(ctx)
	if err != nil {
		return nil, err
	}
	modelBlock.Ordering = count

	// Insert into database

	_, err = r.db.NewInsert().Model(&modelBlock).Exec(ctx)
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
	_, err := r.db.NewUpdate().
		Model(&modelBlock).
		Column("data", "ordering", "points").
		WherePK().
		Exec(ctx)
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

// Reorder reorders the blocks.
func (r *blockRepository) Reorder(ctx context.Context, blockIDs []string) error {
	values := make([]struct {
		ID       string `bun:"id"`
		Ordering int    `bun:"ordering"`
	}, len(blockIDs))
	for i, blockID := range blockIDs {
		values[i].ID = blockID
		values[i].Ordering = i
	}
	vals := r.db.NewValues(&values)
	_, err := r.db.NewUpdate().
		With("_data", vals).
		Model((*models.Block)(nil)).
		TableExpr("_data").
		Set("ordering = _data.ordering").
		Where("block.id = _data.id").
		Exec(ctx)
	return err
}

// FindBlocksAndStatesByOwnerIDAndTeamCode fetches all blocks for an owner with their existing player states.
// Does not create missing states - that's the service layer's responsibility.
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

	playerStates := make([]blocks.PlayerState, 0, len(states))
	for _, state := range states {
		playerStates = append(playerStates, convertModelToPlayerStateData(state))
	}

	return foundBlocks, playerStates, nil
}

// FindBlocksAndStatesByOwnerIDAndTeamCodeWithContext fetches blocks for an owner with specific context and their existing player states.
// Does not create missing states - that's the service layer's responsibility.
func (r *blockRepository) FindBlocksAndStatesByOwnerIDAndTeamCodeWithContext(
	ctx context.Context,
	ownerID string,
	teamCode string,
	blockContext blocks.BlockContext,
) ([]blocks.Block, []blocks.PlayerState, error) {
	if teamCode == "" {
		return nil, nil, errors.New("team code must be set")
	}

	modelBlocks := []models.Block{}
	states := []models.TeamBlockState{}

	err := r.db.NewSelect().
		Model(&modelBlocks).
		Where("owner_id = ? AND context = ?", ownerID, blockContext).
		Order("ordering ASC").
		Scan(ctx)
	if err != nil {
		return nil, nil, err
	}

	err = r.db.NewSelect().
		Model(&states).
		Where("block_id IN (?)", r.db.NewSelect().Model((*models.Block)(nil)).Column("id").Where("owner_id = ? AND context = ?", ownerID, blockContext)).
		Where("team_code = ?", teamCode).
		Scan(ctx)
	if err != nil {
		return nil, nil, err
	}

	foundBlocks, err := convertModelsToBlocks(modelBlocks)
	if err != nil {
		return nil, nil, err
	}

	playerStates := make([]blocks.PlayerState, 0, len(states))
	for _, state := range states {
		playerStates = append(playerStates, convertModelToPlayerStateData(state))
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

// DuplicateBlocksByOwner duplicates all blocks from one owner to another.
// This is more efficient than fetching, converting to domain, and recreating blocks
// because it preserves all fields (including context) at the model layer.
func (r *blockRepository) DuplicateBlocksByOwner(
	ctx context.Context,
	oldOwnerID, newOwnerID string,
) error {
	// Fetch all blocks for the old owner
	var originalBlocks []models.Block
	err := r.db.NewSelect().
		Model(&originalBlocks).
		Where("owner_id = ?", oldOwnerID).
		Scan(ctx)
	if err != nil {
		return err
	}

	// Nothing to duplicate
	if len(originalBlocks) == 0 {
		return nil
	}

	// Create new blocks with new IDs and owner
	newBlocks := make([]models.Block, len(originalBlocks))
	for i, original := range originalBlocks {
		newBlocks[i] = models.Block{
			ID:                 uuid.New().String(),
			OwnerID:            newOwnerID,
			Type:               original.Type,
			Context:            original.Context,
			Data:               original.Data,
			Ordering:           original.Ordering,
			Points:             original.Points,
			ValidationRequired: original.ValidationRequired,
		}
	}

	// Bulk insert all blocks
	_, err = r.db.NewInsert().
		Model(&newBlocks).
		Exec(ctx)

	return err
}

// DuplicateBlocksByOwnerTx duplicates all blocks from one owner to another within a transaction.
func (r *blockRepository) DuplicateBlocksByOwnerTx(
	ctx context.Context,
	tx *bun.Tx,
	oldOwnerID, newOwnerID string,
) error {
	// Fetch all blocks for the old owner
	var originalBlocks []models.Block
	err := tx.NewSelect().
		Model(&originalBlocks).
		Where("owner_id = ?", oldOwnerID).
		Scan(ctx)
	if err != nil {
		return err
	}

	// Nothing to duplicate
	if len(originalBlocks) == 0 {
		return nil
	}

	// Create new blocks with new IDs and owner
	newBlocks := make([]models.Block, len(originalBlocks))
	for i, original := range originalBlocks {
		newBlocks[i] = models.Block{
			ID:                 uuid.New().String(),
			OwnerID:            newOwnerID,
			Type:               original.Type,
			Context:            original.Context,
			Data:               original.Data,
			Ordering:           original.Ordering,
			Points:             original.Points,
			ValidationRequired: original.ValidationRequired,
		}
	}

	// Bulk insert all blocks
	_, err = tx.NewInsert().
		Model(&newBlocks).
		Exec(ctx)

	return err
}
