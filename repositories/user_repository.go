package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/uptrace/bun"
)

var (
	ErrUserNotFound = errors.New("user not found")
)

type UserRepository interface {
	// Create creates a new user in the database
	Create(ctx context.Context, user *models.User) error

	// GetByEmail retrieves a user by their email address
	GetByEmail(ctx context.Context, email string) (*models.User, error)
	// GetByEmailToken retrieves a user by their email token
	GetByEmailToken(ctx context.Context, token string) (*models.User, error)
	// GetByID fetches a user by their ID from the database.
	GetByID(ctx context.Context, userID string) (*models.User, error)
	// GetByEmailAndProvider retrieves a user by their email address and provider
	GetByEmailAndProvider(ctx context.Context, email, provider string) (*models.User, error)

	// Update updates a user in the database
	Update(ctx context.Context, user *models.User) error

	// Delete deletes a user from the database
	// Requires a transaction as related data will also need to be deleted
	Delete(ctx context.Context, tx *bun.Tx, userID string) error
}

type userRepository struct {
	db *bun.DB
}

func NewUserRepository(db *bun.DB) UserRepository {
	return &userRepository{
		db: db,
	}
}

// Update the user in the database.
func (r *userRepository) Update(ctx context.Context, user *models.User) error {
	user.UpdatedAt = time.Now().UTC()
	res, err := r.db.NewUpdate().
		Model(user).
		Column(
			// ID is immutable
			// Provider is immutable
			"name",
			"display_name",
			"email_token",
			"email_token_expiry",
			"email_verified",
			"password",
			"current_instance_id",
			"share_email",
			"work_type",
			"stripe_customer_id",
			"updated_at").
		WherePK().
		Exec(ctx)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if rows == 0 {
		return ErrUserNotFound
	}
	return err
}

// Create creates a new user in the database.
func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	if user.ID == "" {
		uuid := uuid.New()
		user.ID = uuid.String()
	}

	_, err := r.db.NewInsert().Model(user).Exec(ctx)
	return err
}

// GetByEmail retrieves a user by their email address.
func (r *userRepository) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	user := &models.User{}
	err := r.db.NewSelect().
		Model(user).
		Where("email = ?", email).
		Relation("CurrentInstance").
		Relation("Instances", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("is_template = ?", false)
		}).
		Scan(ctx)
	return user, err
}

// GetByEmailToken retrieves a user by their email token.
func (r *userRepository) GetByEmailToken(ctx context.Context, token string) (*models.User, error) {
	user := &models.User{}
	err := r.db.NewSelect().
		Model(user).
		Where("email_token = ?", token).
		Scan(ctx)
	return user, err
}

// GetByID fetches a user by their ID from the database.
func (r *userRepository) GetByID(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	err := r.db.NewSelect().
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
		Relation("Instances", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("is_template = ?", false)
		}).
		Scan(ctx)
	if err != nil {
		return nil, ErrUserNotFound
	}
	return &user, nil
}

// GetByEmailAndProvider retrieves a user by their email address and provider.
func (r *userRepository) GetByEmailAndProvider(ctx context.Context, email, provider string) (*models.User, error) {
	user := &models.User{}
	err := r.db.NewSelect().
		Model(user).
		Where("email = ?", email).
		Where("provider = ? OR provider = ''", provider).
		Relation("CurrentInstance").
		Relation("Instances", func(q *bun.SelectQuery) *bun.SelectQuery {
			return q.Where("is_template = ?", false)
		}).
		Scan(ctx)
	return user, err
}

// Delete deletes a user from the database.
func (r *userRepository) Delete(ctx context.Context, tx *bun.Tx, userID string) error {
	user := &models.User{ID: userID}
	res, err := tx.NewDelete().
		Model(user).
		Where("id = ?", userID).
		Exec(ctx)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if rows == 0 {
		return ErrUserNotFound
	}
	if err != nil {
		return err
	}

	return nil
}
