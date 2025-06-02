package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/nathanhollows/Rapua/v3/internal/contextkeys"
	"github.com/nathanhollows/Rapua/v3/internal/flash"
	"github.com/nathanhollows/Rapua/v3/internal/services"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/admin"
	"github.com/nathanhollows/Rapua/v3/models"
)

type AdminHandler struct {
	Logger              *slog.Logger
	AssetGenerator      services.AssetGenerator
	AuthService         services.AuthService
	BlockService        services.BlockService
	ClueService         services.ClueService
	FacilitatorService  services.FacilitatorService
	GameManagerService  services.GameManagerService
	GameplayService     services.GameplayService
	InstanceService     services.InstanceService
	LocationService     services.LocationService
	NotificationService services.NotificationService
	TeamService         services.TeamService
	TemplateService     services.TemplateService
	UploadService       services.UploadService
	UserService         services.UserService
}

func NewAdminHandler(
	logger *slog.Logger,
	assetGenerator services.AssetGenerator,
	authService services.AuthService,
	blockService services.BlockService,
	clueService services.ClueService,
	facilitatorService services.FacilitatorService,
	gameManagerService services.GameManagerService,
	gameplayService services.GameplayService,
	instanceService services.InstanceService,
	locationService services.LocationService,
	notificationService services.NotificationService,
	teamService services.TeamService,
	templateService services.TemplateService,
	uploadService services.UploadService,
	userService services.UserService,
) *AdminHandler {
	return &AdminHandler{
		Logger:              logger,
		AssetGenerator:      assetGenerator,
		AuthService:         authService,
		BlockService:        blockService,
		ClueService:         clueService,
		FacilitatorService:  facilitatorService,
		GameManagerService:  gameManagerService,
		GameplayService:     gameplayService,
		InstanceService:     instanceService,
		LocationService:     locationService,
		NotificationService: notificationService,
		TeamService:         teamService,
		TemplateService:     templateService,
		UploadService:       uploadService,
		UserService:         userService,
	}
}

// GetUserFromContext retrieves the user from the context.
// User will always be in the context because the middleware.
func (h AdminHandler) UserFromContext(ctx context.Context) *models.User {
	return ctx.Value(contextkeys.UserKey).(*models.User)
}

func (h *AdminHandler) handleError(w http.ResponseWriter, r *http.Request, logMsg string, flashMsg string, params ...interface{}) {
	h.Logger.Error(logMsg, params...)
	err := templates.Toast(*flash.NewError(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error(logMsg+" - rendering template", "error", err)
	}
}

func (h *AdminHandler) handleSuccess(w http.ResponseWriter, r *http.Request, flashMsg string) {
	err := templates.Toast(*flash.NewSuccess(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error("rendering success template", "error", err)
	}
}

// redirect is a helper function to redirect the user to a new page.
// It accounts for htmx requests and redirects the user to the referer.
func (h AdminHandler) redirect(w http.ResponseWriter, r *http.Request, path string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", path)
		return
	}
	http.Redirect(w, r, path, http.StatusFound)
}
