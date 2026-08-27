package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v8/blocks"
	"github.com/nathanhollows/Rapua/v8/game"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
)

// ImportResult summarises what happened during an import.
type ImportResult struct {
	QuestID  string
	Created  ImportStats
	Updated  ImportStats
	Deleted  ImportStats
	Warnings []string
}

// ImportStats counts entities affected by an import operation.
type ImportStats struct {
	Locations  int
	Objectives int
	Blocks     int
	Groups     int
}

// ImportService creates or updates instances from a GameDoc.
type ImportService struct {
	logger               *slog.Logger
	transactor           db.Transactor
	instanceRepo         repositories.QuestRepository
	instanceSettingsRepo repositories.QuestSettingsRepository
	locationRepo         repositories.LocationRepository
	objectiveRepo        repositories.ObjectiveRepository
	blockRepo            repositories.BlockRepository
	markerRepo           repositories.MarkerRepository
}

// NewImportService creates a new ImportService with the provided dependencies.
func NewImportService(
	logger *slog.Logger,
	transactor db.Transactor,
	instanceRepo repositories.QuestRepository,
	instanceSettingsRepo repositories.QuestSettingsRepository,
	locationRepo repositories.LocationRepository,
	objectiveRepo repositories.ObjectiveRepository,
	blockRepo repositories.BlockRepository,
	markerRepo repositories.MarkerRepository,
) *ImportService {
	return &ImportService{
		logger:               logger,
		transactor:           transactor,
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
		locationRepo:         locationRepo,
		objectiveRepo:        objectiveRepo,
		blockRepo:            blockRepo,
		markerRepo:           markerRepo,
	}
}

// ImportCreate parses a GameDoc and creates a brand-new instance.
// The doc ID field is ignored; a fresh UUID is always assigned.
func (s *ImportService) ImportCreate(ctx context.Context, userID string, doc *game.GameDoc) (*ImportResult, error) {
	// Lint the document
	result := game.Lint(doc, blocks.Registry())
	if !result.IsValid() {
		msgs := make([]string, len(result.Errors))
		for i, e := range result.Errors {
			msgs[i] = fmt.Sprintf("[%s] %s: %s", e.Code, e.Path, e.Message)
		}
		return nil, fmt.Errorf("document has %d lint error(s): %v", len(result.Errors), msgs)
	}

	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("import: begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.ErrorContext(ctx, "import: rollback after panic", "error", rbErr)
			}
			panic(p)
		}
	}()

	stats, err := s.importCreate(ctx, tx, userID, doc)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return nil, fmt.Errorf("import: %w; rollback failed: %w", err, rbErr)
		}
		return nil, fmt.Errorf("import: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("import: commit: %w", err)
	}

	var warnings []string
	for _, w := range result.Warnings {
		warnings = append(warnings, fmt.Sprintf("[%s] %s: %s", w.Code, w.Path, w.Message))
	}

	return &ImportResult{
		QuestID:  stats.QuestID,
		Created:  stats.Created,
		Warnings: warnings,
	}, nil
}

