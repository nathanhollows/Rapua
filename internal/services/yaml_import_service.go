package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v7/blocks"
	"github.com/nathanhollows/Rapua/v7/internal/db"
	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/nathanhollows/Rapua/v7/repositories"
	"github.com/uptrace/bun"
)

// ImportWarning represents a non-fatal warning during import.
type ImportWarning struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// YAMLImportService imports QuestDef YAML definitions into instances.
type YAMLImportService struct {
	transactor           db.Transactor
	instanceRepo         repositories.InstanceRepository
	instanceSettingsRepo repositories.InstanceSettingsRepository
	locationRepo         repositories.LocationRepository
	markerRepo           repositories.MarkerRepository
	blockRepo            repositories.BlockRepository
	teamRepo             repositories.TeamRepository
}

// NewYAMLImportService creates a new YAMLImportService.
func NewYAMLImportService(
	transactor db.Transactor,
	instanceRepo repositories.InstanceRepository,
	instanceSettingsRepo repositories.InstanceSettingsRepository,
	locationRepo repositories.LocationRepository,
	markerRepo repositories.MarkerRepository,
	blockRepo repositories.BlockRepository,
	teamRepo repositories.TeamRepository,
) *YAMLImportService {
	return &YAMLImportService{
		transactor:           transactor,
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
		locationRepo:         locationRepo,
		markerRepo:           markerRepo,
		blockRepo:            blockRepo,
		teamRepo:             teamRepo,
	}
}

// validationErrors joins all validation errors into a single error.
func validationErrors(result ValidationResult) error {
	msgs := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		msgs[i] = e.Error()
	}
	return fmt.Errorf("validation failed: %s", strings.Join(msgs, "; "))
}

