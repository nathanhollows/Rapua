package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v3/blocks"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
)

type BlockService struct {
	blockRepo      repositories.BlockRepository
	blockStateRepo repositories.BlockStateRepository
	checkInService CheckInService
	teamService    TeamService
}

func NewBlockService(blockRepo repositories.BlockRepository, blockStateRepo repositories.BlockStateRepository) *BlockService {
	return &BlockService{
		blockRepo:      blockRepo,
		blockStateRepo: blockStateRepo,
	}
}

// GetByBlockID fetches a content block by its ID.
func (s *BlockService) GetByBlockID(ctx context.Context, blockID string) (blocks.Block, error) {
	return s.blockRepo.GetByID(ctx, blockID)
}

// FindByLocationID fetches all content blocks for a location.
func (s *BlockService) FindByLocationID(ctx context.Context, locationID string) (blocks.Blocks, error) {
	if locationID == "" {
		return nil, errors.New("locationID cannot be empty")
	}
	return s.blockRepo.FindByLocationID(ctx, locationID)
}

func (s *BlockService) NewBlock(ctx context.Context, locationID string, blockType string) (blocks.Block, error) {
	if locationID == "" {
		return nil, errors.New("locationID cannot be empty")
	}
	if blockType == "" {
		return nil, errors.New("blockType cannot be empty")
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
func (s *BlockService) UpdateBlock(ctx context.Context, block blocks.Block, data map[string][]string) (blocks.Block, error) {
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

func (s *BlockService) FindByLocationIDAndTeamCodeWithState(ctx context.Context, locationID, teamCode string) ([]blocks.Block, map[string]blocks.PlayerState, error) {
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

func (s *BlockService) GetBlockWithStateByBlockIDAndTeamCode(ctx context.Context, blockID, teamCode string) (blocks.Block, blocks.PlayerState, error) {
	if blockID == "" || teamCode == "" {
		return nil, nil, fmt.Errorf("blockID and teamCode must be set, got blockID: %s, teamCode: %s", blockID, teamCode)
	}

	return s.blockRepo.GetBlockAndStateByBlockIDAndTeamCode(ctx, blockID, teamCode)
}

// Convert block to model.
func (s *BlockService) ConvertBlockToModel(block blocks.Block) models.Block {
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
func (s *BlockService) CheckValidationRequiredForLocation(ctx context.Context, locationID string) (bool, error) {
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
func (s *BlockService) CheckValidationRequiredForCheckIn(ctx context.Context, locationID, teamCode string) (bool, error) {
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
func (s *BlockService) UpdateState(ctx context.Context, state blocks.PlayerState) (blocks.PlayerState, error) {
	return s.blockStateRepo.Update(ctx, state)
}