// ImportUpdate parses a GameDoc and reconciles it with an existing instance.
func (s *ImportService) ImportUpdate( //nolint:gocognit
	ctx context.Context,
	userID, questID string,
	doc *game.GameDoc,
) (*ImportResult, error) {
	// Lint the document
	lintResult := game.Lint(doc, blocks.Registry())
	if !lintResult.IsValid() {
		msgs := make([]string, len(lintResult.Errors))
		for i, e := range lintResult.Errors {
			msgs[i] = fmt.Sprintf("[%s] %s: %s", e.Code, e.Path, e.Message)
		}
		return nil, fmt.Errorf("document has %d lint error(s): %v", len(lintResult.Errors), msgs)
	}

	// Load existing instance
	existing, err := s.instanceRepo.GetByID(ctx, questID)
	if err != nil {
		return nil, fmt.Errorf("import update: load instance: %w", err)
	}
	if existing.UserID != userID {
		return nil, ErrUserNotAuthenticated
	}

	existingLocations, err := s.locationRepo.FindByInstance(ctx, questID)
	if err != nil {
		return nil, fmt.Errorf("import update: load locations: %w", err)
	}
	existingObjectives, err := s.objectiveRepo.FindByQuestID(ctx, questID)
	if err != nil {
		return nil, fmt.Errorf("import update: load objectives: %w", err)
	}

	existingOwnerIDs := make([]string, 0, len(existingLocations)+len(existingObjectives)+1)
	existingOwnerIDs = append(existingOwnerIDs, questID)
	for i := range existingLocations {
		existingOwnerIDs = append(existingOwnerIDs, existingLocations[i].ID)
	}
	for i := range existingObjectives {
		existingOwnerIDs = append(existingOwnerIDs, existingObjectives[i].ID)
	}
	existingBlocks, err := s.blockRepo.FindModelsByOwnerIDs(ctx, existingOwnerIDs)
	if err != nil {
		return nil, fmt.Errorf("import update: load blocks: %w", err)
	}

	// Build indexes for reconciliation
	locByID := make(map[string]*models.Location, len(existingLocations))
	locBySlug := make(map[string]*models.Location, len(existingLocations))
	for i := range existingLocations {
		locByID[existingLocations[i].ID] = &existingLocations[i]
		locBySlug[existingLocations[i].Slug] = &existingLocations[i]
	}
	objByID := make(map[string]*models.Objective, len(existingObjectives))
	objBySlug := make(map[string]*models.Objective, len(existingObjectives))
	for i := range existingObjectives {
		objByID[existingObjectives[i].ID] = &existingObjectives[i]
		objBySlug[existingObjectives[i].Slug] = &existingObjectives[i]
	}
	blockByID := make(map[string]models.Block, len(existingBlocks))
	for _, b := range existingBlocks {
		blockByID[b.ID] = b
	}

	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("import update: begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rbErr := tx.Rollback(); rbErr != nil {
				s.logger.ErrorContext(ctx, "import update: rollback after panic", "error", rbErr)
			}
			panic(p)
		}
	}()

	importResult, err := s.importUpdate(ctx, tx, existing, doc, locByID, locBySlug, objByID, objBySlug, blockByID)
	if err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return nil, fmt.Errorf("import update: %w; rollback failed: %w", err, rbErr)
		}
		return nil, fmt.Errorf("import update: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("import update: commit: %w", err)
	}

	for _, w := range lintResult.Warnings {
		importResult.Warnings = append(importResult.Warnings, fmt.Sprintf("[%s] %s: %s", w.Code, w.Path, w.Message))
	}

	return importResult, nil
}

// --- Internal create implementation ---

type createResult struct {
	QuestID string
	Created ImportStats
}

func (s *ImportService) importCreate(
	ctx context.Context,
	tx *bun.Tx,
	userID string,
	doc *game.GameDoc,
) (*createResult, error) {
	// Create Instance
	newInstance := &models.Quest{
		Name:                  doc.Name,
		UserID:                userID,
		IsQuickStartDismissed: true,
	}
	if err := s.instanceRepo.CreateTx(ctx, tx, newInstance); err != nil {
		return nil, fmt.Errorf("create instance: %w", err)
	}

	// Create QuestSettings
	settings := &models.QuestSettings{
		QuestID:         newInstance.ID,
		MustCheckOut:    doc.Settings.MustCheckOut,
		ShowTeamCount:   doc.Settings.ShowTeamCount,
		EnablePoints:    doc.Settings.EnablePoints,
		ShowLeaderboard: doc.Settings.ShowLeaderboard,
	}
	if err := s.instanceSettingsRepo.CreateTx(ctx, tx, settings); err != nil {
		return nil, fmt.Errorf("create settings: %w", err)
	}

	stats := ImportStats{}
	gs := &models.GameStructure{
		ID:              uuid.New().String(),
		IsRoot:          true,
		Routing:         doc.Structure.Routing,
		CompletionType:  doc.Structure.Completion,
		MinimumRequired: doc.Structure.MinimumRequired,
		LocationIDs:     []string{},
		ObjectiveIDs:    []string{},
		SubGroups:       []models.GameStructure{},
	}

	if err := s.walkCreateChildren(ctx, tx, newInstance.ID, doc.Structure.Children, gs, &stats); err != nil {
		return nil, err
	}

	// Create start/finish blocks
	startCount, err := s.createBlockDocs(ctx, tx, newInstance.ID, doc.Start, game.ContextStart)
	if err != nil {
		return nil, fmt.Errorf("create start blocks: %w", err)
	}
	finishCount, err := s.createBlockDocs(ctx, tx, newInstance.ID, doc.Finish, game.ContextFinish)
	if err != nil {
		return nil, fmt.Errorf("create finish blocks: %w", err)
	}
	stats.Blocks += startCount + finishCount

	// Save GameStructure to Instance
	newInstance.GameStructure = *gs
	if _, err := tx.NewUpdate().
		Model(newInstance).
		Column("game_structure").
		WherePK().
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("save game structure: %w", err)
	}

	return &createResult{QuestID: newInstance.ID, Created: stats}, nil
}

