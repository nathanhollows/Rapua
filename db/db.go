package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/mysqldialect"
	"github.com/uptrace/bun/dialect/sqlitedialect"
	"github.com/uptrace/bun/driver/sqliteshim"
	"github.com/uptrace/bun/extra/bundebug"
)

type transactor struct {
	db *bun.DB
}

type Transactor interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*bun.Tx, error)
}

func NewTransactor(db *bun.DB) Transactor {
	return &transactor{
		db: db,
	}
}

func (t *transactor) BeginTx(ctx context.Context, opts *sql.TxOptions) (*bun.Tx, error) {
	tx, err := t.db.BeginTx(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("beginning transaction: %w", err)
	}
	return &tx, nil
}

func MustOpen() *bun.DB {
	var sqldb *sql.DB
	var err error
	var db *bun.DB

	dataSourceName := os.Getenv("DB_CONNECTION")
	if dataSourceName == "" {
		log.Fatal("DB_CONNECTION not set. Please set DB_CONNECTION in the environment")
	}

	driverName := os.Getenv("DB_TYPE")
	switch driverName {
	case "mysql":
		sqldb, err = sql.Open(driverName, dataSourceName)
		db = bun.NewDB(sqldb, mysqldialect.New())
	case "sqlite3":
		sqldb, err = sql.Open(sqliteshim.ShimName, dataSourceName)
		db = bun.NewDB(sqldb, sqlitedialect.New())
		_, err := sqldb.Exec("PRAGMA journal_mode=WAL;")
		if err != nil {
			log.Fatal(err)
		}
	default:
		panic("unsupported DB_TYPE: " + driverName + ". Supported types are mysql and sqlite3")
	}

	if err != nil {
		panic(err)
	}

	debugEnabled := os.Getenv("BUNDEBUG") == "1" || os.Getenv("BUNDEBUG") == "2"
	db.AddQueryHook(bundebug.NewQueryHook(
		// disable the hook
		bundebug.WithEnabled(debugEnabled),

		// BUNDEBUG=1 logs failed queries
		// BUNDEBUG=2 logs all queries
		bundebug.FromEnv("BUNDEBUG"),
	))

	return db
}
