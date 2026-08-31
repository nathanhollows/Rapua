package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
)

// ExportService converts a live instance into a GameDoc.
type ExportService struct {
	instanceRepo         repositories.QuestRepository
	instanceSettingsRepo repositories.QuestSettingsRepository
	objectiveRepo        repositories.ObjectiveRepository
	blockRepo            repositories.BlockRepository
}

// NewExportService creates a new ExportService with the provided dependencies.
func NewExportService(
	instanceRepo repositories.QuestRepository,
	instanceSettingsRepo repositories.QuestSettingsRepository,
	objectiveRepo repositories.ObjectiveRepository,
	blockRepo repositories.BlockRepository,
) *ExportService {
	return &ExportService{
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
		objectiveRepo:        objectiveRepo,
		blockRepo:            blockRepo,
	}
}

func (s *ExportService) ExportInstance(ctx context.Context, questID string) (*game.GameDoc, []string, error) {
	// 1. Load Instance (includes GameStructure JSON column)
	instance, err := s.instanceRepo.GetByID(ctx, questID)
	if err != nil {
		return nil, nil, fmt.Errorf("export: load instance: %w", err)
	}

	// 2. Load QuestSettings.
	settings, err := s.instanceSettingsRepo.GetByQuestID(ctx, questID)
	if err != nil {
		return nil, nil, fmt.Errorf("export: load settings: %w", err)
	}

	// 3. Load all Objectives.
	objectives, err := s.objectiveRepo.FindByQuestID(ctx, questID)
	if err != nil {
		return nil, nil, fmt.Errorf("export: load objectives: %w", err)
	}

	// 4. Collect all owner IDs: instance itself (start/finish) + all objective IDs.
	ownerIDs := make([]string, 0, len(objectives)+1)
	ownerIDs = append(ownerIDs, questID)
	for i := range objectives {
		ownerIDs = append(ownerIDs, objectives[i].ID)
	}

	// 5. Load all raw blocks in one query
	rawBlocks, err := s.blockRepo.FindModelsByOwnerIDs(ctx, ownerIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("export: load blocks: %w", err)
	}

	// 6. Build map for fast lookup.
	objectiveByID := make(map[string]*models.Objective, len(objectives))
	for i := range objectives {
		objectiveByID[objectives[i].ID] = &objectives[i]
	}

	// Group blocks by ownerID, preserving ordering
	blocksByOwner := make(map[string][]models.Block)
	for _, b := range rawBlocks {
		blocksByOwner[b.OwnerID] = append(blocksByOwner[b.OwnerID], b)
	}

	// 7. Build start/finish from instance-level blocks
	startDoc, finishDoc := s.buildStartFinish(blocksByOwner[questID])

	// 8. Walk GameStructure recursively to build the children tree
	// Slugs are unique document-wide, and objective rows already hold theirs,
	// so they are claimed before any group slug is minted from a name.
	takenSlugs := make(map[string]bool, len(objectiveByID))
	for _, obj := range objectiveByID {
		if obj.Slug != "" {
			takenSlugs[obj.Slug] = true
		}
	}
	root := s.walkStructure(instance.GameStructure, objectiveByID, blocksByOwner, takenSlugs)
	root.Slug = uniqueSlug("root", takenSlugs)
	root.Title = instance.Name

	// 9. Assemble and return GameDoc
	doc := &game.GameDoc{
		Rapua: "v8",
		ID:    questID,
		Name:  instance.Name,
		Settings: game.SettingsDoc{
			ShowTeamCount:   settings.ShowTeamCount,
			EnablePoints:    settings.EnablePoints,
			ShowLeaderboard: settings.ShowLeaderboard,
		},
		Start:     startDoc,
		Finish:    finishDoc,
		Structure: root,
	}

	return doc, nil, nil
}