// ImportCreate creates a new instance from a QuestDef.
func (s *YAMLImportService) ImportCreate(
	ctx context.Context,
	userID string,
	def *models.QuestDef,
	isTemplate bool,
) (*models.Instance, []ImportWarning, error) {
	// Validate first
	result := ValidateQuestDef(def)
	if !result.Valid {
		return nil, nil, validationErrors(result)
	}

	var warnings []ImportWarning
	for _, w := range result.Warnings {
		warnings = append(warnings, ImportWarning(w))
	}

	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Create instance
	instance := &models.Instance{
		Name:       def.Name,
		UserID:     userID,
		IsTemplate: isTemplate,
	}

	if err := s.instanceRepo.CreateTx(ctx, tx, instance); err != nil {
		return nil, nil, fmt.Errorf("creating instance: %w", err)
	}

	// Create settings
	settings := &models.InstanceSettings{
		InstanceID:        instance.ID,
		MustCheckOut:      def.Settings.MustCheckOut,
		ShowTeamCount:     def.Settings.ShowTeamCount,
		EnablePoints:      def.Settings.EnablePoints,
		EnableBonusPoints: def.Settings.EnableBonusPoints,
		ShowLeaderboard:   def.Settings.ShowLeaderboard,
	}
	if err := s.instanceSettingsRepo.CreateTx(ctx, tx, settings); err != nil {
		return nil, nil, fmt.Errorf("creating settings: %w", err)
	}

	// Create stops (locations + markers + blocks)
	slugToLocationID := make(map[string]string)
	for _, stop := range def.Stops {
		locID, err := s.createStop(ctx, tx, instance.ID, stop)
		if err != nil {
			return nil, nil, fmt.Errorf("creating stop %q: %w", stop.Slug, err)
		}
		slugToLocationID[stop.Slug] = locID
	}

	// Build game structure from stages
	gs := buildGameStructure(def.Structure.Stages, slugToLocationID)
	instance.GameStructure = gs

	// Update game structure within the transaction
	if _, err := tx.NewUpdate().Model(instance).Column("game_structure").WherePK().Exec(ctx); err != nil {
		return nil, nil, fmt.Errorf("saving game structure: %w", err)
	}

	// Create start blocks
	if err := s.createBlocksForOwner(ctx, tx, instance.ID, blocks.ContextStart, def.Start); err != nil {
		return nil, nil, fmt.Errorf("creating start blocks: %w", err)
	}

	// Create finish blocks
	if err := s.createBlocksForOwner(ctx, tx, instance.ID, blocks.ContextFinish, def.Finish); err != nil {
		return nil, nil, fmt.Errorf("creating finish blocks: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("committing transaction: %w", err)
	}

	return instance, warnings, nil
}

// ImportUpdate updates an existing instance from a QuestDef.
func (s *YAMLImportService) ImportUpdate( //nolint:gocognit,gocyclo
	ctx context.Context,
	instanceID string,
	def *models.QuestDef,
) (*models.Instance, []ImportWarning, error) {
	// Instance ID must match for round-trip update
	if def.ID == "" {
		return nil, nil, errors.New("YAML has no instance ID: use instance export for round-trip updates")
	}
	if def.ID != instanceID {
		return nil, nil, fmt.Errorf("YAML instance ID %q does not match current instance %q", def.ID, instanceID)
	}

	// Validate first
	result := ValidateQuestDef(def)
	if !result.Valid {
		return nil, nil, validationErrors(result)
	}

	var warnings []ImportWarning
	for _, w := range result.Warnings {
		warnings = append(warnings, ImportWarning(w))
	}

	// Load existing instance
	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, nil, fmt.Errorf("loading instance: %w", err)
	}

	// Load existing locations for matching
	existingLocations, err := s.locationRepo.FindByInstance(ctx, instanceID)
	if err != nil {
		return nil, nil, fmt.Errorf("loading existing locations: %w", err)
	}

	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("beginning transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Update instance name
	instance.Name = def.Name
	if _, err := tx.NewUpdate().Model(instance).Column("name").WherePK().Exec(ctx); err != nil {
		return nil, nil, fmt.Errorf("updating instance: %w", err)
	}

	// Update settings — load existing first to preserve created_at and other fields
	settings, err := s.instanceSettingsRepo.GetByInstanceID(ctx, instanceID)
	if err != nil {
		return nil, nil, fmt.Errorf("loading settings: %w", err)
	}
	settings.MustCheckOut = def.Settings.MustCheckOut
	settings.ShowTeamCount = def.Settings.ShowTeamCount
	settings.EnablePoints = def.Settings.EnablePoints
	settings.EnableBonusPoints = def.Settings.EnableBonusPoints
	settings.ShowLeaderboard = def.Settings.ShowLeaderboard
	if err := s.instanceSettingsRepo.UpdateTx(ctx, tx, settings); err != nil {
		return nil, nil, fmt.Errorf("updating settings: %w", err)
	}

	// Build lookup maps for existing locations: by ID and by slug
	existingByID := make(map[string]*models.Location, len(existingLocations))
	existingBySlug := make(map[string]*models.Location, len(existingLocations))
	for i := range existingLocations {
		existingByID[existingLocations[i].ID] = &existingLocations[i]
		existingBySlug[existingLocations[i].Slug] = &existingLocations[i]
	}

	// Process stops: match by ID first, then by slug
	slugToLocationID := make(map[string]string)
	matchedLocationIDs := make(map[string]bool)

	for _, stop := range def.Stops {
		var matched *models.Location

		// Try matching by ID first
		if stop.ID != "" {
			if loc, ok := existingByID[stop.ID]; ok {
				matched = loc
			}
		}

		// Fall back to matching by slug
		if matched == nil {
			if loc, ok := existingBySlug[stop.Slug]; ok {
				matched = loc
			}
		}

		if matched != nil { //nolint:nestif // update-or-create logic requires checking match then block-level operations
			// Update existing location
			matchedLocationIDs[matched.ID] = true
			matched.Name = stop.Name
			matched.Slug = stop.Slug
			matched.Points = stop.Points

			if err := s.locationRepo.UpdateTx(ctx, tx, matched); err != nil {
				return nil, nil, fmt.Errorf("updating stop %q: %w", stop.Slug, err)
			}

			// Update marker coordinates if provided
			if stop.Marker != nil {
				if err := s.markerRepo.UpdateCoordsTx(ctx, tx, &matched.Marker, stop.Marker.Lat, stop.Marker.Lng); err != nil {
					return nil, nil, fmt.Errorf("updating marker for stop %q: %w", stop.Slug, err)
				}
			}

			// Delete existing blocks and recreate from YAML,
			// preserving player states for blocks with matching IDs
			preserveIDs := collectBlockIDs(stop)
			savedStates, err := s.blockRepo.DeleteByOwnerIDPreservingStates(ctx, tx, matched.ID, preserveIDs)
			if err != nil {
				return nil, nil, fmt.Errorf("deleting blocks for stop %q: %w", stop.Slug, err)
			}
			if err := s.createBlocksForStop(ctx, tx, matched.ID, stop); err != nil {
				return nil, nil, fmt.Errorf("creating blocks for stop %q: %w", stop.Slug, err)
			}
			if len(savedStates) > 0 {
				if _, err := tx.NewInsert().Model(&savedStates).Exec(ctx); err != nil {
					return nil, nil, fmt.Errorf("restoring player states for stop %q: %w", stop.Slug, err)
				}
			}

			slugToLocationID[stop.Slug] = matched.ID
		} else {
			// Create new location
			locID, err := s.createStop(ctx, tx, instanceID, stop)
			if err != nil {
				return nil, nil, fmt.Errorf("creating stop %q: %w", stop.Slug, err)
			}
			slugToLocationID[stop.Slug] = locID
		}
	}

	// Unmatched existing locations → move to root level with warning
	for _, loc := range existingLocations {
		if !matchedLocationIDs[loc.ID] {
			slugToLocationID[loc.Slug] = loc.ID
			warnings = append(warnings, ImportWarning{
				Field:   "stops",
				Message: fmt.Sprintf("existing stop %q not in YAML, moved to root level", loc.Slug),
			})
		}
	}

	// Delete and recreate start/finish blocks, preserving player states for matching IDs
	instanceBlockIDs := collectBlockDefIDs(def.Start)
	instanceBlockIDs = append(instanceBlockIDs, collectBlockDefIDs(def.Finish)...)
	savedInstanceStates, err := s.blockRepo.DeleteByOwnerIDPreservingStates(ctx, tx, instanceID, instanceBlockIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("deleting instance blocks: %w", err)
	}
	if err := s.createBlocksForOwner(ctx, tx, instanceID, blocks.ContextStart, def.Start); err != nil {
		return nil, nil, fmt.Errorf("creating start blocks: %w", err)
	}
	if err := s.createBlocksForOwner(ctx, tx, instanceID, blocks.ContextFinish, def.Finish); err != nil {
		return nil, nil, fmt.Errorf("creating finish blocks: %w", err)
	}
	if len(savedInstanceStates) > 0 {
		if _, err := tx.NewInsert().Model(&savedInstanceStates).Exec(ctx); err != nil {
			return nil, nil, fmt.Errorf("restoring player states for instance blocks: %w", err)
		}
	}

	// Rebuild game structure
	gs := buildGameStructure(def.Structure.Stages, slugToLocationID)
	instance.GameStructure = gs

	if _, err := tx.NewUpdate().Model(instance).Column("game_structure").WherePK().Exec(ctx); err != nil {
		return nil, nil, fmt.Errorf("saving game structure: %w", err)
	}

	// Check for active teams
	teamCount, err := s.teamRepo.CountByInstance(ctx, instanceID)
	if err == nil && teamCount > 0 {
		warnings = append(warnings, ImportWarning{
			Field: "instance",
			Message: fmt.Sprintf(
				"instance has %d active teams; player state may be affected by structural changes",
				teamCount,
			),
		})
	}

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("committing transaction: %w", err)
	}

	return instance, warnings, nil
}