func (s *ImportService) walkCreateChildren(
	ctx context.Context,
	tx *bun.Tx,
	questID string,
	children []game.ChildDoc,
	parentGS *models.GameStructure,
	stats *ImportStats,
) error {
	for _, child := range children {
		if child.Location != nil {
			locID, blockCount, err := s.createLocation(ctx, tx, questID, *child.Location)
			if err != nil {
				return err
			}
			parentGS.LocationIDs = append(parentGS.LocationIDs, locID)
			stats.Locations++
			stats.Blocks += blockCount
		} else if child.Objective != nil {
			objID, blockCount, err := s.createObjective(ctx, tx, questID, *child.Objective)
			if err != nil {
				return err
			}
			parentGS.ObjectiveIDs = append(parentGS.ObjectiveIDs, objID)
			stats.Objectives++
			stats.Blocks += blockCount
		} else if child.Group != nil {
			g := child.Group
			subGS := models.GameStructure{
				ID:              uuid.New().String(),
				Name:            g.Name,
				Color:           g.Color,
				Routing:         g.Routing,
				CompletionType:  g.Completion,
				MinimumRequired: g.MinimumRequired,
				AutoAdvance:     g.AutoAdvance == nil || *g.AutoAdvance,
				When:            g.When,
				LocationIDs:     []string{},
				ObjectiveIDs:    []string{},
				SubGroups:       []models.GameStructure{},
			}
			if err := s.walkCreateChildren(ctx, tx, questID, g.Children, &subGS, stats); err != nil {
				return err
			}
			parentGS.SubGroups = append(parentGS.SubGroups, subGS)
			stats.Groups++
		}
	}
	return nil
}

// createLocation creates a marker (if coords given), a location, and all its blocks.
// Returns the new location ID and the total number of blocks created.
func (s *ImportService) createLocation(
	ctx context.Context,
	tx *bun.Tx,
	questID string,
	locDoc game.LocationDoc,
) (string, int, error) {
	// Create marker
	marker := &models.Marker{Name: locDoc.Name}
	if locDoc.Marker != nil {
		marker.Lat = locDoc.Marker.Lat
		marker.Lng = locDoc.Marker.Lng
	}
	if err := s.markerRepo.CreateTx(ctx, tx, marker); err != nil {
		return "", 0, fmt.Errorf("create marker for %q: %w", locDoc.Slug, err)
	}

	// Create location
	loc := &models.Location{
		QuestID:  questID,
		Slug:     locDoc.Slug,
		Name:     locDoc.Name,
		Points:   locDoc.Points,
		When:     locDoc.When,
		MarkerID: marker.Code,
	}
	if err := s.locationRepo.CreateTx(ctx, tx, loc); err != nil {
		return "", 0, fmt.Errorf("create location %q: %w", locDoc.Slug, err)
	}

	// Create blocks per context
	count := 0
	for _, pair := range []struct {
		docs []game.BlockDoc
		ctx  game.BlockContext
	}{
		{locDoc.Content, game.ContextLocationContent},
		{locDoc.Navigation, game.ContextNavigation},
	} {
		n, err := s.createBlockDocs(ctx, tx, loc.ID, pair.docs, pair.ctx)
		if err != nil {
			return "", 0, fmt.Errorf("create blocks for %q context %q: %w", locDoc.Slug, pair.ctx, err)
		}
		count += n
	}

	return loc.ID, count, nil
}

