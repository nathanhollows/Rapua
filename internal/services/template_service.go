package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nathanhollows/Rapua/v3/db"
	"github.com/nathanhollows/Rapua/v3/helpers"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
	"github.com/uptrace/bun"
)

type TemplateService struct {
	transactor           db.Transactor
	locationService      LocationService
	instanceRepo         repositories.InstanceRepository
	instanceSettingsRepo repositories.InstanceSettingsRepository
	shareLinkRepo        repositories.ShareLinkRepository
}

func NewTemplateService(
	transactor db.Transactor,
	locationService LocationService,
	instanceRepo repositories.InstanceRepository,
	instanceSettingsRepo repositories.InstanceSettingsRepository,
	shareLinkRepo repositories.ShareLinkRepository,
) TemplateService {
	return TemplateService{
		transactor:           transactor,
		locationService:      locationService,
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
		shareLinkRepo:        shareLinkRepo,
	}
}

// CreateFromInstance creates a new template from an existing instance.
func (s *TemplateService) CreateFromInstance(ctx context.Context, userID, instanceID, name string) (*models.Instance, error) {
	if userID == "" {
		return nil, NewValidationError("userID")
	}

	if instanceID == "" {
		return nil, NewValidationError("instanceID")
	}
	if name == "" {
		return nil, NewValidationError("name")
	}

	oldInstance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return nil, fmt.Errorf("finding instance: %w", err)
	}
	if oldInstance == nil {
		return nil, errors.New("instance not found")
	}

	if oldInstance.UserID != userID {
		return nil, ErrPermissionDenied
	}

	if oldInstance.IsTemplate {
		return nil, errors.New("cannot create a template from a template")
	}

	locations, err := s.locationService.FindByInstance(ctx, oldInstance.ID)
	if err != nil {
		return nil, fmt.Errorf("finding locations: %w", err)
	}

	if name == "" {
		return nil, NewValidationError("name")
	}
	if instanceID == "" {
		return nil, NewValidationError("id")
	}

	newInstance := &models.Instance{
		Name:       name,
		UserID:     userID,
		IsTemplate: true,
	}

	if err := s.instanceRepo.Create(ctx, newInstance); err != nil {
		return nil, fmt.Errorf("creating instance: %w", err)
	}

	// Copy locations
	for _, location := range locations {
		_, err := s.locationService.DuplicateLocation(ctx, location, newInstance.ID)
		if err != nil {
			return nil, fmt.Errorf("duplicating location: %w", err)
		}
	}

	// Copy settings
	settings := oldInstance.Settings
	settings.InstanceID = newInstance.ID
	if err := s.instanceSettingsRepo.Create(ctx, &settings); err != nil {
		return nil, fmt.Errorf("creating settings: %w", err)
	}

	return newInstance, nil
}

// LaunchInstance creates a new instance from a template.
func (s *TemplateService) LaunchInstance(ctx context.Context, userID, templateID, name string, regen_location_codes bool) (*models.Instance, error) {
	if userID == "" {
		return nil, NewValidationError("userID")
	}
	if name == "" {
		return nil, NewValidationError("name")
	}

	template, err := s.instanceRepo.GetByID(ctx, templateID)
	if err != nil {
		return nil, fmt.Errorf("finding template: %w", err)
	}

	if template.UserID != userID {
		return nil, ErrPermissionDenied
	}

	if !template.IsTemplate {
		return nil, errors.New("instance is not a template")
	}

	locations, err := s.locationService.FindByInstance(ctx, template.ID)
	if err != nil {
		return nil, fmt.Errorf("finding locations: %w", err)
	}

	newInstance := &models.Instance{
		Name:       name,
		UserID:     userID,
		IsTemplate: false,
	}

	if err := s.instanceRepo.Create(ctx, newInstance); err != nil {
		return nil, fmt.Errorf("creating instance: %w", err)
	}

	// Copy locations
	for _, location := range locations {
		_, err := s.locationService.DuplicateLocation(ctx, location, newInstance.ID)
		if err != nil {
			return nil, fmt.Errorf("duplicating location: %w", err)
		}
	}

	// Copy settings
	settings := template.Settings
	settings.InstanceID = newInstance.ID
	if err := s.instanceSettingsRepo.Create(ctx, &settings); err != nil {
		return nil, fmt.Errorf("creating settings: %w", err)
	}

	return newInstance, nil
}

