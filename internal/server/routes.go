package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/gorilla/csrf"
	"github.com/nathanhollows/Rapua/v8/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v8/internal/filesystem"
	admin "github.com/nathanhollows/Rapua/v8/internal/handlers/admin"
	players "github.com/nathanhollows/Rapua/v8/internal/handlers/players"
	"github.com/nathanhollows/Rapua/v8/internal/handlers/public"
	"github.com/nathanhollows/Rapua/v8/internal/handlers/static"
	"github.com/nathanhollows/Rapua/v8/internal/middlewares"
)

const (
	compressLevel = 5
	csrfKeyLength = 32
)

func setupRouter(
	logger *slog.Logger,
	publicHandler *public.Handler,
	playerHandler *players.PlayerHandler,
	adminHandler *admin.Handler,
) *chi.Mux {
	// Get CSRF key from environment
	csrfKey := os.Getenv("CSRF_KEY")
	if csrfKey == "" {
		logger.Warn("CSRF_KEY not set, using default key - CHANGE IN PRODUCTION")
		csrfKey = "temp-32-byte-long-auth-key-here"
	}
	if len(csrfKey) != csrfKeyLength {
		logger.Warn("CSRF_KEY should be exactly 32 bytes", "length", len(csrfKey))
	}

	// CSRF protection middleware
	// gorilla/csrf v1.7.3 defaults to HTTPS. In production behind a reverse
	// proxy the Host header may differ from the browser's Origin, so we add
	// the public domain from SITE_URL as a trusted origin.
	csrfOpts := []csrf.Option{
		csrf.Secure(os.Getenv("IS_PROD") == "1"),
		csrf.CookieName("csrf"),
		csrf.FieldName("csrf"),
		csrf.Path("/"),
	}
	if siteURL := os.Getenv("SITE_URL"); siteURL != "" {
		if u, err := url.Parse(siteURL); err == nil && u.Host != "" {
			csrfOpts = append(csrfOpts, csrf.TrustedOrigins([]string{u.Host}))
		}
	}
	CSRF := csrf.Protect([]byte(csrfKey), csrfOpts...) //nolint:gocritic // CSRF

	router := chi.NewRouter()

	router.Use(middleware.Compress(compressLevel))
	router.Use(middleware.CleanPath)
	router.Use(middleware.StripSlashes)
	router.Use(middleware.RedirectSlashes)

	// gorilla/csrf v1.7.3 assumes HTTPS by default. Mark requests as
	// plaintext HTTP in dev so the origin/referer check is skipped.
	if os.Getenv("IS_PROD") != "1" {
		router.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				r = csrf.PlaintextHTTPRequest(r)
				next.ServeHTTP(w, r)
			})
		})
	}

	// Routes that bypass CSRF protection
	setupWebhookRoutes(router, adminHandler)
	setupAPIRoutes(router, publicHandler)

	// All other routes with CSRF protection
	router.Group(func(r chi.Router) {
		r.Use(CSRF)
		setupPublicRoutes(r, publicHandler)
		setupPlayerRoutes(r, logger, publicHandler, playerHandler, adminHandler)
		setupAdminRoutes(r, logger, adminHandler)
		setupFacilitatorRoutes(r, logger, adminHandler)
	})

	// Static files
	workDir, _ := os.Getwd()
	filesDir := filesystem.Myfs{Dir: http.Dir(filepath.Join(workDir, "static"))}

	// Image resizing handler for uploads (must come before general static handler)
	router.Get("/static/uploads/*", static.ServeResizedImage(workDir))

	filesystem.FileServer(router, "/static", filesDir)

	return router
}

