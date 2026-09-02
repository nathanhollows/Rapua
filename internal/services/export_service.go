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
	instance, err := s.instanceRepo.GetByID(ctx, questID)
	if err != nil {
		return nil, nil, fmt.Errorf("export: load instance: %w", err)
	}

	settings, err := s.instanceSettingsRepo.GetByQuestID(ctx, questID)
	if err != nil {
		return nil, nil, fmt.Errorf("export: load settings: %w", err)
	}

	// Ordered so siblings follow their position: the document's child order is
	// the table's.
	objectives, err := s.objectiveRepo.FindTreeByQuestID(ctx, questID)
	if err != nil {
		return nil, nil, fmt.Errorf("export: load objectives: %w", err)
	}

	// The quest owns its start and finish blocks; every objective owns its own.
	ownerIDs := make([]string, 0, len(objectives)+1)
	ownerIDs = append(ownerIDs, questID)
	for i := range objectives {
		ownerIDs = append(ownerIDs, objectives[i].ID)
	}

	rawBlocks, err := s.blockRepo.FindModelsByOwnerIDs(ctx, ownerIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("export: load blocks: %w", err)
	}

	blocksByOwner := make(map[string][]models.Block)
	for _, b := range rawBlocks {
		blocksByOwner[b.OwnerID] = append(blocksByOwner[b.OwnerID], b)
	}

	startDoc, finishDoc := s.buildStartFinish(blocksByOwner[questID])

	// Every node is an objective row, so the document's recursion is the
	// table's parent_id and position.
	childrenOf := make(map[string][]models.Objective, len(objectives))
	var rootRow *models.Objective
	for i := range objectives {
		if objectives[i].ParentID == "" {
			// Refused rather than resolved. Taking one and dropping the rest
			// would export a document missing whole subtrees, and re-importing
			// it sweeps those objectives away as orphans: a backup that deletes
			// what it was taken to protect.
			if rootRow != nil {
				return nil, nil, fmt.Errorf(
					"export: %w: quest %s", repositories.ErrAmbiguousRootObjective, questID)
			}
			rootRow = &objectives[i]
			continue
		}
		childrenOf[objectives[i].ParentID] = append(childrenOf[objectives[i].ParentID], objectives[i])
	}
	if rootRow == nil {
		return nil, nil, fmt.Errorf("export: %w: quest %s", repositories.ErrNoRootObjective, questID)
	}
	root := s.buildObjectiveTree(*rootRow, childrenOf, blocksByOwner)

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
		case game.ContextObjectiveProof, game.ContextObjectiveReveal:
			// owned by an objective, not the quest: skip.
		}
	}

	return start, finish
}

// buildObjectiveTree converts one objective row and everything beneath it.
// Children arrive already ordered by position from the repository.
func (s *ExportService) buildObjectiveTree(
	row models.Objective,
	childrenOf map[string][]models.Objective,
	blocksByOwner map[string][]models.Block,
) game.ObjectiveDoc {
	doc := s.buildObjectiveDoc(&row, blocksByOwner[row.ID])
	for _, child := range childrenOf[row.ID] {
		doc.Children = append(doc.Children, s.buildObjectiveTree(child, childrenOf, blocksByOwner))
	}
	return doc
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
		ID:          obj.ID,
		Slug:        obj.Slug,
		Title:       obj.Title,
		Color:       obj.Color,
		Depends:     obj.Depends,
		Routing:     obj.Routing,
		ChildrenMin: obj.ChildrenMin,
		ChildrenMax: obj.ChildrenMax,
		MaxNext:     obj.MaxNext,
		FinishLabel: obj.FinishLabel,
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
