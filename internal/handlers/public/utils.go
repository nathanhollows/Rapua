package public

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/markbates/goth"
	"github.com/nathanhollows/Rapua/v4/internal/flash"
	templates "github.com/nathanhollows/Rapua/v4/internal/templates/public"
	"github.com/nathanhollows/Rapua/v4/models"
)

type DeleteService interface {
	DeleteUser(ctx context.Context, userID string) error
}

type TemplateService interface {
	GetByID(ctx context.Context, id string) (*models.Instance, error)
	GetShareLink(ctx context.Context, id string) (*models.ShareLink, error)
}

// IdentityService handles all authentication operations.
type IdentityService interface {
	// Core authentication
	AuthenticateUser(ctx context.Context, email, password string) (*models.User, error)
	GetAuthenticatedUser(r *http.Request) (*models.User, error)
	IsUserAuthenticated(r *http.Request) bool

	// Email verification
	VerifyEmail(ctx context.Context, token string) error
	SendEmailVerification(ctx context.Context, user *models.User) error

	// OAuth operations
	AllowGoogleLogin() bool
	OAuthLogin(ctx context.Context, provider string, user goth.User) (*models.User, error)
	CheckUserRegisteredWithOAuth(ctx context.Context, provider, userID string) (*models.User, error)
	CreateUserWithOAuth(ctx context.Context, user goth.User) (*models.User, error)
	CompleteUserAuth(w http.ResponseWriter, r *http.Request) (*models.User, error)
}

type EmailService interface {
	SendContactEmail(ctx context.Context, name, contactEmail, content string) error
}

type UserService interface {
	CreateUser(ctx context.Context, user *models.User, passwordConfirm string) error
}

type PublicHandler struct {
	logger          *slog.Logger
	identityService IdentityService
	deleteService   DeleteService
	emailService    EmailService
	templateService TemplateService
	userService     UserService
}

func NewPublicHandler(
	logger *slog.Logger,
	identityService IdentityService,
	deleteService DeleteService,
	emailService EmailService,
	templateService TemplateService,
	userService UserService,
) *PublicHandler {
	return &PublicHandler{
		logger:          logger,
		identityService: identityService,
		deleteService:   deleteService,
		emailService:    emailService,
		templateService: templateService,
		userService:     userService,
	}
}

func (h *PublicHandler) handleError(
	w http.ResponseWriter,
	r *http.Request,
	logMsg string,
	flashMsg string,
	params ...interface{},
) {
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

// GetIdentityService returns the identity service for use in middleware.
func (h *PublicHandler) GetIdentityService() IdentityService {
	return h.identityService
}
