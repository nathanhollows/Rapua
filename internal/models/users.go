package models

import (
	"context"
	"database/sql"
	"errors"

	"github.com/nathanhollows/Rapua/pkg/db"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	baseModel

	ID               string       `bun:",unique,pk,type:varchar(36)" json:"id"`
	Name             string       `bun:",type:varchar(255)" json:"name"`
	Email            string       `bun:",unique,pk" json:"email"`
	EmailVerified    bool         `bun:",type:boolean" json:"email_verified"`
	EmailToken       string       `bun:",type:varchar(36)" json:"email_token"`
	EmailTokenExpiry sql.NullTime `bun:",nullzero" json:"email_token_expiry"`
	Password         string       `bun:",type:varchar(255)" json:"password"`
	Provider         string       `bun:",type:varchar(255)" json:"provider"`

	Instances         Instances `bun:"rel:has-many,join:id=user_id" json:"instances"`
	CurrentInstanceID string    `bun:",type:varchar(36)" json:"current_instance_id"`
	CurrentInstance   Instance  `bun:"rel:has-one,join:current_instance_id=id" json:"current_instance"`
}

type Users []*User

// Save the user to the database
func (u *User) Save(ctx context.Context) error {
	_, err := db.DB.NewInsert().Model(u).Exec(ctx)
	return err
}

// Update the user in the database
func (u *User) Update(ctx context.Context) error {
	_, err := db.DB.NewUpdate().Model(u).WherePK("id").Exec(ctx)
	return err
}

// AuthenticateUser checks the user's credentials and returns the user if they are valid
func AuthenticateUser(ctx context.Context, email, password string) (*User, error) {
	// Find the user by email
	user, err := FindUserByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	// Check the password
	if !user.checkPassword(password) {
		return nil, errors.New("Invalid password")
	} else {
		return user, nil
	}
}

// FindUserByEmail finds a user by their email address
func FindUserByEmail(ctx context.Context, email string) (*User, error) {
	// Find the user by email
	user := &User{}
	err := db.DB.NewSelect().
		Model(user).
		Where("email = ?", email).
		Relation("CurrentInstance").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// FindUserByID finds a user by their user id
func FindUserByID(ctx context.Context, userID string) (*User, error) {
	// Find the user by user id
	user := &User{}
	err := db.DB.NewSelect().
		Model(user).
		Where("User.user_id = ?", userID).
		Relation("CurrentInstance").
		Relation("Instances").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// CheckPassword checks if the given password is correct
func (u *User) checkPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(password))
	if err != nil {
		return false
	}
	return true
}

// hashAndSalt hashes and salts the given password
func hashAndSalt(password string) string {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return ""
	}

	return string(hash)
}
