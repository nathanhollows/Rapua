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
	"github.com/nathanhollows/Rapua/v4/internal/handlers/admin"
	"github.com/nathanhollows/Rapua/v4/internal/handlers/players"
	"github.com/nathanhollows/Rapua/v4/internal/handlers/public"
)

var router *chi.Mux
var server *http.Server

func Start(
	logger *slog.Logger,
	publicHandler *public.PublicHandler,
	playerHandler *players.PlayerHandler,
	adminHandler *admin.Handler,
) {
	router = setupRouter(logger, publicHandler, playerHandler, adminHandler)

	killSig := make(chan os.Signal, 1)

	signal.Notify(killSig, os.Interrupt, syscall.SIGTERM)

	server = &http.Server{
		Addr:              os.Getenv("SERVER_ADDR"),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       1 * time.Minute,
		WriteTimeout:      2 * time.Minute,
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

	logger.Info("Shutting down server")

	// Create a context with a timeout for the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown the server
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server shutdown failed", "error", err)
	}

	logger.Info("Server shutdown complete")
}
