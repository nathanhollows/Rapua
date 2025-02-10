package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/nathanhollows/Rapua/blocks"
	"github.com/nathanhollows/Rapua/db"
	"github.com/nathanhollows/Rapua/models"
	"github.com/nathanhollows/Rapua/repositories"
)

type BlockService interface {
	// NewBlock creates a new content block of the specified type for the given location
	NewBlock(ctx context.Context, locationID string, blockType string) (blocks.Block, error)
	// NewBlockState creates a new player state for the given block and team
	NewBlockState(ctx context.Context, blockID, teamCode string) (blocks.PlayerState, error)
	// NewMockBlockState creates a mock player state (for testing/demo scenarios)
	NewMockBlockState(ctx context.Context, blockID, teamCode string) (blocks.PlayerState, error)

	// GetByBlockID fetches a content block by its ID
	GetByBlockID(ctx context.Context, blockID string) (blocks.Block, error)
	// GetBlockWithStateByBlockIDAndTeamCode fetches a block + its state
	// for the given block ID and team
	GetBlockWithStateByBlockIDAndTeamCode(ctx context.Context, blockID, teamCode string) (blocks.Block, blocks.PlayerState, error)
	// FindByLocationID fetches all content blocks for a location
	FindByLocationID(ctx context.Context, locationID string) (blocks.Blocks, error)
	// FindByLocationIDAndTeamCodeWithState fetches all blocks and their states
	// for the given location and team
	FindByLocationIDAndTeamCodeWithState(ctx context.Context, locationID, teamCode string) ([]blocks.Block, map[string]blocks.PlayerState, error)

	// UpdateBlock updates the data for the given block
	UpdateBlock(ctx context.Context, block blocks.Block, data map[string][]string) (blocks.Block, error)
	// UpdateState updates the player state for a block
	UpdateState(ctx context.Context, state blocks.PlayerState) (blocks.PlayerState, error)
	// ReorderBlocks changes the display/order of blocks at a location
	ReorderBlocks(ctx context.Context, locationID string, blockIDs []string) error

	// DeleteBlock removes the specified block and its associated player states
	DeleteBlock(ctx context.Context, blockID string) error

	// CheckValidationRequiredForLocation checks if any blocks in a location require validation
	CheckValidationRequiredForLocation(ctx context.Context, locationID string) (bool, error)
	// CheckValidationRequiredForCheckIn checks if any blocks still require validation for a check-in
	CheckValidationRequiredForCheckIn(ctx context.Context, locationID, teamCode string) (bool, error)
}
type blockService struct {
	transactor     db.Transactor
	blockRepo      repositories.BlockRepository
	blockStateRepo repositories.BlockStateRepository
}

func NewBlockService(transactor db.Transactor, blockRepo repositories.BlockRepository, blockStateRepo repositories.BlockStateRepository) BlockService {
	return &blockService{
		transactor:     transactor,
		blockRepo:      blockRepo,
		blockStateRepo: blockStateRepo,
	}
}

// GetByBlockID fetches a content block by its ID.
func (s *blockService) GetByBlockID(ctx context.Context, blockID string) (blocks.Block, error) {
	return s.blockRepo.GetByID(ctx, blockID)
}

// FindByLocationID fetches all content blocks for a location.
func (s *blockService) FindByLocationID(ctx context.Context, locationID string) (blocks.Blocks, error) {
	if locationID == "" {
		return nil, errors.New("location must be set")
	}
	return s.blockRepo.FindByLocationID(ctx, locationID)
}

func (s *blockService) NewBlock(ctx context.Context, locationID string, blockType string) (blocks.Block, error) {
	if locationID == "" {
		return nil, errors.New("location must be set")
	}
	if blockType == "" {
		return nil, errors.New("block type must be set")
	}
	// Use the blocks package to create the appropriate block based on the type.
	baseBlock := blocks.BaseBlock{
		Type:       blockType,
		LocationID: locationID,
	}

	// Let the blocks package handle the creation logic.
	block, err := blocks.CreateFromBaseBlock(baseBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to create block of type %s: %w", blockType, err)
	}

	// Store the new block in the repository.
	newBlock, err := s.blockRepo.Create(ctx, block, locationID)
	if err != nil {
		return nil, fmt.Errorf("failed to store block of type %s: %w", blockType, err)
	}

	return newBlock, nil
}

// NewBlockState creates a new block state.
func (s *blockService) NewBlockState(ctx context.Context, blockID, teamCode string) (blocks.PlayerState, error) {
	if blockID == "" {
		return nil, errors.New("blockID must be set")
	}
	if teamCode == "" {
		return nil, errors.New("teamCode must be set")
	}
	state, err := s.blockStateRepo.NewBlockState(ctx, blockID, teamCode)
	if err != nil {
		return nil, fmt.Errorf("creating new block state: %w", err)
	}
	state, err = s.blockStateRepo.Create(ctx, state)
	if err != nil {
		return nil, fmt.Errorf("storing new block state: %w", err)
	}
	return state, nil
}

