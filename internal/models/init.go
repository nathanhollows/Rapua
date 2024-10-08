package models

import (
	"context"
	"log"
	"log/slog"
	"time"

	"github.com/nathanhollows/Rapua/pkg/db"
)

func CreateTables(logger *slog.Logger) {
	var models = []interface{}{
		(*Notification)(nil),
		(*InstanceSettings)(nil),
		(*Location)(nil),
		(*Clue)(nil),
		(*LocationContent)(nil),
		(*Team)(nil),
		(*Marker)(nil),
		(*Scan)(nil),
		(*Instance)(nil),
		(*User)(nil),
	}

	for _, model := range models {
		_, err := db.DB.NewCreateTable().Model(model).IfNotExists().Exec(context.Background())
		if err != nil {
			log.Fatal(err)
		}
	}
}

type baseModel struct {
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"updated_at"`
	DeletedAt time.Time `bun:",soft_delete,nullzero" json:"-"`
}
