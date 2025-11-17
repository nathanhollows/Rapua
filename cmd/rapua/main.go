//go:generate npm run build

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/nathanhollows/Rapua/v6/db"
	admin "github.com/nathanhollows/Rapua/v6/internal/handlers/admin"
	players "github.com/nathanhollows/Rapua/v6/internal/handlers/players"
	public "github.com/nathanhollows/Rapua/v6/internal/handlers/public"
	"github.com/nathanhollows/Rapua/v6/internal/migrations"
	"github.com/nathanhollows/Rapua/v6/internal/scheduler"
	"github.com/nathanhollows/Rapua/v6/internal/server"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/internal/sessions"
	"github.com/nathanhollows/Rapua/v6/internal/storage"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/phsym/console-slog"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"github.com/urfave/cli/v2"
)

const version = "v6.0.2"

func main() {
	logger := slog.New(
		console.NewHandler(os.Stdout, &console.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: true,
		}),
	)
	slog.SetDefault(logger)

	logger.Info("starting application", "version", version)

	// Load environment variables
	if err := godotenv.Load(".env"); err != nil {
		logger.Warn("could not load .env file", "error", err)
	}

	dbc := db.MustOpen(logger)

	// Initialize the migrator
	migrator := migrate.NewMigrator(dbc, migrations.Migrations)

	// Define CLI app for migrations
	app := &cli.App{
		Name:        "Rapua",
		Usage:       "rapua [global options] command [command options] [arguments...]",
		Description: `An open-source platform for location-based games.`,
		Version:     version,
		Commands: []*cli.Command{
			newDBCommand(migrator, logger),
			newCreditsCommand(dbc, logger),
		},
		Action: func(_ *cli.Context) error {
			// Default action: run the app
			runApp(logger, dbc)
			return nil
		},
	}

	// Run CLI or app
	err := app.Run(os.Args)
	_ = dbc.Close() // Ensure Close happens before Exit
	if err != nil {
		logger.Error("application error", "error", err)
		os.Exit(1)
	}
}

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

// addCreditsParams contains parameters for adding credits to a user.
type addCreditsParams struct {
	Email        string
	Credits      int
	Prefix       string
	CustomReason string
}

// addCreditsToUser adds credits to a user account with the given parameters.
func addCreditsToUser(
	ctx context.Context,
	params addCreditsParams,
	creditService *services.CreditService,
	userRepo repositories.UserRepository,
) error {
	// Validate prefix
	var reasonPrefix string
	switch params.Prefix {
	case "Admin":
		reasonPrefix = models.CreditAdjustmentReasonPrefixAdmin
	case "Gift":
		reasonPrefix = models.CreditAdjustmentReasonPrefixGift
	default:
		return fmt.Errorf("invalid prefix %q: must be 'Admin' or 'Gift'", params.Prefix)
	}

	// Validate amount
	if params.Credits <= 0 {
		return errors.New("amount must be greater than 0")
	}

	// Build reason
	reason := reasonPrefix
	if params.CustomReason != "" {
		reason = fmt.Sprintf("%s: %s", reasonPrefix, params.CustomReason)
	}

	// Find user by email
	user, err := userRepo.GetByEmail(ctx, params.Email)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	// Add credits with retry for SQLITE_BUSY
	const (
		maxRetries       = 3
		retryDelayMillis = 100
	)
	for attempt := 1; attempt <= maxRetries; attempt++ {
		err = creditService.AddCredits(ctx, user.ID, 0, params.Credits, reason)
		if err == nil {
			return nil
		}

		if attempt < maxRetries && strings.Contains(err.Error(), "database is locked") {
			time.Sleep(time.Millisecond * retryDelayMillis * time.Duration(attempt))
			continue
		}

		return fmt.Errorf("failed to add credits: %w", err)
	}

	return errors.New("failed after maximum retries")
}

