package models

import (
	"time"

	"github.com/uptrace/bun"
)

// ShareLink represents a link that can be shared to access a template.
type ShareLink struct {
	bun.BaseModel `bun:"table:share_links"`

	ID              string       `bun:"id,pk,type:varchar(36)"`
	TemplateID      string       `bun:"template_id,type:varchar(36),notnull"`
	UserID          string       `bun:"user_id,type:varchar(36)"` // Owner of the link
	CreatedAt       time.Time    `bun:"created_at,nullzero"`
	ExpiresAt       bun.NullTime `bun:"expires_at,nullzero"`
	MaxUses         int          `bun:"max_uses,type:int,default:0"` // 0 means unlimited
	UsedCount       int          `bun:"used_count,type:int,default:0"`
	RegenerateCodes bool         `bun:"regenerate_codes,type:bool"`

	Template *Instance `bun:"rel:belongs-to,join:template_id=id"`
}

// IsExpired checks if the share link is expired.
func (s *ShareLink) IsExpired() bool {
	return (s.UsedCount >= s.MaxUses && s.MaxUses > 0) ||
		(s.ExpiresAt.Time.IsZero() == false &&
			s.ExpiresAt.Time.Before(time.Now()))
}
