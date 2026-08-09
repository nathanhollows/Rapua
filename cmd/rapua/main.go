//go:generate npm run build

package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/joho/godotenv"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	admin "github.com/nathanhollows/Rapua/v8/internal/handlers/admin"
	players "github.com/nathanhollows/Rapua/v8/internal/handlers/players"
	public "github.com/nathanhollows/Rapua/v8/internal/handlers/public"
	"github.com/nathanhollows/Rapua/v8/internal/migrations"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/scheduler"
	"github.com/nathanhollows/Rapua/v8/internal/server"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/internal/sessions"
	"github.com/nathanhollows/Rapua/v8/internal/storage"
	console "github.com/phsym/console-slog"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
	"github.com/urfave/cli/v2"
)

const (
	version    = "v8.0.0"
	uploadsDir = "static/uploads/"
)

func main() {
	logger := slog.New(
		console.NewHandler(os.Stdout, &console.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: true,
		}),
	)
	slog.SetDefault(logger)

	logger.Info("starting application", "version", version)

	if err := godotenv.Load(".env"); err != nil {
		logger.Warn("could not load .env file", "error", err)
	}

	dbc := db.MustOpen(logger)

	migrator := migrate.NewMigrator(dbc, migrations.Migrations)

	app := &cli.App{
		Name:        "Rapua",
		Usage:       "rapua [global options] command [command options] [arguments...]",
		Description: `An open-source platform for location-based games.`,
		Version:     version,
		Commands: []*cli.Command{
			newDBCommand(dbc, migrator, logger),
			newCreditsCommand(dbc, logger),
			newGenerateLoginCommand(dbc, logger),
			newGenSpecCommand(),
		},
		Action: func(_ *cli.Context) error {
			runApp(logger, dbc)
			return nil
		},
	}

	err := app.Run(os.Args)
	_ = dbc.Close()
	if err != nil {
		logger.Error("application error", "error", err)
		os.Exit(1)
	}
}