// Setup the player routes.
func setupPlayerRoutes(
	router chi.Router,
	logger *slog.Logger,
	publicHandler *public.Handler,
	playerHandler *players.PlayerHandler,
	adminHandler *admin.Handler,
) {
	// Home route
	// Takes a GET request to show the home page
	// Takes a POST request to submit the home page form
	router.Route("/play", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.RunMiddleware(logger, playerHandler.GetRunService(), next)
		})

		r.Get("/", playerHandler.Play)
		r.Post("/", playerHandler.PlayPost)
		r.Get("/{code}", playerHandler.PlayWithCode)
	})

	// Show the next available locations
	router.Route("/next", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.PreviewMiddleware(
				logger,
				playerHandler.GetRunService(),
				playerHandler.GetQuestService(),
				adminHandler.GetIdentityService(),
				middlewares.RunMiddleware(logger, playerHandler.GetRunService(),
					middlewares.StartMiddleware(playerHandler.GetRunService(), next)),
			)
		})
		r.Get("/", playerHandler.Next)
		r.Post("/", playerHandler.Next)
	})

	// Advance to next group (manual skip)
	router.Route("/advance", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.RunMiddleware(
				logger,
				playerHandler.GetRunService(),
				middlewares.StartMiddleware(playerHandler.GetRunService(), next),
			)
		})
		r.Post("/", playerHandler.AdvanceGroup)
	})

	router.Route("/blocks", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.PreviewMiddleware(
				logger,
				playerHandler.GetRunService(),
				playerHandler.GetQuestService(),
				adminHandler.GetIdentityService(),
				middlewares.RunMiddleware(logger, playerHandler.GetRunService(),
					middlewares.StartMiddleware(playerHandler.GetRunService(), next)),
			)
		})
		r.Post("/validate", playerHandler.ValidateBlock)
		r.Get("/{id}/team-name-block", playerHandler.GetTeamNameBlock)
		r.Get("/{id}/game-status-alert", playerHandler.GetGameStatusAlertBlock)
		r.Get("/{id}/start-game-button", playerHandler.GetStartGameButtonBlock)
	})

	// Upload route for player media
	router.Route("/upload", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.RunMiddleware(logger, playerHandler.GetRunService(),
				middlewares.StartMiddleware(playerHandler.GetRunService(), next))
		})
		r.Post("/image", playerHandler.UploadImage)
	})

	// Show the start page
	router.Route("/start", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.PreviewMiddleware(
				logger,
				playerHandler.GetRunService(),
				playerHandler.GetQuestService(),
				adminHandler.GetIdentityService(),
				middlewares.RunMiddleware(logger, playerHandler.GetRunService(),
					middlewares.StartMiddleware(playerHandler.GetRunService(), next)),
			)
		})
		r.Get("/", playerHandler.Start)
		r.Get("/team-name-value", playerHandler.GetTeamNameValue)
		r.Get("/team-name-form", playerHandler.GetTeamNameForm)
		r.Post("/team-name", playerHandler.SetTeamName)
	})

	// Ending the game
	router.Route("/complete", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.PreviewMiddleware(
				logger,
				playerHandler.GetRunService(),
				playerHandler.GetQuestService(),
				adminHandler.GetIdentityService(),
				middlewares.RunMiddleware(logger, playerHandler.GetRunService(), next),
			)
		})
		r.Get("/", playerHandler.Complete)
	})

	// Check out of a location
	router.Route("/o", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.RunMiddleware(logger, playerHandler.GetRunService(), next)
		})
		r.Get("/", playerHandler.CheckOut)
		r.Get("/{code:[A-z]{5}}", playerHandler.CheckOut)
		r.Post("/{code:[A-z]{5}}", playerHandler.CheckOutPost)
	})

	router.Route("/checkins", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.PreviewMiddleware(
				logger,
				playerHandler.GetRunService(),
				playerHandler.GetQuestService(),
				adminHandler.GetIdentityService(),
				middlewares.RunMiddleware(logger, playerHandler.GetRunService(),
					middlewares.StartMiddleware(playerHandler.GetRunService(), next)),
			)
		})
		r.Get("/", playerHandler.MyCheckins)
		r.Get("/{slug:[a-z0-9-]+}", playerHandler.CheckInView)
	})

	router.Route("/objective", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.PreviewMiddleware(
				logger,
				playerHandler.GetRunService(),
				playerHandler.GetQuestService(),
				adminHandler.GetIdentityService(),
				middlewares.RunMiddleware(logger, playerHandler.GetRunService(),
					middlewares.StartMiddleware(playerHandler.GetRunService(), next)),
			)
		})
		r.Get("/{slug:[a-z0-9-]+}", playerHandler.ObjectiveView)
		// Sub-routers get their own default NotFoundHandler unless set explicitly
		// (chi does not fall back to the parent router's), which otherwise
		// surfaces chi's bare, unstyled 404 instead of the site's own.
		r.NotFound(publicHandler.NotFound)
	})

	router.Post("/dismiss/{ID}", playerHandler.DismissNotificationPost)
}

