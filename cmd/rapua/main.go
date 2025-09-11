//go:generate npm run build

package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/nathanhollows/Rapua/v4/db"
	admin "github.com/nathanhollows/Rapua/v4/internal/handlers/admin"
	players "github.com/nathanhollows/Rapua/v4/internal/handlers/players"
	public "github.com/nathanhollows/Rapua/v4/internal/handlers/public"
	"github.com/nathanhollows/Rapua/v4/internal/migrations"
	"github.com/nathanhollows/Rapua/v4/internal/scheduler"
	"github.com/nathanhollows/Rapua/v4/internal/server"
	"github.com/nathanhollows/Rapua/v4/internal/services"
	"github.com/nathanhollows/Rapua/v4/internal/sessions"
	"github.com/nathanhollows/Rapua/v4/internal/storage"
	"github.com/nathanhollows/Rapua/v4/repositories"
	"github.com/phsym/console-slog"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"github.com/urfave/cli/v2"
)

const version = "v4.4.0"

func main() {
	logger := slog.New(
		console.NewHandler(os.Stdout, &console.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: true,
		}),
	)
	slog.SetDefault(logger)

	log.Printf("Rapua %s starting...\n", version)

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
		Version:     version,
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
		defer os.Exit(1)
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

					defer func() {
						if err := migrator.Unlock(c.Context); err != nil {
							log.Printf("could not unlock: %v", err)
						}
					}()

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

					defer func() {
						if err := migrator.Unlock(c.Context); err != nil {
							log.Printf("could not unlock: %v", err)
						}
					}()

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
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	checkInRepo := repositories.NewCheckInRepository(dbc)
	clueRepo := repositories.NewClueRepository(dbc)
	creditRepo := repositories.NewCreditRepository(dbc)
	facilitatorRepo := repositories.NewFacilitatorTokenRepo(dbc)
	instanceRepo := repositories.NewInstanceRepository(dbc)
	instanceSettingsRepo := repositories.NewInstanceSettingsRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	notificationRepo := repositories.NewNotificationRepository(dbc)
	shareLinkRepo := repositories.NewShareLinkRepository(dbc)
	teamRepo := repositories.NewTeamRepository(dbc)
	teamStartLogRepo := repositories.NewTeamStartLogRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)
	uploadRepo := repositories.NewUploadRepository(dbc)

	// Initialize transactor for services
	transactor := db.NewTransactor(dbc)

	// Storage for the upload service
	localStorage := storage.NewLocalStorage("static/uploads/")

	// Initialize services
	accessService := services.NewAccessService(
		blockRepo,
		instanceRepo,
		locationRepo,
		markerRepo,
	)
	locationStatsService := services.NewLocationStatsService(locationRepo)
	gameScheduleService := services.NewGameScheduleService(instanceRepo)
	quickstartService := services.NewQuickstartService(instanceRepo)
	markerService := services.NewMarkerService(markerRepo)
	uploadService := services.NewUploadService(uploadRepo, localStorage)
	deleteService := services.NewDeleteService(
		transactor,
		blockRepo,
		blockStateRepo,
		checkInRepo,
		clueRepo,
		instanceRepo,
		instanceSettingsRepo,
		locationRepo,
		markerRepo,
		teamRepo,
		userRepo,
	)
	facilitatorService := services.NewFacilitatorService(facilitatorRepo)
	assetGenerator := services.NewAssetGenerator()
	identityService := services.NewAuthService(userRepo)
	blockService := services.NewBlockService(blockRepo, blockStateRepo)
	clueService := services.NewClueService(clueRepo, locationRepo)
	emailService := services.NewEmailService()
	instanceSettingsService := services.NewInstanceSettingsService(instanceSettingsRepo)
	locationService := services.NewLocationService(clueRepo, locationRepo, markerRepo, blockRepo, markerService)
	navigationService := services.NewNavigationService(locationRepo, teamRepo)
	checkInService := services.NewCheckInService(checkInRepo, locationRepo, teamRepo, locationStatsService, navigationService, blockService)
	notificationService := services.NewNotificationService(notificationRepo, teamRepo)
	userService := services.NewUserService(userRepo, instanceRepo)
	monthlyCreditTopupJob := services.NewMonthlyCreditTopupService(transactor, creditRepo)
	creditService := services.NewCreditService(
		transactor,
		creditRepo,
		*teamStartLogRepo,
		userRepo,
	)
	teamService := services.NewTeamService(transactor, teamRepo, checkInRepo, creditService, blockStateRepo, locationRepo)
	leaderBoardService := services.NewLeaderBoardService(teamRepo)
	instanceService := services.NewInstanceService(
		locationService, *teamService, instanceRepo, instanceSettingsRepo,
	)
	templateService := services.NewTemplateService(
		locationService, instanceRepo, instanceSettingsRepo, shareLinkRepo,
	)

	sessions.Start()

	// Register jobs
	jobs := scheduler.NewScheduler(logger)
	jobs.AddJob(
		"Monthly Credit Top-Up",
		monthlyCreditTopupJob.TopUpCredits,
		scheduler.NextFirstOfMonth,
	)
	jobs.Start()

	// Construct handlers (dependency injection root)
	publicHandler := public.NewPublicHandler(
		logger,
		identityService,
		deleteService,
		emailService,
		&templateService,
		userService,
	)

	playerHandler := players.NewPlayerHandler(
		logger,
		blockService,
		checkInService,
		instanceSettingsService,
		markerService,
		navigationService,
		notificationService,
		teamService,
	)

	adminHandler := admin.NewAdminHandler(
		logger,
		accessService,
		assetGenerator,
		identityService,
		blockService,
		clueService,
		creditService,
		deleteService,
		facilitatorService,
		gameScheduleService,
		instanceService,
		instanceSettingsService,
		locationService,
		markerService,
		navigationService,
		notificationService,
		teamService,
		templateService,
		uploadService,
		userService,
		quickstartService,
		leaderBoardService,
	)

	server.Start(logger, publicHandler, playerHandler, adminHandler, jobs)
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
