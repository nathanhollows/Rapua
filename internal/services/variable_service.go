package services

import (
	"context"
	"fmt"
	"slices"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
)

// VarRef is one block's part in a quest's variables.
type VarRef struct {
	BlockID   string
	BlockName string
	Vars      []string
}

// VarUsage summarises the variables an objective's own blocks touch, for a row
// in the quest tree. Its children are left to their own rows.
type VarUsage struct {
	// Gated lists the blocks hidden until their variables are set.
	Gated []VarRef
	// Sets lists the blocks that set a variable when completed.
	Sets []VarRef
}

// VariableService answers what a quest's blocks set and wait on.
type VariableService struct {
	objectiveRepo repositories.ObjectiveRepository
	blockRepo     repositories.BlockRepository
}

// NewVariableService creates a VariableService.
func NewVariableService(
	objectiveRepo repositories.ObjectiveRepository,
	blockRepo repositories.BlockRepository,
) *VariableService {
	return &VariableService{objectiveRepo: objectiveRepo, blockRepo: blockRepo}
}

// QuestVariables returns every variable some block in the quest can set,
// sorted, for the editor to suggest. Drafts count: an author gating on
// something in a draft is preparing both to go out together.
func (s *VariableService) QuestVariables(ctx context.Context, questID string) ([]string, error) {
	questBlocks, err := s.questBlocks(ctx, questID)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, block := range questBlocks {
		for _, name := range game.DeclaredVars(block) {
			if !slices.Contains(names, name) {
				names = append(names, name)
			}
		}
	}
	slices.Sort(names)
	return names, nil
}

// UsageByOwner returns, per owner, which of its blocks set or wait on a
// variable. Owners with neither are left out.
func (s *VariableService) UsageByOwner(ctx context.Context, questID string) (map[string]VarUsage, error) {
	questBlocks, err := s.questBlocks(ctx, questID)
	if err != nil {
		return nil, err
	}
	usage := make(map[string]VarUsage)
	for _, block := range questBlocks {
		u := usage[block.GetOwnerID()]
		touched := false
		if deps := block.GetDepends(); len(deps) > 0 && !block.RequiresValidation() {
			u.Gated = append(u.Gated, VarRef{BlockID: block.GetID(), BlockName: block.GetName(), Vars: deps})
			touched = true
		}
		if vars := game.DeclaredVars(block); len(vars) > 0 {
			u.Sets = append(u.Sets, VarRef{BlockID: block.GetID(), BlockName: block.GetName(), Vars: vars})
			touched = true
		}
		if touched {
			usage[block.GetOwnerID()] = u
		}
	}
	return usage, nil
}

// questBlocks loads every block in the quest, on its pages and in its
// objectives, in one query.
func (s *VariableService) questBlocks(ctx context.Context, questID string) (blocks.Blocks, error) {
	objectives, err := s.objectiveRepo.FindTreeByQuestID(ctx, questID)
	if err != nil {
		return nil, fmt.Errorf("loading objectives: %w", err)
	}
	ownerIDs := make([]string, 0, len(objectives)+1)
	ownerIDs = append(ownerIDs, questID)
	for i := range objectives {
		ownerIDs = append(ownerIDs, objectives[i].ID)
	}
	rows, err := s.blockRepo.FindModelsByOwnerIDs(ctx, ownerIDs)
	if err != nil {
		return nil, fmt.Errorf("loading blocks: %w", err)
	}
	out := make(blocks.Blocks, 0, len(rows))
	for _, m := range rows {
		block, err := blocks.CreateFromBaseBlock(blocks.BaseBlock{
			ID: m.ID, OwnerID: m.OwnerID, Type: m.Type, Data: m.Data, Order: m.Ordering, Points: m.Points,
		})
		if err != nil {
			// A block type no longer registered sets nothing anyone can see.
			continue
		}
		if err := block.ParseData(); err != nil {
			return nil, fmt.Errorf("parsing block %s: %w", m.ID, err)
		}
		out = append(out, block)
	}
	return out, nil
}
