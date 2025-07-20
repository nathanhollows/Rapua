package services_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/models"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

func setupTemplateService(t *testing.T) (services.TemplateService, services.InstanceService, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	// Initialize repositories
	clueRepo := repositories.NewClueRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	instanceRepo := repositories.NewInstanceRepository(dbc)
	instanceSettingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	shareLinkRepo := repositories.NewShareLinkRepository(dbc)
	checkInRepo := repositories.NewCheckInRepository(dbc)
	teamRepo := repositories.NewTeamRepository(dbc)

	// Initialize services
	markerService := services.NewMarkerService(markerRepo)
	locationService := services.NewLocationService(clueRepo, locationRepo, markerRepo, blockRepo, markerService)
	teamService := services.NewTeamService(teamRepo, checkInRepo, blockStateRepo, locationRepo)
	instanceService := services.NewInstanceService(
		locationService, *teamService, instanceRepo, instanceSettingsRepo,
	)

	templateService := services.NewTemplateService(
		locationService,
		instanceRepo,
		instanceSettingsRepo,
		shareLinkRepo,
	)
	return templateService, instanceService, cleanup
}

func TestTemplateService_CreateFromInstance(t *testing.T) {
	svc, instanceService, cleanup := setupTemplateService(t)
	defer cleanup()

	user := &models.User{ID: "user123", Password: "password", CurrentInstanceID: "instance123"}
	instance, err := instanceService.CreateInstance(context.Background(), "Game1", user)
	assert.NoError(t, err)
	assert.NotEmpty(t, user.ID)

	t.Run("CreateTemplate", func(t *testing.T) {
		tests := []struct {
			name         string
			templateName string
			instanceID   string
			userID       string
			wantErr      bool
		}{
			{"Valid Template", "Template1", instance.ID, user.ID, false},
			{"Empty Template Name", "", instance.ID, user.ID, true},
			{"Invalid Instance ID", "Template1", "invalid", user.ID, true},
			{"Invalid User ID", "Template1", instance.ID, "invalid", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := svc.CreateFromInstance(context.Background(), tt.userID, tt.instanceID, tt.templateName)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestTemplateService_LaunchInstance(t *testing.T) {
	svc, instanceService, cleanup := setupTemplateService(t)
	defer cleanup()

	user := &models.User{ID: "user123", Password: "password", CurrentInstanceID: "instance123"}
	instance, err := instanceService.CreateInstance(context.Background(), "Game1", user)
	assert.NoError(t, err)
	assert.NotEmpty(t, user.ID)

	template, err := svc.CreateFromInstance(context.Background(), user.ID, instance.ID, "Template1")
	assert.NoError(t, err)

	t.Run("LaunchInstance", func(t *testing.T) {
		tests := []struct {
			name         string
			templateID   string
			instanceName string
			userID       string
			wantErr      bool
		}{
			{"Valid Template", template.ID, "Game2", user.ID, false},
			{"Empty Template ID", "", "Game2", user.ID, true},
			{"Empty Instance Name", template.ID, "", user.ID, true},
			{"Invalid Template ID", "invalid", "Game2", user.ID, true},
			{"Invalid User ID", template.ID, "Game2", "invalid", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := svc.LaunchInstance(context.Background(), tt.userID, tt.templateID, tt.instanceName, false)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestTemplateService_LaunchInstanceFromShareLink(t *testing.T) {
	svc, instanceService, cleanup := setupTemplateService(t)
	defer cleanup()

	user := &models.User{ID: "user123", Password: "password", CurrentInstanceID: "instance123"}
	instance, err := instanceService.CreateInstance(context.Background(), "Game1", user)
	assert.NoError(t, err)

	template, err := svc.CreateFromInstance(context.Background(), user.ID, instance.ID, "Template1")
	assert.NoError(t, err)

	// Create a share link for the template
	linkData := services.ShareLinkData{
		TemplateID: template.ID,
		Validity:   "month",
		MaxUses:    5,
		Regenerate: false,
	}
	shareLinkURL, err := svc.CreateShareLink(context.Background(), user.ID, linkData)
	assert.NoError(t, err)

	// Extract share link ID from URL
	shareLinkID := extractShareLinkIDFromURL(shareLinkURL)
	assert.NotEmpty(t, shareLinkID)

	t.Run("LaunchInstanceFromShareLink", func(t *testing.T) {
		tests := []struct {
			name         string
			userID       string
			shareLinkID  string
			instanceName string
			regen        bool
			wantErr      bool
		}{
			{"Valid ShareLink", user.ID, shareLinkID, "Game3", false, false},
			{"Empty ShareLink ID", user.ID, "", "Game3", false, true},
			{"Empty Instance Name", user.ID, shareLinkID, "", false, true},
			{"Invalid ShareLink ID", user.ID, "invalid", "Game3", false, true},
			{"Invalid User ID", "invalid", shareLinkID, "Game3", false, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := svc.LaunchInstanceFromShareLink(context.Background(), tt.userID, tt.shareLinkID, tt.instanceName, tt.regen)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestTemplateService_GetByID(t *testing.T) {
	svc, instanceService, cleanup := setupTemplateService(t)
	defer cleanup()

	user := &models.User{ID: "user123", Password: "password", CurrentInstanceID: "instance123"}
	instance, err := instanceService.CreateInstance(context.Background(), "Game1", user)
	assert.NoError(t, err)

	template, err := svc.CreateFromInstance(context.Background(), user.ID, instance.ID, "Template1")
	assert.NoError(t, err)

	t.Run("GetByID", func(t *testing.T) {
		tests := []struct {
			name       string
			templateID string
			wantErr    bool
		}{
			{"Valid Template ID", template.ID, false},
			{"Empty Template ID", "", true},
			{"Invalid Template ID", "invalid", true},
			{"Non-Template Instance ID", instance.ID, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := svc.GetByID(context.Background(), tt.templateID)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, template.ID, got.ID)
					assert.True(t, got.IsTemplate)
				}
			})
		}
	})
}

func TestTemplateService_GetShareLink(t *testing.T) {
	svc, instanceService, cleanup := setupTemplateService(t)
	defer cleanup()

	user := &models.User{ID: "user123", Password: "password", CurrentInstanceID: "instance123"}
	instance, err := instanceService.CreateInstance(context.Background(), "Game1", user)
	assert.NoError(t, err)

	template, err := svc.CreateFromInstance(context.Background(), user.ID, instance.ID, "Template1")
	assert.NoError(t, err)

	// Create a share link for the template
	linkData := services.ShareLinkData{
		TemplateID: template.ID,
		Validity:   "month",
		MaxUses:    5,
		Regenerate: false,
	}
	shareLinkURL, err := svc.CreateShareLink(context.Background(), user.ID, linkData)
	assert.NoError(t, err)

	// Extract share link ID from URL
	shareLinkID := extractShareLinkIDFromURL(shareLinkURL)
	assert.NotEmpty(t, shareLinkID)

	t.Run("GetShareLink", func(t *testing.T) {
		tests := []struct {
			name        string
			shareLinkID string
			wantErr     bool
		}{
			{"Valid ShareLink ID", shareLinkID, false},
			{"Empty ShareLink ID", "", true},
			{"Invalid ShareLink ID", "invalid", true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := svc.GetShareLink(context.Background(), tt.shareLinkID)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, shareLinkID, got.ID)
					assert.Equal(t, template.ID, got.TemplateID)
					assert.Equal(t, user.ID, got.UserID)
				}
			})
		}
	})
}

func TestTemplateService_Find(t *testing.T) {
	svc, instanceService, cleanup := setupTemplateService(t)
	defer cleanup()

	user := &models.User{ID: "user123", Password: "password", CurrentInstanceID: "instance123"}
	anotherUser := &models.User{ID: "user456", Password: "password", CurrentInstanceID: "instance456"}

	instance, err := instanceService.CreateInstance(context.Background(), "Game1", user)
	assert.NoError(t, err)

	template1, err := svc.CreateFromInstance(context.Background(), user.ID, instance.ID, "Template1")
	assert.NoError(t, err)

	template2, err := svc.CreateFromInstance(context.Background(), user.ID, instance.ID, "Template2")
	assert.NoError(t, err)

	t.Run("Find", func(t *testing.T) {
		tests := []struct {
			name      string
			userID    string
			wantCount int
			wantErr   bool
		}{
			{"Valid User ID", user.ID, 2, false},
			{"Different User ID", anotherUser.ID, 0, false},
			{"Empty User ID", "", 0, true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				templates, err := svc.Find(context.Background(), tt.userID)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.Len(t, templates, tt.wantCount)
					if tt.wantCount > 0 {
						// Check that all returned instances are templates
						for _, tmpl := range templates {
							assert.True(t, tmpl.IsTemplate)
						}
						// Check that the templates we created are included
						templateIDs := make([]string, len(templates))
						for i, tmpl := range templates {
							templateIDs[i] = tmpl.ID
						}
						assert.Contains(t, templateIDs, template1.ID)
						assert.Contains(t, templateIDs, template2.ID)
					}
				}
			})
		}
	})
}

func TestTemplateService_Update(t *testing.T) {
	svc, instanceService, cleanup := setupTemplateService(t)
	defer cleanup()

	user := &models.User{ID: "user123", Password: "password", CurrentInstanceID: "instance123"}
	instance, err := instanceService.CreateInstance(context.Background(), "Game1", user)
	assert.NoError(t, err)

	template, err := svc.CreateFromInstance(context.Background(), user.ID, instance.ID, "Template1")
	assert.NoError(t, err)

	t.Run("Update", func(t *testing.T) {
		tests := []struct {
			name     string
			instance *models.Instance
			wantErr  bool
		}{
			{
				"Valid Update",
				&models.Instance{ID: template.ID, CreatedAt: time.Now(), UpdatedAt: time.Now(), Name: "Updated Template", UserID: user.ID, IsTemplate: true},
				false,
			},
			{
				"Empty Name",
				&models.Instance{ID: template.ID, Name: "", UserID: user.ID, IsTemplate: true},
				true,
			},
			{
				"Empty ID",
				&models.Instance{ID: "", Name: "Updated Template", UserID: user.ID, IsTemplate: true},
				true,
			},
			{
				"Nil Instance",
				nil,
				true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := svc.Update(context.Background(), tt.instance)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)

					// Verify the update
					updated, err := svc.GetByID(context.Background(), template.ID)
					assert.NoError(t, err)
					assert.Equal(t, "Updated Template", updated.Name)
				}
			})
		}
	})
}

func TestTemplateService_CreateShareLink(t *testing.T) {
	svc, instanceService, cleanup := setupTemplateService(t)
	defer cleanup()

	user := &models.User{ID: "user123", Password: "password", CurrentInstanceID: "instance123"}
	instance, err := instanceService.CreateInstance(context.Background(), "Game1", user)
	assert.NoError(t, err)

	template, err := svc.CreateFromInstance(context.Background(), user.ID, instance.ID, "Template1")
	assert.NoError(t, err)

	t.Run("CreateShareLink", func(t *testing.T) {
		tests := []struct {
			name    string
			userID  string
			data    services.ShareLinkData
			wantErr bool
		}{
			{
				"Valid ShareLink - Always",
				user.ID,
				services.ShareLinkData{TemplateID: template.ID, Validity: "always", MaxUses: 0, Regenerate: false},
				false,
			},
			{
				"Valid ShareLink - Day",
				user.ID,
				services.ShareLinkData{TemplateID: template.ID, Validity: "day", MaxUses: 10, Regenerate: true},
				false,
			},
			{
				"Valid ShareLink - Week",
				user.ID,
				services.ShareLinkData{TemplateID: template.ID, Validity: "week", MaxUses: 5, Regenerate: false},
				false,
			},
			{
				"Valid ShareLink - Month",
				user.ID,
				services.ShareLinkData{TemplateID: template.ID, Validity: "month", MaxUses: 1, Regenerate: true},
				false,
			},
			{
				"Empty User ID",
				"",
				services.ShareLinkData{TemplateID: template.ID, Validity: "always", MaxUses: 0, Regenerate: false},
				true,
			},
			{
				"Empty Template ID",
				user.ID,
				services.ShareLinkData{TemplateID: "", Validity: "always", MaxUses: 0, Regenerate: false},
				true,
			},
			{
				"Invalid Template ID",
				user.ID,
				services.ShareLinkData{TemplateID: "invalid", Validity: "always", MaxUses: 0, Regenerate: false},
				true,
			},
			{
				"Invalid Validity",
				user.ID,
				services.ShareLinkData{TemplateID: template.ID, Validity: "invalid", MaxUses: 0, Regenerate: false},
				true,
			},
			{
				"Different User ID",
				"user456",
				services.ShareLinkData{TemplateID: template.ID, Validity: "always", MaxUses: 0, Regenerate: false},
				true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				url, err := svc.CreateShareLink(context.Background(), tt.userID, tt.data)
				if tt.wantErr {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
					assert.NotEmpty(t, url)

					// Verify the URL contains "/templates/"
					assert.Contains(t, url, "/templates/")

					// Extract and verify the share link
					shareLinkID := extractShareLinkIDFromURL(url)
					shareLink, err := svc.GetShareLink(context.Background(), shareLinkID)
					assert.NoError(t, err)
					assert.Equal(t, tt.data.TemplateID, shareLink.TemplateID)
					assert.Equal(t, tt.userID, shareLink.UserID)
					assert.Equal(t, tt.data.MaxUses, shareLink.MaxUses)
					assert.Equal(t, tt.data.Regenerate, shareLink.RegenerateCodes)

					// Verify expiration date
					switch tt.data.Validity {
					case "always":
						assert.False(t, shareLink.ExpiresAt != bun.NullTime{})
					case "day":
						assert.True(t, shareLink.ExpiresAt != bun.NullTime{})
						// Allow 1 second tolerance for test execution time
						assert.WithinDuration(t, time.Now().AddDate(0, 0, 1), shareLink.ExpiresAt.Time, 1*time.Second)
					case "week":
						assert.True(t, shareLink.ExpiresAt != bun.NullTime{})
						assert.WithinDuration(t, time.Now().AddDate(0, 0, 7), shareLink.ExpiresAt.Time, 1*time.Second)
					case "month":
						assert.True(t, shareLink.ExpiresAt != bun.NullTime{})
						assert.WithinDuration(t, time.Now().AddDate(0, 1, 0), shareLink.ExpiresAt.Time, 1*time.Second)
					}
				}
			})
		}
	})
}

// Helper function to extract share link ID from URL
func extractShareLinkIDFromURL(url string) string {
	parts := strings.Split(url, "/templates/")
	if len(parts) != 2 {
		return ""
	}
	return parts[1]
}
