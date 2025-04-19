package server

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/nathanhollows/Rapua/v3/filesystem"
	admin "github.com/nathanhollows/Rapua/v3/internal/handlers/admin"
	players "github.com/nathanhollows/Rapua/v3/internal/handlers/players"
	public "github.com/nathanhollows/Rapua/v3/internal/handlers/public"
	"github.com/nathanhollows/Rapua/v3/internal/middlewares"
)

func setupRouter(
	logger *slog.Logger,
	publicHandler *public.PublicHandler,
	playerHandler *players.PlayerHandler,
	adminHandler *admin.AdminHandler,
) *chi.Mux {

	router := chi.NewRouter()

	router.Use(middleware.Compress(5))
	router.Use(middleware.CleanPath)
	router.Use(middleware.StripSlashes)
	router.Use(middleware.RedirectSlashes)

	setupPublicRoutes(router, publicHandler)
	setupPlayerRoutes(router, playerHandler)
	setupAdminRoutes(router, adminHandler)
	setupFacilitatorRoutes(router, adminHandler)

	// Static files
	workDir, _ := os.Getwd()
	filesDir := filesystem.Myfs{Dir: http.Dir(filepath.Join(workDir, "static"))}
	filesystem.FileServer(router, "/static", filesDir)

	return router
}

// Setup the player routes.
func setupPlayerRoutes(router chi.Router, playerHandler *players.PlayerHandler) {
	// Home route
	// Takes a GET request to show the home page
	// Takes a POST request to submit the home page form
	router.Route("/play", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.TeamMiddleware(playerHandler.TeamService, next)
		})

		r.Get("/", playerHandler.Play)
		r.Post("/", playerHandler.PlayPost)
	})

	// Show the next available locations
	router.Route("/next", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.PreviewMiddleware(playerHandler.TeamService,
				middlewares.TeamMiddleware(playerHandler.TeamService,
					middlewares.LobbyMiddleware(playerHandler.TeamService, next)))
		})
		r.Get("/", playerHandler.Next)
		r.Post("/", playerHandler.Next)
	})

	router.Route("/blocks", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.PreviewMiddleware(playerHandler.TeamService,
				middlewares.TeamMiddleware(playerHandler.TeamService,
					middlewares.LobbyMiddleware(playerHandler.TeamService, next)))
		})
		r.Post("/validate", playerHandler.ValidateBlock)
	})

	// Show the lobby page
	router.Route("/lobby", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.PreviewMiddleware(playerHandler.TeamService,
				middlewares.TeamMiddleware(playerHandler.TeamService,
					middlewares.LobbyMiddleware(playerHandler.TeamService, next)))
		})
		r.Get("/", playerHandler.Lobby)
		r.Post("/team-name", playerHandler.SetTeamName)
	})

	// Ending the game
	router.Route("/finish", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.TeamMiddleware(playerHandler.TeamService, next)
		})
		r.Get("/", playerHandler.Finish)
	})

	// Check in to a location
	router.Route("/s", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.TeamMiddleware(playerHandler.TeamService,
				middlewares.LobbyMiddleware(playerHandler.TeamService, next))
		})
		r.Get("/{code:[A-z]{5}}", playerHandler.CheckIn)
		r.Post("/{code:[A-z]{5}}", playerHandler.CheckInPost)
	})

	// Check out of a location
	router.Route("/o", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.TeamMiddleware(playerHandler.TeamService,
				middlewares.LobbyMiddleware(playerHandler.TeamService, next))
		})
		r.Get("/", playerHandler.CheckOut)
		r.Get("/{code:[A-z]{5}}", playerHandler.CheckOut)
		r.Post("/{code:[A-z]{5}}", playerHandler.CheckOutPost)
	})

	router.Route("/checkins", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.PreviewMiddleware(playerHandler.TeamService,
				middlewares.TeamMiddleware(playerHandler.TeamService,
					middlewares.LobbyMiddleware(playerHandler.TeamService, next)))
		})
		r.Get("/", playerHandler.MyCheckins)
		r.Get("/{id}", playerHandler.CheckInView)
	})

	router.Post("/dismiss/{ID}", playerHandler.DismissNotificationPost)

}

func setupPublicRoutes(router chi.Router, publicHandler *public.PublicHandler) {
	router.Use(func(next http.Handler) http.Handler {
		return middlewares.AuthStatusMiddleware(publicHandler.AuthService, next)
	})

	router.Get("/", publicHandler.Index)
	router.Get("/pricing", publicHandler.Pricing)
	router.Get("/about", publicHandler.About)
	router.Get("/contact", publicHandler.Contact)
	router.Post("/contact", publicHandler.ContactPost)
	router.Get("/privacy", publicHandler.Privacy)

	router.Route("/login", func(r chi.Router) {
		r.Get("/", publicHandler.Login)
		r.Post("/", publicHandler.LoginPost)
	})
	router.Get("/logout", publicHandler.Logout)
	router.Route("/register", func(r chi.Router) {
		r.Get("/", publicHandler.Register)
		r.Post("/", publicHandler.RegisterPost)
	})
	router.Get("/forgot", publicHandler.ForgotPassword)
	router.Post("/forgot", publicHandler.ForgotPasswordPost)

	router.Route("/auth", func(r chi.Router) {
		r.Get("/{provider}", publicHandler.Auth)
		r.Get("/{provider}/callback", publicHandler.AuthCallback)
	})

	router.Route("/verify-email", func(r chi.Router) {
		r.Get("/", publicHandler.VerifyEmail)
		r.Get("/{token}", publicHandler.VerifyEmailWithToken)
		r.Get("/status", publicHandler.VerifyEmailStatus)
		r.Post("/resend", publicHandler.ResendEmailVerification)
	})

	router.Route("/docs", func(r chi.Router) {
		r.Get("/*", publicHandler.Docs)
	})

	router.Route("/templates", func(r chi.Router) {
		r.Get("/{id}", publicHandler.TemplatesPreview)
	})

	router.NotFound(publicHandler.NotFound)

}

