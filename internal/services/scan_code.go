package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/nathanhollows/Rapua/v8/navigation"
)

// ErrNoScanMatch is returned when a scanned value proves nothing the run can
// reach.
var ErrNoScanMatch = errors.New("that code does not match anything you can reach yet")

// ScanCode proves whichever objective the scanned value belongs to, from
// anywhere in the quest. It is the only way to find a secret, which nothing
// lists, and it is what an old v7 poster's /s/ link lands on.
//
// A code already used returns its objective and writes nothing, so a poster
// scanned twice still leads somewhere.
func (s *CheckInService) ScanCode(
	ctx context.Context, team *models.Run, value string, modality blocks.ScanModality,
) (*models.Objective, error) {
	objectives, state, complete, err := s.loader.load(ctx, team)
	if err != nil {
		return nil, fmt.Errorf("loading run state: %w", err)
	}
	frontier := navigation.ComputeFrontier(objectives, state, complete)

	open := make([]models.Objective, 0, len(objectives))
	done := make([]models.Objective, 0, len(objectives))
	for _, obj := range objectives {
		switch frontier.StatusOf(obj.ID) {
		case navigation.StatusAvailable, navigation.StatusFinishable, navigation.StatusHidden:
			open = append(open, obj)
		case navigation.StatusComplete:
			done = append(done, obj)
		case navigation.StatusLocked:
		}
	}

	obj, block, err := s.matchScan(ctx, open, value, modality)
	if err != nil {
		return nil, err
	}
	if obj != nil {
		return obj, s.submitScan(ctx, team, obj, block, value, modality)
	}

	obj, _, err = s.matchScan(ctx, done, value, modality)
	if err != nil {
		return nil, err
	}
	if obj != nil {
		return obj, nil
	}
	return nil, ErrNoScanMatch
}

// matchScan returns the first objective, in tree order, with a proof scan
// accepting the value the way it was read: a block that refuses typing never
// matches a typed one.
func (s *CheckInService) matchScan(
	ctx context.Context, candidates []models.Objective, value string, modality blocks.ScanModality,
) (*models.Objective, *blocks.ScanBlock, error) {
	if len(candidates) == 0 {
		return nil, nil, nil
	}
	ids := make([]string, len(candidates))
	for i, obj := range candidates {
		ids[i] = obj.ID
	}
	rows, err := s.loader.blockRepo.FindModelsByOwnerIDs(ctx, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("loading proof blocks: %w", err)
	}

	scans := make(map[string][]*blocks.ScanBlock, len(candidates))
	for _, row := range rows {
		if row.Context != game.ContextObjectiveProof || row.Type != "scan" {
			continue
		}
		scan := &blocks.ScanBlock{}
		if err := json.Unmarshal(row.Data, scan); err != nil {
			// One unreadable block must not stop every other code in the quest.
			continue
		}
		scan.ID = row.ID
		scans[row.OwnerID] = append(scans[row.OwnerID], scan)
	}

	for i := range candidates {
		idx := slices.IndexFunc(scans[candidates[i].ID], func(b *blocks.ScanBlock) bool { return b.Accepts(value, modality) })
		if idx >= 0 {
			return &candidates[i], scans[candidates[i].ID][idx], nil
		}
	}
	return nil, nil, nil
}

// submitScan goes through the ordinary answer path, so points, events and
// completion are exactly what the objective's own page would have written.
func (s *CheckInService) submitScan(
	ctx context.Context, team *models.Run, obj *models.Objective, block *blocks.ScanBlock,
	value string, modality blocks.ScanModality,
) error {
	// The preview run has no row in runs, so a state row for it trips a foreign
	// key. Preview writes nothing anyway.
	if ctx.Value(contextkeys.PreviewKey) != nil {
		return nil
	}
	// The answer path only updates state rows, and a secret's page has never
	// been opened to create them.
	if _, _, err := s.blockService.FindByOwnerIDAndRunCodeWithStateAndContext(
		ctx, obj.ID, team.Code, team.QuestID, game.ContextObjectiveProof,
	); err != nil {
		return fmt.Errorf("preparing proof state: %w", err)
	}

	field := "code"
	if modality == blocks.ModalityCamera {
		field = "scanned"
	}
	if _, _, err := s.ValidateAndUpdateBlockState(ctx, *team, map[string][]string{
		"block": {block.ID}, field: {value},
	}); err != nil {
		return fmt.Errorf("submitting scan: %w", err)
	}
	return nil
}