// createBlocksForStop creates all blocks for a stop's four contexts.
func (s *YAMLImportService) createBlocksForStop(
	ctx context.Context,
	tx *bun.Tx,
	ownerID string,
	stop models.StopDef,
) error {
	if err := s.createBlocksForOwner(ctx, tx, ownerID, blocks.ContextLocationContent, stop.Content); err != nil {
		return fmt.Errorf("content blocks: %w", err)
	}
	if err := s.createBlocksForOwner(ctx, tx, ownerID, blocks.ContextLocationClues, stop.Clues); err != nil {
		return fmt.Errorf("clue blocks: %w", err)
	}
	if err := s.createBlocksForOwner(ctx, tx, ownerID, blocks.ContextTask, stop.Tasks); err != nil {
		return fmt.Errorf("task blocks: %w", err)
	}
	if err := s.createBlocksForOwner(ctx, tx, ownerID, blocks.ContextCheckpoint, stop.Checkpoint); err != nil {
		return fmt.Errorf("checkpoint blocks: %w", err)
	}
	return nil
}

// createStop creates a location with its marker and blocks from a StopDef.
func (s *YAMLImportService) createStop(
	ctx context.Context,
	tx *bun.Tx,
	instanceID string,
	stop models.StopDef,
) (string, error) {
	// Create marker
	var lat, lng float64
	if stop.Marker != nil {
		lat = stop.Marker.Lat
		lng = stop.Marker.Lng
	}

	marker := models.Marker{
		Name: stop.Name,
		Lat:  lat,
		Lng:  lng,
	}
	if err := s.markerRepo.CreateTx(ctx, tx, &marker); err != nil {
		return "", fmt.Errorf("creating marker: %w", err)
	}

	// Create location within transaction
	location := &models.Location{
		Name:       stop.Name,
		Slug:       stop.Slug,
		InstanceID: instanceID,
		MarkerID:   marker.Code,
		Points:     stop.Points,
	}
	if err := s.locationRepo.CreateTx(ctx, tx, location); err != nil {
		return "", fmt.Errorf("creating location: %w", err)
	}

	// Create blocks for each context
	if err := s.createBlocksForStop(ctx, tx, location.ID, stop); err != nil {
		return "", err
	}

	return location.ID, nil
}

