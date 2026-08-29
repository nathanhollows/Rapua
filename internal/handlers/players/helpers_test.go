package players_test

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/nathanhollows/Rapua/v8/internal/db"
	"github.com/nathanhollows/Rapua/v8/internal/migrations"
	"github.com/nathanhollows/Rapua/v8/internal/repositories"
	"github.com/nathanhollows/Rapua/v8/internal/services"
	"github.com/nathanhollows/Rapua/v8/models"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/migrate"
)

func newTLogger(t *testing.T) *slog.Logger {
	return slog.New(slog.NewTextHandler(testWriter{t}, nil))
}

type testWriter struct{ t *testing.T }

func (w testWriter) Write(p []byte) (int, error) {
	w.t.Logf("%s", p)
	return len(p), nil
}

func setupDB(t *testing.T) (*bun.DB, func()) {
	t.Helper()
	t.Setenv("DB_CONNECTION", "file::memory:?cache=shared")
	t.Setenv("DB_TYPE", "sqlite3")
	dbc := db.MustOpen(newTLogger(t))
	ctx := context.Background()

	migrator := migrate.NewMigrator(dbc, migrations.Migrations)
	if err := migrator.Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := migrator.Lock(ctx); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := migrator.Unlock(ctx); err != nil {
			t.Fatal(err)
		}
	}()
	if _, err := migrator.Migrate(ctx); err != nil {
		t.Fatal(err)
	}

	return dbc, func() { dbc.Close() }
}

// testParents holds a user owning a quest, the minimum a run needs to exist.
type testParents struct {
	UserID  string
	QuestID string
}

func createTestParents(t *testing.T, dbc *bun.DB) testParents {
	t.Helper()
	ctx := context.Background()

	user := &models.User{ID: gofakeit.UUID(), Name: gofakeit.Name(), Email: gofakeit.Email()}
	if _, err := dbc.NewInsert().Model(user).Exec(ctx); err != nil {
		t.Fatalf("createTestParents: insert user: %v", err)
	}

	quest := &models.Quest{ID: gofakeit.UUID(), UserID: user.ID, Name: "test-quest"}
	if _, err := dbc.NewInsert().Model(quest).Exec(ctx); err != nil {
		t.Fatalf("createTestParents: insert quest: %v", err)
	}

	return testParents{UserID: user.ID, QuestID: quest.ID}
}

// createTestRun inserts a run that has not yet started, so StartPlaying still
// has a credit to deduct and a HasStarted flag to flip.
func createTestRun(t *testing.T, dbc *bun.DB, questID string) string {
	t.Helper()
	// Lookups uppercase the code first, so a lowercase insert would never match.
	code := strings.ToUpper(gofakeit.LetterN(6))
	run := &models.Run{ID: gofakeit.UUID(), Code: code, Name: "test-run", QuestID: questID}
	if _, err := dbc.NewInsert().Model(run).Exec(context.Background()); err != nil {
		t.Fatalf("createTestRun: %v", err)
	}
	return code
}

// zeroCredits drains a user's free credits, so the next run they start hits
// ErrInsufficientCredits instead of succeeding.
func zeroCredits(t *testing.T, dbc *bun.DB, userID string) {
	t.Helper()
	_, err := dbc.NewUpdate().
		Model((*models.User)(nil)).
		Set("free_credits = 0").
		Where("id = ?", userID).
		Exec(context.Background())
	if err != nil {
		t.Fatalf("zeroCredits: %v", err)
	}
}

// setupRunService wires a real RunService against the same in-memory database,
// so PlayWithCode is exercised through its actual dependency rather than a mock
// standing in for what StartPlaying decides.
func setupRunService(t *testing.T) (*services.RunService, *bun.DB, func()) {
	t.Helper()
	dbc, cleanup := setupDB(t)

	transactor := db.NewTransactor(dbc)
	blockStateRepo := repositories.NewBlockStateRepository(dbc)
	runRepo := repositories.NewRunRepository(dbc)
	creditRepo := repositories.NewCreditRepository(dbc)
	runStartLogRepo := repositories.NewRunStartLogRepository(dbc)
	varStateRepo := repositories.NewRunVarStateRepository(dbc)
	creditService := services.NewCreditService(transactor, creditRepo, runStartLogRepo, nil)
	objectiveRepo := repositories.NewObjectiveRepository(dbc)
	objectiveContextCompletionRepo := repositories.NewObjectiveContextCompletionRepository(dbc)

	runService := services.NewRunService(
		transactor, runRepo, creditService, blockStateRepo, varStateRepo,
		objectiveRepo, objectiveContextCompletionRepo,
	)
	return runService, dbc, cleanup
}