func setupAdminRoutes(router chi.Router, adminHandler *admin.AdminHandler) {
	router.Route("/admin", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.AdminAuthMiddleware(adminHandler.AuthService, next)
		})
		r.Use(middlewares.AdminCheckInstanceMiddleware)

		r.Route("/quickstart", func(r chi.Router) {
			r.Get("/", adminHandler.Quickstart)
			r.Post("/dismiss", adminHandler.DismissQuickstart)
		})

		r.Get("/", adminHandler.Activity)
		r.Route("/activity", func(r chi.Router) {
			r.Get("/", adminHandler.Activity)
			r.Get("/teams", adminHandler.ActivityTeamsOverview)
			r.Get("/team/{teamCode}", adminHandler.TeamActivity)
		})

		r.Route("/locations", func(r chi.Router) {
			r.Get("/", adminHandler.Locations)
			r.Post("/reorder", adminHandler.ReorderLocations)
			r.Get("/new", adminHandler.LocationNew)
			r.Post("/new", adminHandler.LocationNewPost)
			r.Get("/{id}", adminHandler.LocationEdit)
			r.Post("/{id}", adminHandler.LocationEditPost)
			r.Delete("/{id}", adminHandler.LocationDelete)
			// Assets
			r.Get("/qr/{action}/{id}.{extension}", adminHandler.QRCode)
			r.Get("/qr-codes.zip", adminHandler.GenerateQRCodeArchive)
			r.Get("/poster/{id}.pdf", adminHandler.GeneratePoster)
			r.Get("/posters.pdf", adminHandler.GeneratePosters)
			// Blocks
			r.Route("/{location}/blocks", func(r chi.Router) {
				// r.Get("/", adminHandler.Blocks)
				// r.Post("/", adminHandler.BlocksPost)
				r.Post("/reorder", adminHandler.ReorderBlocks)
				r.Post("/new/{type}", adminHandler.BlockNewPost)
				r.Get("/{blockID}/edit", adminHandler.BlockEdit)
				r.Post("/{blockID}/update", adminHandler.BlockEditPost)
				r.Delete("/{blockID}/delete", adminHandler.BlockDelete)
			})
		})

		r.Route("/teams", func(r chi.Router) {
			r.Get("/", adminHandler.Teams)
			r.Post("/add", adminHandler.TeamsAdd)
			r.Delete("/delete", adminHandler.TeamsDelete)
			r.Post("/reset", adminHandler.TeamsReset)
		})

		r.Route("/experience", func(r chi.Router) {
			r.Get("/", adminHandler.Experience)
			r.Post("/", adminHandler.ExperiencePost)
			r.Post("/preview", adminHandler.ExperiencePreview)
		})

		r.Route("/instances", func(r chi.Router) {
			r.Get("/", adminHandler.Instances)
			r.Post("/new", adminHandler.InstancesCreate)
			r.Get("/{id}", adminHandler.Instances)
			r.Post("/{id}", adminHandler.Instances)
			r.Get("/{id}/switch", adminHandler.InstanceSwitch)
			r.Post("/delete", adminHandler.InstanceDelete)
			r.Post("/duplicate", adminHandler.InstanceDuplicate)
		})

		r.Route("/markdown", func(r chi.Router) {
			r.Post("/preview", adminHandler.PreviewMarkdown)
		})

		r.Route("/schedule", func(r chi.Router) {
			r.Get("/start", adminHandler.StartGame)
			r.Get("/stop", adminHandler.StopGame)
			r.Post("/", adminHandler.ScheduleGame)
		})

		r.Route("/notify", func(r chi.Router) {
			r.Post("/all", adminHandler.NotifyAllPost)
			r.Post("/team", adminHandler.NotifyTeamPost)
		})

		r.Route("/facilitator", func(r chi.Router) {
			r.Get("/create-link", adminHandler.FacilitatorShowModal)
			r.Post("/create-link", adminHandler.FacilitatorCreateTokenLink)
		})

		r.Route("/templates", func(r chi.Router) {
			r.Post("/create", adminHandler.TemplatesCreate)
			r.Delete("/", adminHandler.TemplatesDelete)
			// Launch
			r.Post("/launch", adminHandler.TemplatesLaunch)
			r.Post("/launch-from-link", adminHandler.TemplatesLaunchFromLink)
			// Edit
			r.Get("/{id}/name", adminHandler.TemplatesName)
			r.Get("/{id}/edit/name", adminHandler.TemplatesNameEdit)
			r.Post("/{id}/edit/name", adminHandler.TemplatesNameEditPost)
			// Share
			r.Get("/{id}/share", adminHandler.TemplatesShare)
			r.Post("/{id}/share", adminHandler.TemplatesSharePost)
		})

		r.Route("/media", func(r chi.Router) {
			r.Post("/upload", adminHandler.UploadMedia)
		})

		r.NotFound(adminHandler.NotFound)
	})
}

func setupFacilitatorRoutes(router chi.Router, adminHandler *admin.AdminHandler) {
	router.Route("/facilitator", func(r chi.Router) {
		r.Get("/login/{token}", adminHandler.FacilitatorLogin)
		r.Get("/dashboard", adminHandler.FacilitatorDashboard)
	})
}