// createBlocksForOwner creates blocks from BlockDefs for a given owner and context.
func (s *YAMLImportService) createBlocksForOwner(
	ctx context.Context,
	tx *bun.Tx,
	ownerID string,
	blockContext blocks.BlockContext,
	defs []models.BlockDef,
) error {
	if len(defs) == 0 {
		return nil
	}

	for i, bd := range defs {
		// Convert YAML fields to JSON data
		jsonData, err := blocks.FromYAML(bd.Type, bd.Fields)
		if err != nil {
			return fmt.Errorf("block %d (%s): %w", i, bd.Type, err)
		}

		// Create a base block and hydrate it
		base := blocks.BaseBlock{
			ID:      bd.ID,
			Type:    bd.Type,
			OwnerID: ownerID,
			Order:   i,
			Points:  bd.Points,
			Data:    json.RawMessage(jsonData),
		}

		b, err := blocks.CreateFromBaseBlock(base)
		if err != nil {
			return fmt.Errorf("block %d (%s): %w", i, bd.Type, err)
		}

		if err := b.ParseData(); err != nil {
			return fmt.Errorf("block %d (%s) parse: %w", i, bd.Type, err)
		}

		if _, err := s.blockRepo.CreateTx(ctx, tx, b, ownerID, blockContext); err != nil {
			return fmt.Errorf("block %d (%s): %w", i, bd.Type, err)
		}
	}

	return nil
}

