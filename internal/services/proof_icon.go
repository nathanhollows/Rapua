package services

import (
	"context"
	"fmt"
	"sort"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/models"
)

// firstInteractiveProof names, for each objective, the first block in its proof
// that asks the player for something.
//
// That block is what the objective is really requesting, so the player's row
// can carry its icon and say what is coming without anyone having to classify
// objectives into kinds. A scan looks like a scan whether the code is on a wall
// or on a specimen, which is the ambiguity that made classifying them hard.
//
// Display blocks are skipped because they ask nothing: an objective whose proof
// is a paragraph and a photo upload is asking for the photo. An objective with
// no interactive proof at all contributes no entry, and the row draws its own
// door marker rather than borrowing a misleading icon.
func firstInteractiveProof(rows []models.Block, registry game.BlockRegistry) map[string]string {
	proof := make([]models.Block, 0, len(rows))
	for _, row := range rows {
		if row.Context == game.ContextObjectiveProof && registry.IsInteractive(row.Type) {
			proof = append(proof, row)
		}
	}
	// The author's ordering decides which block is first, not whatever order
	// the rows came back in.
	sort.SliceStable(proof, func(i, j int) bool { return proof[i].Ordering < proof[j].Ordering })

	first := make(map[string]string, len(proof))
	for _, row := range proof {
		if _, seen := first[row.OwnerID]; !seen {
			first[row.OwnerID] = row.Type
		}
	}
	return first
}

// proofBlockTypes names every block type the frontier's proof asks for, which
// is what the quest screen's own actions are derived from: a universal scanner
// is offered because something on offer can be scanned, and a quest with no
// codes in it never shows one.
//
// Every type on every row, not just the first on each: a scan buried under a
// paragraph is still a code somebody can walk up to.
func proofBlockTypes(rows []models.Block) map[string]bool {
	types := make(map[string]bool)
	for _, row := range rows {
		if row.Context == game.ContextObjectiveProof {
			types[row.Type] = true
		}
	}
	return types
}

// proofIcons loads the first interactive proof block for the objectives on
// offer. Only those: the frontier is a handful of rows where the quest may be
// hundreds, and nothing off it has a card to draw.
func (s *NavigationService) proofIcons(
	ctx context.Context, available []models.Objective, registry game.BlockRegistry,
) (map[string]string, map[string]bool, error) {
	if len(available) == 0 {
		// Nothing on offer asks for anything, which is an answer rather than an
		// absence: empty maps read the same to every caller.
		return map[string]string{}, map[string]bool{}, nil
	}
	ownerIDs := make([]string, 0, len(available))
	for _, objective := range available {
		ownerIDs = append(ownerIDs, objective.ID)
	}

	rows, err := s.loader.blockRepo.FindModelsByOwnerIDs(ctx, ownerIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("loading proof blocks: %w", err)
	}
	return firstInteractiveProof(rows, registry), proofBlockTypes(rows), nil
}
