package services

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/nathanhollows/Rapua/v4/helpers"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/uptrace/bun"
)

type TemplateService struct {
	locationService      LocationService
	instanceRepo         repositories.InstanceRepository
	instanceSettingsRepo repositories.InstanceSettingsRepository
	shareLinkRepo        repositories.ShareLinkRepository
}

func NewTemplateService(
	locationService LocationService,
	instanceRepo repositories.InstanceRepository,
	instanceSettingsRepo repositories.InstanceSettingsRepository,
	shareLinkRepo repositories.ShareLinkRepository,
) TemplateService {
	return TemplateService{
		locationService:      locationService,
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
		shareLinkRepo:        shareLinkRepo,
	}
}

// CreateFromInstance creates a new template from an existing instance.
func (s *TemplateService) CreateFromInstance(ctx context.Context, userID, instanceID, name string) (*models.Instance, error) {
	if userID == "" {
		return nil, errors.New("userID cannot be empty")
	}

	if instanceID == "" {
		return nil, errors.New("instanceID cannot be empty")
	}
	if name == "" {
		return nil, errors.New("name cannot be empty")
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
		return nil, errors.New("name cannot be empty")
	}
	if instanceID == "" {
		return nil, errors.New("id cannot be empty")
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
		return nil, errors.New("userID cannot be empty")
	}
	if name == "" {
		return nil, errors.New("name cannot be empty")
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
		return nil, errors.New("userID cannot be empty")
	}
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}
	if shareLinkID == "" {
		return nil, errors.New("shareLinkID cannot be empty")
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
		return nil, errors.New("userID cannot be empty")
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
		return errors.New("instance cannot be empty")
	}
	if instance.ID == "" {
		return errors.New("instance.ID cannot be empty")
	}
	if instance.Name == "" {
		return errors.New("instance.Name cannot be empty")
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
		return "", errors.New("userID cannot be empty")
	}
	if data.TemplateID == "" {
		return "", errors.New("data.InstanceID cannot be empty")
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
		return "", errors.New("data.Validity cannot be empty")
	}

	err = s.shareLinkRepo.Create(ctx, shareLink)
	if err != nil {
		return "", fmt.Errorf("creating share link: %w", err)
	}

	url := helpers.URL("/templates/" + shareLink.ID)

	return url, nil
}