func newCreditsCommand(dbc *bun.DB, logger *slog.Logger) *cli.Command {
	return &cli.Command{
		Name:  "credits",
		Usage: "manage user credits",
		Subcommands: []*cli.Command{
			{
				Name:      "add",
				Usage:     "add credits to a user (use --prefix=Admin or --prefix=Gift, defaults to Admin)",
				ArgsUsage: "<email> <amount> [reason]",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  "prefix",
						Usage: "reason prefix: Admin or Gift",
						Value: "Admin",
					},
				},
				Action: func(c *cli.Context) error {
					//nolint:mnd // Magic number for argument count
					if c.NArg() < 2 {
						return errors.New("usage: rapua credits add <email> <amount> [reason]")
					}

					// Parse arguments
					email := c.Args().Get(0)
					amountStr := c.Args().Get(1)
					//nolint:mnd // Magic number for argument count
					customReason := c.Args().Get(2)
					prefix := c.String("prefix")

					var credits int
					if _, err := fmt.Sscanf(amountStr, "%d", &credits); err != nil {
						return fmt.Errorf("invalid amount %q: must be a number", amountStr)
					}

					// Initialize services (lazy)
					ctx := c.Context
					creditRepo := repositories.NewCreditRepository(dbc)
					teamStartLogRepo := repositories.NewTeamStartLogRepository(dbc)
					userRepo := repositories.NewUserRepository(dbc)
					transactor := db.NewTransactor(dbc)
					creditService := services.NewCreditService(transactor, creditRepo, teamStartLogRepo, userRepo)

					// Call testable function
					params := addCreditsParams{
						Email:        email,
						Credits:      credits,
						Prefix:       prefix,
						CustomReason: customReason,
					}

					err := addCreditsToUser(ctx, params, creditService, userRepo)
					if err != nil {
						return err
					}

					// Get updated user for logging
					user, _ := userRepo.GetByEmail(ctx, email)
					logger.Info("credits added successfully",
						"user", email,
						"amount", credits,
						"new_balance", user.PaidCredits)

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
	creditRepo := repositories.NewCreditRepository(dbc)
	creditPurchaseRepo := repositories.NewCreditPurchaseRepository(dbc)
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
	gameStructureService := services.NewGameStructureService(locationRepo, instanceRepo)
	deleteService := services.NewDeleteService(
		transactor,
		blockRepo,
		blockStateRepo,
		checkInRepo,
		instanceRepo,
		instanceSettingsRepo,
		locationRepo,
		markerRepo,
		teamRepo,
		userRepo,
		creditRepo,
		creditPurchaseRepo,
		teamStartLogRepo,
	)
	duplicationService := services.NewDuplicationService(
		transactor,
		instanceRepo,
		instanceSettingsRepo,
		locationRepo,
		blockRepo,
	)
	facilitatorService := services.NewFacilitatorService(facilitatorRepo)
	assetGenerator := services.NewAssetGenerator()
	identityService := services.NewAuthService(userRepo)
	blockService := services.NewBlockService(blockRepo, blockStateRepo)
	emailService := services.NewEmailService()
	instanceSettingsService := services.NewInstanceSettingsService(instanceSettingsRepo)
	locationService := services.NewLocationService(locationRepo, markerRepo, blockRepo, markerService)

	// Set the relation loader so gameStructureService can load location relations
	gameStructureService.SetRelationLoader(locationService)

	navigationService := services.NewNavigationService(locationRepo, teamRepo, gameStructureService, blockService)
	checkInService := services.NewCheckInService(
		checkInRepo,
		locationRepo,
		teamRepo,
		locationStatsService,
		navigationService,
		blockService,
	)
	notificationService := services.NewNotificationService(notificationRepo, teamRepo)
	userService := services.NewUserService(userRepo, instanceRepo)
	monthlyCreditTopupJob := services.NewMonthlyCreditTopupService(transactor, creditRepo)
	staleCreditCleanupService := services.NewStalePurchaseCleanupService(transactor, logger)
	creditService := services.NewCreditService(
		transactor,
		creditRepo,
		teamStartLogRepo,
		userRepo,
	)
	stripeService := services.NewStripeService(
		transactor,
		creditService,
		creditPurchaseRepo,
		userRepo,
		logger,
	)
	teamService := services.NewTeamService(
		transactor,
		teamRepo,
		checkInRepo,
		creditService,
		blockStateRepo,
		locationRepo,
	)
	leaderBoardService := services.NewLeaderBoardService(teamRepo)
	instanceService := services.NewInstanceService(
		instanceRepo, instanceSettingsRepo,
	)
	templateService := services.NewTemplateService(
		duplicationService, instanceRepo, instanceSettingsRepo, shareLinkRepo,
	)

	sessions.Start()

	// Register jobs
	jobs := scheduler.NewScheduler(logger)
	jobs.AddJob(
		"Monthly Credit Top-Up",
		monthlyCreditTopupJob.TopUpCredits,
		scheduler.NextFirstOfMonth,
	)
	jobs.AddJob(
		"Stale Credit Purchase Cleanup",
		staleCreditCleanupService.CleanupStalePurchases,
		scheduler.NextDaily,
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
		creditService,
		creditPurchaseRepo,
		deleteService,
		duplicationService,
		facilitatorService,
		gameScheduleService,
		gameStructureService,
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
		stripeService,
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
			if err = os.MkdirAll(folder, 0750); err != nil {
				logger.Error("could not create directory", "folder", folder, "error", err)
				os.Exit(1)
			}
		}
	}
}
