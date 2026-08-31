package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
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

func (s *BlockService) GetBlockContext(ctx context.Context, blockID string) (blocks.BlockContext, error) {
	return s.blockRepo.GetContext(ctx, blockID)
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
	// The admin "add block" dropdown only ever offers blocks valid for the
	// zone it's rendered in, but that's a client-side filter, not enforcement:
	// a direct POST (or a stale form after a block's registered contexts
	// change) could still ask to create e.g. a start-only block inside an
	// objective's proof/reveal zone. Reject it here, not just at import/lint
	// time (game.Lint via blocks.Registry already covers that separate path).
	if !blocks.CanBlockBeUsedInContext(blockType, blockContext) {
		return nil, fmt.Errorf("%w: block type %q cannot be used in context %q", ErrBlockNotValidForContext, blockType, blockContext)
	}
	// Use the blocks package to create the appropriate block based on the type.
	baseBlock := blocks.BaseBlock{
		Type:    blockType,
		OwnerID: ownerID,
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
func (s *BlockService) NewBlockState(
	ctx context.Context, blockID, runCode, questID string,
) (blocks.PlayerState, error) {
	if blockID == "" {
		return nil, errors.New("blockID cannot be empty")
	}
	if runCode == "" {
		return nil, errors.New("runCode cannot be empty")
	}
	state, err := s.blockStateRepo.NewBlockState(ctx, blockID, runCode, questID)
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
func (s *BlockService) NewMockBlockState(
	ctx context.Context, blockID, runCode, questID string,
) (blocks.PlayerState, error) {
	if blockID == "" {
		return nil, errors.New("blockID cannot be empty")
	}
	// runCode may be blank
	state, err := s.blockStateRepo.NewBlockState(ctx, blockID, runCode, questID)
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

// FindByOwnerIDAndRunCodeWithState fetches blocks and their states by owner and run code.
// Creates missing states for blocks that require validation.
func (s *BlockService) FindByOwnerIDAndRunCodeWithState(
	ctx context.Context,
	ownerID, runCode, questID string,
) ([]blocks.Block, map[string]blocks.PlayerState, error) {
	if ownerID == "" {
		return nil, nil, errors.New("ownerID must be set")
	}
	foundBlocks, states, err := s.blockRepo.FindBlocksAndStatesByOwnerIDAndRunCode(ctx, ownerID, runCode)
	if err != nil {
		return nil, nil, err
	}

	// Create a map for easier lookup of block states by block ID
	blockStates := make(map[string]blocks.PlayerState, len(foundBlocks))
	for _, state := range states {
		blockStates[state.GetBlockID()] = state
	}

	// Populate missing states
	blockStates, err = s.populateMissingStates(ctx, foundBlocks, blockStates, runCode, questID)
	if err != nil {
		return nil, nil, err
	}

	return foundBlocks, blockStates, nil
}

// FindByOwnerIDAndRunCodeWithStateAndContext fetches blocks and their states by owner, run code, and context.
// Creates missing states for blocks that require validation.
// When runCode is empty (e.g. preview mode), blocks are fetched without DB states and all
// states are created as in-memory mocks — no rows are written to team_block_states.
func (s *BlockService) FindByOwnerIDAndRunCodeWithStateAndContext(
	ctx context.Context,
	ownerID, runCode, questID string,
	blockContext blocks.BlockContext,
) ([]blocks.Block, map[string]blocks.PlayerState, error) {
	if ownerID == "" {
		return nil, nil, errors.New("ownerID must be set")
	}

	var foundBlocks []blocks.Block
	blockStates := make(map[string]blocks.PlayerState)

	if runCode == "" {
		// No real team (e.g. preview): fetch blocks only; states are all mocked below.
		var err error
		foundBlocks, err = s.blockRepo.FindByOwnerIDAndContext(ctx, ownerID, blockContext)
		if err != nil {
			return nil, nil, err
		}
	} else {
		var states []blocks.PlayerState
		var err error
		foundBlocks, states, err = s.blockRepo.FindBlocksAndStatesByOwnerIDAndRunCodeWithContext(
			ctx, ownerID, runCode, blockContext,
		)
		if err != nil {
			return nil, nil, err
		}
		for _, state := range states {
			blockStates[state.GetBlockID()] = state
		}
	}

	// Populate missing states
	var err error
	blockStates, err = s.populateMissingStates(ctx, foundBlocks, blockStates, runCode, questID)
	if err != nil {
		return nil, nil, err
	}

	return foundBlocks, blockStates, nil
}

func (s *BlockService) populateMissingStates(
	ctx context.Context,
	blocks blocks.Blocks,
	existingStates map[string]blocks.PlayerState,
	runCode, questID string,
) (map[string]blocks.PlayerState, error) {
	// Populate missing states - service layer responsibility
	for _, block := range blocks {
		if _, exists := existingStates[block.GetID()]; exists {
			continue
		}

		newState, stateErr := s.createStateForBlock(ctx, block, runCode, questID)
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
	runCode, questID string,
) (blocks.PlayerState, error) {
	// A preview carries the run code "preview", which has no row in runs, so
	// persisting against it trips the foreign key.
	isPreview := ctx.Value(contextkeys.PreviewKey) != nil

	// Create new state based on block validation requirements
	if block.RequiresValidation() && runCode != "" && !isPreview {
		// Persist state for validation-required blocks
		newState, err := s.NewBlockState(ctx, block.GetID(), runCode, questID)
		if err != nil {
			return nil, fmt.Errorf("creating block state for %s: %w", block.GetID(), err)
		}
		return newState, nil
	}

	// Mock state for non-validation blocks
	newState, err := s.NewMockBlockState(ctx, block.GetID(), "", questID)
	if err != nil {
		return nil, fmt.Errorf("creating mock block state for %s: %w", block.GetID(), err)
	}
	return newState, nil
}

func (s *BlockService) GetBlockWithStateByBlockIDAndRunCode(
	ctx context.Context,
	blockID, runCode, questID string,
) (blocks.Block, blocks.PlayerState, error) {
	if blockID == "" || runCode == "" {
		return nil, nil, fmt.Errorf(
			"blockID and runCode must be set, got blockID: %s, runCode: %s",
			blockID,
			runCode,
		)
	}

	return s.blockRepo.GetBlockAndStateByBlockIDAndRunCode(ctx, blockID, runCode, questID)
}

// ConvertBlockToModel converts a block to its model representation.
func (s *BlockService) ConvertBlockToModel(block blocks.Block) models.Block {
	return models.Block{
		ID:                 block.GetID(),
		OwnerID:            block.GetOwnerID(),
		Type:               block.GetType(),
		Ordering:           block.GetOrder(),
		Data:               block.GetData(),
		Points:             block.GetPoints(),
		ValidationRequired: block.RequiresValidation(),
	}
}

// checkValidationRequiredForCheckIn checks whether any blocks in an objective
// context still require validation.
func (s *BlockService) checkValidationRequiredForCheckIn(
	ctx context.Context,
	ownerID, runCode, questID string,
	blockContext game.BlockContext,
) (bool, error) {
	blocks, state, err := s.FindByOwnerIDAndRunCodeWithStateAndContext(
		ctx,
		ownerID,
		runCode,
		questID,
		blockContext,
	)
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
