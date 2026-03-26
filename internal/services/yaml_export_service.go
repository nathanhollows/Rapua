package services

import (
	"context"
	"fmt"

	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/nathanhollows/Rapua/v7/repositories"
)

// YAMLExportService exports instances as QuestDef YAML definitions.
type YAMLExportService struct {
	instanceRepo         repositories.InstanceRepository
	instanceSettingsRepo repositories.InstanceSettingsRepository
	gameStructureService *GameStructureService
	blockRepo            repositories.BlockRepository
}

// NewYAMLExportService creates a new YAMLExportService.
func NewYAMLExportService(
	instanceRepo repositories.InstanceRepository,
	instanceSettingsRepo repositories.InstanceSettingsRepository,
	gameStructureService *GameStructureService,
	blockRepo repositories.BlockRepository,
) *YAMLExportService {
	return &YAMLExportService{
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
		gameStructureService: gameStructureService,
		blockRepo:            blockRepo,
	}
}

// ExportMode controls whether entity IDs are included in the export.
type ExportMode int

const (
	// ExportTemplate produces clean YAML without entity IDs.
	// Suitable for sharing, creating new instances, and version control.
	ExportTemplate ExportMode = iota

	// ExportInstance includes all entity IDs (stop IDs, block IDs).
	// Enables round-trip update: export → edit → import to update the same instance.
	ExportInstance
)

// Export exports an instance as a QuestDef.
// The mode controls whether entity IDs are included in the output.
func (s *YAMLExportService) Export(ctx context.Context, instanceID string, mode ExportMode) (*models.QuestDef, error) {
	// Load instance
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("loading instance: %w", err)
	}

	// Load settings
	settings, err := s.instanceSettingsRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("loading settings: %w", err)
	}

	// Load game structure with all locations and blocks
	gs := &instance.GameStructure
	if err := s.gameStructureService.LoadWithRelations(ctx, instanceID, gs, true); err != nil {
		return nil, fmt.Errorf("loading game structure: %w", err)
	}

	includeIDs := mode == ExportInstance

	// Build the QuestDef
	def := &models.QuestDef{
		Version: 1,
		Name:    instance.Name,
		Settings: models.SettingsDef{
			MustCheckOut:      settings.MustCheckOut,
			ShowTeamCount:     settings.ShowTeamCount,
			EnablePoints:      settings.EnablePoints,
			EnableBonusPoints: settings.EnableBonusPoints,
			ShowLeaderboard:   settings.ShowLeaderboard,
		},
	}
	if includeIDs {
		def.ID = instance.ID
	}

	// Build location slug map (locationID → slug) for stage references
	slugMap := make(map[string]string)
	locationMap := make(map[string]*models.Location)
	collectLocations(gs, slugMap, locationMap)

	// Build stages from game structure (including unassigned stops at root level)
	def.Structure.Stages = buildStageDefs(gs, slugMap)

	// Build stop defs from all locations
	stops, err := buildStopDefs(gs, locationMap, includeIDs)
	if err != nil {
		return nil, fmt.Errorf("building stop defs: %w", err)
	}
	def.Stops = stops

	// Build start/finish blocks (owned by instance ID)
	startBlocks, err := s.blockRepo.FindByOwnerIDAndContext(ctx, instanceID, blocks.ContextStart)
	if err != nil {
		return nil, fmt.Errorf("loading start blocks: %w", err)
	}
	def.Start = exportBlockDefs(startBlocks, includeIDs)

	finishBlocks, err := s.blockRepo.FindByOwnerIDAndContext(ctx, instanceID, blocks.ContextFinish)
	if err != nil {
		return nil, fmt.Errorf("loading finish blocks: %w", err)
	}
	def.Finish = exportBlockDefs(finishBlocks, includeIDs)

	return def, nil
}

// collectLocations recursively collects all locations from the game structure.
func collectLocations(gs *models.GameStructure, slugMap map[string]string, locationMap map[string]*models.Location) {
	for _, loc := range gs.Locations {
		slugMap[loc.ID] = loc.Slug
		locationMap[loc.ID] = loc
	}
	for i := range gs.SubGroups {
		collectLocations(&gs.SubGroups[i], slugMap, locationMap)
	}
}

// buildStageDefs converts GameStructure SubGroups into StageDefs.
// The root group itself is not a stage — only its SubGroups become stages.
// Root-level LocationIDs become an "Unassigned" stage so they survive round-trip.
func buildStageDefs(gs *models.GameStructure, slugMap map[string]string) []models.StageDef {
	var stages []models.StageDef

	// Root-level locations → "Unassigned" stage
	if len(gs.LocationIDs) > 0 {
		unassigned := models.StageDef{
			Name:    "Unassigned",
			Routing: string(gs.Routing),
		}
		unassigned.Stops = make([]string, 0, len(gs.LocationIDs))
		for _, locID := range gs.LocationIDs {
			if slug, ok := slugMap[locID]; ok {
				unassigned.Stops = append(unassigned.Stops, slug)
			}
		}
		stages = append(stages, unassigned)
	}

	for i := range gs.SubGroups {
		stages = append(stages, gameStructureToStageDef(&gs.SubGroups[i], slugMap))
	}

	if len(stages) == 0 {
		return nil
	}
	return stages
}

