package models

import (
	"time"
)

type FacilitatorToken struct {
	Token      string    `bun:"token,pk"`
	QuestID    string    `bun:"quest_id,notnull"`
	Objectives StrArray  `bun:"objectives,type:text"`
	ExpiresAt  time.Time `bun:"expires_at,type:datetime"`
}
