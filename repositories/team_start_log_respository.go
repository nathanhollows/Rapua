package repositories

import (
	"github.com/uptrace/bun"
)

type TeamStartLogRepository struct {
	db *bun.DB
}

func NewTeamStartLogRepository(db *bun.DB) *TeamStartLogRepository {
	return &TeamStartLogRepository{
		db: db,
	}
}
