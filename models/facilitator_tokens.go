package models

import (
	"time"
)

type FacilitatorToken struct {
	Token     string    `bun:"token,pk"`
	QuestID   string    `bun:"quest_id,notnull"`
	Locations StrArray  `bun:"locations,type:text"`
	ExpiresAt time.Time `bun:"expires_at,type:datetime"`
}
