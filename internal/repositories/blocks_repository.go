package repositories

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/models"
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
	// UserOwnsBlock checks if a user owns a block
	UserOwnsBlock(ctx context.Context, userID, blockID string) (bool, error)
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

	// Delete deletes a block by its ID. Cascade handles block states.
	// Callers must delete any upload records for this block before calling.
	Delete(ctx context.Context, tx *bun.Tx, blockID string) error
	// DeleteByOwnerIDPreservingStates deletes all blocks for an owner and returns
	// the player states for blocks whose IDs appear in preserveIDs. The caller
	// must re-insert the returned states after recreating those blocks.
	DeleteByOwnerIDPreservingStates(ctx context.Context, tx *bun.Tx, ownerID string, preserveIDs []string) ([]*models.TeamBlockState, error)

	// Reorder reorders the blocks for a specific location
	Reorder(ctx context.Context, blockIDs []string) error

	// DuplicateBlocksByOwner duplicates all blocks from oldOwnerID to newOwnerID
	// Preserves all block properties including context, ordering, points, etc.
	DuplicateBlocksByOwner(ctx context.Context, oldOwnerID, newOwnerID string) error
	// DuplicateBlocksByOwnerTx duplicates all blocks within a transaction
	DuplicateBlocksByOwnerTx(ctx context.Context, tx *bun.Tx, oldOwnerID, newOwnerID string) error

	// CreateTx creates a new block within a transaction
	CreateTx(
		ctx context.Context,
		tx *bun.Tx,
		block blocks.Block,
		ownerID string,
		blockContext blocks.BlockContext,
	) (blocks.Block, error)

	// BulkCreate inserts multiple blocks for an owner with specific context
	// Blocks should have Order set explicitly; IDs will be generated
	BulkCreate(ctx context.Context, blockList []blocks.Block, ownerID string, blockContext blocks.BlockContext) error

	// FindModelsByOwnerIDs fetches raw model blocks for a list of owner IDs.
	// Unlike FindByOwnerID, the returned records preserve the Context field needed for export.
	FindModelsByOwnerIDs(ctx context.Context, ownerIDs []string) ([]models.Block, error)
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
	return r.convertModelsToBlocks(modelBlocks)
}

