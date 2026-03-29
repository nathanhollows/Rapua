package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"

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

// sqliteDSN converts a plain SQLite path or URI into a URI with
// _pragma=foreign_keys(1) appended so every new connection enforces FK
// constraints without needing SetMaxOpenConns(1).
func sqliteDSN(dsn string) string {
	if !strings.HasPrefix(dsn, "file:") {
		dsn = "file:" + dsn
	}
	sep := "&"
	if !strings.Contains(dsn, "?") {
		sep = "?"
	}
	return dsn + sep + "_pragma=foreign_keys(1)"
}

func MustOpen(logger *slog.Logger) *bun.DB {
	var sqldb *sql.DB
	var err error
	var db *bun.DB

	dataSourceName := os.Getenv("DB_CONNECTION")
	if dataSourceName == "" {
		logger.Error("DB_CONNECTION not set. Please set DB_CONNECTION in the environment")
		os.Exit(1)
	}

	driverName := os.Getenv("DB_TYPE")
	switch driverName {
	case "mysql":
		sqldb, err = sql.Open(driverName, dataSourceName)
		db = bun.NewDB(sqldb, mysqldialect.New())
	case "sqlite3":
		// _pragma=foreign_keys(1) is applied by modernc.org/sqlite on every new
		// connection, so FK enforcement holds regardless of connection pool size.
		// WAL is set once via exec; it persists in the DB file across connections.
		sqldb, err = sql.Open(sqliteshim.ShimName, sqliteDSN(dataSourceName))
		db = bun.NewDB(sqldb, sqlitedialect.New())
		_, pragmaErr := sqldb.ExecContext(context.Background(), "PRAGMA journal_mode=WAL;")
		if pragmaErr != nil {
			logger.Error("set PRAGMA journal_mode=WAL", slog.String("error", pragmaErr.Error()))
			os.Exit(1)
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
