// Package services provides entity duplication with transaction safety.
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/nathanhollows/Rapua/v6/db"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/uptrace/bun"
)

// DuplicationService coordinates cascading duplications across related entities.
type DuplicationService struct {
	transactor           db.Transactor
	instanceRepo         repositories.InstanceRepository
	instanceSettingsRepo repositories.InstanceSettingsRepository
	locationRepo         repositories.LocationRepository
	blockRepo            repositories.BlockRepository
}

// NewDuplicationService creates a new DuplicationService with the provided dependencies.
func NewDuplicationService(
	transactor db.Transactor,
	instanceRepo repositories.InstanceRepository,
	instanceSettingsRepo repositories.InstanceSettingsRepository,
	locationRepo repositories.LocationRepository,
	blockRepo repositories.BlockRepository,
) *DuplicationService {
	return &DuplicationService{
		transactor:           transactor,
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
		locationRepo:         locationRepo,
		blockRepo:            blockRepo,
	}
}

// DuplicateInstance duplicates a non-template instance and all its content with transaction safety.
// Returns ErrUserNotAuthenticated if user doesn't own the source instance.
// Returns error if source instance is a template (use CreateTemplateFromInstance instead).
func (s *DuplicationService) DuplicateInstance(
	ctx context.Context,
	user *models.User,
	sourceInstanceID string,
	name string,
) (*models.Instance, error) {
	return s.duplicateInstanceOrCreateTemplate(ctx, user, sourceInstanceID, name, false)
}

// CreateTemplateFromInstance creates a template from an existing non-template instance.
// Returns ErrUserNotAuthenticated if user doesn't own the source instance.
// Returns error if source instance is already a template.
func (s *DuplicationService) CreateTemplateFromInstance(
	ctx context.Context,
	user *models.User,
	sourceInstanceID string,
	name string,
) (*models.Instance, error) {
	return s.duplicateInstanceOrCreateTemplate(ctx, user, sourceInstanceID, name, true)
}

// duplicateInstanceOrCreateTemplate is the common implementation for duplicating instances and creating templates.
func (s *DuplicationService) duplicateInstanceOrCreateTemplate(
	ctx context.Context,
	user *models.User,
	sourceInstanceID string,
	name string,
	asTemplate bool,
) (*models.Instance, error) {
	if user == nil {
		return nil, ErrUserNotAuthenticated
	}

	if sourceInstanceID == "" {
		return nil, errors.New("sourceInstanceID cannot be empty")
	}

	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	// Verify ownership before starting transaction
	sourceInstance, err := s.instanceRepo.GetByID(ctx, sourceInstanceID)
	if err != nil {
		return nil, fmt.Errorf("finding source instance: %w", err)
	}

	if sourceInstance.UserID != user.ID {
		return nil, ErrUserNotAuthenticated
	}

	if sourceInstance.IsTemplate {
		if asTemplate {
			return nil, errors.New("cannot create template from template")
		}
		return nil, errors.New("cannot duplicate a template instance; use CreateInstanceFromTemplate instead")
	}

	// Start transaction
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				slog.Error("transaction rollback after panic", "error", rollbackErr)
			}
			panic(p)
		}
	}()

	operationName := "duplicating instance"
	if asTemplate {
		operationName = "creating template"
	}

	newInstance, err := s.duplicateInstance(ctx, tx, sourceInstance, user.ID, name, asTemplate)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, fmt.Errorf("%s: %w; rollback failed: %w", operationName, err, rollbackErr)
		}
		return nil, fmt.Errorf("%s: %w", operationName, err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return newInstance, nil
}

// CreateInstanceFromTemplate creates a regular instance from a template.
// Returns error if source is not a template.
func (s *DuplicationService) CreateInstanceFromTemplate(
	ctx context.Context,
	user *models.User,
	templateID string,
	name string,
) (*models.Instance, error) {
	return s.createInstanceFromTemplate(ctx, user, templateID, name, false)
}

// CreateInstanceFromSharedTemplate creates a regular instance from a shared template.
// Bypasses ownership check for share link scenarios.
// Returns error if source is not a template.
func (s *DuplicationService) CreateInstanceFromSharedTemplate(
	ctx context.Context,
	user *models.User,
	templateID string,
	name string,
) (*models.Instance, error) {
	return s.createInstanceFromTemplate(ctx, user, templateID, name, false)
}

// createInstanceFromTemplate is the internal implementation that creates an instance from a template.
func (s *DuplicationService) createInstanceFromTemplate(
	ctx context.Context,
	user *models.User,
	templateID string,
	name string,
	checkOwnership bool,
) (*models.Instance, error) {
	if user == nil {
		return nil, ErrUserNotAuthenticated
	}

	if templateID == "" {
		return nil, errors.New("templateID cannot be empty")
	}

	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	// Verify template exists and is a template
	template, err := s.instanceRepo.GetByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("finding template: %w", err)
	}

	if !template.IsTemplate {
		return nil, errors.New("source is not a template")
	}

	// Check ownership only if requested (not for share links)
	if checkOwnership && template.UserID != user.ID {
		return nil, ErrUserNotAuthenticated
	}

	// Start transaction
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				slog.Error("transaction rollback after panic", "error", rollbackErr)
			}
			panic(p)
		}
	}()

	newInstance, err := s.duplicateInstance(ctx, tx, template, user.ID, name, false)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, fmt.Errorf("creating instance from template: %w; rollback failed: %w", err, rollbackErr)
		}
		return nil, fmt.Errorf("creating instance from template: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return newInstance, nil
}

