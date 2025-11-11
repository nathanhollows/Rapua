package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v6/blocks"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
)

type BlockService struct {
	blockRepo      repositories.BlockRepository
	blockStateRepo repositories.BlockStateRepository
}

func NewBlockService(
	blockRepo repositories.BlockRepository,
	blockStateRepo repositories.BlockStateRepository,
) *BlockService {
	return &BlockService{
		blockRepo:      blockRepo,
		blockStateRepo: blockStateRepo,
	}
}

// GetByBlockID fetches a content block by its ID.
func (s *BlockService) GetByBlockID(ctx context.Context, blockID string) (blocks.Block, error) {
	return s.blockRepo.GetByID(ctx, blockID)
}

// FindByOwnerID fetches all content blocks for an owner (context agnostic).
func (s *BlockService) FindByOwnerID(ctx context.Context, ownerID string) (blocks.Blocks, error) {
	if ownerID == "" {
		return nil, errors.New("ownerID cannot be empty")
	}
	return s.blockRepo.FindByOwnerID(ctx, ownerID)
}

// FindByOwnerIDAndContext fetches all content blocks for an owner with specific context.
func (s *BlockService) FindByOwnerIDAndContext(
	ctx context.Context,
	ownerID string,
	blockContext blocks.BlockContext,
) (blocks.Blocks, error) {
	if ownerID == "" {
		return nil, errors.New("ownerID cannot be empty")
	}
	return s.blockRepo.FindByOwnerIDAndContext(ctx, ownerID, blockContext)
}

// NewBlockWithOwnerAndContext creates a new block for an owner with specific context.
func (s *BlockService) NewBlockWithOwnerAndContext(
	ctx context.Context,
	ownerID string,
	blockContext blocks.BlockContext,
	blockType string,
) (blocks.Block, error) {
	if ownerID == "" {
		return nil, errors.New("ownerID cannot be empty")
	}
	if blockType == "" {
		return nil, errors.New("blockType cannot be empty")
	}
	// Use the blocks package to create the appropriate block based on the type.
	baseBlock := blocks.BaseBlock{
		Type:       blockType,
		LocationID: ownerID, // Use ownerID as LocationID for backward compatibility
	}

	// Let the blocks package handle the creation logic.
	block, err := blocks.CreateFromBaseBlock(baseBlock)
	if err != nil {
		return nil, fmt.Errorf("failed to create block of type %s: %w", blockType, err)
	}

	// Store the new block in the repository.
	newBlock, err := s.blockRepo.Create(ctx, block, ownerID, blockContext)
	if err != nil {
		return nil, fmt.Errorf("failed to store block of type %s: %w", blockType, err)
	}

	return newBlock, nil
}

