package models

import (
	"database/sql"
)

type User struct {
	baseModel

	ID               string       `bun:"id,unique,pk,type:varchar(36)"`
	Name             string       `bun:"name,type:varchar(255)"`
	Email            string       `bun:"email,unique,pk"`
	EmailVerified    bool         `bun:"email_verified,type:boolean"`
	EmailToken       string       `bun:"email_token,type:varchar(36)"`
	EmailTokenExpiry sql.NullTime `bun:"email_token_expiry,nullzero"`
	Password         string       `bun:"password,type:varchar(255)"`
	Provider         string       `bun:"provider,type:varchar(255)"`

	Instances         []Instance `bun:"rel:has-many,join:id=user_id"`
	CurrentInstanceID string     `bun:"current_instance_id,type:varchar(36)"`
	CurrentInstance   Instance   `bun:"rel:has-one,join:current_instance_id=id"`
}