func (s *ImportService) createObjective(
	ctx context.Context,
	tx *bun.Tx,
	questID string,
	objDoc game.ObjectiveDoc,
) (string, int, error) {
	obj := &models.Objective{
		QuestID:    questID,
		Slug:       objDoc.Slug,
		Title:      objDoc.Title,
		When:       objDoc.When,
		ProofSets:  objDoc.Proof.Sets,
		RevealSets: objDoc.Reveal.Sets,
	}
	if err := s.objectiveRepo.CreateTx(ctx, tx, obj); err != nil {
		return "", 0, fmt.Errorf("create objective %q: %w", objDoc.Slug, err)
	}

	count := 0
	for _, pair := range []struct {
		docs []game.BlockDoc
		ctx  game.BlockContext
	}{
		{objDoc.Proof.Blocks, game.ContextObjectiveProof},
		{objDoc.Reveal.Blocks, game.ContextObjectiveReveal},
	} {
		n, err := s.createBlockDocs(ctx, tx, obj.ID, pair.docs, pair.ctx)
		if err != nil {
			return "", 0, fmt.Errorf("create blocks for %q context %q: %w", objDoc.Slug, pair.ctx, err)
		}
		count += n
	}

	return obj.ID, count, nil
}

// createBlockDocs creates all blocks from a slice of BlockDoc for the given owner+context.
func (s *ImportService) createBlockDocs(
	ctx context.Context,
	tx *bun.Tx,
	ownerID string,
	docs []game.BlockDoc,
	blockCtx game.BlockContext,
) (int, error) {
	for i, doc := range docs {
		block, err := docToBlock(doc, ownerID, i)
		if err != nil {
			return i, fmt.Errorf("block[%d]: %w", i, err)
		}
		if _, err := s.blockRepo.CreateTx(ctx, tx, block, ownerID, blockCtx); err != nil {
			return i, fmt.Errorf("create block[%d] type=%q: %w", i, block.GetType(), err)
		}
	}
	return len(docs), nil
}

// --- Internal update implementation ---

func (s *ImportService) importUpdate(
	ctx context.Context,
	tx *bun.Tx,
	existing *models.Quest,
	doc *game.GameDoc,
	locByID map[string]*models.Location,
	locBySlug map[string]*models.Location,
	objByID map[string]*models.Objective,
	objBySlug map[string]*models.Objective,
	blockByID map[string]models.Block,
) (*ImportResult, error) {
	result := &ImportResult{QuestID: existing.ID}

	// Update instance name
	existing.Name = doc.Name
	if err := s.instanceRepo.UpdateTx(ctx, tx, existing); err != nil {
		return nil, fmt.Errorf("update instance: %w", err)
	}

	// Update settings
	settings, err := s.instanceSettingsRepo.GetByQuestID(ctx, existing.ID)
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	settings.MustCheckOut = doc.Settings.MustCheckOut
	settings.ShowTeamCount = doc.Settings.ShowTeamCount
	settings.EnablePoints = doc.Settings.EnablePoints
	settings.ShowLeaderboard = doc.Settings.ShowLeaderboard
	if err := s.instanceSettingsRepo.UpdateTx(ctx, tx, settings); err != nil {
		return nil, fmt.Errorf("update settings: %w", err)
	}

	// Track which existing locations/objectives appeared in the doc (to find orphans).
	seenLocIDs := make(map[string]bool)
	seenObjIDs := make(map[string]bool)

	// Build new GameStructure
	gs := &models.GameStructure{
		ID:              existing.GameStructure.ID,
		IsRoot:          true,
		Routing:         doc.Structure.Routing,
		CompletionType:  doc.Structure.Completion,
		MinimumRequired: doc.Structure.MinimumRequired,
		LocationIDs:     []string{},
		ObjectiveIDs:    []string{},
		SubGroups:       []models.GameStructure{},
	}

	if err := s.walkUpdateChildren(ctx, tx, existing.ID, doc.Structure.Children, gs,
		locByID, locBySlug, objByID, objBySlug, blockByID, seenLocIDs, seenObjIDs, result); err != nil {
		return nil, err
	}

	// Reconcile start/finish blocks
	if err := s.reconcileInstanceBlocks(
		ctx,
		tx,
		existing.ID,
		doc.Start,
		game.ContextStart,
		blockByID,
		result,
	); err != nil {
		return nil, err
	}
	if err := s.reconcileInstanceBlocks(
		ctx,
		tx,
		existing.ID,
		doc.Finish,
		game.ContextFinish,
		blockByID,
		result,
	); err != nil {
		return nil, err
	}

	// Delete locations not seen in the doc
	for locID, loc := range locByID {
		if !seenLocIDs[locID] {
			// blocks.owner_id has no FK, so the row delete below would not cascade.
			if _, err := s.blockRepo.DeleteByOwnerIDPreservingStates(ctx, tx, locID, nil); err != nil {
				return nil, fmt.Errorf("delete blocks for orphan location %q: %w", loc.Slug, err)
			}
			if err := s.locationRepo.Delete(ctx, tx, locID); err != nil {
				return nil, fmt.Errorf("delete orphan location %q: %w", loc.Slug, err)
			}
			result.Deleted.Locations++
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("location %q (id=%s) was in DB but not in document; deleted", loc.Slug, locID))
		}
	}

	for objID, obj := range objByID {
		if !seenObjIDs[objID] {
			// blocks.owner_id has no FK, so the row delete below would not cascade.
			if _, err := s.blockRepo.DeleteByOwnerIDPreservingStates(ctx, tx, objID, nil); err != nil {
				return nil, fmt.Errorf("delete blocks for orphan objective %q: %w", obj.Slug, err)
			}
			if err := s.objectiveRepo.Delete(ctx, tx, objID); err != nil {
				return nil, fmt.Errorf("delete orphan objective %q: %w", obj.Slug, err)
			}
			result.Deleted.Objectives++
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("objective %q (id=%s) was in DB but not in document; deleted", obj.Slug, objID))
		}
	}

	// Save updated GameStructure
	existing.GameStructure = *gs
	if _, err := tx.NewUpdate().
		Model(existing).
		Column("game_structure").
		WherePK().
		Exec(ctx); err != nil {
		return nil, fmt.Errorf("save game structure: %w", err)
	}

	return result, nil
}

