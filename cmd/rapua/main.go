//go:generate npm run build

package main

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/nathanhollows/Rapua/db"
	"github.com/nathanhollows/Rapua/internal/migrations"
	"github.com/nathanhollows/Rapua/internal/server"
	"github.com/nathanhollows/Rapua/internal/services"
	"github.com/nathanhollows/Rapua/internal/sessions"
	"github.com/nathanhollows/Rapua/repositories"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"github.com/urfave/cli/v2"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	// Load environment variables
	if err := godotenv.Load(".env"); err != nil {
		logger.Warn("could not load .env file", "error", err)
	}

	db := db.MustOpen()
	defer db.Close()

	// Initialize the migrator
	migrator := migrate.NewMigrator(db, migrations.Migrations)

	// Define CLI app for migrations
	app := &cli.App{
		Name:        "Rapua",
		Usage:       "rapua [global options] command [command options] [arguments...]",
		Description: `An open-source platform for location-based games.`,
		Version:     "3.2.0",
		Commands: []*cli.Command{
			newDBCommand(migrator),
		},
		Action: func(c *cli.Context) error {
			// Default action: run the app
			runApp(logger, db)
			return nil
		},
	}

	// Run CLI or app
	if err := app.Run(os.Args); err != nil {
		logger.Error("application error", "error", err)
		os.Exit(1)
	}
}

func newDBCommand(migrator *migrate.Migrator) *cli.Command {
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
					defer migrator.Unlock(c.Context)

					group, err := migrator.Migrate(c.Context)
					if err != nil {
						return err
					}
					if group.IsZero() {
						fmt.Println("database is up-to-date")
					} else {
						fmt.Printf("migrated to %s\n", group)
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
					defer migrator.Unlock(c.Context)

					group, err := migrator.Rollback(c.Context)
					if err != nil {
						return err
					}
					if group.IsZero() {
						fmt.Println("no migrations to rollback")
					} else {
						fmt.Printf("rolled back %s\n", group)
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
					fmt.Printf("migrations: %s\n", ms)
					fmt.Printf("unapplied migrations: %s\n", ms.Unapplied())
					fmt.Printf("last migration group: %s\n", ms.LastGroup())
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
					fmt.Printf("created migration %s (%s)\n", mf.Name, mf.Path)
					return nil
				},
			},
		},
	}
}

func runApp(logger *slog.Logger, dbc *bun.DB) {
	initialiseFolders(logger)

	// Initialize repositories
	facilitatorRepo := repositories.NewFacilitatorTokenRepo(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	teamRepo := repositories.NewTeamRepository(dbc)
	checkInRepo := repositories.NewCheckInRepository(dbc)
	clueRepo := repositories.NewClueRepository(dbc)
	instanceSettingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	notificationRepo := repositories.NewNotificationRepository(dbc)
	instanceRepo := repositories.NewInstanceRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)

	// Initialize transactor for services
	transactor := db.NewTransactor(dbc)

	// Initialize services
	facilitatorService := services.NewFacilitatorService(facilitatorRepo)
	assetGenerator := services.NewAssetGenerator()
	authService := services.NewAuthService(userRepo)
	blockService := services.NewBlockService(transactor, blockRepo, blockStateRepo)
	checkInService := services.NewCheckInService(checkInRepo, locationRepo, teamRepo)
	clueService := services.NewClueService(clueRepo, locationRepo)
	emailService := services.NewEmailService()
	locationService := services.NewLocationService(transactor, clueRepo, locationRepo, markerRepo, blockRepo)
	navigationService := services.NewNavigationService()
	notificationService := services.NewNotificationService(notificationRepo, teamRepo)
	teamService := services.NewTeamService(transactor, teamRepo, checkInRepo, blockStateRepo, locationRepo)
	userService := services.NewUserService(transactor, userRepo, instanceRepo)
	gameplayService := services.NewGameplayService(
		checkInService, locationService, teamService, blockService, navigationService, markerRepo,
	)
	gameManagerService := services.NewGameManagerService(
		transactor,
		locationService, userService, teamService,
		markerRepo, clueRepo, instanceRepo, instanceSettingsRepo,
	)

	sessions.Start()
	server.Start(
		logger,
		assetGenerator,
		authService,
		blockService,
		checkInService,
		clueService,
		emailService,
		gameManagerService,
		gameplayService,
		locationService,
		navigationService,
		notificationService,
		teamService,
		userService,
		facilitatorService,
	)
}

func initialiseFolders(logger *slog.Logger) {
	folders := []string{
		"assets/", "assets/codes/", "assets/codes/png/", "assets/codes/svg/",
		"assets/fonts/", "assets/posters/",
	}

	for _, folder := range folders {
		if _, err := os.Stat(folder); err != nil {
			if err = os.MkdirAll(folder, 0755); err != nil {
				logger.Error("could not create directory", "folder", folder, "error", err)
				os.Exit(1)
			}
		}
	}
}
