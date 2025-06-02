package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v3/db"
	"github.com/nathanhollows/Rapua/v3/models"
	"github.com/uptrace/bun"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type UserRepository interface {
	// Create creates a new user in the database
	Create(ctx context.Context, user *models.User) error
	// Update updates a user in the database
	Update(ctx context.Context, user *models.User) error
	// FindByEmail retrieves a user by their email address
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	// FindByID fetches a user by their ID from the database.
	FindByID(ctx context.Context, userID string) (*models.User, error)
	// FindByEmailAndProvider retrieves a user by their email address and provider
	FindByEmailAndProvider(ctx context.Context, email, provider string) (*models.User, error)
	// ResetEventBoost resets the event boost for a user
	ResetEventBoost(ctx context.Context, userID string) error
}

type userRepository struct{}

func NewUserRepository() UserRepository {
	return &userRepository{}
}

// Update the user in the database
func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	_, err := db.DB.NewUpdate().
		Model(user).
		WherePK().
		Exec(ctx)
	return err
}

// Create creates a new user in the database
func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	if user.ID == "" {
		uuid := uuid.New()
		user.ID = uuid.String()
	}

	_, err := db.DB.NewInsert().Model(user).Exec(ctx)
	return err
}

// FindByEmail retrieves a user by their email address
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	err := db.DB.NewSelect().
		Model(user).
		Where("email = ?", email).
		Relation("CurrentInstance").
		Relation("Instances").
		Scan(ctx)
	return user, err
}

// FindByID fetches a user by their ID from the database.
func (r *userRepository) FindByID(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	err := db.DB.NewSelect().
		Model(&user).
		Where("user.id = ?", userID).
		Relation("CurrentInstance").
		Relation("CurrentInstance.Settings").
		Relation("CurrentInstance.Teams").
		// Locations ordered by Order
		Relation("CurrentInstance.Locations", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Order("order ASC")
		}).
		Relation("CurrentInstance.Locations.Marker").
		Relation("Instances").
		Scan(ctx)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

// FindByEmailAndProvider retrieves a user by their email address and provider
func (r *userRepository) FindByEmailAndProvider(ctx context.Context, email, provider string) (*models.User, error) {
	user := &models.User{}
	err := db.DB.NewSelect().
		Model(user).
		Where("email = ?", email).
		Where("provider = ? OR provider = ''", provider).
		Relation("CurrentInstance").
		Relation("Instances").
		Scan(ctx)
	return user, err
}

// ResetEventBoost resets the event boost for a user
func (r *userRepository) ResetEventBoost(ctx context.Context, userID string) error {
	_, err := db.DB.NewUpdate().
		Model(&models.User{ID: userID}).
		Column("event_boost_expiry").
		Set("event_boost_expiry = NULL").
		Exec(ctx)
	return err
}