// DuplicateLocation duplicates a location and all its blocks with transaction safety.
func (s *DuplicationService) DuplicateLocation(
	ctx context.Context,
	sourceLocation models.Location,
	newInstanceID string,
) (*models.Location, error) {
	if newInstanceID == "" {
		return nil, errors.New("newInstanceID cannot be empty")
	}

	if sourceLocation.ID == "" {
		return nil, errors.New("sourceLocation.ID cannot be empty")
	}

	// Start transaction
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				slog.Error("transaction rollback after panic", "error", rollbackErr)
			}
			panic(p)
		}
	}()

	newLocation, err := s.duplicateLocation(ctx, tx, sourceLocation, newInstanceID)
	if err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return nil, fmt.Errorf("duplicating location: %w; rollback failed: %w", err, rollbackErr)
		}
		return nil, fmt.Errorf("duplicating location: %w", err)
	}

	err = tx.Commit()
	if err != nil {
		return nil, fmt.Errorf("committing transaction: %w", err)
	}

	return newLocation, nil
}

// duplicateInstance is the internal implementation that works within a transaction.
func (s *DuplicationService) duplicateInstance(
	ctx context.Context,
	tx *bun.Tx,
	sourceInstance *models.Instance,
	userID string,
	name string,
	asTemplate bool,
) (*models.Instance, error) {
	// Create new instance with copied game structure
	newInstance := &models.Instance{
		Name:          name,
		UserID:        userID,
		IsTemplate:    asTemplate,
		GameStructure: sourceInstance.GameStructure,
	}

	if err := s.instanceRepo.CreateTx(ctx, tx, newInstance); err != nil {
		return nil, fmt.Errorf("creating instance: %w", err)
	}

	// Duplicate settings
	settings := sourceInstance.Settings
	settings.InstanceID = newInstance.ID // InstanceID is the primary key
	if err := s.instanceSettingsRepo.CreateTx(ctx, tx, &settings); err != nil {
		return nil, fmt.Errorf("creating instance settings: %w", err)
	}

	// Get all locations from source instance
	locations, err := s.locationRepo.FindByInstance(ctx, sourceInstance.ID)
	if err != nil {
		return nil, fmt.Errorf("finding locations: %w", err)
	}

	// Duplicate all locations and build ID mapping
	locationIDMap := make(map[string]string, len(locations))
	for _, location := range locations {
		newLocation, dupErr := s.duplicateLocation(ctx, tx, location, newInstance.ID)
		if dupErr != nil {
			return nil, fmt.Errorf("duplicating location %s: %w", location.ID, dupErr)
		}
		locationIDMap[location.ID] = newLocation.ID
	}

	// Remap location IDs in the game structure
	s.remapLocationIDs(&newInstance.GameStructure, locationIDMap)

	// Update the instance with the remapped game structure
	_, err = tx.NewUpdate().
		Model(newInstance).
		Column("game_structure").
		WherePK().
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("updating game structure: %w", err)
	}

	return newInstance, nil
}

// duplicateLocation is the internal implementation that works within a transaction.
func (s *DuplicationService) duplicateLocation(
	ctx context.Context,
	tx *bun.Tx,
	sourceLocation models.Location,
	newInstanceID string,
) (*models.Location, error) {
	// Create new location (copy all fields except ID and InstanceID)
	newLocation := sourceLocation
	newLocation.ID = "" // Reset ID so a new one is generated
	newLocation.InstanceID = newInstanceID

	err := s.locationRepo.CreateTx(ctx, tx, &newLocation)
	if err != nil {
		return nil, fmt.Errorf("creating location: %w", err)
	}

	// Duplicate all blocks from old location to new location
	err = s.blockRepo.DuplicateBlocksByOwnerTx(ctx, tx, sourceLocation.ID, newLocation.ID)
	if err != nil {
		return nil, fmt.Errorf("duplicating blocks: %w", err)
	}

	return &newLocation, nil
}

// remapLocationIDs recursively updates all location IDs in the game structure
// using the provided mapping from old IDs to new IDs.
func (s *DuplicationService) remapLocationIDs(group *models.GameStructure, idMap map[string]string) {
	// Remap location IDs in this group
	for i, oldID := range group.LocationIDs {
		if newID, exists := idMap[oldID]; exists {
			group.LocationIDs[i] = newID
		}
	}

	// Recursively remap in subgroups
	for i := range group.SubGroups {
		s.remapLocationIDs(&group.SubGroups[i], idMap)
	}
}
