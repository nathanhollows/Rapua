package main

import (
	"log/slog"
	"strings"

	"github.com/urfave/cli/v2"
	"github.com/uptrace/bun/migrate"
)

//nolint:gocognit // CLI complexity acceptable
func newDBCommand(migrator *migrate.Migrator, logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "db",
		Usage: "database migrations",
		Subcommands: []*cli.Command{
			{
				Name:  "init",
				Usage: "create migration tables",
				Action: func(c *cli.Context) error {
					return migrator.Init(c.Context)
				},
			},
			{
				Name:  "migrate",
				Usage: "apply database migrations",
				Action: func(c *cli.Context) error {
					if err := migrator.Lock(c.Context); err != nil {
						return err
					}

					defer func() {
						if err := migrator.Unlock(c.Context); err != nil {
							logger.Error("could not unlock", "error", err)
						}
					}()

					group, err := migrator.Migrate(c.Context)
					if err != nil {
						return err
					}
					if group.IsZero() {
						logger.Info("database is up-to-date")
					} else {
						logger.Info("migrated", "group", group)
					}
					return nil
				},
			},
			{
				Name:  "rollback",
				Usage: "rollback the last migration group",
				Action: func(c *cli.Context) error {
					if err := migrator.Lock(c.Context); err != nil {
						return err
					}

					defer func() {
						if err := migrator.Unlock(c.Context); err != nil {
							logger.Error("could not unlock", "error", err)
						}
					}()

					group, err := migrator.Rollback(c.Context)
					if err != nil {
						return err
					}
					if group.IsZero() {
						logger.Info("no migrations to rollback")
					} else {
						logger.Info("rolled back", "group", group)
					}
					return nil
				},
			},
			{
				Name:  "status",
				Usage: "print migrations status",
				Action: func(c *cli.Context) error {
					ms, err := migrator.MigrationsWithStatus(c.Context)
					if err != nil {
						return err
					}
					logger.Info("migration status",
						"migrations", ms,
						"unapplied", ms.Unapplied(),
						"last_group", ms.LastGroup())
					return nil
				},
			},
			{
				Name:  "create_go",
				Usage: "create Go migration",
				Action: func(c *cli.Context) error {
					name := strings.Join(c.Args().Slice(), "_")
					mf, err := migrator.CreateGoMigration(c.Context, name)
					if err != nil {
						return err
					}
					logger.Info("created migration", "name", mf.Name, "path", mf.Path)
					return nil
				},
			},
		},
	}
}
