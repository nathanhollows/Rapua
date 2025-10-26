package services

import (
	"context"
	"errors"
	"time"

	"github.com/nathanhollows/Rapua/v4/helpers"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/uptrace/bun"
)

type TemplateService struct {
	duplicationService   *DuplicationService
	instanceRepo         repositories.InstanceRepository
	instanceSettingsRepo repositories.InstanceSettingsRepository
	shareLinkRepo        repositories.ShareLinkRepository
}

func NewTemplateService(
	duplicationService *DuplicationService,
	instanceRepo repositories.InstanceRepository,
	instanceSettingsRepo repositories.InstanceSettingsRepository,
	shareLinkRepo repositories.ShareLinkRepository,
) TemplateService {
	return TemplateService{
		duplicationService:   duplicationService,
		instanceRepo:         instanceRepo,
		instanceSettingsRepo: instanceSettingsRepo,
		shareLinkRepo:        shareLinkRepo,
	}
}

// CreateFromInstance creates a new template from an existing instance.
func (s *TemplateService) CreateFromInstance(
	ctx context.Context,
	userID, instanceID, name string,
) (*models.Instance, error) {
	if userID == "" {
		return nil, errors.New("userID cannot be empty")
	}

	if instanceID == "" {
		return nil, errors.New("instanceID cannot be empty")
	}
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	// Get user for duplication service
	user := &models.User{ID: userID}

	// Use duplication service to create template
	newTemplate, err := s.duplicationService.CreateTemplateFromInstance(ctx, user, instanceID, name)
	if err != nil {
		return nil, errors.New("creating template from instance: " + err.Error())
	}

	return newTemplate, nil
}

// LaunchInstance creates a new instance from a template.
func (s *TemplateService) LaunchInstance(
	ctx context.Context,
	userID, templateID, name string,
	_ bool, // TODO: regen marker IDs. This prevents sharing IDs.
) (*models.Instance, error) {
	if userID == "" {
		return nil, errors.New("userID cannot be empty")
	}
	if name == "" {
		return nil, errors.New("name cannot be empty")
	}

	// Get user for duplication service
	user := &models.User{ID: userID}

	// Use duplication service - it validates template ownership and status
	newInstance, err := s.duplicationService.CreateInstanceFromTemplate(ctx, user, templateID, name)
	if err != nil {
		return nil, errors.New("creating instance from template: " + err.Error())
	}

	return newInstance, nil
}

// LaunchInstanceFromShareLink creates a new instance from a share link.
func (s *TemplateService) LaunchInstanceFromShareLink(
	ctx context.Context,
	userID, shareLinkID string,
	name string,
	_ bool, // TODO: regen marker IDs. This prevents sharing IDs.
) (*models.Instance, error) {
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
		return nil, errors.New("finding share link: " + err.Error())
	}

	// Thankfully this checks the expiration date and max uses
	if shareLink.IsExpired() {
		return nil, errors.New("share link is expired")
	}

	if shareLink.UserID != userID {
		return nil, ErrPermissionDenied
	}

	// Get template owner for duplication (share links use template owner's ID)
	template, err := s.instanceRepo.GetByID(ctx, shareLink.TemplateID)
	if err != nil {
		return nil, errors.New("finding template: " + err.Error())
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

	// Use duplication service with shared template method (bypasses ownership check)
	user := &models.User{ID: userID}
	newInstance, err := s.duplicationService.CreateInstanceFromSharedTemplate(ctx, user, shareLink.TemplateID, name)
	if err != nil {
		return nil, errors.New("creating instance from shared template: " + err.Error())
	}
	// Increment the used count
	shareLink.UsedCount++
	if shareLink.MaxUses > 0 && shareLink.UsedCount >= shareLink.MaxUses {
		shareLink.ExpiresAt = bun.NullTime{Time: time.Now()}
	}
	if err = s.shareLinkRepo.Use(ctx, shareLink); err != nil {
		return nil, errors.New("updating share link: " + err.Error())
	}

	return newInstance, nil
}

// GetByID retrieves a template by ID.
func (s *TemplateService) GetByID(ctx context.Context, id string) (*models.Instance, error) {
	instance, err := s.instanceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.New("finding instance: " + err.Error())
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
		return nil, errors.New("finding share link: " + err.Error())
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
		return nil, errors.New("finding instances: " + err.Error())
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
		return errors.New("updating instance: " + err.Error())
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
		return "", errors.New("finding instance: " + err.Error())
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
	case always:
		shareLink.ExpiresAt = bun.NullTime{}
	case day:
		shareLink.ExpiresAt = bun.NullTime{Time: time.Now().AddDate(0, 0, 1)}
	case week:
		shareLink.ExpiresAt = bun.NullTime{Time: time.Now().AddDate(0, 0, daysInWeek)}
	case month:
		shareLink.ExpiresAt = bun.NullTime{Time: time.Now().AddDate(0, 1, 0)}
	default:
		return "", errors.New("data.Validity cannot be empty")
	}

	err = s.shareLinkRepo.Create(ctx, shareLink)
	if err != nil {
		return "", errors.New("creating share link: " + err.Error())
	}

	url := helpers.URL("/templates/" + shareLink.ID)

	return url, nil
}