func (s *ImportService) walkUpdateChildren(
	ctx context.Context,
	tx *bun.Tx,
	questID string,
	children []game.ChildDoc,
	parentGS *models.GameStructure,
	locByID map[string]*models.Location,
	locBySlug map[string]*models.Location,
	objByID map[string]*models.Objective,
	objBySlug map[string]*models.Objective,
	blockByID map[string]models.Block,
	seenLocIDs map[string]bool,
	seenObjIDs map[string]bool,
	result *ImportResult,
) error {
	for _, child := range children {
		if child.Location != nil {
			locID, err := s.reconcileLocation(ctx, tx, questID, *child.Location,
				locByID, locBySlug, blockByID, seenLocIDs, result)
			if err != nil {
				return err
			}
			parentGS.LocationIDs = append(parentGS.LocationIDs, locID)
		} else if child.Objective != nil {
			objID, err := s.reconcileObjective(ctx, tx, questID, *child.Objective,
				objByID, objBySlug, blockByID, seenObjIDs, result)
			if err != nil {
				return err
			}
			parentGS.ObjectiveIDs = append(parentGS.ObjectiveIDs, objID)
		} else if child.Group != nil {
			g := child.Group
			groupID := g.ID
			if groupID == "" {
				groupID = uuid.New().String()
			}
			subGS := models.GameStructure{
				ID:              groupID,
				Name:            g.Name,
				Color:           g.Color,
				Routing:         g.Routing,
				CompletionType:  g.Completion,
				MinimumRequired: g.MinimumRequired,
				AutoAdvance:     g.AutoAdvance == nil || *g.AutoAdvance,
				When:            g.When,
				LocationIDs:     []string{},
				ObjectiveIDs:    []string{},
				SubGroups:       []models.GameStructure{},
			}
			if err := s.walkUpdateChildren(ctx, tx, questID, g.Children, &subGS,
				locByID, locBySlug, objByID, objBySlug, blockByID, seenLocIDs, seenObjIDs, result); err != nil {
				return err
			}
			parentGS.SubGroups = append(parentGS.SubGroups, subGS)
		}
	}
	return nil
}