// buildStartFinish splits instance-level blocks into start and finish arrays.
// Always returns non-nil slices so "start" and "finish" are present in the output.
func (s *ExportService) buildStartFinish(instanceBlocks []models.Block) ([]game.BlockDoc, []game.BlockDoc) {
	start := []game.BlockDoc{}
	finish := []game.BlockDoc{}

	// Sort by ordering to preserve intended sequence
	sort.Slice(instanceBlocks, func(i, j int) bool {
		return instanceBlocks[i].Ordering < instanceBlocks[j].Ordering
	})

	for _, b := range instanceBlocks {
		doc := modelBlockToDoc(b, true)
		switch b.Context {
		case game.ContextStart:
			start = append(start, doc)
		case game.ContextFinish:
			finish = append(finish, doc)
		}
	}

	return start, finish
}

// walkStructure converts a stored group node into the objective that now
// represents it. Storage keeps objectives and subgroups in separate arrays, so
// the single ordered children list puts objectives first, matching the order
// the blob itself documents.
func (s *ExportService) walkStructure(
	gs models.GameStructure,
	objectiveByID map[string]*models.Objective,
	blocksByOwner map[string][]models.Block,
	takenSlugs map[string]bool,
) game.ObjectiveDoc {
	children := make([]game.ObjectiveDoc, 0, len(gs.ObjectiveIDs)+len(gs.SubGroups))

	for _, objID := range gs.ObjectiveIDs {
		obj, ok := objectiveByID[objID]
		if !ok {
			continue
		}
		children = append(children, s.buildObjectiveDoc(obj, blocksByOwner[objID]))
	}

	for _, subGroup := range gs.SubGroups {
		child := s.walkStructure(subGroup, objectiveByID, blocksByOwner, takenSlugs)
		child.Slug = uniqueSlug(sectionSlug(subGroup), takenSlugs)
		child.Title = sectionTitle(subGroup)
		children = append(children, child)
	}

	minChildren, maxChildren := bandFromGroup(gs)
	return game.ObjectiveDoc{
		ID:          gs.ID,
		Color:       gs.Color,
		Depends:     gs.Depends,
		Routing:     gs.Routing,
		ChildrenMin: minChildren,
		ChildrenMax: maxChildren,
		MaxNext:     gs.MaxNext,
		FinishLabel: gs.FinishLabel,
		Children:    children,
	}
}

func (s *ExportService) buildObjectiveDoc(obj *models.Objective, objBlocks []models.Block) game.ObjectiveDoc {
	sort.Slice(objBlocks, func(i, j int) bool {
		return objBlocks[i].Ordering < objBlocks[j].Ordering
	})

	proof := []game.BlockDoc{}
	reveal := []game.BlockDoc{}

	for _, b := range objBlocks {
		doc := modelBlockToDoc(b, true)
		switch b.Context {
		case game.ContextObjectiveProof:
			proof = append(proof, doc)
		case game.ContextObjectiveReveal:
			reveal = append(reveal, doc)
		case game.ContextStart, game.ContextFinish:
			// not valid for objective blocks: skip.
		}
	}

	return game.ObjectiveDoc{
		ID:      obj.ID,
		Slug:    obj.Slug,
		Title:   obj.Title,
		Depends: obj.Depends,
		Proof: game.ObjectiveContextDoc{
			Blocks: proof,
			Sets:   obj.ProofSets,
		},
		Reveal: game.ObjectiveContextDoc{
			Blocks: reveal,
			Sets:   obj.RevealSets,
		},
	}
}

// modelBlockToDoc converts a models.Block to a game.BlockDoc.
// If includeID is true, the block's ID is included (for round-trip exports).
func modelBlockToDoc(b models.Block, includeID bool) game.BlockDoc {
	doc := make(game.BlockDoc)

	// Unmarshal block-specific data fields
	var fields map[string]any
	if len(b.Data) > 0 {
		_ = json.Unmarshal(b.Data, &fields)
		for k, v := range fields {
			doc[k] = v
		}
	}

	// Inject promoted fields
	doc["type"] = b.Type
	if b.Points != 0 {
		doc["points"] = b.Points
	}
	if includeID && b.ID != "" {
		doc["id"] = b.ID
	}

	return doc
}
