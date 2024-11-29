package server

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi"
	admin "github.com/nathanhollows/Rapua/internal/handlers/admin"
	players "github.com/nathanhollows/Rapua/internal/handlers/players"
	public "github.com/nathanhollows/Rapua/internal/handlers/public"
	"github.com/nathanhollows/Rapua/internal/services"
)

var router *chi.Mux
var server *http.Server

func Start(logger *slog.Logger,
	assetGenerator services.AssetGenerator,
	authService services.AuthService,
	blockService services.BlockService,
	checkInService services.CheckInService,
	clueService services.ClueService,
	emailService services.EmailService,
	gameManagerService services.GameManagerService,
	gameplayService services.GameplayService,
	locationService services.LocationService,
	navigationService services.NavigationService,
	notificationService services.NotificationService,
	teamService services.TeamService,
	userService services.UserService,
) {
	// Public routes
	publicHandler := public.NewPublicHandler(
		logger,
		authService,
		userService,
	)

	// Player routes
	playerHandler := players.NewPlayerHandler(
		logger,
		blockService,
		gameplayService,
		notificationService,
		teamService,
	)

	// Admin routes
	adminHandler := admin.NewAdminHandler(
		logger,
		assetGenerator,
		authService,
		blockService,
		clueService,
		gameManagerService,
		gameplayService,
		locationService,
		notificationService,
		teamService,
		userService,
	)
	router = setupRouter(logger, publicHandler, playerHandler, adminHandler)

	killSig := make(chan os.Signal, 1)

	signal.Notify(killSig, os.Interrupt, syscall.SIGTERM)

	server = &http.Server{
		Addr:    os.Getenv("SERVER_ADDR"),
		Handler: router,
	}

	go func() {
		err := server.ListenAndServe()
		if errors.Is(err, http.ErrServerClosed) {
			log.Println("Server closed")
		} else {
			log.Fatalf("Server failed to start: %v", err)
			os.Exit(1)
		}
	}()

	logger.Info("Server started", "addr", os.Getenv("SERVER_ADDR"))
	<-killSig

	slog.Info("Shutting down server")

	// Create a context with a timeout for the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Server shutdown failed", "error", err)
	}

	logger.Info("Server shutdown complete")
}