func setupPublicRoutes(router chi.Router, publicHandler *public.Handler) {
	router.Use(func(next http.Handler) http.Handler {
		return middlewares.AuthStatusMiddleware(publicHandler.GetIdentityService(), next)
	})

	// CSRF token refresh endpoint
	router.Get("/csrf-token", func(w http.ResponseWriter, r *http.Request) {
		// Verify user has valid session
		status := contextkeys.GetUserStatus(r.Context())
		if !status.IsAdminLoggedIn {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		token := csrf.Token(r)
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if err := json.NewEncoder(w).Encode(map[string]string{"token": token}); err != nil {
			http.Error(w, "Failed to encode token", http.StatusInternalServerError)
			return
		}
	})

	router.Get("/", publicHandler.Index)
	router.Get("/pricing", publicHandler.Pricing)
	router.Get("/about", publicHandler.About)
	router.Get("/contact", publicHandler.Contact)
	router.Post("/contact", publicHandler.ContactPost)
	router.Get("/privacy", publicHandler.Privacy)
	router.Get("/terms", publicHandler.Terms)

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
	router.Get("/reset/success", publicHandler.ResetPasswordSuccess)
	router.Get("/reset/{token}", publicHandler.ResetPassword)
	router.Post("/reset/{token}", publicHandler.ResetPasswordPost)

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

	// Magic login link (CLI-generated)
	router.Get("/login/magic/{token}", publicHandler.MagicLogin)

	router.Route("/docs", func(r chi.Router) {
		r.Get("/*", publicHandler.Docs)
	})

	router.Route("/templates", func(r chi.Router) {
		r.Get("/{id}", publicHandler.TemplatesPreview)
	})

	router.NotFound(publicHandler.NotFound)
}

func setupAdminRoutes(router chi.Router, logger *slog.Logger, adminHandler *admin.Handler) {
	router.Route("/admin", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return middlewares.AdminAuthMiddleware(
				logger,
				adminHandler.GetIdentityService(),
				adminHandler.GetQuestLoader(),
				next,
			)
		})
		r.Use(middlewares.AdminCheckInstanceMiddleware)

		r.Route("/quickstart", func(r chi.Router) {
			r.Get("/", adminHandler.Quickstart)
			r.Post("/dismiss", adminHandler.DismissQuickstart)
		})

		r.Get("/", adminHandler.Activity)
		r.Route("/activity", func(r chi.Router) {
			r.Get("/", adminHandler.Activity)
			r.Get("/runs", adminHandler.ActivityRunsOverview)
			r.Get("/run/{runCode}", adminHandler.RunActivity)
			r.Get("/stats", adminHandler.ActivityStats)
			r.Get("/locations", adminHandler.ActivityLocations)
		})

		// Wildcard rather than {code}.{ext} so a code carrying a dot or a slash
		// still resolves; the extension is split off the end here.
		r.Get("/qr/*", adminHandler.CodeQRCode)

		r.Route("/quest", func(r chi.Router) {
			r.Get("/", adminHandler.Locations)
			r.Post("/reorder", adminHandler.ReorderLocations)
			r.Post("/structure", adminHandler.SaveGameStructure)
			r.Get("/new", adminHandler.LocationNew)
			r.Get("/start", adminHandler.StartPageEdit)
			r.Get("/complete", adminHandler.CompletePageEdit)
		})

		r.Route("/objective", func(r chi.Router) {
			r.Get("/{slug:[a-z0-9-]+}", adminHandler.LocationEdit)
			r.Post("/{slug:[a-z0-9-]+}", adminHandler.LocationEditPost)
			r.Delete("/{slug:[a-z0-9-]+}", adminHandler.LocationDelete)
		})

		// RESTful blocks API
		r.Route("/blocks", func(r chi.Router) {
			// Primary RESTful endpoints
			r.Post("/", adminHandler.BlockCreate)         // POST /admin/blocks?owner=uuid&context=ctx&type=type
			r.Get("/", adminHandler.BlockList)            // GET /admin/blocks?owner=uuid&context=ctx
			r.Get("/{id}", adminHandler.BlockGet)         // GET /admin/blocks/{id}
			r.Put("/{id}", adminHandler.BlockUpdate)      // PUT /admin/blocks/{id}
			r.Delete("/{id}", adminHandler.BlockDelete)   // DELETE /admin/blocks/{id}
			r.Post("/reorder", adminHandler.BlockReorder) // POST /admin/blocks/reorder
		})
		r.Route("/runs", func(r chi.Router) {
			r.Get("/", adminHandler.Runs)
			r.Post("/add", adminHandler.RunsAdd)
			r.Get("/{runCode}", adminHandler.RunOverview)
			r.Delete("/{runCode}", adminHandler.RunDelete)
			r.Post("/{runCode}/reset", adminHandler.RunReset)
		})

		r.Route("/experience", func(r chi.Router) {
			r.Get("/", adminHandler.Experience)
			r.Post("/", adminHandler.ExperiencePost)
		})

		r.Route("/quests", func(r chi.Router) {
			r.Get("/", adminHandler.Quests)
			r.Post("/new", adminHandler.QuestsCreate)
			r.Post("/import", adminHandler.ImportInstanceCreate)
			r.Post("/import/check", adminHandler.ImportCheck)
			r.Get("/{id}", adminHandler.Quests)
			r.Post("/{id}", adminHandler.Quests)
			r.Get("/{id}/switch", adminHandler.QuestSwitch)
			r.Get("/{id}/name", adminHandler.QuestsName)
			r.Get("/{id}/edit/name", adminHandler.QuestsNameEdit)
			r.Post("/{id}/edit/name", adminHandler.QuestsNameEditPost)
			r.Get("/{id}/export", adminHandler.ExportQuest)
			r.Post("/{id}/import", adminHandler.ImportInstanceUpdate)
			r.Post("/delete", adminHandler.QuestDelete)
			r.Post("/duplicate", adminHandler.QuestDuplicate)
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

		r.Route("/settings", func(r chi.Router) {
			r.Get("/", adminHandler.Settings)
			r.Get("/profile", adminHandler.SettingsProfile)
			r.Post("/profile", adminHandler.SettingsProfilePost)
			r.Get("/appearance", adminHandler.SettingsAppearance)
			r.Route("/credits", func(r chi.Router) {
				r.Get("/chart", adminHandler.SettingsCreditUsageChart)
				r.Get("/", adminHandler.SettingsCreditUsage)
			})
			r.Get("/security", adminHandler.SettingsSecurity)
			r.Post("/security", adminHandler.SettingsSecurityPost)
			r.Delete("/delete-account", adminHandler.DeleteAccount)
		})

		// Credit purchase endpoints
		r.Route("/credits", func(r chi.Router) {
			r.Post("/purchase/create-session", adminHandler.CreateCheckoutSession)
			r.Get("/success", adminHandler.CreditPurchaseSuccess)
			r.Get("/cancel", adminHandler.CreditPurchaseCancel)
		})

		r.NotFound(adminHandler.NotFound)
	})
}

func setupFacilitatorRoutes(router chi.Router, _ *slog.Logger, adminHandler *admin.Handler) {
	router.Route("/facilitator", func(r chi.Router) {
		r.Get("/login/{token}", adminHandler.FacilitatorLogin)
		r.Get("/dashboard", adminHandler.FacilitatorDashboard)
	})
}

// setupWebhookRoutes sets up webhook routes that bypass CSRF protection.
func setupWebhookRoutes(router chi.Router, adminHandler *admin.Handler) {
	// Webhook routes are registered before CSRF middleware, so they bypass it
	router.Post("/webhooks/stripe", adminHandler.StripeWebhook)
}

// setupAPIRoutes sets up public API routes that bypass CSRF protection.
func setupAPIRoutes(router chi.Router, publicHandler *public.Handler) {
	router.Route("/api/v7", func(r chi.Router) {
		r.Get("/spec", publicHandler.SpecJSON)
		r.Post("/lint", publicHandler.LintDoc)
	})
}