// FindModelsByOwnerIDs fetches raw model blocks for multiple owner IDs in one query.
func (r *blockRepository) FindModelsByOwnerIDs(ctx context.Context, ownerIDs []string) ([]models.Block, error) {
	if len(ownerIDs) == 0 {
		return nil, nil
	}
	var modelBlocks []models.Block
	err := r.db.NewSelect().
		Model(&modelBlocks).
		Where("owner_id IN (?)", bun.In(ownerIDs)).
		Order("owner_id ASC", "ordering ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return modelBlocks, nil
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
	return r.convertModelsToBlocks(modelBlocks)
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

// UserOwnsBlock checks if a user owns a block by checking ownership of the block's owner (instance or location).
func (r *blockRepository) UserOwnsBlock(ctx context.Context, userID, blockID string) (bool, error) {
	// Query to check block ownership through instances
	// For start/complete blocks: owner_id IS the instance_id
	// For location blocks: owner_id IS the location_id, which belongs to an instance
	count, err := r.db.NewSelect().
		Model((*models.Block)(nil)).
		ColumnExpr("1").
		Where("id = ?", blockID).
		WhereGroup(" AND ", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.
				// Instance-owned blocks (start/complete contexts)
				WhereGroup(" OR ", func(q *bun.SelectQuery) *bun.SelectQuery {
					return q.
						Where(
							"context IN (?)",
							bun.In([]blocks.BlockContext{blocks.ContextStart, blocks.ContextFinish}),
						).
						Where("owner_id IN (SELECT id FROM instances WHERE user_id = ?)", userID)
				}).
				// Location-owned blocks (all other contexts)
				WhereGroup(" OR ", func(q *bun.SelectQuery) *bun.SelectQuery {
					return q.
						Where("context NOT IN (?)", bun.In([]blocks.BlockContext{blocks.ContextStart, blocks.ContextFinish})).
						Where("owner_id IN (SELECT id FROM locations WHERE instance_id IN (SELECT id FROM instances WHERE user_id = ?))", userID)
				})
		}).
		Limit(1).
		Count(ctx)
	if err != nil {
		return false, err
	}
	return count > 0, nil
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

// CreateTx creates a new block within a transaction.
func (r *blockRepository) CreateTx(
	ctx context.Context,
	tx *bun.Tx,
	block blocks.Block,
	ownerID string,
	blockContext blocks.BlockContext,
) (blocks.Block, error) {
	blockID := block.GetID()
	if blockID == "" {
		blockID = uuid.New().String()
	}
	modelBlock := models.Block{
		ID:                 blockID,
		OwnerID:            ownerID,
		Type:               block.GetType(),
		Context:            blockContext,
		Data:               block.GetData(),
		Ordering:           block.GetOrder(),
		Points:             block.GetPoints(),
		ValidationRequired: block.RequiresValidation(),
	}

	_, err := tx.NewInsert().Model(&modelBlock).Exec(ctx)
	if err != nil {
		return nil, err
	}
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
		OwnerID:            block.GetOwnerID(),
		Type:               block.GetType(),
		Context:            blocks.ContextLocationContent, // Set context for polymorphic relation
		Ordering:           block.GetOrder(),
		Data:               block.GetData(),
		Points:             block.GetPoints(),
		ValidationRequired: block.RequiresValidation(),
	}
}

func (r *blockRepository) convertModelsToBlocks(modelBlocks []models.Block) (blocks.Blocks, error) {
	b := make(blocks.Blocks, 0, len(modelBlocks))
	for _, modelBlock := range modelBlocks {
		block, err := convertModelToBlock(&modelBlock)
		if err != nil {
			// Skip unknown block types gracefully - they may exist in another branch
			if errors.Is(err, blocks.ErrBlockTypeNotFound) {
				continue
			}
			return nil, err
		}
		b = append(b, block)
	}
	return b, nil
}

func convertModelToBlock(model *models.Block) (blocks.Block, error) {
	// Convert model to block
	newBlock, err := blocks.CreateFromBaseBlock(blocks.BaseBlock{
		ID:      model.ID,
		OwnerID: model.OwnerID,
		Type:    model.Type,
		Data:    model.Data,
		Order:   model.Ordering,
		Points:  model.Points,
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

// DeleteByOwnerIDPreservingStates deletes all blocks for an owner.
// Player states for blocks in preserveIDs are saved and returned so the caller
// can re-insert them after recreating those blocks with the same IDs.
// Cascade handles states for all other blocks.
func (r *blockRepository) DeleteByOwnerIDPreservingStates(
	ctx context.Context,
	tx *bun.Tx,
	ownerID string,
	preserveIDs []string,
) ([]*models.TeamBlockState, error) {
	// Save states for blocks that will be recreated with the same IDs.
	// Cascade will wipe them when the blocks are deleted; the caller re-inserts them.
	var savedStates []*models.TeamBlockState
	if len(preserveIDs) > 0 {
		err := tx.NewSelect().
			Model(&savedStates).
			Where("block_id IN (?)", bun.In(preserveIDs)).
			Scan(ctx)
		if err != nil {
			return nil, fmt.Errorf("saving preserved states: %w", err)
		}
	}

	// Delete all blocks — cascade handles states for all of them
	_, err := tx.NewDelete().Model(&models.Block{}).Where("owner_id = ?", ownerID).Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("deleting blocks for owner %s: %w", ownerID, err)
	}
	return savedStates, nil
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

	foundBlocks, err := r.convertModelsToBlocks(modelBlocks)
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

	foundBlocks, err := r.convertModelsToBlocks(modelBlocks)
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

// BulkCreate inserts multiple blocks for an owner with specific context.
// Converts domain blocks to models and inserts them efficiently.
func (r *blockRepository) BulkCreate(
	ctx context.Context,
	blockList []blocks.Block,
	ownerID string,
	blockContext blocks.BlockContext,
) error {
	if len(blockList) == 0 {
		return nil
	}

	modelBlocks := make([]models.Block, len(blockList))
	for i, block := range blockList {
		modelBlocks[i] = models.Block{
			ID:                 uuid.New().String(),
			OwnerID:            ownerID,
			Type:               block.GetType(),
			Context:            blockContext,
			Data:               block.GetData(),
			Ordering:           block.GetOrder(),
			Points:             block.GetPoints(),
			ValidationRequired: block.RequiresValidation(),
		}
	}

	_, err := r.db.NewInsert().Model(&modelBlocks).Exec(ctx)
	return err
}
