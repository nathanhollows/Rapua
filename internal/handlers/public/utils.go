package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/markbates/goth"
	"github.com/nathanhollows/Rapua/v3/internal/flash"
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

// IdentityService handles core authentication operations
type IdentityService interface {
	AuthenticateUser(ctx context.Context, email, password string) (*models.User, error)
	GetAuthenticatedUser(r *http.Request) (*models.User, error)
	IsUserAuthenticated(r *http.Request) bool
	VerifyEmail(ctx context.Context, token string) error
	SendEmailVerification(ctx context.Context, user *models.User) error
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
type EmailService interface {
	SendContactEmail(ctx context.Context, name, contactEmail, content string) error
}

type UserService interface {
	// CreateUser creates a new user
	CreateUser(ctx context.Context, user *models.User, passwordConfirm string) error
}

// AuthService (optional) can compose the individual services if needed
type AuthService interface {
	OAuthService
}

type PublicHandler struct {
	logger          *slog.Logger
	AuthService     AuthService
	deleteService   DeleteService
	emailService    EmailService
	IdentityService IdentityService
	templateService FindTemplateService
	userService     UserService
}

func NewPublicHandler(
	logger *slog.Logger,
	authService AuthService,
	deleteService DeleteService,
	emailService EmailService,
	identityService IdentityService,
	templateService FindTemplateService,
	userService UserService,
) *PublicHandler {
	return &PublicHandler{
		logger:          logger,
		AuthService:     authService,
		deleteService:   deleteService,
		emailService:    emailService,
		IdentityService: identityService,
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
