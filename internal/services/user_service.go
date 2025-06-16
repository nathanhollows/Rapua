package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v3/db"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/nathanhollows/Rapua/v3/repositories"
	"github.com/nathanhollows/Rapua/v3/security"
)

// Password-related errors
var (
	ErrPasswordsDoNotMatch    = errors.New("passwords do not match")
	ErrIncorrectOldPassword   = errors.New("current password is incorrect")
	ErrEmptyPassword          = errors.New("password cannot be empty")
	ErrPasswordUpdateFailed   = errors.New("failed to update password")
)

type UserService interface {
	// CreateUser creates a new user
	CreateUser(ctx context.Context, user *models.User, passwordConfirm string) error

	// GetUserByEmail retrieves a user by their email address
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)

	// UpdateUser updates a user
	UpdateUser(ctx context.Context, user *models.User) error
	
	// UpdateUserProfile updates a user's profile with form data
	UpdateUserProfile(ctx context.Context, user *models.User, profile map[string]string) error
	
	// ChangePassword changes a user's password
	ChangePassword(ctx context.Context, user *models.User, oldPassword, newPassword, confirmPassword string) error
	
	// SwitchInstance switches the user's current instance
	SwitchInstance(ctx context.Context, user *models.User, instanceID string) error

	// DeleteUser deletes a user
	DeleteUser(ctx context.Context, userID string) error
}

type userService struct {
	transactor   db.Transactor
	instanceRepo repositories.InstanceRepository
	userRepo     repositories.UserRepository
}

func NewUserService(transactor db.Transactor, userRepository repositories.UserRepository, instanceRepository repositories.InstanceRepository) UserService {
	return &userService{
		transactor:   transactor,
		instanceRepo: instanceRepository,
		userRepo:     userRepository,
	}
}

// CreateUser creates a new user in the database.
func (s *userService) CreateUser(ctx context.Context, user *models.User, passwordConfirm string) error {
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

	return s.userRepo.Create(ctx, user)
}

// UpdateUser updates a user in the database.
func (s *userService) UpdateUser(ctx context.Context, user *models.User) error {
	return s.userRepo.Update(ctx, user)
}

// UpdateUserProfile updates a user's profile information
// Only updates the fields that are present in the profile map
func (s *userService) UpdateUserProfile(ctx context.Context, user *models.User, profile map[string]string) error {
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
		if workType == "other" {
			otherWorkType, hasOther := profile["other_work_type"]
			if hasOther && otherWorkType != "" {
				user.WorkType.String = otherWorkType
				user.WorkType.Valid = true
			} else {
				user.WorkType.Valid = false
			}
		} else if workType != "" {
			user.WorkType.String = workType
			user.WorkType.Valid = true
		} else {
			user.WorkType.Valid = false
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
func (s *userService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	return s.userRepo.GetByEmail(ctx, email)
}

// DeleteUser deletes a user from the database.
func (s *userService) DeleteUser(ctx context.Context, userID string) error {
	tx, err := s.transactor.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}

	// Ensure rollback on failure
	defer func() {
		if p := recover(); p != nil {
			err := tx.Rollback()
			if err != nil {
				fmt.Println("failed to rollback transaction:", err)
			}
			panic(p)
		}
	}()

	err = s.userRepo.Delete(ctx, tx, userID)
	if err != nil {
		if err := tx.Rollback(); err != nil {
			return err
		}
	}

	err = s.instanceRepo.DeleteByUser(ctx, tx, userID)
	if err != nil {
		if err := tx.Rollback(); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// ChangePassword changes a user's password
func (s *userService) ChangePassword(ctx context.Context, user *models.User, oldPassword, newPassword, confirmPassword string) error {
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
func (s *userService) SwitchInstance(ctx context.Context, user *models.User, instanceID string) error {
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
