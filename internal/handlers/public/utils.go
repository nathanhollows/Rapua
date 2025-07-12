package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/markbates/goth"
	"github.com/nathanhollows/Rapua/v3/internal/flash"
	"github.com/nathanhollows/Rapua/v3/internal/services"
	templates "github.com/nathanhollows/Rapua/v3/internal/templates/public"
	"github.com/nathanhollows/Rapua/v3/models"
)

type DeleteService interface {
	DeleteUser(ctx context.Context, userID string) error
}

type FindTemplateService interface {
	GetByID(ctx context.Context, id string) (*models.Instance, error)
	GetShareLink(ctx context.Context, id string) (*models.ShareLink, error)
}

// UserAuthenticator handles core authentication operations
type UserAuthenticator interface {
	AuthenticateUser(ctx context.Context, email, password string) (*models.User, error)
	GetAuthenticatedUser(r *http.Request) (*models.User, error)
	IsUserAuthenticated(r *http.Request) bool
}

// OAuthService manages OAuth-specific authentication flows
type OAuthService interface {
	AllowGoogleLogin() bool
	OAuthLogin(ctx context.Context, provider string, user goth.User) (*models.User, error)
	CheckUserRegisteredWithOAuth(ctx context.Context, provider, userID string) (*models.User, error)
	CreateUserWithOAuth(ctx context.Context, user goth.User) (*models.User, error)
	CompleteUserAuth(w http.ResponseWriter, r *http.Request) (*models.User, error)
}

// EmailVerificationService handles email-related authentication tasks
type EmailVerificationService interface {
	VerifyEmail(ctx context.Context, token string) error
	SendEmailVerification(ctx context.Context, user *models.User) error
}

// AuthService (optional) can compose the individual services if needed
type AuthService interface {
	UserAuthenticator
	OAuthService
	EmailVerificationService
}

type PublicHandler struct {
	logger          *slog.Logger
	AuthService     AuthService
	deleteService   DeleteService
	emailService    services.EmailService
	templateService FindTemplateService
	userService     services.UserService
}

func NewPublicHandler(
	logger *slog.Logger,
	authService AuthService,
	deleteService DeleteService,
	emailService services.EmailService,
	templateService FindTemplateService,
	userService services.UserService,
) *PublicHandler {
	return &PublicHandler{
		logger:          logger,
		AuthService:     authService,
		deleteService:   deleteService,
		emailService:    emailService,
		templateService: templateService,
		userService:     userService,
	}
}

func (h *PublicHandler) handleError(w http.ResponseWriter, r *http.Request, logMsg string, flashMsg string, params ...interface{}) {
	h.logger.Error(logMsg, params...)
	err := templates.Toast(*flash.NewError(flashMsg)).Render(r.Context(), w)
	if err != nil {
		h.logger.Error(logMsg+" - rendering template", "error", err)
	}
}

// redirect is a helper function to redirect the user to a new page.
// It accounts for htmx requests and redirects the user to the referer.
func (h *PublicHandler) redirect(w http.ResponseWriter, r *http.Request, path string) {
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", path)
		return
	}
	http.Redirect(w, r, path, http.StatusFound)
}
