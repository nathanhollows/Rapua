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

// ErrPasswordsDoNotMatch is returned when the passwords do not match.
var (
	ErrPasswordsDoNotMatch = errors.New("passwords do not match")
)

type UserService interface {
	// CreateUser creates a new user
	CreateUser(ctx context.Context, user *models.User, passwordConfirm string) error

	// GetUserByEmail retrieves a user by their email address
	GetUserByEmail(ctx context.Context, email string) (*models.User, error)

	// UpdateUser updates a user
	UpdateUser(ctx context.Context, user *models.User) error
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
