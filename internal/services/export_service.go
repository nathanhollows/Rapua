package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/nathanhollows/Rapua/v7/game"
	"github.com/nathanhollows/Rapua/v7/internal/repositories"
	"github.com/nathanhollows/Rapua/v7/models"
)

// ExportService converts a live instance into a GameDoc.
type ExportService struct {
	instanceRepo         repositories.InstanceRepository
	instanceSettingsRepo repositories.InstanceSettingsRepository
	locationRepo         repositories.LocationRepository
	blockRepo            repositories.BlockRepository
}

// NewExportService creates a new ExportService with the provided dependencies.
func NewExportService(
	instanceRepo repositories.InstanceRepository,
	instanceSettingsRepo repositories.InstanceSettingsRepository,
	locationRepo repositories.LocationRepository,
	blockRepo repositories.BlockRepository,
) *ExportService {
	return &ExportService{
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
		locationRepo:         locationRepo,
		blockRepo:            blockRepo,
	}
}

// ExportInstance serialises a live instance to a v7 GameDoc.
func (s *ExportService) ExportInstance(ctx context.Context, instanceID string) (*game.GameDoc, error) {
	// 1. Load Instance (includes GameStructure JSON column)
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("export: load instance: %w", err)
	}

	// 2. Load InstanceSettings
	settings, err := s.instanceSettingsRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("export: load settings: %w", err)
	}

	// 3. Load all Locations with Marker relation
	locations, err := s.locationRepo.FindByInstance(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("export: load locations: %w", err)
	}

	// 4. Collect all owner IDs: instance itself (start/finish) + all location IDs
	ownerIDs := make([]string, 0, len(locations)+1)
	ownerIDs = append(ownerIDs, instanceID)
	for i := range locations {
		ownerIDs = append(ownerIDs, locations[i].ID)
	}

	// 5. Load all raw blocks in one query
	rawBlocks, err := s.blockRepo.FindModelsByOwnerIDs(ctx, ownerIDs)
	if err != nil {
		return nil, fmt.Errorf("export: load blocks: %w", err)
	}

	// 6. Build maps for fast lookup
	locationByID := make(map[string]*models.Location, len(locations))
	for i := range locations {
		locationByID[locations[i].ID] = &locations[i]
	}

	// Group blocks by ownerID, preserving ordering
	blocksByOwner := make(map[string][]models.Block)
	for _, b := range rawBlocks {
		blocksByOwner[b.OwnerID] = append(blocksByOwner[b.OwnerID], b)
	}

	// 7. Build start/finish from instance-level blocks
	startDoc, finishDoc := s.buildStartFinish(blocksByOwner[instanceID])

	// 8. Walk GameStructure recursively to build the children tree
	children := s.walkStructure(instance.GameStructure, locationByID, blocksByOwner)

	// 9. Assemble and return GameDoc
	doc := &game.GameDoc{
		Rapua: "v7",
		ID:    instanceID,
		Name:  instance.Name,
		Settings: game.SettingsDoc{
			MustCheckOut:      settings.MustCheckOut,
			ShowTeamCount:     settings.ShowTeamCount,
			EnablePoints:      settings.EnablePoints,
			EnableBonusPoints: settings.EnableBonusPoints,
			ShowLeaderboard:   settings.ShowLeaderboard,
		},
		Start:  startDoc,
		Finish: finishDoc,
		Structure: game.StructureDoc{
			Routing:         instance.GameStructure.Routing,
			Completion:      instance.GameStructure.CompletionType,
			MinimumRequired: instance.GameStructure.MinimumRequired,
			Children:        children,
		},
	}

	return doc, nil
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
		case game.ContextLocationContent, game.ContextNavigation:
			// not valid for instance-level blocks; skip
		}
	}

	return start, finish
}

// walkStructure recursively converts a GameStructure node into []game.ChildDoc.
func (s *ExportService) walkStructure(
	gs models.GameStructure,
	locationByID map[string]*models.Location,
	blocksByOwner map[string][]models.Block,
) []game.ChildDoc {
	children := make([]game.ChildDoc, 0)

	for _, locID := range gs.LocationIDs {
		loc, ok := locationByID[locID]
		if !ok {
			continue
		}
		locDoc := s.buildLocationDoc(loc, blocksByOwner[locID])
		children = append(children, game.ChildDoc{Location: &locDoc})
	}

	for _, subGroup := range gs.SubGroups {
		groupChildren := s.walkStructure(subGroup, locationByID, blocksByOwner)
		groupDoc := game.GroupDoc{
			ID:              subGroup.ID,
			Name:            subGroup.Name,
			Color:           subGroup.Color,
			Routing:         subGroup.Routing,
			Completion:      subGroup.CompletionType,
			MinimumRequired: subGroup.MinimumRequired,
			When:            subGroup.When,
			Children:        groupChildren,
		}
		groupDoc.AutoAdvance = &subGroup.AutoAdvance
		children = append(children, game.ChildDoc{Group: &groupDoc})
	}

	return children
}

// buildLocationDoc converts a Location model + its blocks into a LocationDoc.
func (s *ExportService) buildLocationDoc(loc *models.Location, locBlocks []models.Block) game.LocationDoc {
	// Sort by ordering
	sort.Slice(locBlocks, func(i, j int) bool {
		return locBlocks[i].Ordering < locBlocks[j].Ordering
	})

	content := []game.BlockDoc{}
	var navigation []game.BlockDoc

	for _, b := range locBlocks {
		doc := modelBlockToDoc(b, true)
		switch b.Context {
		case game.ContextLocationContent:
			content = append(content, doc)
		case game.ContextNavigation:
			navigation = append(navigation, doc)
		case game.ContextStart, game.ContextFinish:
			// not valid for location blocks; skip
		}
	}

	locDoc := game.LocationDoc{
		ID:         loc.ID,
		Slug:       loc.Slug,
		Name:       loc.Name,
		Points:     loc.Points,
		When:       loc.When,
		Content:    content,
		Navigation: navigation,
	}

	if loc.Marker.IsMapped() {
		locDoc.Marker = &game.MarkerDoc{
			Lat: loc.Marker.Lat,
			Lng: loc.Marker.Lng,
		}
	}

	return locDoc
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
