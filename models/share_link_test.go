package models_test

import (
	"testing"
	"time"

	"github.com/nathanhollows/Rapua/v5/models"
	"github.com/uptrace/bun"
)

func TestShareLink_IsExpired(t *testing.T) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	tests := []struct {
		name        string
		shareLink   models.ShareLink
		wantExpired bool
	}{
		{
			name: "not expired - no expiration date, unlimited uses",
			shareLink: models.ShareLink{
				ExpiresAt: bun.NullTime{},
				MaxUses:   0,
				UsedCount: 5,
			},
			wantExpired: false,
		},
		{
			name: "not expired - future expiration date, unlimited uses",
			shareLink: models.ShareLink{
				ExpiresAt: bun.NullTime{Time: tomorrow},
				MaxUses:   0,
				UsedCount: 10,
			},
			wantExpired: false,
		},
		{
			name: "not expired - no expiration date, usage limit not reached",
			shareLink: models.ShareLink{
				ExpiresAt: bun.NullTime{},
				MaxUses:   10,
				UsedCount: 5,
			},
			wantExpired: false,
		},
		{
			name: "not expired - future expiration date, usage limit not reached",
			shareLink: models.ShareLink{
				ExpiresAt: bun.NullTime{Time: tomorrow},
				MaxUses:   10,
				UsedCount: 9,
			},
			wantExpired: false,
		},
		{
			name: "expired - past expiration date, unlimited uses",
			shareLink: models.ShareLink{
				ExpiresAt: bun.NullTime{Time: yesterday},
				MaxUses:   0,
				UsedCount: 5,
			},
			wantExpired: true,
		},
		{
			name: "expired - no expiration date, usage limit reached",
			shareLink: models.ShareLink{
				ExpiresAt: bun.NullTime{},
				MaxUses:   10,
				UsedCount: 10,
			},
			wantExpired: true,
		},
		{
			name: "expired - no expiration date, usage limit exceeded",
			shareLink: models.ShareLink{
				ExpiresAt: bun.NullTime{},
				MaxUses:   10,
				UsedCount: 11,
			},
			wantExpired: true,
		},
		{
			name: "expired - past expiration date, usage limit not reached",
			shareLink: models.ShareLink{
				ExpiresAt: bun.NullTime{Time: yesterday},
				MaxUses:   10,
				UsedCount: 5,
			},
			wantExpired: true,
		},
		{
			name: "expired - future expiration date, usage limit reached",
			shareLink: models.ShareLink{
				ExpiresAt: bun.NullTime{Time: tomorrow},
				MaxUses:   10,
				UsedCount: 10,
			},
			wantExpired: true,
		},
		{
			name: "zero value time with valid=true should not be expired",
			shareLink: models.ShareLink{
				ExpiresAt: bun.NullTime{Time: time.Time{}},
				MaxUses:   0,
				UsedCount: 0,
			},
			wantExpired: false,
		},
		{
			name: "edge case - max uses is 1 and used count is 1",
			shareLink: models.ShareLink{
				ExpiresAt: bun.NullTime{},
				MaxUses:   1,
				UsedCount: 1,
			},
			wantExpired: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotExpired := tt.shareLink.IsExpired()
			if gotExpired != tt.wantExpired {
				t.Errorf("ShareLink.IsExpired() = %v, want %v", gotExpired, tt.wantExpired)
			}
		})
	}
}
