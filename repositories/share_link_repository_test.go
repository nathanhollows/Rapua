package repositories_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v7/models"
	"github.com/nathanhollows/Rapua/v7/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func setupShareLinkRepo(t *testing.T) (repositories.ShareLinkRepository, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	shareLinkRepository := repositories.NewShareLinkRepository(dbc)

	return shareLinkRepository, dbc, cleanup
}

func TestShareLinkRepository_Create(t *testing.T) {
	repo, dbc, cleanup := setupShareLinkRepo(t)
	defer cleanup()

	parents := createTestParents(t, dbc)

	tests := []struct {
		name      string
		link      *models.ShareLink
		expectErr bool
	}{
		{
			name: "Valid ShareLink",
			link: &models.ShareLink{
				TemplateID: parents.InstanceID,
				ExpiresAt:  bun.NullTime{Time: time.Now().Add(time.Hour)},
			},
			expectErr: false,
		},
		{
			name:      "Nil ShareLink",
			link:      &models.ShareLink{},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Create(context.Background(), tt.link)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.NotEmpty(t, tt.link.ID) // Ensure ID was generated
			}
		})
	}
}

func TestShareLinkRepository_GetByID(t *testing.T) {
	repo, dbc, cleanup := setupShareLinkRepo(t)
	defer cleanup()

	parents := createTestParents(t, dbc)

	tests := []struct {
		name      string
		setup     func() *models.ShareLink
		action    func(*models.ShareLink) error
		expectErr bool
	}{
		{
			name: "Valid ShareLink",
			setup: func() *models.ShareLink {
				link := &models.ShareLink{
					TemplateID: parents.InstanceID,
					ExpiresAt:  bun.NullTime{Time: time.Now().Add(time.Hour)},
				}
				err := repo.Create(context.Background(), link)
				require.NoError(t, err)
				return link
			},
			action: func(link *models.ShareLink) error {
				_, err := repo.GetByID(context.Background(), link.ID)
				return err
			},
			expectErr: false,
		},
		{
			name: "Invalid ShareLink",
			setup: func() *models.ShareLink {
				return &models.ShareLink{ID: "nonexistent-id"}
			},
			action: func(link *models.ShareLink) error {
				_, err := repo.GetByID(context.Background(), link.ID)
				return err
			},
			expectErr: true,
		},
		{
			name: "Expired ShareLink",
			setup: func() *models.ShareLink {
				link := &models.ShareLink{
					TemplateID: parents.InstanceID,
					ExpiresAt:  bun.NullTime{Time: time.Now().Add(-time.Hour)},
				}
				err := repo.Create(context.Background(), link)
				require.NoError(t, err)
				return link
			},
			action: func(link *models.ShareLink) error {
				_, err := repo.GetByID(context.Background(), link.ID)
				return err
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link := tt.setup()
			err := tt.action(link)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestShareLinkRepository_Use(t *testing.T) {
	repo, dbc, cleanup := setupShareLinkRepo(t)
	defer cleanup()

	parents := createTestParents(t, dbc)

	tests := []struct {
		name      string
		setup     func() *models.ShareLink
		action    func(*models.ShareLink) error
		expectErr bool
	}{
		{
			name: "Use once",
			setup: func() *models.ShareLink {
				link := &models.ShareLink{
					TemplateID: parents.InstanceID,
					ExpiresAt:  bun.NullTime{Time: time.Now().Add(time.Hour)},
				}
				err := repo.Create(context.Background(), link)
				require.NoError(t, err)
				return link
			},
			action: func(link *models.ShareLink) error {
				err := repo.Use(context.Background(), link)
				if err != nil {
					return err
				}

				if link.UsedCount == 1 {
					return fmt.Errorf("used count not incremented: %d", link.UsedCount)
				}

				return nil
			},
			expectErr: false,
		},
		{
			name: "Expired link",
			setup: func() *models.ShareLink {
				link := &models.ShareLink{
					TemplateID: parents.InstanceID,
					ExpiresAt:  bun.NullTime{Time: time.Now().Add(-time.Hour)},
				}
				err := repo.Create(context.Background(), link)
				require.NoError(t, err)
				return link
			},
			action: func(link *models.ShareLink) error {
				err := repo.Use(context.Background(), link)
				return err
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			link := tt.setup()
			if err := tt.action(link); err != nil {
				require.Error(t, err)
			}
			err := tt.action(link)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
