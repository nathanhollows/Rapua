package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/nathanhollows/Rapua/v3/internal/sessions"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
	"github.com/nathanhollows/Rapua/v3/security"
)

var (
	ErrUserNotAuthenticated = errors.New("user not authenticated")
	ErrSessionNotFound      = errors.New("session not found")
	ErrInvalidToken         = errors.New("invalid token")
	ErrTokenExpired         = errors.New("token expired")
	ErrUserAlreadyVerified  = errors.New("user already verified")
	ErrRateLimitExceeded    = errors.New("rate limit exceeded")
)

type AuthService interface {
	AuthenticateUser(ctx context.Context, email, password string) (*models.User, error)
	GetAuthenticatedUser(r *http.Request) (*models.User, error)
	IsUserAuthenticated(r *http.Request) bool
	AllowGoogleLogin() bool
	OAuthLogin(ctx context.Context, provider string, user goth.User) (*models.User, error)
	CheckUserRegisteredWithOAuth(ctx context.Context, provider, userID string) (*models.User, error)
	CreateUserWithOAuth(ctx context.Context, user goth.User) (*models.User, error)
	CompleteUserAuth(w http.ResponseWriter, r *http.Request) (*models.User, error)
	VerifyEmail(ctx context.Context, token string) error
	SendEmailVerification(ctx context.Context, user *models.User) error
}

type authService struct {
	userRepository repositories.UserRepository
	emailService   EmailService
}

func NewAuthService(userRepository repositories.UserRepository) AuthService {
	return &authService{
		userRepository: userRepository,
		emailService:   NewEmailService(),
	}
}

// AuthenticateUser authenticates the user with the given email and password.
func (s *authService) AuthenticateUser(ctx context.Context, email, password string) (*models.User, error) {
	if email == "" || password == "" {
		return nil, errors.New("email and password are required")
	}

	user, err := s.userRepository.GetByEmail(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("error getting user by email: %w", err)
	}

	if !security.CheckPasswordHash(password, user.Password) {
		return nil, errors.New("invalid email or password")
	}

	return user, nil
}

// IsUserAuthenticated checks if the user is authenticated.
func (s *authService) IsUserAuthenticated(r *http.Request) bool {
	session, err := sessions.Get(r, "admin")
	if err != nil {
		return false
	}

	_, ok := session.Values["user_id"].(string)
	return ok
}

// GetAuthenticatedUser retrieves the authenticated user from the session.
func (s *authService) GetAuthenticatedUser(r *http.Request) (*models.User, error) {
	session, err := sessions.Get(r, "admin")
	if err != nil {
		return nil, err
	}

	userID, ok := session.Values["user_id"].(string)
	if !ok || userID == "" {
		return nil, errors.New("user not authenticated")
	}

	user, err := s.userRepository.GetByID(r.Context(), userID)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// Check if the system allows google login (env var set).
func (s *authService) AllowGoogleLogin() bool {
	provider, err := goth.GetProvider("google")
	return err == nil && provider != nil
}

// OAuthLogin handles User Login via OAuth.
func (s *authService) OAuthLogin(ctx context.Context, provider string, oauthUser goth.User) (*models.User, error) {
	existingUser, err := s.userRepository.GetByEmail(ctx, oauthUser.Email)
	if err != nil {
		// User doesn't exist, create a new one
		newUser, err := s.CreateUserWithOAuth(ctx, oauthUser)
		if err != nil {
			return nil, fmt.Errorf("creating user with OAuth: %w", err)
		}
		return newUser, nil
	}

	return existingUser, nil
}

// CheckUserRegisteredWithOAuth looks for user already registered with OAuth.
func (s *authService) CheckUserRegisteredWithOAuth(ctx context.Context, provider, email string) (*models.User, error) {
	user, err := s.userRepository.GetByEmailAndProvider(ctx, email, provider)
	if err != nil {
		return nil, fmt.Errorf("getting user by email and provider: %w", err)
	}

	return user, nil
}

// CreateUserWithOAuth creates a new user if logging in with OAuth for the first time.
func (s *authService) CreateUserWithOAuth(ctx context.Context, user goth.User) (*models.User, error) {
	var provider models.Provider
	switch user.Provider {
	case "google":
		provider = models.ProviderGoogle
	case "email":
		provider = models.ProviderEmail
	default:
		return nil, fmt.Errorf("unsupported provider: %s", user.Provider)
	}

	uuid := uuid.New()
	newUser := models.User{
		ID:       uuid.String(),
		Name:     user.Name,
		Email:    user.Email,
		Password: "",
		Provider: provider,
	}

	err := s.userRepository.Create(ctx, &newUser)
	if err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	return &newUser, nil
}

// CompleteUserAuth completes the user authentication process.
func (s *authService) CompleteUserAuth(w http.ResponseWriter, r *http.Request) (*models.User, error) {
	gothUser, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		return nil, fmt.Errorf("completing user auth: %w", err)
	}

	user, err := s.OAuthLogin(r.Context(), gothUser.Provider, gothUser)
	if err != nil {
		return nil, fmt.Errorf("OAuth login: %w", err)
	}

	return user, nil
}

// VerifyEmail verifies the user's email address.
func (s *authService) VerifyEmail(ctx context.Context, token string) error {
	user, err := s.userRepository.GetByEmailToken(ctx, token)
	if err != nil {
		return ErrInvalidToken
	}

	if user.EmailToken != token {
		return ErrInvalidToken
	}

	if user.EmailTokenExpiry.Time.Before(time.Now()) {
		return ErrTokenExpired
	}

	user.EmailVerified = true
	user.EmailToken = ""
	user.EmailTokenExpiry = sql.NullTime{}

	err = s.userRepository.Update(ctx, user)
	if err != nil {
		return fmt.Errorf("updating user: %w", err)
	}

	return nil
}

// SendVerificationEmail sends a verification email to the user.
func (s *authService) SendEmailVerification(ctx context.Context, user *models.User) error {
	// If the user is already verified, return an error
	if user.EmailVerified {
		return ErrUserAlreadyVerified
	}

	// Reset the email token and expiry
	token := uuid.New().String()
	user.EmailToken = token
	user.EmailTokenExpiry = sql.NullTime{
		Time:  time.Now().Add(15 * time.Minute),
		Valid: true,
	}

	err := s.userRepository.Update(ctx, user)
	if err != nil {
		return fmt.Errorf("updating user: %w", err)
	}

	_, err = s.emailService.SendVerificationEmail(ctx, *user)
	if err != nil {
		return fmt.Errorf("sending verification email: %w", err)
	}

	// Send email
	return nil
}
