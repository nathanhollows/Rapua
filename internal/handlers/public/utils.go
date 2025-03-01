package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/nathanhollows/Rapua/v3/internal/flash"
	"github.com/nathanhollows/Rapua/v3/internal/services"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/public"
	"github.com/nathanhollows/Rapua/v3/models"
)

type FindTemplateService interface {
	GetByID(ctx context.Context, id string) (*models.Instance, error)
	GetShareLink(ctx context.Context, id string) (*models.ShareLink, error)
}

type PublicHandler struct {
	Logger          *slog.Logger
	AuthService     services.AuthService
	EmailService    services.EmailService
	TemplateService FindTemplateService
	UserService     services.UserService
}

func NewPublicHandler(
	logger *slog.Logger,
	authService services.AuthService,
	emailService services.EmailService,
	templateService FindTemplateService,
	userService services.UserService,
) *PublicHandler {
	return &PublicHandler{
		Logger:          logger,
		AuthService:     authService,
		EmailService:    emailService,
		TemplateService: templateService,
		UserService:     userService,
	}
}

func (h *PublicHandler) handleError(w http.ResponseWriter, r *http.Request, logMsg string, flashMsg string, params ...interface{}) {
	h.Logger.Error(logMsg, params...)
	err := templates.Toast(*flash.NewError(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.Logger.Error(logMsg+" - rendering template", "error", err)
	}
}
