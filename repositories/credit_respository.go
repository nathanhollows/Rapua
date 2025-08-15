package repositories

import (
	"github.com/uptrace/bun"
)

type CreditRepository struct {
	db *bun.DB
}

func NewCreditRepository(db *bun.DB) *CreditRepository {
	return &CreditRepository{
		db: db,
	}
}