// NewBlockState creates a new block state.
func (s *BlockService) NewBlockState(ctx context.Context, blockID, teamCode string) (blocks.PlayerState, error) {
	if blockID == "" {
		return nil, errors.New("blockID cannot be empty")
	}
	if teamCode == "" {
		return nil, errors.New("teamCode cannot be empty")
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
func (s *BlockService) NewMockBlockState(ctx context.Context, blockID, teamCode string) (blocks.PlayerState, error) {
	if blockID == "" {
		return nil, errors.New("blockID cannot be empty")
	}
	// teamCode may be blank
	state, err := s.blockStateRepo.NewBlockState(ctx, blockID, teamCode)
	if err != nil {
		return nil, fmt.Errorf("creating new block state: %w", err)
	}
	return state, nil
}

// UpdateBlock updates a block.
func (s *BlockService) UpdateBlock(
	ctx context.Context,
	block blocks.Block,
	data map[string][]string,
) (blocks.Block, error) {
	err := block.UpdateBlockData(data)
	if err != nil {
		return nil, fmt.Errorf("updating block data: %w", err)
	}
	return s.blockRepo.Update(ctx, block)
}

// ReorderBlocks reorders the blocks in a location.
func (s *BlockService) ReorderBlocks(ctx context.Context, blockIDs []string) error {
	return s.blockRepo.Reorder(ctx, blockIDs)
}

// FindByOwnerIDAndTeamCodeWithState fetches blocks and their states by owner and team code.
// Creates missing states for blocks that require validation.
func (s *BlockService) FindByOwnerIDAndTeamCodeWithState(
	ctx context.Context,
	ownerID, teamCode string,
) ([]blocks.Block, map[string]blocks.PlayerState, error) {
	if ownerID == "" {
		return nil, nil, errors.New("ownerID must be set")
	}
	foundBlocks, states, err := s.blockRepo.FindBlocksAndStatesByOwnerIDAndTeamCode(ctx, ownerID, teamCode)
	if err != nil {
		return nil, nil, err
	}

	// Create a map for easier lookup of block states by block ID
	blockStates := make(map[string]blocks.PlayerState, len(foundBlocks))
	for _, state := range states {
		blockStates[state.GetBlockID()] = state
	}

	// Populate missing states
	blockStates, err = s.populateMissingStates(ctx, foundBlocks, blockStates, teamCode)
	if err != nil {
		return nil, nil, err
	}

	return foundBlocks, blockStates, nil
}

// FindByOwnerIDAndTeamCodeWithStateAndContext fetches blocks and their states by owner, team code, and context.
// Creates missing states for blocks that require validation.
func (s *BlockService) FindByOwnerIDAndTeamCodeWithStateAndContext(
	ctx context.Context,
	ownerID, teamCode string,
	blockContext blocks.BlockContext,
) ([]blocks.Block, map[string]blocks.PlayerState, error) {
	if ownerID == "" {
		return nil, nil, errors.New("ownerID must be set")
	}
	foundBlocks, states, err := s.blockRepo.FindBlocksAndStatesByOwnerIDAndTeamCodeWithContext(
		ctx,
		ownerID,
		teamCode,
		blockContext,
	)
	if err != nil {
		return nil, nil, err
	}

	// Create a map for easier lookup of block states by block ID
	blockStates := make(map[string]blocks.PlayerState, len(foundBlocks))
	for _, state := range states {
		blockStates[state.GetBlockID()] = state
	}

	// Populate missing states
	blockStates, err = s.populateMissingStates(ctx, foundBlocks, blockStates, teamCode)
	if err != nil {
		return nil, nil, err
	}

	return foundBlocks, blockStates, nil
}

func (s *BlockService) populateMissingStates(
	ctx context.Context,
	blocks blocks.Blocks,
	existingStates map[string]blocks.PlayerState,
	teamCode string,
) (map[string]blocks.PlayerState, error) {
	// Populate missing states - service layer responsibility
	for _, block := range blocks {
		if _, exists := existingStates[block.GetID()]; exists {
			continue
		}

		newState, stateErr := s.createStateForBlock(ctx, block, teamCode)
		if stateErr != nil {
			return nil, stateErr
		}
		existingStates[block.GetID()] = newState
	}
	return existingStates, nil
}

func (s *BlockService) createStateForBlock(
	ctx context.Context,
	block blocks.Block,
	teamCode string,
) (blocks.PlayerState, error) {
	// Create new state based on block validation requirements
	if block.RequiresValidation() && teamCode != "" {
		// Persist state for validation-required blocks
		newState, err := s.NewBlockState(ctx, block.GetID(), teamCode)
		if err != nil {
			return nil, fmt.Errorf("creating block state for %s: %w", block.GetID(), err)
		}
		return newState, nil
	}

	// Mock state for non-validation blocks
	newState, err := s.NewMockBlockState(ctx, block.GetID(), "")
	if err != nil {
		return nil, fmt.Errorf("creating mock block state for %s: %w", block.GetID(), err)
	}
	return newState, nil
}

func (s *BlockService) GetBlockWithStateByBlockIDAndTeamCode(
	ctx context.Context,
	blockID, teamCode string,
) (blocks.Block, blocks.PlayerState, error) {
	if blockID == "" || teamCode == "" {
		return nil, nil, fmt.Errorf(
			"blockID and teamCode must be set, got blockID: %s, teamCode: %s",
			blockID,
			teamCode,
		)
	}

	return s.blockRepo.GetBlockAndStateByBlockIDAndTeamCode(ctx, blockID, teamCode)
}

// ConvertBlockToModel converts a block to its model representation.
func (s *BlockService) ConvertBlockToModel(block blocks.Block) models.Block {
	return models.Block{
		ID:                 block.GetID(),
		OwnerID:            block.GetLocationID(), // Use GetLocationID as OwnerID for backward compatibility
		Type:               block.GetType(),
		Ordering:           block.GetOrder(),
		Data:               block.GetData(),
		Points:             block.GetPoints(),
		ValidationRequired: block.RequiresValidation(),
	}
}

// CheckValidationRequiredForLocation checks if any blocks in a location require validation.
func (s *BlockService) CheckValidationRequiredForLocation(ctx context.Context, locationID string) (bool, error) {
	blocks, err := s.FindByOwnerID(ctx, locationID)
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
func (s *BlockService) CheckValidationRequiredForCheckIn(
	ctx context.Context,
	locationID, teamCode string,
) (bool, error) {
	blocks, state, err := s.FindByOwnerIDAndTeamCodeWithState(ctx, locationID, teamCode)
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
func (s *BlockService) UpdateState(ctx context.Context, state blocks.PlayerState) (blocks.PlayerState, error) {
	return s.blockStateRepo.Update(ctx, state)
}
