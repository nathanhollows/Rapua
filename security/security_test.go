package security_test

import (
	"testing"

	"github.com/nathanhollows/Rapua/v4/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashPassword(t *testing.T) {
	t.Parallel()
	password := "testpassword"
	hash, err := security.HashPassword(password)
	require.NoError(t, err, "hashing the password should not produce an error")

	// Ensure hash is not empty
	assert.NotEmpty(t, hash, "hashed password should not be empty")
}

func TestCheckPasswordHash(t *testing.T) {
	t.Parallel()
	password := "testpassword"
	hash, err := security.HashPassword(password)
	require.NoError(t, err, "hashing the password should not produce an error")

	// Check if the password matches the hash
	matches := security.CheckPasswordHash(password, hash)
	assert.True(t, matches, "password should match the hash")

	// Ensure a different password doesn't match the hash
	incorrectPassword := "wrongpassword"
	matches = security.CheckPasswordHash(incorrectPassword, hash)
	assert.False(t, matches, "incorrect password should not match the hash")
}

func TestHashPasswordError(t *testing.T) {
	t.Parallel()
	// Test an empty password, bcrypt will not error on this but it's good practice to ensure non-empty passwords are enforced elsewhere
	password := ""
	hash, err := security.HashPassword(password)
	require.NoError(t, err, "hashing an empty password should not produce an error if allowed")

	// Ensure the hash is still valid for empty string comparison, this checks bcrypt consistency
	matches := security.CheckPasswordHash(password, hash)
	assert.True(t, matches, "empty password should match the hash of the empty password if allowed to hash")
}