// buildGameStructure converts StageDefs into a root GameStructure.
// Stages named "Unassigned" have their stops placed at root level instead of in a subgroup.
func buildGameStructure(stages []models.StageDef, slugToLocationID map[string]string) models.GameStructure {
	root := models.GameStructure{
		ID:             uuid.New().String(),
		Name:           "",
		Color:          "",
		Routing:        models.RouteStrategyFreeRoam,
		Navigation:     models.NavigationMap,
		CompletionType: models.CompletionAll,
		IsRoot:         true,
		LocationIDs:    []string{},
		SubGroups:      make([]models.GameStructure, 0),
	}

	// Find stops not assigned to any stage — put them at root level
	assignedSlugs := make(map[string]bool)
	collectAssignedSlugs(stages, assignedSlugs)

	// Collect unassigned slugs and sort for deterministic ordering
	var unassignedSlugs []string
	for slug := range slugToLocationID {
		if !assignedSlugs[slug] {
			unassignedSlugs = append(unassignedSlugs, slug)
		}
	}
	sort.Strings(unassignedSlugs)
	for _, slug := range unassignedSlugs {
		root.LocationIDs = append(root.LocationIDs, slugToLocationID[slug])
	}

	// Build stage subgroups
	for _, stage := range stages {
		if stage.Name == "Unassigned" {
			// "Unassigned" stage → put stops at root level, not as a subgroup
			for _, slug := range stage.Stops {
				if locID, ok := slugToLocationID[slug]; ok {
					root.LocationIDs = append(root.LocationIDs, locID)
				}
			}
			continue
		}
		root.SubGroups = append(root.SubGroups, stageDefToGameStructure(stage, slugToLocationID))
	}

	return root
}

// collectAssignedSlugs collects all slugs referenced in stages recursively.
func collectAssignedSlugs(stages []models.StageDef, assigned map[string]bool) {
	for _, stage := range stages {
		for _, slug := range stage.Stops {
			assigned[slug] = true
		}
		collectAssignedSlugs(stage.Stages, assigned)
	}
}

// stageDefToGameStructure converts a StageDef to a GameStructure.
func stageDefToGameStructure(stage models.StageDef, slugToLocationID map[string]string) models.GameStructure {
	gs := models.GameStructure{
		ID:              uuid.New().String(),
		Name:            stage.Name,
		Color:           stage.Color,
		Routing:         models.RouteStrategy(stage.Routing),
		Navigation:      models.NavigationMode(stage.Navigation),
		CompletionType:  models.CompletionType(stage.Completion),
		MinimumRequired: stage.MinimumRequired,
		MaxNext:         stage.MaxNext,
		AutoAdvance:     stage.AutoAdvance,
		IsRoot:          false,
		LocationIDs:     make([]string, 0),
		SubGroups:       make([]models.GameStructure, 0),
	}

	// Set defaults for empty enum values
	if gs.Routing == "" {
		gs.Routing = models.RouteStrategyFreeRoam
	}
	if gs.Navigation == "" {
		gs.Navigation = models.NavigationMap
	}
	if gs.CompletionType == "" {
		gs.CompletionType = models.CompletionAll
	}
	if gs.Color == "" {
		gs.Color = "primary"
	}

	// Map slugs to location IDs
	for _, slug := range stage.Stops {
		if locID, ok := slugToLocationID[slug]; ok {
			gs.LocationIDs = append(gs.LocationIDs, locID)
		}
	}

	// Recurse into nested stages
	for _, subStage := range stage.Stages {
		gs.SubGroups = append(gs.SubGroups, stageDefToGameStructure(subStage, slugToLocationID))
	}

	return gs
}

// collectBlockIDs returns all non-empty block IDs from a stop's block defs.
func collectBlockIDs(stop models.StopDef) []string {
	var ids []string
	ids = append(ids, collectBlockDefIDs(stop.Content)...)
	ids = append(ids, collectBlockDefIDs(stop.Clues)...)
	ids = append(ids, collectBlockDefIDs(stop.Tasks)...)
	ids = append(ids, collectBlockDefIDs(stop.Checkpoint)...)
	return ids
}

// collectBlockDefIDs returns all non-empty IDs from a slice of BlockDefs.
func collectBlockDefIDs(defs []models.BlockDef) []string {
	var ids []string
	for _, bd := range defs {
		if bd.ID != "" {
			ids = append(ids, bd.ID)
		}
	}
	return ids
}