// NewMockBlockState creates a new mock block state.
func (s *blockService) NewMockBlockState(ctx context.Context, blockID, teamCode string) (blocks.PlayerState, error) {
	if blockID == "" {
		return nil, errors.New("blockID must be set")
	}
	// teamCode may be blank
	state, err := s.blockStateRepo.NewBlockState(ctx, blockID, teamCode)
	if err != nil {
		return nil, fmt.Errorf("creating new block state: %w", err)
	}
	return state, nil
}

// UpdateBlock updates a block.
func (s *blockService) UpdateBlock(ctx context.Context, block blocks.Block, data map[string][]string) (blocks.Block, error) {
	err := block.UpdateBlockData(data)
	if err != nil {
		return nil, fmt.Errorf("updating block data: %w", err)
	}
	return s.blockRepo.Update(ctx, block)
}

// DeleteBlock deletes a block.
func (s *blockService) DeleteBlock(ctx context.Context, blockID string) error {
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}

	// Ensure rollback on failure
	defer func() {
		if p := recover(); p != nil {
			err := tx.Rollback()
			log.Printf("recovered from panic, rolling back transaction: %v", err)
			panic(p)
		}
	}()

	if err := s.blockRepo.Delete(ctx, tx, blockID); err != nil {
		tx.Rollback()
		return fmt.Errorf("deleting block: %w", err)
	}

	if err := s.blockStateRepo.DeleteByBlockID(ctx, tx, blockID); err != nil {
		tx.Rollback()
		return fmt.Errorf("deleting block state: %w", err)
	}

	return tx.Commit()
}

// ReorderBlocks reorders the blocks in a location.
func (s *blockService) ReorderBlocks(ctx context.Context, locationID string, blockIDs []string) error {
	return s.blockRepo.Reorder(ctx, locationID, blockIDs)
}

func (s *blockService) FindByLocationIDAndTeamCodeWithState(ctx context.Context, locationID, teamCode string) ([]blocks.Block, map[string]blocks.PlayerState, error) {
	if locationID == "" {
		return nil, nil, errors.New("locationID must be set")
	}
	foundBlocks, states, err := s.blockRepo.FindBlocksAndStatesByLocationIDAndTeamCode(ctx, locationID, teamCode)
	if err != nil {
		return nil, nil, err
	}

	// Create a map for easier lookup of block states by block ID
	blockStates := make(map[string]blocks.PlayerState, len(states))
	for _, state := range states {
		blockStates[state.GetBlockID()] = state
	}

	return foundBlocks, blockStates, nil
}

func (s *blockService) GetBlockWithStateByBlockIDAndTeamCode(ctx context.Context, blockID, teamCode string) (blocks.Block, blocks.PlayerState, error) {
	if blockID == "" || teamCode == "" {
		return nil, nil, fmt.Errorf("blockID and teamCode must be set, got blockID: %s, teamCode: %s", blockID, teamCode)
	}

	return s.blockRepo.GetBlockAndStateByBlockIDAndTeamCode(ctx, blockID, teamCode)
}

// Convert block to model.
func (s *blockService) ConvertBlockToModel(block blocks.Block) models.Block {
	return models.Block{
		ID:                 block.GetID(),
		LocationID:         block.GetLocationID(),
		Type:               block.GetType(),
		Ordering:           block.GetOrder(),
		Data:               block.GetData(),
		Points:             block.GetPoints(),
		ValidationRequired: block.RequiresValidation(),
	}
}

// CheckValidationRequiredForLocation checks if any blocks in a location require validation.
func (s *blockService) CheckValidationRequiredForLocation(ctx context.Context, locationID string) (bool, error) {
	blocks, err := s.FindByLocationID(ctx, locationID)
	if err != nil {
		return false, err
	}

	for _, block := range blocks {
		if block.RequiresValidation() {
			return true, nil
		}
	}

	return false, nil
}

// CheckValidationRequiredForCheckIn checks if any blocks still require validation for a check in.
func (s *blockService) CheckValidationRequiredForCheckIn(ctx context.Context, locationID, teamCode string) (bool, error) {
	blocks, state, err := s.FindByLocationIDAndTeamCodeWithState(ctx, locationID, teamCode)
	if err != nil {
		return false, err
	}

	for _, block := range blocks {
		if block.RequiresValidation() {
			if state[block.GetID()] == nil {
				return true, nil
			}
			if state[block.GetID()].IsComplete() {
				continue
			}
			return true, nil
		}
	}

	return false, nil
}

// UpdateState updates the player state for a block.
func (s *blockService) UpdateState(ctx context.Context, state blocks.PlayerState) (blocks.PlayerState, error) {
	return s.blockStateRepo.Update(ctx, state)
}