// gameStructureToStageDef converts a single GameStructure (non-root) to a StageDef.
func gameStructureToStageDef(gs *models.GameStructure, slugMap map[string]string) models.StageDef {
	stage := models.StageDef{
		Name:            gs.Name,
		Color:           gs.Color,
		Routing:         string(gs.Routing),
		Navigation:      string(gs.Navigation),
		Completion:      string(gs.CompletionType),
		MinimumRequired: gs.MinimumRequired,
		MaxNext:         gs.MaxNext,
		AutoAdvance:     gs.AutoAdvance,
	}

	// Convert location IDs to slugs
	if len(gs.LocationIDs) > 0 {
		stage.Stops = make([]string, 0, len(gs.LocationIDs))
		for _, locID := range gs.LocationIDs {
			if slug, ok := slugMap[locID]; ok {
				stage.Stops = append(stage.Stops, slug)
			}
		}
	}

	// Recurse into nested stages
	if len(gs.SubGroups) > 0 {
		stage.Stages = make([]models.StageDef, 0, len(gs.SubGroups))
		for i := range gs.SubGroups {
			stage.Stages = append(stage.Stages, gameStructureToStageDef(&gs.SubGroups[i], slugMap))
		}
	}

	return stage
}

// buildStopDefs builds StopDef entries from all locations in the game structure.
// Locations are collected in structure order (depth-first).
func buildStopDefs(
	gs *models.GameStructure,
	locationMap map[string]*models.Location,
	includeIDs bool,
) ([]models.StopDef, error) {
	var locationIDs []string
	collectLocationIDsOrdered(gs, &locationIDs)

	stops := make([]models.StopDef, 0, len(locationIDs))
	for _, locID := range locationIDs {
		loc, ok := locationMap[locID]
		if !ok {
			continue
		}
		stop, err := locationToStopDef(loc, includeIDs)
		if err != nil {
			return nil, err
		}
		stops = append(stops, stop)
	}
	return stops, nil
}

// collectLocationIDsOrdered collects location IDs in structure order (depth-first).
func collectLocationIDsOrdered(gs *models.GameStructure, ids *[]string) {
	*ids = append(*ids, gs.LocationIDs...)
	for i := range gs.SubGroups {
		collectLocationIDsOrdered(&gs.SubGroups[i], ids)
	}
}

// locationToStopDef converts a Location (with loaded blocks) to a StopDef.
func locationToStopDef(loc *models.Location, includeIDs bool) (models.StopDef, error) {
	stop := models.StopDef{
		Slug:   loc.Slug,
		Name:   loc.Name,
		Points: loc.Points,
	}
	if includeIDs {
		stop.ID = loc.ID
	}

	// Add marker if mapped
	if loc.Marker.IsMapped() {
		stop.Marker = &models.MarkerDef{
			Lat: loc.Marker.Lat,
			Lng: loc.Marker.Lng,
		}
	}

	// Split blocks by context
	var content, clues, tasks, checkpoint []models.Block
	for i := range loc.Blocks {
		switch loc.Blocks[i].Context {
		case blocks.ContextLocationContent:
			content = append(content, loc.Blocks[i])
		case blocks.ContextLocationClues:
			clues = append(clues, loc.Blocks[i])
		case blocks.ContextTask:
			tasks = append(tasks, loc.Blocks[i])
		case blocks.ContextCheckpoint:
			checkpoint = append(checkpoint, loc.Blocks[i])
		case blocks.ContextStart, blocks.ContextFinish:
			// Start/Finish blocks belong to the instance, not locations; skip here.
		}
	}

	contentBlocks, err := modelsToBlocks(content)
	if err != nil {
		return stop, fmt.Errorf("stop %s content: %w", loc.Slug, err)
	}
	clueBlocks, err := modelsToBlocks(clues)
	if err != nil {
		return stop, fmt.Errorf("stop %s clues: %w", loc.Slug, err)
	}
	taskBlocks, err := modelsToBlocks(tasks)
	if err != nil {
		return stop, fmt.Errorf("stop %s tasks: %w", loc.Slug, err)
	}
	checkpointBlocks, err := modelsToBlocks(checkpoint)
	if err != nil {
		return stop, fmt.Errorf("stop %s checkpoint: %w", loc.Slug, err)
	}

	stop.Content = exportBlockDefs(contentBlocks, includeIDs)
	stop.Clues = exportBlockDefs(clueBlocks, includeIDs)
	stop.Tasks = exportBlockDefs(taskBlocks, includeIDs)
	stop.Checkpoint = exportBlockDefs(checkpointBlocks, includeIDs)

	return stop, nil
}

// modelsToBlocks converts model Block records to blocks.Block instances via ParseData.
func modelsToBlocks(modelBlocks []models.Block) (blocks.Blocks, error) {
	if len(modelBlocks) == 0 {
		return nil, nil
	}

	result := make(blocks.Blocks, 0, len(modelBlocks))
	for _, mb := range modelBlocks {
		base := blocks.BaseBlock{
			ID:      mb.ID,
			OwnerID: mb.OwnerID,
			Type:    mb.Type,
			Order:   mb.Ordering,
			Points:  mb.Points,
			Data:    mb.Data,
		}
		b, err := blocks.CreateFromBaseBlock(base)
		if err != nil {
			return nil, fmt.Errorf("block %s (type %s): %w", mb.ID, mb.Type, err)
		}
		if err := b.ParseData(); err != nil {
			return nil, fmt.Errorf("block %s (type %s) data: %w", mb.ID, mb.Type, err)
		}
		result = append(result, b)
	}
	return result, nil
}

// exportBlockDefs converts parsed Block instances to BlockDef entries for YAML export.
func exportBlockDefs(blks blocks.Blocks, includeIDs bool) []models.BlockDef {
	if len(blks) == 0 {
		return nil
	}

	defs := make([]models.BlockDef, 0, len(blks))
	for _, b := range blks {
		def := models.BlockDef{
			Type:   b.GetType(),
			Points: b.GetPoints(),
		}
		if includeIDs {
			def.ID = b.GetID()
		}

		// Export block-specific fields via ToYAML
		if exporter, ok := b.(blocks.YAMLExporter); ok {
			def.Fields = exporter.ToYAML()
		}

		defs = append(defs, def)
	}
	return defs
}
