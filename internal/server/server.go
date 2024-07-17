package server

import (
	"log"
	"net/http"
	"os"

	"github.com/go-chi/chi"
	"github.com/nathanhollows/Rapua/internal/routes"
	"github.com/nathanhollows/Rapua/internal/services"
	"golang.org/x/exp/slog"
)

var router *chi.Mux
var server *http.Server

func Start() {
	gameplayService := &services.GameplayService{}
	gameManagerService := &services.GameManagerService{}

	router = routes.SetupRouter(gameplayService, gameManagerService)

	server = &http.Server{
		Addr:    os.Getenv("SERVER_ADDR"),
		Handler: router,
	}
	slog.Info("Server started", "addr", os.Getenv("SERVER_ADDR"))
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
