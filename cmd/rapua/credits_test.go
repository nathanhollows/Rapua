package main

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/nathanhollows/Rapua/v6/db"
	"github.com/nathanhollows/Rapua/v6/internal/migrations"
	"github.com/nathanhollows/Rapua/v6/internal/services"
	"github.com/nathanhollows/Rapua/v6/models"
	"github.com/nathanhollows/Rapua/v6/repositories"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

func setupTestDB(t *testing.T) (*bun.DB, func()) {
	t.Helper()

	t.Setenv("DB_CONNECTION", "file::memory:?cache=shared")
	t.Setenv("DB_TYPE", "sqlite3")

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	dbc := db.MustOpen(logger)

	migrator := migrate.NewMigrator(dbc, migrations.Migrations)
	if err := migrator.Init(context.Background()); err != nil {
		t.Fatalf("failed to init migrations: %v", err)
	}
	if _, err := migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	cleanup := func() {
		if _, err := migrator.Rollback(context.Background()); err != nil {
			t.Logf("failed to rollback migrations: %v", err)
		}
		dbc.Close()
	}

	return dbc, cleanup
}

func setupCreditsTest(t *testing.T) (*services.CreditService, repositories.UserRepository, func()) {
	t.Helper()
	dbc, cleanup := setupTestDB(t)

	creditRepo := repositories.NewCreditRepository(dbc)
	teamStartLogRepo := repositories.NewTeamStartLogRepository(dbc)
	userRepo := repositories.NewUserRepository(dbc)
	transactor := db.NewTransactor(dbc)

	creditService := services.NewCreditService(transactor, creditRepo, teamStartLogRepo, userRepo)

	return creditService, userRepo, cleanup
}

func TestAddCreditsToUser_Success(t *testing.T) {
	creditService, userRepo, cleanup := setupCreditsTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user
	testUser := &models.User{
		Email:       "test@example.com",
		PaidCredits: 10,
	}
	err := userRepo.Create(ctx, testUser)
	require.NoError(t, err)

	params := addCreditsParams{
		Email:        "test@example.com",
		Credits:      100,
		Prefix:       "Admin",
		CustomReason: "Test credit",
	}

	err = addCreditsToUser(ctx, params, creditService, userRepo)
	require.NoError(t, err)

	// Verify credits were added
	user, err := userRepo.GetByEmail(ctx, "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, 110, user.PaidCredits, "Credits should be added to existing balance")
}

func TestAddCreditsToUser_InvalidPrefix(t *testing.T) {
	creditService, userRepo, cleanup := setupCreditsTest(t)
	defer cleanup()

	ctx := context.Background()

	params := addCreditsParams{
		Email:   "test@example.com",
		Credits: 100,
		Prefix:  "Invalid",
	}

	err := addCreditsToUser(ctx, params, creditService, userRepo)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid prefix")
}

func TestAddCreditsToUser_NegativeAmount(t *testing.T) {
	creditService, userRepo, cleanup := setupCreditsTest(t)
	defer cleanup()

	ctx := context.Background()

	params := addCreditsParams{
		Email:   "test@example.com",
		Credits: -10,
		Prefix:  "Admin",
	}

	err := addCreditsToUser(ctx, params, creditService, userRepo)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "amount must be greater than 0")
}

func TestAddCreditsToUser_ZeroAmount(t *testing.T) {
	creditService, userRepo, cleanup := setupCreditsTest(t)
	defer cleanup()

	ctx := context.Background()

	params := addCreditsParams{
		Email:   "test@example.com",
		Credits: 0,
		Prefix:  "Admin",
	}

	err := addCreditsToUser(ctx, params, creditService, userRepo)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "amount must be greater than 0")
}

func TestAddCreditsToUser_UserNotFound(t *testing.T) {
	creditService, userRepo, cleanup := setupCreditsTest(t)
	defer cleanup()

	ctx := context.Background()

	params := addCreditsParams{
		Email:   "nonexistent@example.com",
		Credits: 100,
		Prefix:  "Admin",
	}

	err := addCreditsToUser(ctx, params, creditService, userRepo)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "user not found")
}

func TestAddCreditsToUser_GiftPrefix(t *testing.T) {
	creditService, userRepo, cleanup := setupCreditsTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user
	testUser := &models.User{
		Email:       "test@example.com",
		PaidCredits: 25,
	}
	err := userRepo.Create(ctx, testUser)
	require.NoError(t, err)

	params := addCreditsParams{
		Email:        "test@example.com",
		Credits:      50,
		Prefix:       "Gift",
		CustomReason: "Welcome bonus",
	}

	err = addCreditsToUser(ctx, params, creditService, userRepo)
	require.NoError(t, err)

	// Verify credits were added
	user, err := userRepo.GetByEmail(ctx, "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, 75, user.PaidCredits)
}

func TestAddCreditsToUser_NoCustomReason(t *testing.T) {
	creditService, userRepo, cleanup := setupCreditsTest(t)
	defer cleanup()

	ctx := context.Background()

	// Create test user
	testUser := &models.User{
		Email:       "test@example.com",
		PaidCredits: 5,
	}
	err := userRepo.Create(ctx, testUser)
	require.NoError(t, err)

	params := addCreditsParams{
		Email:   "test@example.com",
		Credits: 75,
		Prefix:  "Admin",
		// No CustomReason - should use just the prefix
	}

	err = addCreditsToUser(ctx, params, creditService, userRepo)
	require.NoError(t, err)

	// Verify credits were added
	user, err := userRepo.GetByEmail(ctx, "test@example.com")
	require.NoError(t, err)
	assert.Equal(t, 80, user.PaidCredits)
}