// reconcileLocation matches a LocationDoc to an existing location (by ID or slug),
// updates it, or creates a new one if no match is found.
func (s *ImportService) reconcileLocation(
	ctx context.Context,
	tx *bun.Tx,
	questID string,
	locDoc game.LocationDoc,
	locByID map[string]*models.Location,
	locBySlug map[string]*models.Location,
	blockByID map[string]models.Block,
	seenLocIDs map[string]bool,
	result *ImportResult,
) (string, error) {
	// Try to match existing location
	var existingLoc *models.Location
	if locDoc.ID != "" {
		existingLoc = locByID[locDoc.ID]
	}
	if existingLoc == nil && locDoc.Slug != "" {
		existingLoc = locBySlug[locDoc.Slug]
	}

	if existingLoc == nil {
		// Create new location
		newLocID, blockCount, err := s.createLocation(ctx, tx, questID, locDoc)
		if err != nil {
			return "", err
		}
		result.Created.Locations++
		result.Created.Blocks += blockCount
		return newLocID, nil
	}

	// Update existing location
	seenLocIDs[existingLoc.ID] = true
	existingLoc.Name = locDoc.Name
	existingLoc.Slug = locDoc.Slug
	existingLoc.Points = locDoc.Points
	existingLoc.When = locDoc.When

	if err := s.locationRepo.UpdateTx(ctx, tx, existingLoc); err != nil {
		return "", fmt.Errorf("update location %q: %w", locDoc.Slug, err)
	}

	// Update marker coordinates if provided
	if locDoc.Marker != nil {
		if err := s.markerRepo.UpdateCoordsTx(ctx, tx,
			&models.Marker{Code: existingLoc.MarkerID},
			locDoc.Marker.Lat, locDoc.Marker.Lng); err != nil {
			return "", fmt.Errorf("update marker for %q: %w", locDoc.Slug, err)
		}
	}

	// Reconcile blocks per context
	for _, pair := range []struct {
		docs []game.BlockDoc
		ctx  game.BlockContext
	}{
		{locDoc.Content, game.ContextLocationContent},
		{locDoc.Navigation, game.ContextNavigation},
	} {
		if err := s.reconcileBlocks(ctx, tx, existingLoc.ID, pair.docs, pair.ctx, blockByID, result); err != nil {
			return "", err
		}
	}

	result.Updated.Locations++
	return existingLoc.ID, nil
}

// reconcileObjective matches an ObjectiveDoc to an existing objective by ID or
// slug; creates one when there is no match.
func (s *ImportService) reconcileObjective(
	ctx context.Context,
	tx *bun.Tx,
	questID string,
	objDoc game.ObjectiveDoc,
	objByID map[string]*models.Objective,
	objBySlug map[string]*models.Objective,
	blockByID map[string]models.Block,
	seenObjIDs map[string]bool,
	result *ImportResult,
) (string, error) {
	var existingObj *models.Objective
	if objDoc.ID != "" {
		existingObj = objByID[objDoc.ID]
	}
	if existingObj == nil && objDoc.Slug != "" {
		existingObj = objBySlug[objDoc.Slug]
	}

	if existingObj == nil {
		newObjID, blockCount, err := s.createObjective(ctx, tx, questID, objDoc)
		if err != nil {
			return "", err
		}
		result.Created.Objectives++
		result.Created.Blocks += blockCount
		return newObjID, nil
	}

	seenObjIDs[existingObj.ID] = true
	existingObj.Title = objDoc.Title
	existingObj.Slug = objDoc.Slug
	existingObj.When = objDoc.When
	existingObj.ProofSets = objDoc.Proof.Sets
	existingObj.RevealSets = objDoc.Reveal.Sets

	if err := s.objectiveRepo.UpdateTx(ctx, tx, existingObj); err != nil {
		return "", fmt.Errorf("update objective %q: %w", objDoc.Slug, err)
	}

	for _, pair := range []struct {
		docs []game.BlockDoc
		ctx  game.BlockContext
	}{
		{objDoc.Proof.Blocks, game.ContextObjectiveProof},
		{objDoc.Reveal.Blocks, game.ContextObjectiveReveal},
	} {
		if err := s.reconcileBlocks(ctx, tx, existingObj.ID, pair.docs, pair.ctx, blockByID, result); err != nil {
			return "", err
		}
	}

	result.Updated.Objectives++
	return existingObj.ID, nil
}