func runApp(logger *slog.Logger, dbc *bun.DB) { //nolint:funlen // Main setup function
	initialiseFolders(logger)

	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	blockRepo := repositories.NewBlockRepository(dbc, blockStateRepo)
	checkInRepo := repositories.NewCheckInRepository(dbc)
	creditRepo := repositories.NewCreditRepository(dbc)
	creditPurchaseRepo := repositories.NewCreditPurchaseRepository(dbc)
	facilitatorRepo := repositories.NewFacilitatorTokenRepo(dbc)
	instanceRepo := repositories.NewQuestRepository(dbc)
	instanceSettingsRepo := repositories.NewQuestSettingsRepository(dbc)
	locationRepo := repositories.NewLocationRepository(dbc)
	markerRepo := repositories.NewMarkerRepository(dbc)
	spaceRepo := repositories.NewSpaceRepository(dbc)
	notificationRepo := repositories.NewNotificationRepository(dbc)
	shareLinkRepo := repositories.NewShareLinkRepository(dbc)
	teamRepo := repositories.NewRunRepository(dbc)
	runStartLogRepo := repositories.NewRunStartLogRepository(dbc)
	teamVarStateRepo := repositories.NewRunVarStateRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)
	uploadRepo := repositories.NewUploadRepository(dbc)

	transactor := db.NewTransactor(dbc)
	localStorage := storage.NewLocalStorage("static/uploads/")

	accessService := services.NewAccessService(blockRepo, instanceRepo, locationRepo, markerRepo)
	locationStatsService := services.NewLocationStatsService(locationRepo)
	gameScheduleService := services.NewGameScheduleService(instanceRepo)
	quickstartService := services.NewQuickstartService(instanceRepo)
	markerService := services.NewMarkerService(markerRepo)
	uploadService := services.NewUploadService(uploadRepo, localStorage)
	gameStructureService := services.NewGameStructureService(locationRepo, instanceRepo)
	deleteService := services.NewDeleteService(
		transactor, instanceRepo, locationRepo, markerRepo,
		teamRepo, uploadRepo, dbc, uploadsDir, logger,
	)
	duplicationService := services.NewDuplicationService(
		logger, transactor, instanceRepo, instanceSettingsRepo, locationRepo, blockRepo,
	)
	facilitatorService := services.NewFacilitatorService(facilitatorRepo)
	assetGenerator := services.NewAssetGenerator()
	identityService := services.NewAuthService(userRepo)
	blockService := services.NewBlockService(blockRepo, blockStateRepo)
	emailService := services.NewEmailService()
	instanceSettingsService := services.NewQuestSettingsService(instanceSettingsRepo)
	locationService := services.NewLocationService(locationRepo, markerRepo, blockRepo, markerService)

	gameStructureService.SetRelationLoader(locationService)

	navigationService := services.NewNavigationService(
		locationRepo,
		teamRepo,
		teamVarStateRepo,
		gameStructureService,
		blockService,
		logger,
	)
	checkInService := services.NewCheckInService(
		checkInRepo, locationRepo, teamRepo,
		locationStatsService, navigationService, blockService, teamVarStateRepo,
	)
	notificationService := services.NewNotificationService(notificationRepo, teamRepo)
	userService := services.NewUserService(userRepo, instanceRepo)
	monthlyCreditTopupJob := services.NewMonthlyCreditTopupService(transactor, creditRepo, logger)
	staleCreditCleanupService := services.NewStalePurchaseCleanupService(transactor, logger)
	orphanedUploadsCleanupService := services.NewOrphanedUploadsCleanupService(uploadRepo, logger, uploadsDir)
	creditService := services.NewCreditService(transactor, creditRepo, runStartLogRepo, userRepo)
	stripeService := services.NewStripeService(transactor, creditService, creditPurchaseRepo, userRepo, logger)
	runService := services.NewRunService(
		transactor, teamRepo, checkInRepo, creditService, blockStateRepo, locationRepo, teamVarStateRepo,
	)
	leaderBoardService := services.NewLeaderBoardService()
	spaceService := services.NewSpaceService(spaceRepo)
	questService := services.NewQuestService(instanceRepo, instanceSettingsRepo, blockRepo)
	templateService := services.NewTemplateService(
		duplicationService, instanceRepo, instanceSettingsRepo, shareLinkRepo,
	)
	exportService := services.NewExportService(instanceRepo, instanceSettingsRepo, locationRepo, blockRepo)
	importService := services.NewImportService(
		logger, transactor, instanceRepo, instanceSettingsRepo, locationRepo, blockRepo, markerRepo,
	)

	sessions.Start()

	jobs := scheduler.NewScheduler(logger)
	jobs.AddJob("Monthly Credit Top-Up", monthlyCreditTopupJob.TopUpCredits, scheduler.NextFirstOfMonth)
	jobs.AddJob("Stale Credit Purchase Cleanup", staleCreditCleanupService.CleanupStalePurchases, scheduler.NextDaily)
	jobs.AddJob(
		"Orphaned Uploads Cleanup",
		func(ctx context.Context) error {
			if err := orphanedUploadsCleanupService.CleanupOrphanedUploads(ctx); err != nil {
				return err
			}
			return orphanedUploadsCleanupService.CleanupEmptyDirectories(ctx)
		},
		scheduler.NextDaily,
	)

	sessionKey := os.Getenv("SESSION_KEY")
	if sessionKey == "" {
		logger.Warn("SESSION_KEY not set - magic login links will not work")
		sessionKey = "default-key-for-development-only"
	}
	magicTokenService := services.NewMagicTokenService([]byte(sessionKey))

	publicHandler := public.NewHandler(
		logger, identityService, deleteService, emailService,
		&templateService, userService, magicTokenService,
	)
	playerHandler := players.NewPlayerHandler(
		logger, blockService, checkInService, questService, locationService,
		markerService, navigationService, notificationService, runService, uploadService,
	)
	adminHandler := admin.NewAdminHandler(
		logger, accessService, assetGenerator, identityService, blockService,
		creditService, creditPurchaseRepo, deleteService, duplicationService,
		exportService, importService, facilitatorService, gameScheduleService,
		gameStructureService, instanceRepo, questService, instanceSettingsService,
		locationService, markerService, navigationService, notificationService,
		runService, spaceService, templateService, uploadService, userService, quickstartService,
		leaderBoardService, stripeService,
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
			if err = os.MkdirAll(folder, 0o750); err != nil {
				logger.Error("could not create directory", "folder", folder, "error", err)
				os.Exit(1)
			}
		}
	}
}