// LaunchInstanceFromShareLink creates a new instance from a share link.
func (s *TemplateService) LaunchInstanceFromShareLink(ctx context.Context, userID, shareLinkID string, name string, regen bool) (*models.Instance, error) {
	if userID == "" {
		return nil, NewValidationError("userID")
	}
	if name == "" {
		return nil, NewValidationError("name")
	}
	if shareLinkID == "" {
		return nil, NewValidationError("shareLinkID")
	}
	shareLink, err := s.shareLinkRepo.GetByID(ctx, shareLinkID)
	if err != nil {
		return nil, fmt.Errorf("finding share link: %w", err)
	}

	// Thankfully this checks the expiration date and max uses
	if shareLink.IsExpired() {
		return nil, errors.New("share link is expired")
	}

	if shareLink.UserID != userID {
		return nil, ErrPermissionDenied
	}

	template, err := s.instanceRepo.GetByID(ctx, shareLink.TemplateID)
	if err != nil {
		return nil, fmt.Errorf("finding template: %w", err)
	}
	if template == nil {
		return nil, errors.New("template not found")
	}
	if !template.IsTemplate {
		return nil, errors.New("instance is not a template")
	}
	if template.UserID != shareLink.UserID {
		return nil, ErrPermissionDenied
	}
	locations, err := s.locationService.FindByInstance(ctx, template.ID)
	if err != nil {
		return nil, fmt.Errorf("finding locations: %w", err)
	}
	newInstance := &models.Instance{
		Name:       name,
		UserID:     userID,
		IsTemplate: false,
	}
	if err := s.instanceRepo.Create(ctx, newInstance); err != nil {
		return nil, fmt.Errorf("creating instance: %w", err)
	}
	// Copy locations
	for _, location := range locations {
		_, err := s.locationService.DuplicateLocation(ctx, location, newInstance.ID)
		if err != nil {
			return nil, fmt.Errorf("duplicating location: %w", err)
		}
	}
	// Copy settings
	settings := template.Settings
	settings.InstanceID = newInstance.ID
	if err := s.instanceSettingsRepo.Create(ctx, &settings); err != nil {
		return nil, fmt.Errorf("creating settings: %w", err)
	}
	// Increment the used count
	shareLink.UsedCount++
	if shareLink.MaxUses > 0 && shareLink.UsedCount >= shareLink.MaxUses {
		shareLink.ExpiresAt = bun.NullTime{Time: time.Now()}
	}
	if err := s.shareLinkRepo.Use(ctx, shareLink); err != nil {
		return nil, fmt.Errorf("updating share link: %w", err)
	}

	return newInstance, nil
}

// GetByID retrieves a template by ID.
func (s *TemplateService) GetByID(ctx context.Context, id string) (*models.Instance, error) {
	instance, err := s.instanceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("finding instance: %w", err)
	}
	if !instance.IsTemplate {
		return nil, errors.New("instance is not a template")
	}
	return instance, nil
}

// GetShareLink retrieves a share link by ID.
func (s *TemplateService) GetShareLink(ctx context.Context, id string) (*models.ShareLink, error) {
	shareLink, err := s.shareLinkRepo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("finding share link: %w", err)
	}
	return shareLink, nil
}

// Find retrieves all templates.
func (s *TemplateService) Find(ctx context.Context, userID string) ([]models.Instance, error) {
	if userID == "" {
		return nil, NewValidationError("userID")
	}
	instances, err := s.instanceRepo.FindTemplates(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("finding templates: %w", err)
	}
	return instances, nil
}

// Update updates a template.
func (s *TemplateService) Update(ctx context.Context, instance *models.Instance) error {
	if instance == nil {
		return NewValidationError("instance")
	}
	if instance.ID == "" {
		return NewValidationError("instance.ID")
	}
	if instance.Name == "" {
		return NewValidationError("instance.Name")
	}

	err := s.instanceRepo.Update(ctx, instance)
	if err != nil {
		return fmt.Errorf("updating instance: %w", err)
	}
	return nil
}

type ShareLinkData struct {
	TemplateID string
	Validity   string
	MaxUses    int
	Regenerate bool
}

// CreateShareLink creates a share link for a template.
func (s *TemplateService) CreateShareLink(ctx context.Context, userID string, data ShareLinkData) (string, error) {
	if userID == "" {
		return "", NewValidationError("userID")
	}
	if data.TemplateID == "" {
		return "", NewValidationError("data.InstanceID")
	}

	instance, err := s.instanceRepo.GetByID(ctx, data.TemplateID)
	if err != nil {
		return "", fmt.Errorf("finding instance: %w", err)
	}

	if instance.UserID != userID {
		return "", ErrPermissionDenied
	}

	shareLink := &models.ShareLink{
		TemplateID:      instance.ID,
		UserID:          userID,
		MaxUses:         data.MaxUses,
		CreatedAt:       time.Now(),
		RegenerateCodes: data.Regenerate,
	}

	switch data.Validity {
	case "always":
		shareLink.ExpiresAt = bun.NullTime{}
	case "day":
		shareLink.ExpiresAt = bun.NullTime{Time: time.Now().AddDate(0, 0, 1)}
	case "week":
		shareLink.ExpiresAt = bun.NullTime{Time: time.Now().AddDate(0, 0, 7)}
	case "month":
		shareLink.ExpiresAt = bun.NullTime{Time: time.Now().AddDate(0, 1, 0)}
	default:
		return "", NewValidationError("data.Validity")
	}

	err = s.shareLinkRepo.Create(ctx, shareLink)
	if err != nil {
		return "", fmt.Errorf("creating share link: %w", err)
	}

	url := helpers.URL("/templates/" + shareLink.ID)

	return url, nil
}