// reconcileBlocks reconciles blocks for a given owner+context.
// Blocks with IDs are matched and updated; blocks without IDs are created.
// Blocks in DB not present in doc are deleted.
func (s *ImportService) reconcileBlocks( //nolint:gocognit
	ctx context.Context,
	tx *bun.Tx,
	ownerID string,
	docs []game.BlockDoc,
	blockCtx game.BlockContext,
	blockByID map[string]models.Block,
	result *ImportResult,
) error {
	// Collect block IDs that appear in the doc for this context
	seenBlockIDs := make(map[string]bool)

	for i, doc := range docs {
		idVal, hasID := doc["id"]
		idStr, _ := idVal.(string)
		if hasID && idStr != "" {
			// Match existing block by ID
			if _, exists := blockByID[idStr]; exists {
				seenBlockIDs[idStr] = true
				// Update: replace Data and Points
				blockType, _ := doc["type"].(string)
				data, points := docToDataAndPoints(doc)
				existing := blockByID[idStr]
				existing.Type = blockType
				existing.Data = data
				existing.Points = points
				existing.Ordering = i

				if _, err := tx.NewUpdate().
					Model(&existing).
					Column("type", "data", "points", "ordering").
					WherePK().
					Exec(ctx); err != nil {
					return fmt.Errorf("update block %q: %w", idStr, err)
				}
				result.Updated.Blocks++
				continue
			}
		}

		// Create new block (ID absent or not found in DB)
		block, err := docToBlock(doc, ownerID, i)
		if err != nil {
			return fmt.Errorf("block[%d]: %w", i, err)
		}
		if _, err := s.blockRepo.CreateTx(ctx, tx, block, ownerID, blockCtx); err != nil {
			return fmt.Errorf("create block[%d]: %w", i, err)
		}
		result.Created.Blocks++
	}

	// Delete blocks in DB for this owner+context that were not in the doc
	for id, b := range blockByID {
		if b.OwnerID == ownerID && b.Context == blockCtx && !seenBlockIDs[id] {
			if err := s.blockRepo.Delete(ctx, tx, id); err != nil {
				return fmt.Errorf("delete block %q: %w", id, err)
			}
			result.Deleted.Blocks++
		}
	}

	return nil
}

// reconcileInstanceBlocks reconciles start/finish blocks owned by the instance.
func (s *ImportService) reconcileInstanceBlocks(
	ctx context.Context,
	tx *bun.Tx,
	questID string,
	docs []game.BlockDoc,
	blockCtx game.BlockContext,
	blockByID map[string]models.Block,
	result *ImportResult,
) error {
	return s.reconcileBlocks(ctx, tx, questID, docs, blockCtx, blockByID, result)
}

// --- Helpers ---

// docToBlock converts a BlockDoc to a blocks.Block ready for DB insertion.
// ownerID and ordering are set on the base block.
func docToBlock(doc game.BlockDoc, ownerID string, ordering int) (blocks.Block, error) {
	blockType, _ := doc["type"].(string)
	if blockType == "" {
		return nil, errors.New("block doc missing type field")
	}

	data, bPoints := docToDataAndPoints(doc)

	base := blocks.BaseBlock{
		OwnerID: ownerID,
		Type:    blockType,
		Data:    data,
		Order:   ordering,
		Points:  bPoints,
	}

	block, err := blocks.CreateFromBaseBlock(base)
	if err != nil {
		return nil, fmt.Errorf("unknown block type %q: %w", blockType, err)
	}
	if err := block.ParseData(); err != nil {
		return nil, fmt.Errorf("parse data for block type %q: %w", blockType, err)
	}
	return block, nil
}

// docToDataAndPoints extracts the block-specific data JSON and points from a BlockDoc.
// The "type", "id", and "points" fields are stripped; the rest is marshaled as Data.
func docToDataAndPoints(doc game.BlockDoc) (json.RawMessage, int) {
	var points int
	if pVal, ok := doc["points"]; ok {
		switch v := pVal.(type) {
		case float64:
			points = int(v)
		case int:
			points = v
		}
	}

	// Build data map without the promoted fields
	dataMap := make(map[string]any, len(doc))
	for k, v := range doc {
		if k == "type" || k == "id" || k == "points" {
			continue
		}
		dataMap[k] = v
	}

	data, _ := json.Marshal(dataMap)
	return data, points
}
