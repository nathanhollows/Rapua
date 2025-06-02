package repositories_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v3/db"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/uptrace/bun"
)

func setupShareLinkRepo(t *testing.T) (repositories.ShareLinkRepository, db.Transactor, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)

	shareLinkRepository := repositories.NewShareLinkRepository(dbc)

	return shareLinkRepository, transactor, cleanup
}

func TestShareLinkRepository_Create(t *testing.T) {
	repo, _, cleanup := setupShareLinkRepo(t)
	defer cleanup()

	tests := []struct {
		name      string
		link      *models.ShareLink
		expectErr bool
	}{
		{
			name: "Valid ShareLink",
			link: &models.ShareLink{
				TemplateID: uuid.New().String(),
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
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, tt.link.ID) // Ensure ID was generated
			}
		})
	}
}

func TestShareLinkRepository_GetByID(t *testing.T) {
	repo, _, cleanup := setupShareLinkRepo(t)
	defer cleanup()

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
					TemplateID: uuid.New().String(),
					ExpiresAt:  bun.NullTime{Time: time.Now().Add(time.Hour)},
				}
				err := repo.Create(context.Background(), link)
				assert.NoError(t, err)
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
				return &models.ShareLink{ID: uuid.New().String()}
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
					TemplateID: uuid.New().String(),
					ExpiresAt:  bun.NullTime{Time: time.Now().Add(-time.Hour)},
				}
				err := repo.Create(context.Background(), link)
				assert.NoError(t, err)
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
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestShareLinkRepository_Use(t *testing.T) {
	repo, _, cleanup := setupShareLinkRepo(t)
	defer cleanup()

	tests := []struct {
		name      string
		setup     func() *models.ShareLink
		action    func(*models.ShareLink) error
		expectErr bool
	}{
		{
			name: "Use once",
			setup: func() *models.ShareLink {
				link := &models.ShareLink{TemplateID: uuid.New().String(), ExpiresAt: bun.NullTime{Time: time.Now().Add(time.Hour)}}
				err := repo.Create(context.Background(), link)
				assert.NoError(t, err)
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
				link := &models.ShareLink{TemplateID: uuid.New().String(), ExpiresAt: bun.NullTime{Time: time.Now().Add(-time.Hour)}}
				err := repo.Create(context.Background(), link)
				assert.NoError(t, err)
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
				assert.Error(t, err)
			}
			err := tt.action(link)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
