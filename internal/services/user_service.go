package services

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v5/config"
	"github.com/nathanhollows/Rapua/v5/helpers"
	"github.com/nathanhollows/Rapua/v5/models"
	"github.com/nathanhollows/Rapua/v5/repositories"
	"github.com/nathanhollows/Rapua/v5/security"
)

// Password-related errors.
var (
	ErrPasswordsDoNotMatch  = errors.New("passwords do not match")
	ErrIncorrectOldPassword = errors.New("current password is incorrect")
	ErrEmptyPassword        = errors.New("password cannot be empty")
	ErrPasswordUpdateFailed = errors.New("failed to update password")
)

type UserService struct {
	instanceRepo repositories.InstanceRepository
	userRepo     repositories.UserRepository
}

func NewUserService(
	userRepository repositories.UserRepository,
	instanceRepository repositories.InstanceRepository,
) *UserService {
	return &UserService{
		instanceRepo: instanceRepository,
		userRepo:     userRepository,
	}
}

// CreateUser creates a new user in the database.
func (s *UserService) CreateUser(ctx context.Context, user *models.User, passwordConfirm string) error {
	// Confirm passwords match
	if user.Password != passwordConfirm {
		return ErrPasswordsDoNotMatch
	}

	// Hash the password
	hashedPassword, err := security.HashPassword(user.Password)
	if err != nil {
		return err
	}
	user.Password = hashedPassword

	// Generate UUID for user
	user.ID = uuid.New().String()

	// Set monthly credit limit based on email
	user.MonthlyCreditLimit = config.GetFreeCreditsForEmail(user.Email, helpers.IsEducationalEmailHeuristic)

	// Set initial free credits to the monthly limit
	user.FreeCredits = user.MonthlyCreditLimit

	return s.userRepo.Create(ctx, user)
}

// UpdateUser updates a user in the database.
func (s *UserService) UpdateUser(ctx context.Context, user *models.User) error {
	return s.userRepo.Update(ctx, user)
}

// UpdateUserProfile updates a user's profile information
// Only updates the fields that are present in the profile map.
func (s *UserService) UpdateUserProfile(ctx context.Context, user *models.User, profile map[string]string) error {
	// Update basic fields only if provided
	if name, exists := profile["name"]; exists {
		user.Name = name
	}

	// Handle nullable display name field if provided
	if displayName, exists := profile["display_name"]; exists {
		if displayName != "" {
			user.DisplayName.String = displayName
			user.DisplayName.Valid = true
		} else {
			user.DisplayName.Valid = false
		}
	}

	// Handle work type if provided
	if workType, exists := profile["work_type"]; exists {
		switch workType {
		case "other":
			otherWorkType, hasOther := profile["other_work_type"]
			if hasOther && otherWorkType != "" {
				user.WorkType.String = otherWorkType
				user.WorkType.Valid = true
			} else {
				user.WorkType.Valid = false
			}
		case "":
			user.WorkType.Valid = false
		default:
			user.WorkType.String = workType
			user.WorkType.Valid = true
		}
	}

	// Theme is handled client-side with localStorage

	// Handle share email checkbox only if provided
	if _, exists := profile["show_email"]; exists {
		user.ShareEmail = profile["show_email"] == "on"
	}

	return s.userRepo.Update(ctx, user)
}

// GetUserByEmail retrieves a user by their email address.
func (s *UserService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.userRepo.GetByEmail(ctx, email)
}

// ChangePassword changes a user's password.
func (s *UserService) ChangePassword(
	ctx context.Context,
	user *models.User,
	oldPassword, newPassword, confirmPassword string,
) error {
	// Make sure the user is using email/password authentication
	if user.Provider != models.ProviderEmail {
		return errors.New("cannot change password for SSO accounts")
	}

	// Check that new password is not empty first
	if newPassword == "" {
		return ErrEmptyPassword
	}

	// Check that new passwords match before verifying old password
	if newPassword != confirmPassword {
		return ErrPasswordsDoNotMatch
	}

	// Verify the old password
	if !security.CheckPasswordHash(oldPassword, user.Password) {
		return ErrIncorrectOldPassword
	}

	// Hash the new password
	hashedPassword, err := security.HashPassword(newPassword)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	// Update the user's password
	user.Password = hashedPassword
	err = s.userRepo.Update(ctx, user)
	if err != nil {
		return ErrPasswordUpdateFailed
	}

	return nil
}

// SwitchInstance implements InstanceService.
func (s *UserService) SwitchInstance(ctx context.Context, user *models.User, instanceID string) error {
	if user == nil {
		return ErrUserNotAuthenticated
	}

	instance, err := s.instanceRepo.GetByID(ctx, instanceID)
	if err != nil {
		return errors.New("instance not found")
	}

	if instance.IsTemplate {
		return errors.New("cannot switch to a template")
	}

	// Make sure the user has permission to switch to this instance
	if instance.UserID != user.ID {
		return ErrPermissionDenied
	}

	user.CurrentInstanceID = instance.ID
	err = s.userRepo.Update(ctx, user)
	if err != nil {
		return fmt.Errorf("updating user: %w", err)
	}

	return nil
}
